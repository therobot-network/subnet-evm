package llmprecompile

import (
	"fmt"
	"log"
	"math/big"
	"strings"

	"github.com/ava-labs/subnet-evm/precompile/contract"
	"github.com/ethereum/go-ethereum/common"
)

func systemPrimitiveStep(currentPC *big.Int, step Step, llmAddr common.Address, stateDB contract.StateDB, accessibleState contract.AccessibleState, remainingGas uint64) (*big.Int, uint64, error) {

	switch step.Method {
		case "answerUserQuestion":
		// Fetch `question` and `answer` only once
			questionRaw, err := getLookupValue(step.Args[0], stateDB)
			if err != nil {
				log.Printf("Error fetching question: %v", err)
				return nil, 0, fmt.Errorf("failed to fetch question: %w", err)
			}
			answerRaw, err := getLookupValue(step.Args[1], stateDB)
			if err != nil {
				log.Printf("Error fetching answer: %v", err)
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
				log.Printf("Error fetching arg: %v", err)
				return nil, 0, fmt.Errorf("failed to fetch arg: %w", err)
			}
			// if err := updateMemoryInState(stateDB, llmAddr, step.Output[0], arg, step.Args[0].AbiType); err != nil {
			if err := updatePlanLocalState(stateDB, llmAddr, step.Output[0], arg); err != nil {
				log.Printf("Error: Failed to update memory in state for step %d. Error: %v", currentPC.Int64(), err)
				return currentPC, remainingGas, err
			}
			log.Printf("Successfully updated memory in state for assign step under key: %s.",  step.Output)
		
		case "JumpIfNot":
			jumpTarget := new(big.Int)
		
			// Ensure correct ABI type for jumpTarget
			jumpTargetStr, err := getLookupValue(step.Args[0], stateDB)
			if err != nil {
				log.Printf("Error fetching jumpTarget: %v", err)
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
				log.Printf("Error fetching condition: %v", err)
				return nil, 0, fmt.Errorf("failed to fetch condition: %w", err)
			}

			var condition bool
			switch v := conditionStr.(type) {
			case string:
				condition = strings.ToLower(v) == "true"
			case bool:
				condition = v
			default:
				log.Printf("Unexpected type for conditionStr: %T", v)
				return nil, 0, fmt.Errorf("invalid type for conditionStr: expected string or bool, got %T", v)
			}

			log.Printf("JumpIfNot: Parsed Condition=%s as bool=%t", conditionStr, condition)
		
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
					log.Printf("Error fetching arg[%d]: %v", i, err)
					return currentPC, remainingGas, fmt.Errorf("failed to fetch arg[%d]: %w", i, err)
				}
				rawArrayValues[i] = value
			}

			log.Printf("Collected raw array values: %+v", rawArrayValues)

			// Log each element and its type
			for i, v := range rawArrayValues {
				log.Printf("rawArrayValues[%d]: Value=%v, Type=%T", i, v, v)
			}

			if err := updatePlanLocalState(stateDB, llmAddr, step.Output[0], rawArrayValues); err != nil {
				log.Printf("Error: Failed to update memory in state for step %d. Error: %v", currentPC.Int64(), err)
				return currentPC, remainingGas, err
			}
			log.Printf("Successfully updated memory in state for assignArray step under key: %s.",  step.Output)
	
		case "assignDict":
			if len(step.Args)%2 != 0 {
				log.Printf("Error: assignDict requires an even number of args, got %d", len(step.Args))
				return currentPC, remainingGas, fmt.Errorf("assignDict expects an even number of arguments (key-value pairs)")
			}
		
			dict := make(map[string]interface{})
		
			for i := 0; i < len(step.Args); i += 2 {
				keyArg := step.Args[i]
				valueArg := step.Args[i+1]
		
				keyRaw, err := getLookupValue(keyArg, stateDB)
				if err != nil {
					log.Printf("Error fetching key arg[%d]: %v", i, err)
					return currentPC, remainingGas, fmt.Errorf("failed to fetch key arg[%d]: %w", i, err)
				}
		
				valueRaw, err := getLookupValue(valueArg, stateDB)
				if err != nil {
					log.Printf("Error fetching value arg[%d]: %v", i+1, err)
					return currentPC, remainingGas, fmt.Errorf("failed to fetch value arg[%d]: %w", i+1, err)
				}
		
				// Convert key to string
				keyStr, ok := keyRaw.(string)
				if !ok {
					keyStr = fmt.Sprintf("%v", keyRaw)
					log.Printf("Warning: key arg[%d] was not a string, converted to: %s", i, keyStr)
				}
		
				dict[keyStr] = valueRaw
				log.Printf("assignDict pair: key=%s, value=%v (type=%T)", keyStr, valueRaw, valueRaw)
			}
		
			if err := updatePlanLocalState(stateDB, llmAddr, step.Output[0], dict); err != nil {
				log.Printf("Error: Failed to update memory in state for step %d. Error: %v", currentPC.Int64(), err)
				return currentPC, remainingGas, err
			}
		
			log.Printf("Successfully updated memory in state for assignDict step under key: %s", step.Output[0])
		
		case "getDict":
			if len(step.Args) < 2 {
				log.Printf("Error: getDict expects 3 arguments and 1 output, got %d args and %d outputs", len(step.Args), len(step.Output))
				return currentPC, remainingGas, fmt.Errorf("getDict requires exactly 3 args and 1 output")
			}
		
			// Arg 0: name of dict (lookup key)
			dictRaw, err := getLookupValue(step.Args[0], stateDB)
			if err != nil {
				log.Printf("Error: failed to fetch dict for getDict: %v", err)
				return currentPC, remainingGas, fmt.Errorf("failed to fetch dict: %w", err)
			}
		
			// Ensure it's a map
			dict, ok := dictRaw.(map[string]interface{})
			if !ok {
				log.Printf("Error: value at dict key is not a map[string]interface{}: %T", dictRaw)
				return currentPC, remainingGas, fmt.Errorf("expected dict to be map[string]interface{}, got %T", dictRaw)
			}
		
			// Arg 1: key to look up
			keyRaw, err := getLookupValue(step.Args[1], stateDB)
			if err != nil {
				log.Printf("Error: failed to fetch key for getDict: %v", err)
				return currentPC, remainingGas, fmt.Errorf("failed to fetch dict lookup key: %w", err)
			}
		
			// Coerce key to string
			keyStr := fmt.Sprintf("%v", keyRaw)
		

		
			// Get result or default
			val, exists := dict[keyStr]
			if !exists {
				defaultValue, err := getLookupValue(step.Args[2], stateDB)
				if err != nil {
					log.Printf("Error: failed to fetch default value for getDict: %v", err)
					return currentPC, remainingGas, fmt.Errorf("failed to fetch default value: %w", err)
				}
				log.Printf("Key not found in dict: %s — using default value: %v", keyStr, defaultValue)
				val = defaultValue
			} else {
				log.Printf("Found key in dict: %s => %v", keyStr, val)
			}
		
			// Store result
			if err := updatePlanLocalState(stateDB, llmAddr, step.Output[0], val); err != nil {
				log.Printf("Error: Failed to store getDict result. OutputKey=%s. Error: %v", step.Output[0], err)
				return currentPC, remainingGas, err
			}
		
			log.Printf("Successfully stored getDict result under key: %s | Value: %v", step.Output[0], val)		
		
		case "setDict":
			if len(step.Args) != 3 {
				log.Printf("Error: setDict expects exactly 3 arguments, got %d", len(step.Args))
				return currentPC, remainingGas, fmt.Errorf("setDict requires exactly 3 arguments")
			}
		
			// Arg 0: dict name (lookup key)
			dictNameArg := step.Args[0]
			dictKey := fmt.Sprintf("%v", dictNameArg.Lookup) // We assume this is always a lookup key
		
			// Load existing dictionary
			dictRaw, err := getLookupValue(dictNameArg, stateDB)
			if err != nil {
				log.Printf("Error: failed to load dictionary %s: %v", dictKey, err)
				return currentPC, remainingGas, fmt.Errorf("failed to load dictionary: %w", err)
			}
		
			var dict map[string]interface{}
			if dictRaw == nil {
				dict = make(map[string]interface{})
				log.Printf("Initialized new dict under key: %s", dictKey)
			} else {
				var ok bool
				dict, ok = dictRaw.(map[string]interface{})
				if !ok {
					return currentPC, remainingGas, fmt.Errorf("expected map[string]interface{}, got %T", dictRaw)
				}
			}
		
			// Arg 1: key
			keyRaw, err := getLookupValue(step.Args[1], stateDB)
			if err != nil {
				log.Printf("Error fetching dict key arg: %v", err)
				return currentPC, remainingGas, fmt.Errorf("failed to fetch dict key: %w", err)
			}
			keyStr := fmt.Sprintf("%v", keyRaw)
		
			// Arg 2: value
			value, err := getLookupValue(step.Args[2], stateDB)
			if err != nil {
				log.Printf("Error fetching dict value arg: %v", err)
				return currentPC, remainingGas, fmt.Errorf("failed to fetch dict value: %w", err)
			}
		
			// Set the key/value
			dict[keyStr] = value
			log.Printf("Updated dict[%s] = %v", keyStr, value)
		
			// Store the updated dict back
			if err := updatePlanLocalState(stateDB, llmAddr, dictKey, dict); err != nil {
				log.Printf("Error: Failed to update state with updated dict %s: %v", dictKey, err)
				return currentPC, remainingGas, err
			}
		
			log.Printf("Successfully updated dict under key: %s", dictKey)

		case "toArray":
			if len(step.Args) != 2 || len(step.Output) != 1 {
				log.Printf("Error: toArray expects exactly 2 arguments and 1 output, got %d args and %d outputs", len(step.Args), len(step.Output))
				return currentPC, remainingGas, fmt.Errorf("toArray requires 2 args and 1 output")
			}
		
			// Arg 0: dict (lookup)
			dictRaw, err := getLookupValue(step.Args[0], stateDB)
			if err != nil {
				log.Printf("Error fetching dict from lookup: %v", err)
				return currentPC, remainingGas, fmt.Errorf("failed to fetch dictionary: %w", err)
			}
		
			dict, ok := dictRaw.(map[string]interface{})
			if !ok {
				return currentPC, remainingGas, fmt.Errorf("expected a dictionary (map[string]interface{}), got %T", dictRaw)
			}
		
			// Arg 1: mode (value: "keys", "values", or "dict")
			modeRaw, err := getLookupValue(step.Args[1], stateDB)
			if err != nil {
				log.Printf("Error fetching mode: %v", err)
				return currentPC, remainingGas, fmt.Errorf("failed to fetch mode: %w", err)
			}
		
			modeStr := strings.ToLower(fmt.Sprintf("%v", modeRaw))
			var outputArray []interface{}
		
			switch modeStr {
			case "keys", "dict":
				for k := range dict {
					outputArray = append(outputArray, k)
				}
			case "values":
				for _, v := range dict {
					outputArray = append(outputArray, v)
				}
			default:
				return currentPC, remainingGas, fmt.Errorf("invalid toArray mode: %s (expected 'keys', 'values', or 'dict')", modeStr)
			}
		
			log.Printf("toArray (%s) result: %v", modeStr, outputArray)
		
			if err := updatePlanLocalState(stateDB, llmAddr, step.Output[0], outputArray); err != nil {
				log.Printf("Error storing toArray result: %v", err)
				return currentPC, remainingGas, err
			}
		
			log.Printf("Successfully stored toArray result to key: %s", step.Output[0])
			
		case "forItems":
			if len(step.Args) != 1 || len(step.Output) != 2 {
				log.Printf("Error: forItems expects 1 argument and 2 outputs, got %d args and %d outputs", len(step.Args), len(step.Output))
				return currentPC, remainingGas, fmt.Errorf("forItems requires 1 arg (dict) and 2 outputs (keys, values)")
			}
		
			// Fetch the dictionary from lookup
			dictRaw, err := getLookupValue(step.Args[0], stateDB)
			if err != nil {
				log.Printf("Error fetching dictionary for forItems: %v", err)
				return currentPC, remainingGas, fmt.Errorf("failed to fetch dictionary: %w", err)
			}
		
			dict, ok := dictRaw.(map[string]interface{})
			if !ok {
				return currentPC, remainingGas, fmt.Errorf("expected a map[string]interface{}, got %T", dictRaw)
			}
		
			var keys []interface{}
			var values []interface{}
		
			for k, v := range dict {
				keys = append(keys, k)
				values = append(values, v)
			}
		
			log.Printf("forItems: keys=%v, values=%v", keys, values)
		
			if err := updatePlanLocalState(stateDB, llmAddr, step.Output[0], keys); err != nil {
				log.Printf("Error storing keys output in forItems: %v", err)
				return currentPC, remainingGas, err
			}
		
			if err := updatePlanLocalState(stateDB, llmAddr, step.Output[1], values); err != nil {
				log.Printf("Error storing values output in forItems: %v", err)
				return currentPC, remainingGas, err
			}
		
			log.Printf("Successfully stored forItems results under keys: %s, %s", step.Output[0], step.Output[1])
		
	
	}    
    return currentPC.Add(currentPC, big.NewInt(1)), remainingGas, nil
}


