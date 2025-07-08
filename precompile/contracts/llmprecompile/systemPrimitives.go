package llmprecompile

import (
	"fmt"

	// "log"

	"github.com/ethereum/go-ethereum/log"

	"github.com/ava-labs/subnet-evm/precompile/contract"
)

func systemPrimitiveMethod(methodName string, inputsRaw []interface{}, accessibleState contract.AccessibleState, suppliedGas uint64) (interface{}, uint64, error) {
	stateDB := accessibleState.GetStateDB()
	remainingGas := suppliedGas

	switch methodName {
	case "answerUserQuestion":
		if len(inputsRaw) < 2 {
			return nil, remainingGas, fmt.Errorf("answerUserQuestion requires 2 arguments: question, answer")
		}
		questionRaw := inputsRaw[0]
		answerRaw := inputsRaw[1]
		// Extract string if input is map with 'data' key
		question := extractStringFromInput(questionRaw)
		answer := extractStringFromInput(answerRaw)
		if contract.IsDurangoActivated(accessibleState) {
			log.Info("Durango is activated, proceeding with event emission")
			eventData := QuestionAnswerEventData{
				Question: question,
				Answer:   answer,
			}
			log.Info("Prepared QuestionAnswerEventData", "question", question, "answer", answer)
			QuestionAnswerEventGasCost := GetQuestionAnswerEventGasCost(eventData)
			var err error
			remainingGas, err = contract.DeductGas(remainingGas, QuestionAnswerEventGasCost)
			if err != nil {
				log.Info("Failed to deduct gas for QuestionAnswerEvent", "error", err)
				return nil, remainingGas, err
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
			return true, remainingGas, nil
		} else {
			log.Info("Durango is not activated, skipping event emission")
			return "durango_not_activated", remainingGas, nil
		}
	default:
		return nil, remainingGas, fmt.Errorf("unknown system primitive method: %s", methodName)
	}
}

// extractStringFromInput extracts the string from a raw input, handling map[data:..., type:string] and plain string
func extractStringFromInput(input interface{}) string {
	if m, ok := input.(map[string]interface{}); ok {
		if data, ok := m["data"]; ok {
			if s, ok := data.(string); ok {
				return s
			}
		}
	}
	if s, ok := input.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", input)
}