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

// getLookupValue retrieves a value from the lookup table or directly from the Operand.
func getLookupValue(arg Operand, stateDB contract.StateDB) (interface{}, error) {
	// If Value is explicitly set (even to ""), use it
	if arg.Value != nil {
		return *arg.Value, nil
	}

	// If Lookup is not set or is explicitly empty, stop here
	if arg.Lookup == nil {
		return nil, nil
	}

	lookupKey := *arg.Lookup // safely dereference once

	lookupData, err := getLargeState(stateDB, ContractAddress, lookupStorageKey)
	if err != nil {
		log.Info("Failed to retrieve lookup state", "Error", err)
		return nil, fmt.Errorf("failed to retrieve lookup storage: %w", err)
	}

	if len(lookupData) == 0 {
		log.Info("Lookup state is empty", "LookupKey", lookupKey)
		return nil, nil
	}

	var lookupMap map[string]interface{}
	if err := json.Unmarshal(lookupData, &lookupMap); err != nil {
		log.Info("Failed to decode lookup JSON", "Error", err)
		return nil, fmt.Errorf("failed to decode lookup JSON: %w", err)
	}

	val, exists := lookupMap[lookupKey]
	if !exists {
		log.Info("Lookup key not found", "LookupKey", lookupKey)
		return nil, nil
	}

	log.Info("Found lookup value", "LookupKey", lookupKey, "Value", val)
	return val, nil
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



func ProcessArguments(inputs abi.Arguments, args []Operand, stateDB contract.StateDB) ([]interface{}, error) {
	if len(inputs) != len(args) {
		return nil, fmt.Errorf("mismatch between expected input count (%d) and provided arguments (%d)", len(inputs), len(args))
	}

	packedArgs := make([]interface{}, len(args))
	for i, input := range inputs {
		arg := args[i]

		// Step 1: Load value from state or direct value
		argValue, err := getLookupValue(arg, stateDB)
		if err != nil {
			log.Info("Failed fetching argument value", "ArgIndex", i, "Error", err)
			return nil, fmt.Errorf("failed to fetch argument value from lookup storage: %w", err)
		}

		// Step 2: Get the ABI type
		abiType, err := abi.NewType(input.Type.String(), "", nil)
		if err != nil {
			log.Info("Failed creating ABI type", "ArgIndex", i, "Type", input.Type.String(), "Error", err)
			return nil, fmt.Errorf("failed to create ABI type: %w", err)
		}

		// Step 3: Convert to ABI-compatible value
		convertedValue, err := convertToABIType(argValue, abiType)
		if err != nil {
			log.Info("Failed converting value to ABI type", "ArgIndex", i, "RawValue", argValue, "ExpectedType", abiType.String(), "Error", err)
			return nil, fmt.Errorf("failed to convert value for argument %d: %w", i, err)
		}

		log.Info("Processed argument", "ArgIndex", i, "RawValue", argValue, "ConvertedValue", convertedValue, "ExpectedType", input.Type.String())
		packedArgs[i] = convertedValue
	}

	return packedArgs, nil
}

func getContractAddress(contract Operand, stateDB contract.StateDB) (common.Address, error) {
	addrValue, err := getLookupValue(contract, stateDB)
	if err != nil {
		log.Info("Failed fetching contract address", "error", err)
		return common.Address{}, fmt.Errorf("failed to fetch contract address from lookup storage: %w", err)
	}

	if addrStr, ok := addrValue.(string); ok {
		if addrStr == "" {
			log.Info("Address string is empty, returning zero address")
			return common.Address{}, nil
		}
		if common.IsHexAddress(addrStr) {
			addr := common.HexToAddress(addrStr)
			log.Info("Converted string to contract address", "address", addr.Hex())
			return addr, nil
		}
		log.Info("Invalid Ethereum address string", "input", addrStr)
		return common.Address{}, fmt.Errorf("invalid contract address string: %s", addrStr)
	}

	if addrValue == nil {
		log.Info("Lookup value is nil, returning zero address")
		return common.Address{}, nil
	}

	if addr, ok := addrValue.(common.Address); ok {
		log.Info("Retrieved contract address", "address", addr.Hex())
		return addr, nil
	}

	log.Info("Invalid contract address type", "value", addrValue)
	return common.Address{}, fmt.Errorf("invalid contract address type: %T", addrValue)
}

func getPCFromState(stateDB contract.StateDB, addr common.Address) (*big.Int, error) {
	currentPCBytes := stateDB.GetState(addr, pcKey)
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
	stateDB.SetState(addr, pcKey, valueToSave)
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

// deletePlanFromState removes all chunks of data stored under the given key in the state.
func deletePlanFromState(stateDB contract.StateDB, addr common.Address, key common.Hash) {
	log.Info("Deleting plan from state", "key", key.Hex())

	for i := 0; ; i++ {
		chunkKey := common.BytesToHash(append(key.Bytes(), byte(i)))
		chunk := stateDB.GetState(addr, chunkKey)
		if chunk == (common.Hash{}) {
			log.Info("No more chunks found during deletion", "lastIndex", i)
			break
		}

		stateDB.SetState(addr, chunkKey, common.Hash{})
		log.Info("Deleted chunk", "index", i, "chunkKey", chunkKey.Hex())
	}

	log.Info("Completed deletion of all chunks", "key", key.Hex())
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

// setLargeState stores [data] and includes its total length as an 8-byte prefix. TODO: remove extra byes from previous
func setLargeState(stateDB contract.StateDB, addr common.Address, key common.Hash, data []byte) {

    // 1) Write length + data
    // The first chunk stores the length in the first 8 bytes
    // The rest is data chunking
    totalLen := uint64(len(data))
    prefix := make([]byte, 8)
    binary.BigEndian.PutUint64(prefix, totalLen) // 8-byte length prefix

    fullData := append(prefix, data...) // Combine length prefix + actual data

    // 2) Store [fullData] in 32-byte chunks
    chunkSize := common.HashLength // 32
    chunks := (len(fullData) + chunkSize - 1) / chunkSize
    for i := 0; i < chunks; i++ {
        start := i * chunkSize
        end := start + chunkSize
        if end > len(fullData) {
            end = len(fullData)
        }

        // Pad the chunk to 32 bytes if needed
        chunkData := make([]byte, chunkSize)
        copy(chunkData, fullData[start:end])

        chunkKey := common.BytesToHash(append(key.Bytes(), byte(i)))
        stateDB.SetState(addr, chunkKey, common.BytesToHash(chunkData))
    }
}

func getLargeState(stateDB contract.StateDB, addr common.Address, key common.Hash) ([]byte, error) {
    // 1) Read the first chunk for length prefix
    firstChunkKey := common.BytesToHash(append(key.Bytes(), byte(0)))
    firstChunk := stateDB.GetState(addr, firstChunkKey).Bytes()
    if len(firstChunk) == 0 {
        // No data at all
        // return nil, fmt.Errorf("no data found for key %s", key.Hex())
        return []byte{}, nil
    }

    // The first 8 bytes store the total length
    if len(firstChunk) < 8 {
        return nil, fmt.Errorf("invalid length prefix in first chunk")
    }
    totalLen := binary.BigEndian.Uint64(firstChunk[:8])

    // Full data includes the first chunk's leftover part after length prefix
    data := append([]byte(nil), firstChunk[8:]...) // Copy remainder after length
    chunkIndex := 1

    // 2) Read subsequent chunks until we collect totalLen
    bytesNeeded := int(totalLen) - len(data)
    for bytesNeeded > 0 {
        chunkKey := common.BytesToHash(append(key.Bytes(), byte(chunkIndex)))
        chunk := stateDB.GetState(addr, chunkKey).Bytes()
        if len(chunk) == 0 {
            // Means no more data is stored
            break
        }
        data = append(data, chunk...)
        bytesNeeded = int(totalLen) - len(data)
        chunkIndex++
    }

    // 3) If data is longer than totalLen, truncate
    if len(data) > int(totalLen) {
        data = data[:totalLen]
    }

    // If data is still less than totalLen, user stored incomplete data
    if len(data) < int(totalLen) {
        return nil, fmt.Errorf("incomplete data retrieved for key %s: expected %d bytes, got %d",
            key.Hex(), totalLen, len(data))
    }

    return data, nil
}

