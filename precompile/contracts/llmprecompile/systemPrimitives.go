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
	
		

		// todo:
		// assignDict
		// getDict
		// readDict
	}    
    return currentPC.Add(currentPC, big.NewInt(1)), remainingGas, nil
}


