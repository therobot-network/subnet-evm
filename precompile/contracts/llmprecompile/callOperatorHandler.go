package llmprecompile

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ava-labs/subnet-evm/precompile/contract"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
)

// handleCallOp advances the instruction pointer for 'call' operator (no-op for now)
func handleCallOp(
	step ActionStep,
	ip InstructionPointer,
	stateDB contract.StateDB,
	addr common.Address,
	remainingGas uint64,
	accessibleState contract.AccessibleState,
) (InstructionPointer, uint64, error) {
	// If 'object' is present, would dispatch to a method on that object (not implemented)
	if step.Operands.Object != nil {
		log.Warn("handleCallOp: object dispatch not implemented", "object", step.Operands.Object)
		ip.advance()
		return ip, remainingGas, nil
	}

	var methodName string
	if step.Operands.Method != nil {
		if step.Operands.Method.Type != "string" {
			return ip, remainingGas, fmt.Errorf("handleCallOp: method type is not 'string' (got '%s')", step.Operands.Method.Type)
		}
		var ok bool
		methodName, ok = step.Operands.Method.Data.(string)
		if !ok {
			return ip, remainingGas, fmt.Errorf("handleCallOp: method name is not a string (got %T)", step.Operands.Method.Data)
		}
	} else {
		return ip, remainingGas, fmt.Errorf("handleCallOp: method is nil")
	}

	switch methodName {
	case "answerUserQuestion":
		log.Info("handleCallOp: dispatching to answerUserQuestion")
		return answerUserQuestion(step, ip, stateDB, addr, remainingGas, accessibleState)
	default:
		log.Warn("handleCallOp: unknown method, error", "method", methodName)
		return ip, remainingGas, fmt.Errorf("handleCallOp: unknown method '%s'", methodName)
	}
}

// answerUserQuestion handles a QuestionAnswer step as a 'call' method:
// it expects the question as the first positional arg, and the answer as the second positional arg (pop or value).
func answerUserQuestion(
	step ActionStep,
	ip InstructionPointer,
	stateDB contract.StateDB,
	addr common.Address,
	remainingGas uint64,
	accessibleState contract.AccessibleState,
) (InstructionPointer, uint64, error) {
	callArgs := step.Operands.Args
	if len(callArgs.Positional) < 2 {
		return ip, remainingGas, fmt.Errorf("answerUserQuestion: missing positional arguments (need at least 2)")
	}
	questionVal := &callArgs.Positional[0]
	answerVal := &callArgs.Positional[1]
	// 2) Evaluate question
	var questionStr string
	if questionVal.Type == "string" {
		if s, ok := questionVal.Data.(string); ok {
			questionStr = s
		} else {
			return ip, remainingGas, fmt.Errorf("answerUserQuestion: question is not a string (got %T)", questionVal.Data)
		}
	} else {
		return ip, remainingGas, fmt.Errorf("answerUserQuestion: question type is not string (got %s)", questionVal.Type)
	}
	log.Info("answerUserQuestion: raw question", "q", questionStr)
	// 3) Evaluate answer
	aVal, err := evaluateOperand(answerVal, stateDB, addr)
	if err != nil {
		log.Error("answerUserQuestion: failed to evaluate answer", "error", err)
		return ip, remainingGas, err
	}
	log.Info("answerUserQuestion: answer Value", "aVal", aVal)
	// 4) Coerce answer to string (same as before)
	var answerStr string
	if s, ok := aVal.Data.(string); ok {
		answerStr = s
	} else if aVal.Type == "list" {
		// Convert []Value to []interface{} of their Data fields
		vals, ok := aVal.Data.([]Value)
		if !ok {
			log.Error("answerUserQuestion: list answer is not []Value", "data", aVal.Data)
			answerStr = fmt.Sprintf("%v", aVal.Data)
		} else {
			arr := make([]interface{}, len(vals))
			for i, v := range vals {
				arr[i] = v.Data
			}
			// Marshal as JSON array
			bytes, err := json.Marshal(arr)
			if err != nil {
				log.Error("answerUserQuestion: failed to marshal list answer", "err", err)
				answerStr = fmt.Sprintf("%v", arr)
			} else {
				answerStr = string(bytes)
			}
		}
	} else if aVal.Type == "tuple" {
		// Convert []Value to Python-style tuple string
		vals, ok := aVal.Data.([]Value)
		if !ok {
			log.Error("answerUserQuestion: tuple answer is not []Value", "data", aVal.Data)
			answerStr = fmt.Sprintf("%v", aVal.Data)
		} else {
			elems := make([]string, len(vals))
			for i, v := range vals {
				elems[i] = fmt.Sprintf("%v", v.Data)
			}
			answerStr = "(" + strings.Join(elems, ", ") + ")"
		}
	} else {
		answerStr = fmt.Sprintf("%v", aVal.Data)
	}
	// 5) If Durango is active, emit the event
	if contract.IsDurangoActivated(accessibleState) {
		log.Info("Durango activated, emitting QuestionAnswerEvent",
			"question", questionStr, "answer", answerStr,
		)

		event := QuestionAnswerEventData{
			Question: questionStr,
			Answer:   answerStr,
		}
		cost := GetQuestionAnswerEventGasCost(event)
		if remainingGas, err = contract.DeductGas(remainingGas, cost); err != nil {
			log.Error("Failed to deduct gas for QuestionAnswerEvent", "error", err)
			return InstructionPointer{}, 0, err
		}

		topics, data, err := PackQuestionAnswerEvent(event)
		if err != nil {
			log.Error("Failed to pack QuestionAnswerEvent", "error", err)
			return InstructionPointer{}, remainingGas, err
		}
		stateDB.AddLog(
			ContractAddress,
			topics,
			data,
			accessibleState.GetBlockContext().Number().Uint64(),
		)
	} else {
		log.Info("Durango not activated, skipping QuestionAnswerEvent")
	}
	// 6) Advance the IP and return
	ip.advance()
	return ip, remainingGas, nil
}