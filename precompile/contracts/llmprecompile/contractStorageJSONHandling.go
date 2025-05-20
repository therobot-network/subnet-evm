package llmprecompile

import (
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

	// "log"
	"math/big"
	"reflect"
	"strings"

	"github.com/ethereum/go-ethereum/log"

	"github.com/ava-labs/subnet-evm/accounts/abi"
	"github.com/ava-labs/subnet-evm/precompile/contract"

	_ "embed"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

// getLookupValue retrieves a Value for a given key from on-chain lookup storage.
func getLookupValue(
    lookupKey string,
    stateDB contract.StateDB,
) (Value, error) {
    // 1) Read the raw JSON blob
    raw, err := getLargeState(stateDB, ContractAddress, lookupStorageKey)
    if err != nil {
        log.Info("Failed to retrieve lookup state", "Error", err)
        return Value{}, fmt.Errorf("failed to retrieve lookup storage: %w", err)
    }
    // 2) If empty, treat as “not found”
    if len(raw) == 0 {
        return Value{}, nil
    }

    // 3) Unmarshal into map[string]Value
    var lookupMap map[string]Value
    if err := json.Unmarshal(raw, &lookupMap); err != nil {
        log.Info("Failed to decode lookup JSON", "Error", err)
        return Value{}, fmt.Errorf("failed to decode lookup JSON: %w", err)
    }

    // 4) Look up the specific key
    v, ok := lookupMap[lookupKey]
    if !ok {
        log.Info("Lookup key not found", "LookupKey", lookupKey)
        return Value{}, nil
    }

    log.Info("Found lookup value", "LookupKey", lookupKey, "Value", v)
    return v, nil
}


// convertToABIType dynamically converts a value into the expected ABI type.
func convertToABIType(value interface{}, abiType abi.Type) (interface{}, error) {
	switch abiType.T {
	case abi.AddressTy:
		strValue, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("expected string for address, got %T", value)
		}
		if !common.IsHexAddress(strValue) {
			return nil, fmt.Errorf("invalid address format: %s", strValue)
		}
		return common.HexToAddress(strValue), nil

	case abi.UintTy, abi.IntTy:
		switch v := value.(type) {
		case *big.Int:
			return v, nil
		case float64:
			return big.NewInt(int64(v)), nil
		case string:
			bigIntValue, success := new(big.Int).SetString(v, 10)
			if !success {
				return nil, fmt.Errorf("invalid integer string: %s", v)
			}
			return bigIntValue, nil
		default:
			return nil, fmt.Errorf("unsupported integer input type: %T", value)
		}

	case abi.BoolTy:
		switch v := value.(type) {
		case bool:
			return v, nil
		case string:
			if v == "true" {
				return true, nil
			} else if v == "false" {
				return false, nil
			}
			return nil, fmt.Errorf("invalid boolean string: %s", v)
		default:
			return nil, fmt.Errorf("unsupported boolean input type: %T", value)
		}

	case abi.StringTy:
		strValue, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("expected string, got %T", value)
		}
		return strValue, nil

	case abi.BytesTy:
		switch v := value.(type) {
		case string:
			return hex.DecodeString(strings.TrimPrefix(v, "0x"))
		case []byte:
			return v, nil
		default:
			return nil, fmt.Errorf("unsupported bytes input type: %T", value)
		}

	case abi.FixedBytesTy:
		switch v := value.(type) {
		case string:
			b, err := hex.DecodeString(strings.TrimPrefix(v, "0x"))
			if err != nil {
				return nil, fmt.Errorf("failed to decode hex string: %w", err)
			}
			if len(b) != abiType.Size {
				return nil, fmt.Errorf("expected fixed bytes length %d, got %d", abiType.Size, len(b))
			}
			return b, nil
		case []byte:
			if len(v) != abiType.Size {
				return nil, fmt.Errorf("expected fixed bytes length %d, got %d", abiType.Size, len(v))
			}
			return v, nil
		default:
			return nil, fmt.Errorf("unsupported fixed bytes input type: %T", value)
		}

	case abi.SliceTy, abi.ArrayTy:
		valueVal := reflect.ValueOf(value)
		if valueVal.Kind() != reflect.Slice {
			return nil, fmt.Errorf("expected slice for array type, got %T", value)
		}

		var rawElems []interface{}
		for i := 0; i < valueVal.Len(); i++ {
			rawElems = append(rawElems, valueVal.Index(i).Interface())
		}

		var result []interface{}
		for i, elem := range rawElems {
			convertedElem, err := convertToABIType(elem, *abiType.Elem)
			if err != nil {
				return nil, fmt.Errorf("failed to convert array element %d: %w", i, err)
			}
			result = append(result, convertedElem)
		}

		switch abiType.Elem.T {
		case abi.UintTy, abi.IntTy:
			typed := make([]*big.Int, len(result))
			for i, v := range result {
				typed[i] = v.(*big.Int)
			}
			return typed, nil
		case abi.BoolTy:
			typed := make([]bool, len(result))
			for i, v := range result {
				typed[i] = v.(bool)
			}
			return typed, nil
		case abi.AddressTy:
			typed := make([]common.Address, len(result))
			for i, v := range result {
				typed[i] = v.(common.Address)
			}
			return typed, nil
		case abi.StringTy:
			typed := make([]string, len(result))
			for i, v := range result {
				typed[i] = v.(string)
			}
			return typed, nil
		case abi.BytesTy, abi.FixedBytesTy:
			typed := make([][]byte, len(result))
			for i, v := range result {
				typed[i] = v.([]byte)
			}
			return typed, nil
		case abi.TupleTy:
			return result, nil
		default:
			return result, nil
		}

	default:
		return nil, fmt.Errorf("unsupported ABI type: %s", abiType.String())
	}
}



// func ProcessArguments(inputs abi.Arguments, args []Operand, stateDB contract.StateDB) ([]interface{}, error) {
// 	if len(inputs) != len(args) {
// 		return nil, fmt.Errorf("mismatch between expected input count (%d) and provided arguments (%d)", len(inputs), len(args))
// 	}

// 	packedArgs := make([]interface{}, len(args))
// 	for i, input := range inputs {
// 		arg := args[i]

// 		// Step 1: Load value from state or direct value
// 		argValue, err := getLookupValue(arg, stateDB)
// 		if err != nil {
// 			log.Info("Failed fetching argument value", "ArgIndex", i, "Error", err)
// 			return nil, fmt.Errorf("failed to fetch argument value from lookup storage: %w", err)
// 		}

// 		// Step 2: Get the ABI type
// 		abiType, err := abi.NewType(input.Type.String(), "", nil)
// 		if err != nil {
// 			log.Info("Failed creating ABI type", "ArgIndex", i, "Type", input.Type.String(), "Error", err)
// 			return nil, fmt.Errorf("failed to create ABI type: %w", err)
// 		}

// 		// Step 3: Convert to ABI-compatible value
// 		convertedValue, err := convertToABIType(argValue, abiType)
// 		if err != nil {
// 			log.Info("Failed converting value to ABI type", "ArgIndex", i, "RawValue", argValue, "ExpectedType", abiType.String(), "Error", err)
// 			return nil, fmt.Errorf("failed to convert value for argument %d: %w", i, err)
// 		}

// 		log.Info("Processed argument", "ArgIndex", i, "RawValue", argValue, "ConvertedValue", convertedValue, "ExpectedType", input.Type.String())
// 		packedArgs[i] = convertedValue
// 	}

// 	return packedArgs, nil
// }

// func getContractAddress(contract Operand, stateDB contract.StateDB) (common.Address, error) {
// 	addrValue, err := getLookupValue(contract, stateDB)
// 	if err != nil {
// 		log.Info("Failed fetching contract address", "error", err)
// 		return common.Address{}, fmt.Errorf("failed to fetch contract address from lookup storage: %w", err)
// 	}

// 	if addrStr, ok := addrValue.(string); ok {
// 		if addrStr == "" {
// 			log.Info("Address string is empty, returning zero address")
// 			return common.Address{}, nil
// 		}
// 		if common.IsHexAddress(addrStr) {
// 			addr := common.HexToAddress(addrStr)
// 			log.Info("Converted string to contract address", "address", addr.Hex())
// 			return addr, nil
// 		}
// 		log.Info("Invalid Ethereum address string", "input", addrStr)
// 		return common.Address{}, fmt.Errorf("invalid contract address string: %s", addrStr)
// 	}

// 	if addrValue == nil {
// 		log.Info("Lookup value is nil, returning zero address")
// 		return common.Address{}, nil
// 	}

// 	if addr, ok := addrValue.(common.Address); ok {
// 		log.Info("Retrieved contract address", "address", addr.Hex())
// 		return addr, nil
// 	}

// 	log.Info("Invalid contract address type", "value", addrValue)
// 	return common.Address{}, fmt.Errorf("invalid contract address type: %T", addrValue)
// }

func getPCFromState(stateDB contract.StateDB, addr common.Address) (*big.Int, error) {
	currentPCBytes := stateDB.GetState(addr, common.BytesToHash(pcKeyPrefix))
	if currentPCBytes == (common.Hash{}) {
		return nil, errors.New("program counter not initialized")
	}

	if currentPCBytes == (common.Hash{1}) {
		return big.NewInt(0), nil
	}

	currentPC := new(big.Int).SetBytes(currentPCBytes.Bytes())
	log.Info("Retrieved program counter from state", "address", addr.Hex(), "pc", currentPC.String())
	return currentPC, nil
}

func savePCToState(stateDB contract.StateDB, addr common.Address, pc *big.Int) {
	valueToSave := common.BytesToHash(pc.Bytes())
	if pc.Sign() == 0 {
		valueToSave = common.Hash{1}
	}
	stateDB.SetState(addr, common.BytesToHash(pcKeyPrefix), valueToSave)
	log.Info("Saved program counter to state", "address", addr.Hex(), "pc", pc.String())
}

// Temporary function. Later we will use a DB
func getContractPrimitive(stateDB contract.StateDB, addr common.Address, address string) (contract string, primitive string) {
	log.Info("Fetching contract primitive", "target_address", address)

	parsedAddress := common.HexToAddress(address)
	addressHash := common.BytesToHash(parsedAddress.Bytes())
	fullKey := crypto.Keccak256Hash(append(addressToPrimitiveName.Bytes(), addressHash.Bytes()...))

	log.Info("Looking up primitive mapping", "lookup_address", addr.Hex(), "hashed_target", addressHash.Hex(), "full_key", fullKey.Hex())

	contractBytes, err := getLargeState(stateDB, addr, fullKey)
	if err != nil {
		log.Info("Error retrieving contract from state", "error", err)
		return "", ""
	}
	if len(contractBytes) == 0 {
		log.Info("No contract found for address", "address", address)
		return "", ""
	}

	contract = string(contractBytes)
	log.Info("Found contract", "address", address, "contract", contract)

	primitive, exists := primitiveABI[contract]
	if !exists {
		log.Info("No ABI found for contract", "contract", contract)
		return contract, ""
	}

	if len(primitive) > 500 {
		log.Info("Found primitive for contract", "contract", contract, "primitive", primitive[:500])
	}else{
		log.Info("Found primitive for contract", "contract", contract, "primitive", primitive)
	}
	return contract, primitive
}

func updatePlanLocalState(stateDB contract.StateDB, addr common.Address, key string, storageData interface{}) error {
	if key == "" {
		return nil
	}

	raw, err := getLargeState(stateDB, addr, lookupStorageKey)
	if err != nil {
		return fmt.Errorf("failed to get existing state: %w", err)
	}

	var jsonMap map[string]interface{}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &jsonMap); err != nil {
			return fmt.Errorf("failed to parse existing JSON state: %w", err)
		}
	} else {
		jsonMap = make(map[string]interface{})
	}

	jsonMap[key] = storageData

	updated, err := json.Marshal(jsonMap)
	if err != nil {
		return fmt.Errorf("failed to marshal updated JSON: %w", err)
	}

	setLargeState(stateDB, addr, lookupStorageKey, updated)
	log.Info("Updated plan-local state", "key", key, "value", storageData)

	snapshot := string(updated)
	if len(snapshot) > 5000 {
		snapshot = snapshot[:5000] + "...[truncated]"
	}
	log.Info("Plan-local state snapshot (capped)", "snapshot", snapshot)

	return nil
}


func storeLookupEntries(stateDB contract.StateDB, addr common.Address, lookupJsonString string) (map[string]interface{}, error) {
	if lookupJsonString == "" {
		log.Info("No lookup entries to store")
		return map[string]interface{}{}, nil
	}

	var lookupMap map[string]interface{}
	if err := json.Unmarshal([]byte(lookupJsonString), &lookupMap); err != nil {
		log.Info("Failed to parse lookup JSON", "error", err)
		return nil, fmt.Errorf("failed to unmarshal lookup JSON: %w", err)
	}

	for key, val := range lookupMap {
		if err := updatePlanLocalState(stateDB, addr, key, val); err != nil {
			log.Info("Failed to store lookup entry", "key", key, "error", err)
			return nil, fmt.Errorf("failed to store lookup entry for key %s: %w", key, err)
		}
		log.Info("Stored lookup entry", "key", key, "value", val)
	}

	return lookupMap, nil
}

func GetPromptCounter(stateDB contract.StateDB) *big.Int {
	value := stateDB.GetState(ContractAddress, promptCounterKey)
	if value == (common.Hash{}) {
		log.Info("Prompt counter not found in state, initializing to 0")
		return big.NewInt(0)
	}
	counter := new(big.Int).SetBytes(value.Bytes())
	log.Info("Retrieved prompt counter", "counter", counter.String())
	return counter
}

// IncrementPromptCounter increments the value of promptCounter in the StateDB by 1.
func IncrementPromptCounter(stateDB contract.StateDB) *big.Int {
	currentCounter := GetPromptCounter(stateDB)
	nextCounter := new(big.Int).Add(currentCounter, big.NewInt(1))

	// Store the new value in the StateDB
	stateDB.SetState(ContractAddress, promptCounterKey, common.BigToHash(nextCounter))

	return nextCounter // Return the current value before incrementing
}

func storeFunctionDefinition(
    stateDB contract.StateDB,
    addr common.Address,
    funcName string,
    funcDef RobotFunction,
) error {
    // if there's no name, nothing to store
    if funcName == "" {
        log.Info("Function name is empty, skipping storage")
        return nil
    }

    // compute a unique storage key: keccak256(prefix || funcName)
    fullKey := crypto.Keccak256Hash(
        append(robotFunctionPrefix, []byte(funcName)...),
    )

    // marshal the struct into JSON
    funcData, err := json.Marshal(funcDef)
    if err != nil {
        log.Info("Failed to marshal function definition",
            "functionName", funcName,
            "error", err,
        )
        return err
    }

    // store the JSON blob in state
    if err := setLargeState(stateDB, addr, fullKey, funcData); err != nil {
        log.Info("Failed to store function definition",
            "functionName", funcName,
            "error", err,
        )
        return err
    }

    log.Info("Stored function definition", "functionName", funcName)
    return nil
}

func getFunctionDefinition(stateDB contract.StateDB, addr common.Address, funcName string) (RobotFunction, error) {
	if funcName == "" {
		log.Info("Function name is empty, returning empty definition")
		return RobotFunction{}, nil
	}

	fullKey := crypto.Keccak256Hash(
		append(robotFunctionPrefix, []byte(funcName)...),
	)

	funcData, err := getLargeState(stateDB, addr, fullKey)
	if err != nil {
		log.Info("Error retrieving function definition", "functionName", funcName, "error", err)
		return RobotFunction{}, fmt.Errorf("failed to retrieve function definition: %w", err)
	}
	if len(funcData) == 0 {
		log.Info("No function definition found", "functionName", funcName)
		return RobotFunction{}, nil
	}

	var funcDef RobotFunction
	if err := json.Unmarshal(funcData, &funcDef); err != nil {
		log.Info("Failed to unmarshal function definition", "functionName", funcName, "error", err)
		return RobotFunction{}, fmt.Errorf("failed to unmarshal function definition: %w", err)
	}

	return funcDef, nil
}

func deleteFunctionDefinition(stateDB contract.StateDB, addr common.Address, funcName string) {
	if funcName == "" {
		log.Info("Function name is empty, skipping deletion")
		return
	}

	fullKey := crypto.Keccak256Hash(
		append(robotFunctionPrefix, []byte(funcName)...),
	)

	deleteLargeState(stateDB, addr, fullKey)
	log.Info("Deleted function definition", "functionName", funcName)
}

// pushIPFrame serializes and pushes an InstructionPointer onto the on-chain IP stack.
func pushIPFrame(
    stateDB contract.StateDB,
    addr common.Address,
    ip InstructionPointer,
) error {
    // Marshal the IP frame
    data, err := json.Marshal(ip)
    if err != nil {
        return fmt.Errorf("pushIPFrame marshal: %w", err)
    }
    // Read current stack length
    lenHash := stateDB.GetState(addr, ipStackLenKey)
    lenBytes := lenHash.Bytes()
    length := binary.BigEndian.Uint64(lenBytes[len(lenBytes)-8:])
    // Store at slotKey = baseKey || length
    idx := make([]byte, 8)
    binary.BigEndian.PutUint64(idx, length)
    slotKey := common.BytesToHash(append(ipStackBaseKey.Bytes(), idx...))
    if err := setLargeState(stateDB, addr, slotKey, data); err != nil {
        return fmt.Errorf("pushIPFrame write: %w", err)
    }
    // Increment length
    newLen := length + 1
    newLenBytes := make([]byte, 32)
    binary.BigEndian.PutUint64(newLenBytes[24:], newLen)
    stateDB.SetState(addr, ipStackLenKey, common.BytesToHash(newLenBytes))
    return nil
}

// IPFrame represents a popped InstructionPointer from the call stack.
type IPFrame struct {
    IP InstructionPointer
	Function RobotFunction
}

// popIPFrame pops the top InstructionPointer frame. Returns (frame, true) or (zero, false) if empty.
func popIPFrame(
    stateDB contract.StateDB,
    addr common.Address,
) (IPFrame, bool) {
    // Read current length
    lenHash := stateDB.GetState(addr, ipStackLenKey)
    lenBytes := lenHash.Bytes()
    length := binary.BigEndian.Uint64(lenBytes[len(lenBytes)-8:])
    if length == 0 {
        return IPFrame{}, false
    }
    // Compute slotKey for last frame
    newLen := length - 1
    idx := make([]byte, 8)
    binary.BigEndian.PutUint64(idx, newLen)
    slotKey := common.BytesToHash(append(ipStackBaseKey.Bytes(), idx...))
    // Retrieve and clear
    data, err := getLargeState(stateDB, addr, slotKey)
    if err != nil {
        return IPFrame{}, false
    }
    stateDB.SetState(addr, slotKey, common.Hash{})
    // Decode
    var ip InstructionPointer
    if err := json.Unmarshal(data, &ip); err != nil {
        return IPFrame{}, false
    }
    // Update length
    newLenBytes := make([]byte, 32)
    binary.BigEndian.PutUint64(newLenBytes[24:], newLen)
    stateDB.SetState(addr, ipStackLenKey, common.BytesToHash(newLenBytes))
	// Retrieve function definition
	funcDef, err := getFunctionDefinition(stateDB, addr, ip.robotFunction)
    return IPFrame{IP: ip, Function: funcDef}, true
}

func clearPlanLocalState(stateDB contract.StateDB, addr common.Address, storedStateHashes []common.Hash) {
	log.Info("Clearing plan-local state")

	deleteLargeState(stateDB, addr, stepsKey)
	deleteLargeState(stateDB, addr, lookupStorageKey)
	// deleteLargeState(stateDB, addr, pcKeyPrefix)

	// for  _, hash := range storedStateHashes {
	// 	deleteLargeState(stateDB, addr, hash)
	// }

	log.Info("Cleared plan-local state")
}
