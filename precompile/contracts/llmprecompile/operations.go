package llmprecompile

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/ava-labs/subnet-evm/precompile/contract"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"
)

// evaluateOperand takes one *Value slot (literal, lookup, or pop) and returns its full Value.
func evaluateOperand(
    v *Value,
    stateDB contract.StateDB,
    addr common.Address,
) (Value, error) {
    log.Info("evaluateOperand ENTER", "raw", v)

    if v == nil {
        return Value{}, fmt.Errorf("evaluateOperand: nil operand")
    }
    // 1) stack‐pop
    if v.Pop {
        log.Info("evaluateOperand: Pop", "v", v)
        data, err := popFromVarStack(stateDB, addr)
        if err != nil {
            log.Error("evaluateOperand: pop error", "err", err)
            return Value{}, err
        }
        var out Value
        if err := json.Unmarshal(data, &out); err != nil {
            log.Error("evaluateOperand: unmarshal pop", "bytes", string(data), "err", err)
            return Value{}, err
        }
        if out.Type == "list" {
            // Ensure Data is []Value, not []interface{}
            // Marshal the Data back to JSON, then unmarshal as []Value
            raw, err := json.Marshal(out.Data)
            if err != nil {
                log.Error("evaluateOperand: marshal list data failed", "err", err)
                return Value{}, err
            }
            var vals []Value
            if err := json.Unmarshal(raw, &vals); err != nil {
                log.Error("evaluateOperand: unmarshal list data as []Value failed", "err", err)
                return Value{}, err
            }
            out.Data = vals
        }
        log.Info("evaluateOperand: popped result", "value", out)
        return out, nil
    }
    // 2) lookup
    if v.Lookup != nil {
        log.Info("evaluateOperand: Lookup", "key", *v.Lookup)
        out, err := getLookupValue(*v.Lookup, stateDB)
        if err != nil {
            log.Error("evaluateOperand: lookup error", "err", err)
            return Value{}, err
        }
        log.Info("evaluateOperand: lookup result", "value", out)
        return out, nil
    }
    // 3) literal
    if v.Data != nil {
        log.Info("evaluateOperand: literal", "data", v.Data, "type", v.Type)
        return *v, nil
    }
    // 4) unsupported
    log.Error("evaluateOperand: unsupported operand", "raw", v)
    return Value{}, fmt.Errorf("evaluateOperand: unsupported operand %+v", v)
}


// handleBinaryOp evaluates a binary operator, wraps the result in a Value, pushes it onto the varStack, and advances the IP.
func handleBinaryOp(
    step ActionStep,
    ip InstructionPointer,
    stateDB contract.StateDB,
    addr common.Address,
    remainingGas uint64,
) (InstructionPointer, uint64, error) {
        log.Info("handleBinaryOp ENTER", "op", step.Operator, "idx", ip.index)

    leftVal, err := evaluateOperand(step.Operands.Left, stateDB, addr)
    if err != nil {
        log.Error("handleBinaryOp left eval failed", "err", err)
        return ip, remainingGas, err
    }
    rightVal, err := evaluateOperand(step.Operands.Right, stateDB, addr)
    if err != nil {
        log.Error("handleBinaryOp right eval failed", "err", err)
        return ip, remainingGas, err
    }
    log.Info("handleBinaryOp operands", "L", leftVal, "R", rightVal)

    var result interface{}
    var resultType string

    switch step.Operator {
    case "plus":
        if leftVal.Type == "list" && rightVal.Type == "list" {
            // Merge two lists
            leftList, ok1 := leftVal.Data.([]interface{})
            rightList, ok2 := rightVal.Data.([]interface{})
            if !ok1 || !ok2 {
                err := fmt.Errorf("plus: operands are not lists")
                log.Error("handleBinaryOp", "error", err)
                return ip, remainingGas, err
            }
            merged := append(leftList, rightList...)
            result = merged
            resultType = "list"
        } else if leftVal.Type == "string" || rightVal.Type == "string" {
            if leftVal.Type != "string" || rightVal.Type != "string" {
                err := fmt.Errorf("cannot mix string and non-string with +")
                log.Error("handleBinaryOp", "error", err)
                return ip, remainingGas, err
            }
            result = leftVal.Data.(string) + rightVal.Data.(string)
            resultType = "string"
        } else if leftVal.Type == "float" || rightVal.Type == "float" {
            lf, rf := toFloat(leftVal), toFloat(rightVal)
            result = lf + rf
            resultType = "float"
        } else {
            li, ri := toInt(leftVal), toInt(rightVal)
            result = li + ri
            resultType = "int"
        }

    case "minus":
        if leftVal.Type == "float" || rightVal.Type == "float" {
            lf, rf := toFloat(leftVal), toFloat(rightVal)
            result = lf - rf
            resultType = "float"
        } else {
            li, ri := toInt(leftVal), toInt(rightVal)
            result = li - ri
            resultType = "int"
        }

    case "multiply":
        switch {
        case leftVal.Type == "string" && rightVal.Type == "int":
            result = strings.Repeat(leftVal.Data.(string), toInt(rightVal))
            resultType = "string"
        case rightVal.Type == "string" && leftVal.Type == "int":
            result = strings.Repeat(rightVal.Data.(string), toInt(leftVal))
            resultType = "string"
        case leftVal.Type == "float" || rightVal.Type == "float":
            lf, rf := toFloat(leftVal), toFloat(rightVal)
            result = lf * rf
            resultType = "float"
        default:
            result = toInt(leftVal) * toInt(rightVal)
            resultType = "int"
        }

    case "divide":
        if toFloat(rightVal) == 0 {
            err := fmt.Errorf("divide by zero")
            log.Error("handleBinaryOp", "error", err)
            return ip, remainingGas, err
        }
        if leftVal.Type == "float" || rightVal.Type == "float" {
            lf, rf := toFloat(leftVal), toFloat(rightVal)
            result = lf / rf
            resultType = "float"
        } else {
            result = toInt(leftVal) / toInt(rightVal)
            resultType = "int"
        }

    default:
        err := fmt.Errorf("unsupported operator: %s", step.Operator)
        log.Error("handleBinaryOp", "error", err)
        return ip, remainingGas, err
    }

    log.Info("handleBinaryOp computed",
        "result", result,
        "type", resultType,
    )

    wrapped := Value{Data: result, Type: resultType}
    data, err := json.Marshal(wrapped)
    if err != nil {
        log.Error("handleBinaryOp: marshal wrapped result failed", "error", err)
        return ip, remainingGas, err
    }
    log.Info("handleBinaryOp pushing result to varStack", "wrapped", wrapped)

    if err := pushToVarStack(stateDB, addr, data); err != nil {
        log.Error("handleBinaryOp: pushToVarStack failed", "error", err)
        return ip, remainingGas, err
    }

    ip.advance()
    log.Info("handleBinaryOp EXIT", "newIndex", ip.index)
    return ip, remainingGas, nil
}

// toFloat converts an int or float Value to float64.
func toFloat(v Value) float64 {
    switch x := v.Data.(type) {
    case float64:
        return x
    case int:
        return float64(x)
    case string:
        f, err := strconv.ParseFloat(x, 64)
        if err != nil {
            log.Error("toFloat: failed to parse string", "value", x, "err", err)
            return 0
        }
        return f
    default:
        log.Error("toFloat: unexpected data type", "type", fmt.Sprintf("%T", v.Data))
        return 0
    }
}

func toInt(v Value) int {
    switch x := v.Data.(type) {
    case int:
        return x
    case float64:
        return int(x)
    case string:
        i, err := strconv.Atoi(x)
        if err != nil {
            log.Error("toInt: failed to parse string", "value", x, "err", err)
            return 0
        }
        return i
    default:
        log.Error("toInt: unexpected data type", "type", fmt.Sprintf("%T", v.Data))
        return 0
    }
}

// answerUserQuestion handles a QuestionAnswer step:
// it pops the question & answer Values, converts them to strings,
// emits a QuestionAnswerEvent if Durango is activated, and advances the IP.
func answerUserQuestion(
    step ActionStep,
    ip InstructionPointer,
    stateDB contract.StateDB,
    addr common.Address,
    remainingGas uint64,
    accessibleState contract.AccessibleState,
) (InstructionPointer, uint64, error) {
    // 1) Pull question directly from the JSON‐string field
    if step.Operands.Question == nil {
        return ip, remainingGas, fmt.Errorf("answerUserQuestion: missing question")
    }
    questionStr := *step.Operands.Question
    log.Info("answerUserQuestion: raw question", "q", questionStr)

    // 2) Evaluate answer
    aVal, err := evaluateOperand(step.Operands.Answer, stateDB, addr)
    if err != nil {
        log.Error("answerUserQuestion: failed to evaluate answer", "error", err)
        return ip, remainingGas, err
    }
    log.Info("answerUserQuestion: answer Value", "aVal", aVal)

    // 3) Coerce answer to string
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
    } else {
        answerStr = fmt.Sprintf("%v", aVal.Data)
    }

    // 4) If Durango is active, emit the event
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

    // 5) Advance the IP and return
    ip.advance()
    return ip, remainingGas, nil
}



// pushToVarStack pushes a JSON-encoded entry onto a growing stack without reading the entire stack.
func pushToVarStack(
    stateDB contract.StateDB,
    addr common.Address,
    data []byte,
) error {
    // 1) Read current length
    lengthHash := stateDB.GetState(addr, varStackLenKey)
    lengthBytes := lengthHash.Bytes()
    length := binary.BigEndian.Uint64(lengthBytes[len(lengthBytes)-8:])
    log.Info("pushToVarStack: current length", "length", length)

    // 2) Compute slot key
    indexPrefix := make([]byte, 8)
    binary.BigEndian.PutUint64(indexPrefix, length)
    slotKey := crypto.Keccak256Hash(append(varStackBaseKey.Bytes(), indexPrefix...))
    log.Info("pushToVarStack: storing element", "slotKey", slotKey.Hex(), "dataLen", len(data))

    if err := setLargeState(stateDB, addr, slotKey, data); err != nil {
        log.Error("pushToVarStack: write element failed", "err", err)
        return fmt.Errorf("pushToVarStack: write element: %w", err)
    }

    // 3) Increment and store back new length
    newLen := length + 1
    newLenBytes := make([]byte, 32)
    binary.BigEndian.PutUint64(newLenBytes[24:], newLen)
    stateDB.SetState(addr, varStackLenKey, common.BytesToHash(newLenBytes))
    log.Info("pushToVarStack: updated length", "newLength", newLen)

    return nil
}

// popFromVarStack pops the most recent entry without reading the entire stack.
func popFromVarStack(
    stateDB contract.StateDB,
    addr common.Address,
) ([]byte, error) {
    // 1) Read current length
    lengthHash := stateDB.GetState(addr, varStackLenKey)
    lengthBytes := lengthHash.Bytes()
    length := binary.BigEndian.Uint64(lengthBytes[len(lengthBytes)-8:])
    log.Info("popFromVarStack: current length", "length", length)
    if length == 0 {
        log.Error("popFromVarStack: empty stack")
        return nil, fmt.Errorf("popFromVarStack: empty stack")
    }

    // 2) Compute slot of last element
    newLen := length - 1
    indexPrefix := make([]byte, 8)
    binary.BigEndian.PutUint64(indexPrefix, newLen)
    slotKey := crypto.Keccak256Hash(append(varStackBaseKey.Bytes(), indexPrefix...))
    log.Info("popFromVarStack: reading element", "slotKey", slotKey.Hex())

    // 3) Retrieve and delete element
    data, err := getLargeState(stateDB, addr, slotKey)
    if err != nil {
        log.Error("popFromVarStack: read element failed", "err", err)
        return nil, fmt.Errorf("popFromVarStack: read element: %w", err)
    }
    stateDB.SetState(addr, slotKey, common.Hash{})
    log.Info("popFromVarStack: element retrieved", "dataLen", len(data))

    // 4) Store decremented length
    newLenBytes := make([]byte, 32)
    binary.BigEndian.PutUint64(newLenBytes[24:], newLen)
    stateDB.SetState(addr, varStackLenKey, common.BytesToHash(newLenBytes))
    log.Info("popFromVarStack: updated length", "newLength", newLen)

    return data, nil
}
