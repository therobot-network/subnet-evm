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
	"github.com/shopspring/decimal"
)

var zero = decimal.NewFromInt(0)

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
        if out.Type == "list" || out.Type == "tuple" {
            // Ensure Data is []Value, not []interface{}
            // Marshal the Data back to JSON, then unmarshal as []Value
            raw, err := json.Marshal(out.Data)
            if err != nil {
                log.Error("evaluateOperand: marshal list/tuple data failed", "err", err)
                return Value{}, err
            }
            var vals []Value
            if err := json.Unmarshal(raw, &vals); err != nil {
                log.Error("evaluateOperand: unmarshal list/tuple data as []Value failed", "err", err)
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
        } else if leftVal.Type == "tuple" && rightVal.Type == "tuple" {
            leftTuple, ok1 := leftVal.Data.([]interface{})
            rightTuple, ok2 := rightVal.Data.([]interface{})
            if !ok1 || !ok2 {
                err := fmt.Errorf("plus: operands are not tuples")
                log.Error("handleBinaryOp", "error", err)
                return ip, remainingGas, err
            }
            merged := append(leftTuple, rightTuple...)
            result = merged
            resultType = "tuple"
        } else if leftVal.Type == "string" || rightVal.Type == "string" {
            if leftVal.Type != "string" || rightVal.Type != "string" {
                err := fmt.Errorf("cannot mix string and non-string with +")
                log.Error("handleBinaryOp", "error", err)
                return ip, remainingGas, err
            }
            result = leftVal.Data.(string) + rightVal.Data.(string)
            resultType = "string"
        } else if leftVal.Type == "float" || rightVal.Type == "float" {
            ldec, errL := decimalFromValue(leftVal)
            if errL != nil {
                log.Error("handleBinaryOp: left decimalFromValue failed", "err", errL)
                return ip, remainingGas, errL
            }
            rdec, errR := decimalFromValue(rightVal)
            if errR != nil {
                log.Error("handleBinaryOp: right decimalFromValue failed", "err", errR)
                return ip, remainingGas, errR
            }
            sum := ldec.Add(rdec)
            result = sum.String()
            resultType = "float"
        } else if leftVal.Type == rightVal.Type {
            li, errL := toInt(leftVal)
            if errL != nil {
                log.Error("handleBinaryOp: left toInt failed", "err", errL)
                return ip, remainingGas, errL
            }
            ri, errR := toInt(rightVal)
            if errR != nil {
                log.Error("handleBinaryOp: right toInt failed", "err", errR)
                return ip, remainingGas, errR
            }
            result = li + ri
            resultType = leftVal.Type
        } else {
            err := fmt.Errorf("plus: cannot add types %s and %s", leftVal.Type, rightVal.Type)
            log.Error("handleBinaryOp", "error", err)
            return ip, remainingGas, err
        }
    case "minus":
        if leftVal.Type == "float" || rightVal.Type == "float" {
            ldec, errL := decimalFromValue(leftVal)
            if errL != nil {
                log.Error("handleBinaryOp: left decimalFromValue failed", "err", errL)
                return ip, remainingGas, errL
            }
            rdec, errR := decimalFromValue(rightVal)
            if errR != nil {
                log.Error("handleBinaryOp: right decimalFromValue failed", "err", errR)
                return ip, remainingGas, errR
            }
            diff := ldec.Sub(rdec)
            result = diff.String()
            resultType = "float"
        } else {
            li, errL := toInt(leftVal)
            if errL != nil {
                log.Error("handleBinaryOp: left toInt failed", "err", errL)
                return ip, remainingGas, errL
            }
            ri, errR := toInt(rightVal)
            if errR != nil {
                log.Error("handleBinaryOp: right toInt failed", "err", errR)
                return ip, remainingGas, errR
            }
            result = li - ri
            resultType = "int"
        }
    case "multiply":
        switch {
        case leftVal.Type == "string" && rightVal.Type == "int":
            ri, errR := toInt(rightVal)
            if errR != nil {
                log.Error("handleBinaryOp: right toInt failed", "err", errR)
                return ip, remainingGas, errR
            }
            result = strings.Repeat(leftVal.Data.(string), ri)
            resultType = "string"
        case rightVal.Type == "string" && leftVal.Type == "int":
            li, errL := toInt(leftVal)
            if errL != nil {
                log.Error("handleBinaryOp: left toInt failed", "err", errL)
                return ip, remainingGas, errL
            }
            result = strings.Repeat(rightVal.Data.(string), li)
            resultType = "string"
        case leftVal.Type == "float" || rightVal.Type == "float":
            ldec, errL := decimalFromValue(leftVal)
            if errL != nil {
                log.Error("handleBinaryOp: left decimalFromValue failed", "err", errL)
                return ip, remainingGas, errL
            }
            rdec, errR := decimalFromValue(rightVal)
            if errR != nil {
                log.Error("handleBinaryOp: right decimalFromValue failed", "err", errR)
                return ip, remainingGas, errR
            }
            prod := ldec.Mul(rdec)
            result = prod.String()
            resultType = "float"
        default:
            li, errL := toInt(leftVal)
            if errL != nil {
                log.Error("handleBinaryOp: left toInt failed", "err", errL)
                return ip, remainingGas, errL
            }
            ri, errR := toInt(rightVal)
            if errR != nil {
                log.Error("handleBinaryOp: right toInt failed", "err", errR)
                return ip, remainingGas, errR
            }
            result = li * ri
            resultType = "int"
        }
    case "divide":
        // Always perform float division using decimal, even for int inputs
        ldec, errL := decimalFromValue(leftVal)
        if errL != nil {
            log.Error("handleBinaryOp: left decimalFromValue failed", "err", errL)
            return ip, remainingGas, errL
        }
        rdec, errR := decimalFromValue(rightVal)
        if errR != nil {
            log.Error("handleBinaryOp: right decimalFromValue failed", "err", errR)
            return ip, remainingGas, errR
        }
        zero := decimal.NewFromInt(0)
        if rdec.Equal(zero) {
            err := fmt.Errorf("divide by zero")
            log.Error("handleBinaryOp", "error", err)
            return ip, remainingGas, err
        }
        quot := ldec.Div(rdec)
        result = quot.String()
        resultType = "float"
    case "floorDivide":
        // Floor division: always use decimal, return int if both operands are int, else string (Python: 7//3=2, 7//2.5=2.0, 7.5//2.5=3.0)
        ldec, errL := decimalFromValue(leftVal)
        if errL != nil {
            log.Error("handleBinaryOp: left decimalFromValue failed", "err", errL)
            return ip, remainingGas, errL
        }
        rdec, errR := decimalFromValue(rightVal)
        if errR != nil {
            log.Error("handleBinaryOp: right decimalFromValue failed", "err", errR)
            return ip, remainingGas, errR
        }
        
        if rdec.Equal(zero) {
            err := fmt.Errorf("floor division by zero")
            log.Error("handleBinaryOp", "error", err)
            return ip, remainingGas, err
        }
        quot := ldec.Div(rdec)
        floored := quot.Floor()
        // If both operands are int, return int type, else string (float)
        if leftVal.Type == "int" && rightVal.Type == "int" {
            result = floored.IntPart()
            resultType = "int"
        } else {
            // Always show .0 for float floor division (Python: 7//2.5 = 2.0)
            result = floored.StringFixed(1)
            resultType = "float"
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
    log.Info("handleBinaryOp: about to marshal wrapped result", "wrapped", wrapped, "dataType", fmt.Sprintf("%T", wrapped.Data))
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

// handleAssignOp implements the 'assign' operator: assigns a value to a variable in the lookup table.
func handleAssignOp(
    step ActionStep,
    ip InstructionPointer,
    stateDB contract.StateDB,
    addr common.Address,
    remainingGas uint64,
) (InstructionPointer, uint64, error) {
    if step.Operands.Target == nil {
        err := fmt.Errorf("assign: missing target")
        log.Error("handleAssignOp", "error", err)
        return ip, remainingGas, err
    }
    key := *step.Operands.Target
    val, err := evaluateOperand(step.Operands.Value, stateDB, addr)
    if err != nil {
        log.Error("assign: failed to evaluate value", "err", err)
        return ip, remainingGas, err
    }
    if err := updatePlanLocalState(stateDB, addr, key, val); err != nil {
        log.Error("assign: failed to update lookup table", "err", err)
        return ip, remainingGas, err
    }
    log.Info("assign: updated lookup table", "key", key, "value", val)
    ip.advance()
    log.Info("handleAssignOp EXIT", "newIndex", ip.index)
    return ip, remainingGas, nil
}

// toInt converts an int or float Value to int, returns error if not possible.
func toInt(v Value) (int, error) {
    switch x := v.Data.(type) {
    case int:
        return x, nil
    case float64:
        return int(x), nil
    case string:
        i, err := strconv.Atoi(x)
        if err != nil {
            log.Error("toInt: failed to parse string", "value", x, "err", err)
            return 0, fmt.Errorf("toInt: failed to parse string '%s': %w", x, err)
        }
        return i, nil
    default:
        log.Error("toInt: unexpected data type", "type", fmt.Sprintf("%T", v.Data))
        return 0, fmt.Errorf("toInt: unexpected data type %T", v.Data)
    }
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

// decimalFromValue converts a Value to decimal.Decimal for fixed-point math.
func decimalFromValue(v Value) (decimal.Decimal, error) {
	if v.Type == "int" {
		switch x := v.Data.(type) {
		case int:
			return decimal.NewFromInt(int64(x)), nil
		case float64:
			return decimal.NewFromFloat(x), nil
		case string:
			return decimal.NewFromString(x)
		}
	} else if v.Type == "float" {
		switch x := v.Data.(type) {
		case float64:
			return decimal.NewFromFloat(x), nil
		case string:
			return decimal.NewFromString(x)
		}
	}
	return decimal.Decimal{}, fmt.Errorf("decimalFromValue: unsupported type %s/%T", v.Type, v.Data)
}
