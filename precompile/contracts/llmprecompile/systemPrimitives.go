package llmprecompile

import (
	"encoding/json"
	"fmt"

	// "log"
	"math/big"
	"reflect"
	"strconv"
	"strings"

	"github.com/ethereum/go-ethereum/log"

	"github.com/ava-labs/subnet-evm/precompile/contract"
	"github.com/ethereum/go-ethereum/common"
)

func systemPrimitiveStep(currentPC *big.Int, step Step, llmAddr common.Address, stateDB contract.StateDB, accessibleState contract.AccessibleState, remainingGas uint64) (*big.Int, uint64, error) {

	switch step.Method {
	case "answerUserQuestion":
	// Fetch `question` and `answer` only once
		questionRaw, err := getLookupValue(step.Args[0], stateDB)
		if err != nil {
			log.Info("Error fetching question: %v", err)
			return nil, 0, fmt.Errorf("failed to fetch question: %w", err)
		}
		answerRaw, err := getLookupValue(step.Args[1], stateDB)
		if err != nil {
			log.Info("Error fetching answer: %v", err)
			return nil, 0, fmt.Errorf("failed to fetch answer: %w", err)
		}

		// Ensure `question` and `answer` are strings only if contract activation requires them
		if contract.IsDurangoActivated(accessibleState) {
			question := fmt.Sprintf("%v", questionRaw) // Convert to string
			answer := fmt.Sprintf("%v", answerRaw)     // Convert to string
				eventData := QuestionAnswerEventData{
				Question: question,
				Answer:   answer,
			}
			QuestionAnswerEventGasCost := GetQuestionAnswerEventGasCost(eventData)
			if remainingGas, err = contract.DeductGas(remainingGas, QuestionAnswerEventGasCost); err != nil {
				return nil, 0, err
			}

			topics, data, err := PackQuestionAnswerEvent(eventData)
			if err != nil {
				return nil, remainingGas, err
			}
			stateDB.AddLog(
				ContractAddress,
				topics,
				data,
				accessibleState.GetBlockContext().Number().Uint64(),
			)
		}
	
	case "assign":
		arg, err := getLookupValue(step.Args[0], stateDB)
		if err != nil {
			log.Info("Error fetching arg: %v", err)
			return nil, 0, fmt.Errorf("failed to fetch arg: %w", err)
		}
		// if err := updateMemoryInState(stateDB, llmAddr, step.Output[0], arg, step.Args[0].AbiType); err != nil {
		if err := updatePlanLocalState(stateDB, llmAddr, step.Output[0], arg); err != nil {
			log.Info("Error: Failed to update memory in state for step %d. Error: %v", currentPC.Int64(), err)
			return currentPC, remainingGas, err
		}
		log.Info("Successfully updated memory in state for assign step under key: %s.",  step.Output)
	
	case "JumpIfNot":
		jumpTarget := new(big.Int)
	
		// Ensure correct ABI type for jumpTarget
		jumpTargetStr, err := getLookupValue(step.Args[0], stateDB)
		if err != nil {
			log.Info("Error fetching jumpTarget: %v", err)
			return nil, 0, fmt.Errorf("failed to fetch jumpTarget: %w", err)
		}
	
		// Convert `jumpTargetIF` to *big.Int
		// jumpTarget, ok := jumpTargetIF.(*big.Int)
		jumpTarget, ok := new(big.Int).SetString(jumpTargetStr.(string), 10)
		if !ok {
			return nil, 0, fmt.Errorf("JumpIfNot: failed to convert string '%s' to big.Int", jumpTargetStr)
		}
	
		// Ensure correct ABI type for condition
		conditionStr, err := getLookupValue(step.Args[1], stateDB)
		if err != nil {
			log.Info("Error fetching condition: %v", err)
			return nil, 0, fmt.Errorf("failed to fetch condition: %w", err)
		}

		var condition bool
		switch v := conditionStr.(type) {
		case string:
			condition = strings.ToLower(v) == "true"
		case bool:
			condition = v
		default:
			log.Info("Unexpected type for conditionStr: %T", v)
			return nil, 0, fmt.Errorf("invalid type for conditionStr: expected string or bool, got %T", v)
		}

		log.Info("JumpIfNot: Parsed Condition=%s as bool=%t", conditionStr, condition)
	
		// Perform jump if condition is false
		if !condition {
			return jumpTarget, remainingGas, nil
		}
	
	case "assignArray":			
		// Collect all arguments into an array
		rawArrayValues := make([]interface{}, len(step.Args))
		for i, arg := range step.Args {
			value, err := getLookupValue(arg, stateDB)
			if err != nil {
				log.Info("Error fetching arg[%d]: %v", i, err)
				return currentPC, remainingGas, fmt.Errorf("failed to fetch arg[%d]: %w", i, err)
			}
			rawArrayValues[i] = value
		}

		log.Info("Collected raw array values: %+v", rawArrayValues)

		// Log each element and its type
		for i, v := range rawArrayValues {
			log.Info("rawArrayValues[%d]: Value=%v, Type=%T", i, v, v)
		}

		if err := updatePlanLocalState(stateDB, llmAddr, step.Output[0], rawArrayValues); err != nil {
			log.Info("Error: Failed to update memory in state for step %d. Error: %v", currentPC.Int64(), err)
			return currentPC, remainingGas, err
		}
		log.Info("Successfully updated memory in state for assignArray step under key: %s.",  step.Output)

	case "assignDict":
		if len(step.Args)%2 != 0 {
			log.Info("Error: assignDict requires an even number of args, got %d", len(step.Args))
			return currentPC, remainingGas, fmt.Errorf("assignDict expects an even number of arguments (key-value pairs)")
		}
	
		dict := make(map[string]interface{})
	
		for i := 0; i < len(step.Args); i += 2 {
			keyArg := step.Args[i]
			valueArg := step.Args[i+1]
	
			keyRaw, err := getLookupValue(keyArg, stateDB)
			if err != nil {
				log.Info("Error fetching key arg[%d]: %v", i, err)
				return currentPC, remainingGas, fmt.Errorf("failed to fetch key arg[%d]: %w", i, err)
			}
	
			valueRaw, err := getLookupValue(valueArg, stateDB)
			if err != nil {
				log.Info("Error fetching value arg[%d]: %v", i+1, err)
				return currentPC, remainingGas, fmt.Errorf("failed to fetch value arg[%d]: %w", i+1, err)
			}
	
			// Convert key to string
			keyStr, ok := keyRaw.(string)
			if !ok {
				keyStr = fmt.Sprintf("%v", keyRaw)
				log.Info("Warning: key arg[%d] was not a string, converted to: %s", i, keyStr)
			}
	
			dict[keyStr] = valueRaw
			log.Info("assignDict pair: key=%s, value=%v (type=%T)", keyStr, valueRaw, valueRaw)
		}
	
		if err := updatePlanLocalState(stateDB, llmAddr, step.Output[0], dict); err != nil {
			log.Info("Error: Failed to update memory in state for step %d. Error: %v", currentPC.Int64(), err)
			return currentPC, remainingGas, err
		}
	
		log.Info("Successfully updated memory in state for assignDict step under key: %s", step.Output[0])
	
	case "getDict":
		if len(step.Args) < 2 {
			log.Info("Error: getDict expects 3 arguments and 1 output, got %d args and %d outputs", len(step.Args), len(step.Output))
			return currentPC, remainingGas, fmt.Errorf("getDict requires exactly 3 args and 1 output")
		}
	
		// Arg 0: name of dict (lookup key)
		dictRaw, err := getLookupValue(step.Args[0], stateDB)
		if err != nil {
			log.Info("Error: failed to fetch dict for getDict: %v", err)
			return currentPC, remainingGas, fmt.Errorf("failed to fetch dict: %w", err)
		}
	
		// Ensure it's a map
		dict, ok := dictRaw.(map[string]interface{})
		if !ok {
			log.Info("Error: value at dict key is not a map[string]interface{}: %T", dictRaw)
			return currentPC, remainingGas, fmt.Errorf("expected dict to be map[string]interface{}, got %T", dictRaw)
		}
	
		// Arg 1: key to look up
		keyRaw, err := getLookupValue(step.Args[1], stateDB)
		if err != nil {
			log.Info("Error: failed to fetch key for getDict: %v", err)
			return currentPC, remainingGas, fmt.Errorf("failed to fetch dict lookup key: %w", err)
		}
	
		// Coerce key to string
		keyStr := fmt.Sprintf("%v", keyRaw)
	

	
		// Get result or default
		val, exists := dict[keyStr]
		if !exists {
			defaultValue, err := getLookupValue(step.Args[2], stateDB)
			if err != nil {
				log.Info("Error: failed to fetch default value for getDict: %v", err)
				return currentPC, remainingGas, fmt.Errorf("failed to fetch default value: %w", err)
			}
			log.Info("Key not found in dict: %s — using default value: %v", keyStr, defaultValue)
			val = defaultValue
		} else {
			log.Info("Found key in dict: %s => %v", keyStr, val)
		}
	
		// Store result
		if err := updatePlanLocalState(stateDB, llmAddr, step.Output[0], val); err != nil {
			log.Info("Error: Failed to store getDict result. OutputKey=%s. Error: %v", step.Output[0], err)
			return currentPC, remainingGas, err
		}
	
		log.Info("Successfully stored getDict result under key: %s | Value: %v", step.Output[0], val)		
	
		case "setDict":
			if len(step.Args) != 3 {
				log.Info("Invalid argument count for setDict", "expected", 3, "got", len(step.Args))
				return currentPC, remainingGas, fmt.Errorf("setDict requires exactly 3 arguments")
			}
		
			dictNameArg := step.Args[0]
			dictKey := fmt.Sprintf("%v", dictNameArg.Lookup)
		
			dictRaw, err := getLookupValue(dictNameArg, stateDB)
			if err != nil {
				log.Info("Failed to load dictionary", "dictKey", dictKey, "error", err)
				return currentPC, remainingGas, fmt.Errorf("failed to load dictionary: %w", err)
			}
		
			var dict map[string]interface{}
			if dictRaw == nil {
				dict = make(map[string]interface{})
				log.Info("Initialized new dictionary", "dictKey", dictKey)
			} else {
				var ok bool
				dict, ok = dictRaw.(map[string]interface{})
				if !ok {
					log.Info("Invalid dictionary type", "dictKey", dictKey, "type", fmt.Sprintf("%T", dictRaw))
					return currentPC, remainingGas, fmt.Errorf("expected map[string]interface{}, got %T", dictRaw)
				}
			}
		
			keyRaw, err := getLookupValue(step.Args[1], stateDB)
			if err != nil {
				log.Info("Failed to fetch dictionary key", "error", err)
				return currentPC, remainingGas, fmt.Errorf("failed to fetch dict key: %w", err)
			}
			keyStr := fmt.Sprintf("%v", keyRaw)
		
			value, err := getLookupValue(step.Args[2], stateDB)
			if err != nil {
				log.Info("Failed to fetch dictionary value", "error", err)
				return currentPC, remainingGas, fmt.Errorf("failed to fetch dict value: %w", err)
			}
		
			dict[keyStr] = value
			log.Info("Updated dictionary entry", "dictKey", dictKey, "key", keyStr, "value", value)
		
			if err := updatePlanLocalState(stateDB, llmAddr, dictKey, dict); err != nil {
				log.Info("Failed to store updated dictionary", "dictKey", dictKey, "error", err)
				return currentPC, remainingGas, err
			}
		
			log.Info("Successfully stored updated dictionary", "dictKey", dictKey)
		
		case "toArray":
			if len(step.Args) != 2 || len(step.Output) != 1 {
				log.Info("Invalid argument/output count for toArray", "args", len(step.Args), "outputs", len(step.Output))
				return currentPC, remainingGas, fmt.Errorf("toArray requires 2 args and 1 output")
			}
		
			inputRaw, err := getLookupValue(step.Args[0], stateDB)
			if err != nil {
				log.Info("Failed to fetch input for toArray", "error", err)
				return currentPC, remainingGas, fmt.Errorf("failed to fetch input: %w", err)
			}
		
			modeRaw, err := getLookupValue(step.Args[1], stateDB)
			if err != nil {
				log.Info("Failed to fetch mode for toArray", "error", err)
				return currentPC, remainingGas, fmt.Errorf("failed to fetch mode: %w", err)
			}
		
			modeStr := strings.ToLower(fmt.Sprintf("%v", modeRaw))
			var outputArray []interface{}
		
			switch modeStr {
			case "keys", "dict":
				dict, ok := inputRaw.(map[string]interface{})
				if !ok {
					log.Info("Input is not a dictionary", "mode", modeStr, "type", fmt.Sprintf("%T", inputRaw))
					return currentPC, remainingGas, fmt.Errorf("expected a dictionary for mode '%s', got %T", modeStr, inputRaw)
				}
				for k := range dict {
					outputArray = append(outputArray, k)
				}
			case "values":
				dict, ok := inputRaw.(map[string]interface{})
				if !ok {
					log.Info("Input is not a dictionary", "mode", "values", "type", fmt.Sprintf("%T", inputRaw))
					return currentPC, remainingGas, fmt.Errorf("expected a dictionary for mode 'values', got %T", inputRaw)
				}
				for _, v := range dict {
					outputArray = append(outputArray, v)
				}
			case "list", "tuple":
				list, ok := inputRaw.([]interface{})
				if !ok {
					rv := reflect.ValueOf(inputRaw)
					if rv.Kind() == reflect.Slice || rv.Kind() == reflect.Array {
						for i := 0; i < rv.Len(); i++ {
							outputArray = append(outputArray, rv.Index(i).Interface())
						}
					} else {
						log.Info("Input is not a list/tuple", "mode", modeStr, "type", fmt.Sprintf("%T", inputRaw))
						return currentPC, remainingGas, fmt.Errorf("expected a list or array for mode '%s', got %T", modeStr, inputRaw)
					}
				} else {
					outputArray = list
				}
			default:
				log.Info("Invalid toArray mode", "mode", modeStr)
				return currentPC, remainingGas, fmt.Errorf("invalid toArray mode: %s", modeStr)
			}
		
			log.Info("Converted to array", "mode", modeStr, "result", outputArray)
		
			if err := updatePlanLocalState(stateDB, llmAddr, step.Output[0], outputArray); err != nil {
				log.Info("Failed to store toArray result", "key", step.Output[0], "error", err)
				return currentPC, remainingGas, err
			}
		
			log.Info("Successfully stored toArray result", "key", step.Output[0])
	
		case "forItems":
			if len(step.Args) != 1 || len(step.Output) != 2 {
				log.Info("Invalid argument/output count for forItems", "args", len(step.Args), "outputs", len(step.Output))
				return currentPC, remainingGas, fmt.Errorf("forItems requires 1 arg (dict) and 2 outputs (keys, values)")
			}
		
			dictRaw, err := getLookupValue(step.Args[0], stateDB)
			if err != nil {
				log.Info("Failed to fetch dictionary for forItems", "error", err)
				return currentPC, remainingGas, fmt.Errorf("failed to fetch dictionary: %w", err)
			}
			
			dict, ok := dictRaw.(map[string]interface{})
			if !ok {
				log.Info("Invalid dictionary type for forItems", "type", fmt.Sprintf("%T", dictRaw))
				return currentPC, remainingGas, fmt.Errorf("expected a map[string]interface{}, got %T", dictRaw)
			}
			
			var keys []interface{}
			var values []interface{}
			
			for k, v := range dict {
				keys = append(keys, k)
				values = append(values, v)
			}
			
			log.Info("forItems extracted keys and values", "keys", keys, "values", values)
			
			if err := updatePlanLocalState(stateDB, llmAddr, step.Output[0], keys); err != nil {
				log.Info("Failed to store forItems keys", "error", err)
				return currentPC, remainingGas, err
			}
			
			if err := updatePlanLocalState(stateDB, llmAddr, step.Output[1], values); err != nil {
				log.Info("Failed to store forItems values", "error", err)
				return currentPC, remainingGas, err
			}
			
			log.Info("Successfully stored forItems result", "keysOutput", step.Output[0], "valuesOutput", step.Output[1])
			
		case "len":
			if len(step.Args) != 1 || len(step.Output) != 1 {
				log.Info("Invalid argument/output count for len", "args", len(step.Args), "outputs", len(step.Output))
				return currentPC, remainingGas, fmt.Errorf("len requires 1 arg and 1 output")
			}
	
			rawValue, err := getLookupValue(step.Args[0], stateDB)
			if err != nil {
				log.Info("Failed to fetch input for len", "error", err)
				return currentPC, remainingGas, fmt.Errorf("failed to fetch input for len: %w", err)
			}
			
			var length int
			switch v := rawValue.(type) {
			case []interface{}:
				length = len(v)
			case map[string]interface{}:
				length = len(v)
			default:
				val := reflect.ValueOf(rawValue)
				if val.Kind() == reflect.Slice || val.Kind() == reflect.Array || val.Kind() == reflect.Map {
					length = val.Len()
				} else {
					log.Info("Unsupported input type for len", "type", fmt.Sprintf("%T", rawValue))
					return currentPC, remainingGas, fmt.Errorf("len only supports arrays and maps, got %T", rawValue)
				}
			}
			
			log.Info("Computed length", "length", length, "inputType", fmt.Sprintf("%T", rawValue))
			
			if err := updatePlanLocalState(stateDB, llmAddr, step.Output[0], length); err != nil {
				log.Info("Failed to store len result", "error", err)
				return currentPC, remainingGas, err
			}
			
			log.Info("Successfully stored len result", "outputKey", step.Output[0], "length", length)
		
		case "index":
			if len(step.Args) != 2 || len(step.Output) != 1 {
				log.Info("Invalid argument/output count for index", "args", len(step.Args), "outputs", len(step.Output))
				return currentPC, remainingGas, fmt.Errorf("index requires 2 args and 1 output")
			}

			arrayRaw, err := getLookupValue(step.Args[0], stateDB)
			if err != nil {
				log.Info("Failed to fetch array for index", "error", err)
				return currentPC, remainingGas, fmt.Errorf("failed to fetch array: %w", err)
			}

			indexRaw, err := getLookupValue(step.Args[1], stateDB)
			if err != nil {
				log.Info("Failed to fetch index for index", "error", err)
				return currentPC, remainingGas, fmt.Errorf("failed to fetch index: %w", err)
			}

			var array []interface{}
			if casted, ok := arrayRaw.([]interface{}); ok {
				array = casted
			} else {
				val := reflect.ValueOf(arrayRaw)
				if val.Kind() == reflect.Slice || val.Kind() == reflect.Array {
					array = make([]interface{}, val.Len())
					for i := 0; i < val.Len(); i++ {
						array[i] = val.Index(i).Interface()
					}
				} else {
					log.Info("Unsupported array type for index", "type", fmt.Sprintf("%T", arrayRaw))
					return currentPC, remainingGas, fmt.Errorf("expected array/slice, got %T", arrayRaw)
				}
			}

			var index int
			switch v := indexRaw.(type) {
			case float64:
				index = int(v)
			case string:
				parsed, err := strconv.Atoi(v)
				if err != nil {
					log.Info("Invalid string index", "value", v, "error", err)
					return currentPC, remainingGas, fmt.Errorf("invalid string index: %s", v)
				}
				index = parsed
			case int:
				index = v
			case int64:
				index = int(v)
			case json.Number:
				parsed, err := v.Int64()
				if err != nil {
					log.Info("Invalid json.Number index", "value", v, "error", err)
					return currentPC, remainingGas, fmt.Errorf("invalid json.Number index: %v", err)
				}
				index = int(parsed)
			default:
				log.Info("Unsupported index type", "type", fmt.Sprintf("%T", indexRaw))
				return currentPC, remainingGas, fmt.Errorf("unsupported index type: %T", indexRaw)
			}

			if index < 0 || index >= len(array) {
				log.Info("Index out of range", "index", index, "arrayLen", len(array))
				return currentPC, remainingGas, fmt.Errorf("index out of bounds: %d", index)
			}

			val := array[index]
			log.Info("Fetched array element by index", "index", index, "value", val)

			if err := updatePlanLocalState(stateDB, llmAddr, step.Output[0], val); err != nil {
				log.Info("Failed to store index result", "error", err)
				return currentPC, remainingGas, err
			}

			log.Info("Successfully stored index result", "outputKey", step.Output[0], "value", val)

	
	}    
    return currentPC.Add(currentPC, big.NewInt(1)), remainingGas, nil
}


