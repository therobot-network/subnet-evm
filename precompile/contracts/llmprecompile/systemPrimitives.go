package llmprecompile

import (
	"fmt"
	"log"
	"math/big"

	"github.com/ava-labs/subnet-evm/precompile/contract"
	"github.com/ethereum/go-ethereum/common"
)

func systemPrimitiveStep(currentPC *big.Int, step Step, llmAddr common.Address, stateDB contract.StateDB, accessibleState contract.AccessibleState, remainingGas uint64) (*big.Int, uint64, error) {

	switch step.Method {
		case "answerUserQuestion":
			// Ensure expected types are set before calling getLookupValue
		step.Args[0].AbiType = "string"
		step.Args[1].AbiType = "string"

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
			question, ok1 := questionRaw.(string)
			answer, ok2 := answerRaw.(string)
			if !ok1 || !ok2 {
				log.Printf("Error: Expected string types for question and answer, got %T and %T", questionRaw, answerRaw)
				return nil, 0, fmt.Errorf("invalid types for question/answer: %T, %T", questionRaw, answerRaw)
			}
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
			if err := updateMemoryInState(stateDB, llmAddr, step.Output[0], arg, step.Args[0].AbiType); err != nil {
				log.Printf("Error: Failed to update memory in state for step %d. Error: %v", currentPC.Int64(), err)
				return currentPC, remainingGas, err
			}
			log.Printf("Successfully updated memory in state for assign step under key: %s.",  step.Output)
		
		case "JumpIfNot":
			jumpTarget := new(big.Int)
		
			// Ensure correct ABI type for jumpTarget
			step.Args[0].AbiType = "uint256"
			jumpTargetIF, err := getLookupValue(step.Args[0], stateDB)
			if err != nil {
				log.Printf("Error fetching jumpTarget: %v", err)
				return nil, 0, fmt.Errorf("failed to fetch jumpTarget: %w", err)
			}
		
			// Convert `jumpTargetIF` to *big.Int
			jumpTarget, ok := jumpTargetIF.(*big.Int)
			if !ok {
				return nil, 0, fmt.Errorf("JumpIfNot: expected uint256 but got %T", jumpTargetIF)
			}
		
			// Ensure correct ABI type for condition
			step.Args[1].AbiType = "bool"
			conditionIF, err := getLookupValue(step.Args[1], stateDB)
			if err != nil {
				log.Printf("Error fetching condition: %v", err)
				return nil, 0, fmt.Errorf("failed to fetch condition: %w", err)
			}
		
			// Convert `conditionIF` to bool
			condition, ok := conditionIF.(bool)
			if !ok {
				return nil, 0, fmt.Errorf("JumpIfNot: expected bool but got %T", conditionIF)
			}
		
			log.Printf("JumpIfNot: Jump=%t, JumpTarget=%s", !condition, jumpTarget.String())
		
			// Perform jump if condition is false
			if !condition {
				return jumpTarget, remainingGas, nil
			}
		case "assignArray":
			if len(step.Output) == 0 {
				log.Printf("Error: No output key specified for assignArray step")
				return currentPC, remainingGas, fmt.Errorf("assignArray step missing output key")
			}
		
			outputKey := step.Output[0]
		
			// Collect all arguments into an array
			arrayValues := make([]interface{}, len(step.Args))
			for i, arg := range step.Args {
				value, err := getLookupValue(arg, stateDB)
				if err != nil {
					log.Printf("Error fetching arg[%d]: %v", i, err)
					return currentPC, remainingGas, fmt.Errorf("failed to fetch arg[%d]: %w", i, err)
				}
				arrayValues[i] = value
			}
		
			// Store the entire array under one key
			if err := updateMemoryInState(stateDB, llmAddr, outputKey, arrayValues, step.Args[0].AbiType+"[]"); err != nil {
				log.Printf("Error: Failed to update memory in state for step %d, Output=%s. Error: %v", currentPC.Int64(), outputKey, err)
				return currentPC, remainingGas, err
			}
		
			log.Printf("Successfully stored array in memory for assignArray step under key: %s.", outputKey)
		

		// todo:
		// assignDict
		// getDict
		// readDict
	}    
    return currentPC.Add(currentPC, big.NewInt(1)), remainingGas, nil
}