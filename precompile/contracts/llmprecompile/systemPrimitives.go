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
		// Fetch `question` and `answer`
		questionRaw, err := getLookupValue(step.Args[0], stateDB)
		if err != nil {
			log.Info("Failed to fetch question", "error", err)
			return nil, 0, fmt.Errorf("failed to fetch question: %w", err)
		}
	
		answerRaw, err := getLookupValue(step.Args[1], stateDB)
		if err != nil {
			log.Info("Failed to fetch answer", "error", err)
			return nil, 0, fmt.Errorf("failed to fetch answer: %w", err)
		}
	
		// Check if Durango activation is required
		if contract.IsDurangoActivated(accessibleState) {
			log.Info("Durango is activated, proceeding with event emission")
	
			question := fmt.Sprintf("%v", questionRaw) // Convert to string
			answer := fmt.Sprintf("%v", answerRaw)     // Convert to string
			eventData := QuestionAnswerEventData{
				Question: question,
				Answer:   answer,
			}
			log.Info("Prepared QuestionAnswerEventData", "question", question, "answer", answer)
	
			QuestionAnswerEventGasCost := GetQuestionAnswerEventGasCost(eventData)
	
			if remainingGas, err = contract.DeductGas(remainingGas, QuestionAnswerEventGasCost); err != nil {
				log.Info("Failed to deduct gas for QuestionAnswerEvent", "error", err)
				return nil, 0, err
			}	
			topics, data, err := PackQuestionAnswerEvent(eventData)
			if err != nil {
				log.Info("Failed to pack QuestionAnswerEvent", "error", err)
				return nil, remainingGas, err
			}	
			stateDB.AddLog(
				ContractAddress,
				topics,
				data,
				accessibleState.GetBlockContext().Number().Uint64(),
			)
		} else {
			log.Info("Durango is not activated, skipping event emission")
		}
	
	
	case "assign":
		arg, err := getLookupValue(step.Args[0], stateDB)
		if err != nil {
			log.Info("Failed to fetch argument", "Error", err)
			return nil, 0, fmt.Errorf("failed to fetch arg: %w", err)
		}
		if err := updatePlanLocalState(stateDB, llmAddr, step.Output[0], arg); err != nil {
			log.Info("Failed to update memory in state", "Step", currentPC.Int64(), "Error", err)
			return currentPC, remainingGas, err
		}
		log.Info("Successfully updated memory in state for assign step", "Output", step.Output)
	
	case "JumpIfNot":
		jumpTarget := new(big.Int)
	
		jumpTargetStr, err := getLookupValue(step.Args[0], stateDB)
		if err != nil {
			log.Info("Failed to fetch jumpTarget", "Error", err)
			return nil, 0, fmt.Errorf("failed to fetch jumpTarget: %w", err)
		}
	
		jumpTarget, ok := new(big.Int).SetString(jumpTargetStr.(string), 10)
		if !ok {
			return nil, 0, fmt.Errorf("JumpIfNot: failed to convert string '%s' to big.Int", jumpTargetStr)
		}
	
		conditionStr, err := getLookupValue(step.Args[1], stateDB)
		if err != nil {
			log.Info("Failed to fetch condition", "Error", err)
			return nil, 0, fmt.Errorf("failed to fetch condition: %w", err)
		}
	
		var condition bool
		switch v := conditionStr.(type) {
		case string:
			condition = strings.ToLower(v) == "true"
		case bool:
			condition = v
		default:
			log.Info("Invalid type for conditionStr", "Type", fmt.Sprintf("%T", v))
			return nil, 0, fmt.Errorf("invalid type for conditionStr: expected string or bool, got %T", v)
		}
	
		log.Info("Parsed JumpIfNot condition", "RawCondition", conditionStr, "ParsedBool", condition)
	
		if !condition {
			return jumpTarget, remainingGas, nil
		}
	
	case "assignArray":
		rawArrayValues := make([]interface{}, len(step.Args))
		for i, arg := range step.Args {
			value, err := getLookupValue(arg, stateDB)
			if err != nil {
				log.Info("Failed to fetch array value", "index", i, "error", err)
				return currentPC, remainingGas, fmt.Errorf("failed to fetch arg[%d]: %w", i, err)
			}
			rawArrayValues[i] = value
		}
	
		log.Info("Collected raw array values", "Values", rawArrayValues)
	
		for i, v := range rawArrayValues {
			log.Info("Array element", "index", i, "value", v, "type", fmt.Sprintf("%T", v))
		}
	
		if err := updatePlanLocalState(stateDB, llmAddr, step.Output[0], rawArrayValues); err != nil {
			log.Info("Failed to update memory in state for assignArray step", "pc", currentPC.Int64(), "error", err)
			return currentPC, remainingGas, err
		}
		log.Info("Successfully updated memory in state for assignArray step", "OutputKey", step.Output[0])
	
	case "assignDict":
		if len(step.Args)%2 != 0 {
			log.Info("Invalid argument count for assignDict", "got", len(step.Args))
			return currentPC, remainingGas, fmt.Errorf("assignDict expects an even number of arguments (key-value pairs)")
		}
	
		dict := make(map[string]interface{})
	
		for i := 0; i < len(step.Args); i += 2 {
			keyArg := step.Args[i]
			valueArg := step.Args[i+1]
	
			keyRaw, err := getLookupValue(keyArg, stateDB)
			if err != nil {
				log.Info("Failed to fetch key", "index", i, "error", err)
				return currentPC, remainingGas, fmt.Errorf("failed to fetch key arg[%d]: %w", i, err)
			}
	
			valueRaw, err := getLookupValue(valueArg, stateDB)
			if err != nil {
				log.Info("Failed to fetch value", "index", i+1, "error", err)
				return currentPC, remainingGas, fmt.Errorf("failed to fetch value arg[%d]: %w", i+1, err)
			}
	
			keyStr, ok := keyRaw.(string)
			if !ok {
				keyStr = fmt.Sprintf("%v", keyRaw)
				log.Info("Key was not a string, auto-converted", "index", i, "convertedKey", keyStr)
			}
	
			dict[keyStr] = valueRaw
			log.Info("Added dict pair", "key", keyStr, "value", valueRaw, "valueType", fmt.Sprintf("%T", valueRaw))
		}
	
		if err := updatePlanLocalState(stateDB, llmAddr, step.Output[0], dict); err != nil {
			log.Info("Failed to update memory in state for assignDict step", "pc", currentPC.Int64(), "error", err)
			return currentPC, remainingGas, err
		}
	
		log.Info("Successfully updated memory in state for assignDict step", "OutputKey", step.Output[0])
	
	case "getDict":
		if len(step.Args) < 2 {
			log.Info("Invalid arguments for getDict", "argsCount", len(step.Args), "outputsCount", len(step.Output))
			return currentPC, remainingGas, fmt.Errorf("getDict requires exactly 3 args and 1 output")
		}
	
		dictRaw, err := getLookupValue(step.Args[0], stateDB)
		if err != nil {
			log.Info("Failed to fetch dict for getDict", "error", err)
			return currentPC, remainingGas, fmt.Errorf("failed to fetch dict: %w", err)
		}
	
		dict, ok := dictRaw.(map[string]interface{})
		if !ok {
			log.Info("Dict is not a valid map", "actualType", fmt.Sprintf("%T", dictRaw))
			return currentPC, remainingGas, fmt.Errorf("expected dict to be map[string]interface{}, got %T", dictRaw)
		}
	
		keyRaw, err := getLookupValue(step.Args[1], stateDB)
		if err != nil {
			log.Info("Failed to fetch key for getDict", "error", err)
			return currentPC, remainingGas, fmt.Errorf("failed to fetch dict lookup key: %w", err)
		}
	
		keyStr := fmt.Sprintf("%v", keyRaw)
	
		val, exists := dict[keyStr]
		if !exists {
			defaultValue, err := getLookupValue(step.Args[2], stateDB)
			if err != nil {
				log.Info("Failed to fetch default value for getDict", "error", err)
				return currentPC, remainingGas, fmt.Errorf("failed to fetch default value: %w", err)
			}
			log.Info("Key not found, using default", "lookupKey", keyStr, "defaultValue", defaultValue)
			val = defaultValue
		} else {
			log.Info("Found key in dict", "lookupKey", keyStr, "value", val)
		}
	
		if err := updatePlanLocalState(stateDB, llmAddr, step.Output[0], val); err != nil {
			log.Info("Failed to store getDict result", "OutputKey", step.Output[0], "error", err)
			return currentPC, remainingGas, err
		}
	
		log.Info("Successfully stored getDict result", "OutputKey", step.Output[0], "Value", val)
	
	case "setDict":
		if len(step.Args) != 3 {
			log.Info("Invalid argument count for setDict", "expected", 3, "got", len(step.Args))
			return currentPC, remainingGas, fmt.Errorf("setDict requires exactly 3 arguments")
		}
	
		dictNameArg := step.Args[0]
		if dictNameArg.Lookup == nil {
			log.Info("Lookup field is nil in dictNameArg")
			return currentPC, remainingGas, fmt.Errorf("lookup field is nil")
		}
		dictKey := *dictNameArg.Lookup
	
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
				log.Info("Invalid dictionary type", "dictKey", dictKey, "actualType", fmt.Sprintf("%T", dictRaw))
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
				log.Info("Input is not a dictionary", "mode", modeStr, "type", fmt.Sprintf("%T", inputRaw))
				return currentPC, remainingGas, fmt.Errorf("expected a dictionary for mode '%s', got %T", modeStr, inputRaw)
			}
			for _, v := range dict {
				outputArray = append(outputArray, v)
			}
	
		case "list", "tuple":
			list, ok := inputRaw.([]interface{})
			if ok {
				outputArray = list
			} else {
				rv := reflect.ValueOf(inputRaw)
				if rv.Kind() == reflect.Slice || rv.Kind() == reflect.Array {
					for i := 0; i < rv.Len(); i++ {
						outputArray = append(outputArray, rv.Index(i).Interface())
					}
				} else {
					log.Info("Input is not a list/tuple", "mode", modeStr, "type", fmt.Sprintf("%T", inputRaw))
					return currentPC, remainingGas, fmt.Errorf("expected a list or array for mode '%s', got %T", modeStr, inputRaw)
				}
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
			log.Info("Failed to store forItems keys", "keyOutput", step.Output[0], "error", err)
			return currentPC, remainingGas, err
		}
	
		if err := updatePlanLocalState(stateDB, llmAddr, step.Output[1], values); err != nil {
			log.Info("Failed to store forItems values", "valueOutput", step.Output[1], "error", err)
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
	case "and", "or", "not", "is":
		log.Info("in case and or not is")

		return handleBoolOps(currentPC, step, llmAddr, stateDB, remainingGas)
	
	}    
    return currentPC.Add(currentPC, big.NewInt(1)), remainingGas, nil
}


func handleBoolOps(currentPC *big.Int, step Step, llmAddr common.Address, stateDB contract.StateDB, remainingGas uint64) (*big.Int, uint64, error) {
	log.Info("in handleBoolOps")
    switch step.Method {
    case "and":
		log.Info("in case and")

        return boolAnd(currentPC, step, llmAddr, stateDB, remainingGas)
    case "or":
        return boolOr(currentPC, step, llmAddr, stateDB, remainingGas)
    case "not":
        return boolNot(currentPC, step, llmAddr, stateDB, remainingGas)
    case "is":
        return boolIs(currentPC, step, llmAddr, stateDB, remainingGas)
    default:
        log.Info("Unknown boolean method", "method", step.Method)
        return currentPC, remainingGas, fmt.Errorf("unknown boolean method: %s", step.Method)
    }
}

func parseBoolFromString(rawValue interface{}) (bool, error) {
    strVal, ok := rawValue.(string)
    if !ok {
        return false, fmt.Errorf("expected string input, got %T", rawValue)
    }

    switch strVal {
    case "true":
        return true, nil
    case "false":
        return false, nil
    default:
        return false, fmt.Errorf("invalid boolean string value: %s", strVal)
    }
}


func boolAnd(currentPC *big.Int, step Step, llmAddr common.Address, stateDB contract.StateDB, remainingGas uint64) (*big.Int, uint64, error) {
	log.Info("in function boolAnd")
    if len(step.Args) < 2 || len(step.Output) != 1 {
        log.Info("Invalid argument/output count for and", "args", len(step.Args), "outputs", len(step.Output))
        return currentPC, remainingGas, fmt.Errorf("and requires at least two args and one output")
    }

    result := true
    for _, arg := range step.Args {
        rawValue, err := getLookupValue(arg, stateDB)
        if err != nil {
            log.Info("Failed to fetch input for and", "error", err)
            return currentPC, remainingGas, fmt.Errorf("failed to fetch input for and: %w", err)
        }

        boolVal, err := parseBoolFromString(rawValue)
        if err != nil {
            log.Info("Unsupported input value for and", "value", rawValue, "error", err)
            return currentPC, remainingGas, fmt.Errorf("and expects string 'true' or 'false', got %v", rawValue)
        }

        result = result && boolVal
    }

    log.Info("Computed and result", "result", result)

    if err := updatePlanLocalState(stateDB, llmAddr, step.Output[0], result); err != nil {
        log.Info("Failed to store and result", "error", err)
        return currentPC, remainingGas, err
    }

    log.Info("Successfully stored and result", "outputKey", step.Output[0], "result", result)

    return currentPC.Add(currentPC, big.NewInt(1)), remainingGas, nil
}


func boolIs(currentPC *big.Int, step Step, llmAddr common.Address, stateDB contract.StateDB, remainingGas uint64) (*big.Int, uint64, error) {
    if len(step.Args) != 2 || len(step.Output) != 1 {
        log.Info("Invalid argument/output count for is", "args", len(step.Args), "outputs", len(step.Output))
        return currentPC, remainingGas, fmt.Errorf("is requires exactly two args and one output")
    }

    rawValue1, err := getLookupValue(step.Args[0], stateDB)
    log.Info("rawValue1 is result", "result", rawValue1)
	if rawValue1 == "None" {
		rawValue1 = nil
	}
	
    if err != nil {
        log.Info("Failed to fetch first input for is", "error", err)
        return currentPC, remainingGas, fmt.Errorf("failed to fetch first input for is: %w", err)
    }

    rawValue2, err := getLookupValue(step.Args[1], stateDB)
    log.Info("rawValue2 is result", "result", rawValue2)
	if rawValue2 == "None" {
		rawValue2 = nil
	}

    if err != nil {
        log.Info("Failed to fetch second input for is", "error", err)
        return currentPC, remainingGas, fmt.Errorf("failed to fetch second input for is: %w", err)
    }

    result := rawValue1 == rawValue2

    log.Info("Computed is result", "result", result)

    if err := updatePlanLocalState(stateDB, llmAddr, step.Output[0], result); err != nil {
        log.Info("Failed to store is result", "error", err)
        return currentPC, remainingGas, err
    }

    log.Info("Successfully stored is result", "outputKey", step.Output[0], "result", result)

    return currentPC.Add(currentPC, big.NewInt(1)), remainingGas, nil
}

func boolNot(currentPC *big.Int, step Step, llmAddr common.Address, stateDB contract.StateDB, remainingGas uint64) (*big.Int, uint64, error) {
    if len(step.Args) != 1 || len(step.Output) != 1 {
        log.Info("Invalid argument/output count for not", "args", len(step.Args), "outputs", len(step.Output))
        return currentPC, remainingGas, fmt.Errorf("not requires exactly one arg and one output")
    }

    rawValue, err := getLookupValue(step.Args[0], stateDB)
    if err != nil {
        log.Info("Failed to fetch input for not", "error", err)
        return currentPC, remainingGas, fmt.Errorf("failed to fetch input for not: %w", err)
    }

    boolVal, err := parseBoolFromString(rawValue)
    if err != nil {
        log.Info("Unsupported input value for not", "value", rawValue, "error", err)
        return currentPC, remainingGas, fmt.Errorf("not expects string 'true' or 'false', got %v", rawValue)
    }

    result := !boolVal

    log.Info("Computed not result", "result", result)

    if err := updatePlanLocalState(stateDB, llmAddr, step.Output[0], result); err != nil {
        log.Info("Failed to store not result", "error", err)
        return currentPC, remainingGas, err
    }

    log.Info("Successfully stored not result", "outputKey", step.Output[0], "result", result)

    return currentPC.Add(currentPC, big.NewInt(1)), remainingGas, nil
}


func boolOr(currentPC *big.Int, step Step, llmAddr common.Address, stateDB contract.StateDB, remainingGas uint64) (*big.Int, uint64, error) {
    if len(step.Args) < 2 || len(step.Output) != 1 {
        log.Info("Invalid argument/output count for or", "args", len(step.Args), "outputs", len(step.Output))
        return currentPC, remainingGas, fmt.Errorf("or requires at least two args and one output")
    }

    result := false
    for _, arg := range step.Args {
        rawValue, err := getLookupValue(arg, stateDB)
        if err != nil {
            log.Info("Failed to fetch input for or", "error", err)
            return currentPC, remainingGas, fmt.Errorf("failed to fetch input for or: %w", err)
        }

        boolVal, err := parseBoolFromString(rawValue)
        if err != nil {
            log.Info("Unsupported input value for or", "value", rawValue, "error", err)
            return currentPC, remainingGas, fmt.Errorf("or expects string 'true' or 'false', got %v", rawValue)
        }

        result = result || boolVal
    }

    log.Info("Computed or result", "result", result)

    if err := updatePlanLocalState(stateDB, llmAddr, step.Output[0], result); err != nil {
        log.Info("Failed to store or result", "error", err)
        return currentPC, remainingGas, err
    }

    log.Info("Successfully stored or result", "outputKey", step.Output[0], "result", result)

    return currentPC.Add(currentPC, big.NewInt(1)), remainingGas, nil
}

