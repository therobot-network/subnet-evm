package llmprecompile

import (
	"fmt"
	"log"
	"math/big"
	"strings"

	"github.com/ava-labs/subnet-evm/accounts/abi"
	"github.com/ava-labs/subnet-evm/precompile/contract"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

func systemPrimitiveStep(currentPC *big.Int, step Step, llmAddr common.Address, stateDB contract.StateDB, accessibleState contract.AccessibleState, remainingGas uint64) (*big.Int, uint64, error) {

	switch step.Method {
		case "answerUserQuestion":
		// Fetch `question` and `answer` only once
			if step.Args[0].AbiType == "" {
				step.Args[0].AbiType = "string"
			}
			questionRaw, err := getLookupValue(step.Args[0], stateDB)
			if err != nil {
				log.Printf("Error fetching question: %v", err)
				return nil, 0, fmt.Errorf("failed to fetch question: %w", err)
			}
			if step.Args[1].AbiType == "" {
				step.Args[1].AbiType = "string"
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
			if err := updateMemoryInState(stateDB, llmAddr, step.Output[0], arg, step.Args[0].AbiType); err != nil {
				log.Printf("Error: Failed to update memory in state for step %d. Error: %v", currentPC.Int64(), err)
				return currentPC, remainingGas, err
			}
			log.Printf("Successfully updated memory in state for assign step under key: %s.",  step.Output)
		
		case "JumpIfNot":
			jumpTarget := new(big.Int)
		
			// Ensure correct ABI type for jumpTarget
			step.Args[0].AbiType = "string"
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

			if step.Args[1].AbiType == "" {
				step.Args[1].AbiType = "string"
			}
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
			outputKey := step.Output[0]
			keyHash := crypto.Keccak256Hash([]byte(outputKey)) // Key for this entry
    		outputKeyHash := crypto.Keccak256Hash(append(lookupStorageKey.Bytes(), keyHash.Bytes()...))

			// Clear existing value if present
			if stateDB.GetState(llmAddr, outputKeyHash) != (common.Hash{}) {
				log.Printf("Clearing existing value for key: %s", outputKey)
				stateDB.SetState(llmAddr, outputKeyHash, common.Hash{})
			}
		
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
		
			// If empty, store an empty array of the expected type
			abiElementType := step.Args[0].AbiType // Base type, e.g., "uint256"
		
			var abiArray []byte
			var err error

			abiArray, err = convertToABISlice(rawArrayValues, abiElementType)
			if err != nil {
				log.Printf("Error: Failed to convert array for ABI encoding: %v", err)
				return currentPC, remainingGas, fmt.Errorf("failed to convert array for ABI encoding: %w", err)
			}
			
    		setLargeState(stateDB, llmAddr, outputKeyHash, abiArray)
		
			log.Printf("Successfully stored array in memory for assignArray step under key: %s.", outputKey)
		

		// todo:
		// assignDict
		// getDict
		// readDict
	}    
    return currentPC.Add(currentPC, big.NewInt(1)), remainingGas, nil
}

func convertToABISlice(data interface{}, elementType string) ([]byte, error) {
    values, ok := data.([]interface{})
    if !ok {
        return nil, fmt.Errorf("expected []interface{}, got %T", data)
    }

    var convertedSlice interface{}

    switch elementType {
    case "uint256":
        result := make([]*big.Int, len(values))
        for i, v := range values {
            switch val := v.(type) {
            case *big.Int:
                result[i] = val
            case int64:
                result[i] = big.NewInt(val)
            case uint64:
                result[i] = new(big.Int).SetUint64(val)
            case string:
                bigIntVal, success := new(big.Int).SetString(val, 10)
                if !success {
                    return nil, fmt.Errorf("failed to parse string %s to *big.Int", val)
                }
                result[i] = bigIntVal
            default:
                return nil, fmt.Errorf("expected *big.Int, int64, or uint64 for uint256[], got %T", v)
            }
        }
        convertedSlice = result

    case "address":
        result := make([]common.Address, len(values))
        for i, v := range values {
            switch val := v.(type) {
            case common.Address:
                result[i] = val
            case string:
                if common.IsHexAddress(val) {
                    result[i] = common.HexToAddress(val)
                } else {
                    return nil, fmt.Errorf("invalid Ethereum address: %s", val)
                }
            default:
                return nil, fmt.Errorf("expected common.Address or valid hex string for address[], got %T", v)
            }
        }
        convertedSlice = result

    case "bool":
        result := make([]bool, len(values))
        for i, v := range values {
            if val, ok := v.(bool); ok {
                result[i] = val
            } else {
                return nil, fmt.Errorf("expected bool for bool[], got %T", v)
            }
        }
        convertedSlice = result

    case "string":
        result := make([]string, len(values))
        for i, v := range values {
            if val, ok := v.(string); ok {
                result[i] = val
            } else {
                return nil, fmt.Errorf("expected string for string[], got %T", v)
            }
        }
        convertedSlice = result

    default:
        return nil, fmt.Errorf("unsupported array type: %s", elementType)
    }

    // ✅ Encode the converted slice using ABI
    abiType, err := abi.NewType(elementType+"[]", "", nil)
    if err != nil {
        return nil, fmt.Errorf("failed to create ABI type: %w", err)
    }

    abiArgs := abi.Arguments{{Type: abiType}}
    encodedData, err := abiArgs.Pack(convertedSlice)
    if err != nil {
        return nil, fmt.Errorf("failed to ABI encode array: %w", err)
    }

    return encodedData, nil
}


