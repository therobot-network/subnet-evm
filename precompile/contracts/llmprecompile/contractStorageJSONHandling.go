package llmprecompile

import (
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/big"
	"reflect"
	"strings"

	"github.com/ava-labs/subnet-evm/accounts/abi"
	"github.com/ava-labs/subnet-evm/precompile/contract"

	_ "embed"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

func getLookupValue(arg Arg, stateDB contract.StateDB) (interface{}, error) {
    // If Value is provided directly, use it
    if arg.Value != "" {
        return arg.Value, nil
    }

    // If there's no lookup key, return nil
    if arg.Lookup == "" {
        return nil, nil
    }

    // Load the full lookup JSON object from state
    lookupData, err := getLargeState(stateDB, ContractAddress, lookupStorageKey)
    if err != nil {
        log.Printf("Error retrieving lookup state: %v", err)
        return nil, fmt.Errorf("failed to retrieve lookup storage: %w", err)
    }

    // If nothing is stored, return nil
    if len(lookupData) == 0 {
        log.Printf("Lookup state is empty for key: %s", arg.Lookup)
        return nil, nil
    }

    // Parse JSON into a generic map
    var lookupMap map[string]interface{}
    if err := json.Unmarshal(lookupData, &lookupMap); err != nil {
        log.Printf("Error decoding lookup JSON: %v", err)
        return nil, fmt.Errorf("failed to decode lookup JSON: %w", err)
    }

    // Extract the specific value
    val, exists := lookupMap[arg.Lookup]
    if !exists {
        log.Printf("Lookup key not found: %s", arg.Lookup)
        return nil, nil
    }

    log.Printf("Found lookup value | Key=%s | Value=%+v", arg.Lookup, val)
    return val, nil
}

// Dynamically converts a string value to the expected ABI type
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
        // dynamic bytes
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

        // Reconstruct properly typed slice
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
            return result, nil // For tuple[], pass as []interface{}
        default:
            return result, nil // Generic fallback
        }

    // case abi.TupleTy:
    //     if valueMap, ok := value.(map[string]interface{}); ok {
    //         // Tuple from map[string]interface{} (e.g., JSON decoding)
    //         result := make([]interface{}, len(abiType.TupleElems))
    //         for i, elem := range abiType.TupleElems {
    //             val, exists := valueMap[elem.Name]
    //             if !exists {
    //                 return nil, fmt.Errorf("tuple field %q missing", elem.Name)
    //             }
    //             converted, err := convertToABIType(val, *elem)
    //             if err != nil {
    //                 return nil, fmt.Errorf("failed to convert tuple field %q: %w", elem.Name, err)
    //             }
    //             result[i] = converted
    //         }
    //         return result, nil
    //     }

    //     if valueSlice, ok := value.([]interface{}); ok {
    //         if len(valueSlice) != len(abiType.TupleElems) {
    //             return nil, fmt.Errorf("tuple length mismatch: expected %d, got %d", len(abiType.TupleElems), len(valueSlice))
    //         }

    //         result := make([]interface{}, len(valueSlice))
    //         for i := range valueSlice {
    //             converted, err := convertToABIType(valueSlice[i], *abiType.TupleElems[i])
    //             if err != nil {
    //                 return nil, fmt.Errorf("failed to convert tuple element %d: %w", i, err)
    //             }
    //             result[i] = converted
    //         }
    //         return result, nil
    //     }

    //     return nil, fmt.Errorf("unsupported tuple format: %T", value)

    default:
        return nil, fmt.Errorf("unsupported ABI type: %s", abiType.String())
    }
}


func ProcessArguments(inputs abi.Arguments, args []Arg, stateDB contract.StateDB) ([]interface{}, error) {
    if len(inputs) != len(args) {
        return nil, fmt.Errorf("mismatch between expected input count (%d) and provided arguments (%d)", len(inputs), len(args))
    }

    packedArgs := make([]interface{}, len(args))
    for i, input := range inputs {
        arg := args[i]

        // Step 1: Load value from state or direct value
        argValue, err := getLookupValue(arg, stateDB)
        if err != nil {
            log.Printf("Failed fetching argument value: %v", err)
            return nil, fmt.Errorf("failed to fetch argument value from lookup storage: %w", err)
        }

        // Step 2: Get the ABI type
        abiType, err := abi.NewType(input.Type.String(), "", nil)
        if err != nil {
            return nil, fmt.Errorf("failed to create ABI type: %w", err)
        }

        // Step 3: Convert to ABI-compatible value
        convertedValue, err := convertToABIType(argValue, abiType)
        if err != nil {
            return nil, fmt.Errorf("failed to convert value for argument %d: %w", i, err)
        }

        log.Printf("Processed Arg[%d]: RawValue=%v, ConvertedValue=%v, ExpectedType=%s", i, argValue, convertedValue, input.Type.String())
        packedArgs[i] = convertedValue
    }

    return packedArgs, nil
}

func getContractAddress(contract Arg, stateDB contract.StateDB) (common.Address, error) {
    // Retrieve contract address using ABI decoding
    addrValue, err := getLookupValue(contract, stateDB)
    if err != nil {
        log.Printf("Failed fetching contract address: %v", err)
        return common.Address{}, fmt.Errorf("failed to fetch contract address from lookup storage: %w", err)
    }

    // If getLookupValue returns nil, return the zero address
    if addrValue == nil {
        log.Printf("Lookup value is nil, returning zero address.")
        return common.Address{}, nil
    }

    // If the value is already a common.Address, return it
    if addr, ok := addrValue.(common.Address); ok {
        log.Printf("Successfully retrieved contract address: %s", addr.Hex())
        return addr, nil
    }

    // If the value is a string, attempt to convert it to common.Address
    if addrStr, ok := addrValue.(string); ok {
        if common.IsHexAddress(addrStr) {
            addr := common.HexToAddress(addrStr)
            log.Printf("Successfully converted string to contract address: %s", addr.Hex())
            return addr, nil
        }
        log.Printf("Error: Retrieved string is not a valid Ethereum address: %s", addrStr)
        return common.Address{}, fmt.Errorf("invalid contract address string: %s", addrStr)
    }

    log.Printf("Error: Retrieved value is not a valid Ethereum address: %v", addrValue)
    return common.Address{}, fmt.Errorf("invalid contract address type: %T", addrValue)
}

// Utility function to retrieve the program counter
func getPCFromState(stateDB contract.StateDB, addr common.Address) (*big.Int, error) {
    currentPCBytes := stateDB.GetState(addr, pcKey)
    if currentPCBytes == (common.Hash{}) {
        return nil, errors.New("program counter not initialized")
    }

    // Converting value from 1 to 0
    // savePCToState stores 1 when pc value is 0
    if currentPCBytes == (common.Hash{1}) {
        return big.NewInt(0), nil
    }

    currentPC := new(big.Int).SetBytes(currentPCBytes.Bytes())
    return currentPC, nil
}

// Utility function to save the program counter to state
func savePCToState(stateDB contract.StateDB, addr common.Address, pc *big.Int) {
    // Convert the big.Int value to a padded byte array
    valueToSave := common.BytesToHash(pc.Bytes())
    // using 1 if counter value is 0, getPCFromState throws "program counter not initialized"
    // error when value is 0
    if pc.Sign() == 0 { // Check if pc == 0
        valueToSave = common.Hash{1} // Use a unique marker for 0
    }
    stateDB.SetState(addr, pcKey, valueToSave)
}

// Temporary function. Later we will use a DB
func getContractPrimitive(stateDB contract.StateDB, addr common.Address, address string) (contract string, primitive string) {
    log.Printf("Fetching contract primitive for address: %s", address)

    // Compute the key for retrieving the contract address
    parsedAddress := common.HexToAddress(address)
    addressHash := common.BytesToHash(parsedAddress.Bytes())
    fullKey := crypto.Keccak256Hash(append(addressToPrimitiveName.Bytes(), addressHash.Bytes()...))

    log.Printf(
        "Attempting to fetch primitive: Address=%s, AddrForStorage=%s, addressHash=%s, fullKey=%s",
        address,
        addr.Hex(),
        addressHash.Hex(),
        fullKey.Hex(),
    )
    

    // Retrieve the contract from storage
    contractBytes, err := getLargeState(stateDB, addr, fullKey)
    if err != nil {
        log.Printf("Error retrieving contract from state: %v", err)
        return "", ""
    }
    if len(contractBytes) == 0 {
        log.Printf("No contract found for address: %s", address)
        return "", "" // Return empty values instead of an error
    }

    contract = string(contractBytes)
    log.Printf("Found contract: %s for address: %s", contract, address)

    // Retrieve the primitive associated with this contract
    primitive, exists := primitiveABI[contract]
    if !exists {
        log.Printf("No primitive found for contract: %s", contract)
        return contract, "" // Return contract but empty primitive
    }

    log.Printf("Found primitive: %.150s for contract: %s", primitive, contract)
    return contract, primitive
}

func updatePlanLocalState(stateDB contract.StateDB, addr common.Address, key string, storageData interface{}) error {
    if key == "" {
        return nil
    }

    // Step 1: Retrieve the current JSON map from storage
    raw, err := getLargeState(stateDB, addr, lookupStorageKey)
    if err != nil {
        return fmt.Errorf("failed to get existing state: %w", err)
    }

    // Step 2: Decode JSON into a map[string]interface{}
    var jsonMap map[string]interface{}
    if len(raw) > 0 {
        if err := json.Unmarshal(raw, &jsonMap); err != nil {
            return fmt.Errorf("failed to parse existing JSON state: %w", err)
        }
    } else {
        jsonMap = make(map[string]interface{})
    }

    // Step 3: Insert or update the value
    jsonMap[key] = storageData

    // Step 4: Encode back to JSON
    updated, err := json.Marshal(jsonMap)
    if err != nil {
        return fmt.Errorf("failed to marshal updated JSON: %w", err)
    }

    // Step 5: Store in state
    setLargeState(stateDB, addr, lookupStorageKey, updated)
    log.Printf("Updated plan-local JSON | Key=%s | Value=%v", key, storageData)

    snapshot := string(updated)
    if len(snapshot) > 1000 {
        snapshot = snapshot[:1000] + "...[truncated]"
    }
    log.Printf("Current plan-local state snapshot (capped): %s", snapshot)

    return nil
}

// deletePlanFromState removes all chunks of data stored under the given key in the state.
func deletePlanFromState(stateDB contract.StateDB, addr common.Address, key common.Hash) {
    log.Printf("Deleting plan from state with key: %s", key.Hex())

    for i := 0; ; i++ {
        chunkKey := common.BytesToHash(append(key.Bytes(), byte(i)))
        chunk := stateDB.GetState(addr, chunkKey)
        if chunk == (common.Hash{}) {
            // Stop if no more chunks are found
            log.Printf("No more chunks found after index %d. Deletion complete.", i)
            break
        }

        // Clear the chunk by setting it to an empty hash
        stateDB.SetState(addr, chunkKey, common.Hash{})
        log.Printf("Deleted chunk %d with key: %s", i, chunkKey.Hex())
    }

    log.Printf("All chunks under key %s have been deleted.", key.Hex())
}

func storeLookupEntries(stateDB contract.StateDB, addr common.Address, lookupJsonString string) (map[string]interface{}, error) {
    if lookupJsonString == "" {
        log.Printf("No lookup entries")
        return map[string]interface{}{}, nil
    }

    // Step 1: Unmarshal the JSON into a generic map
    var lookupMap map[string]interface{}
    if err := json.Unmarshal([]byte(lookupJsonString), &lookupMap); err != nil {
        return nil, fmt.Errorf("failed to unmarshal lookup JSON: %w", err)
    }

    // Step 2: Iterate and store each key/value pair
    for key, val := range lookupMap {
        if err := updatePlanLocalState(stateDB, addr, key, val); err != nil {
            log.Printf("Error: Failed to store lookup entry for key %s. Error: %v", key, err)
            return nil, fmt.Errorf("failed to store lookup entry for key %s: %w", key, err)
        }

        log.Printf("Successfully stored lookup entry: Key=%s, Value=%v", key, val)
    }

    return lookupMap, nil
}

// GetPromptCounter retrieves the current value of the promptCounter from the StateDB.
func GetPromptCounter(stateDB contract.StateDB) *big.Int {
	value := stateDB.GetState(ContractAddress, promptCounterKey)
	if value == (common.Hash{}) {
		// If not found, initialize to 1
		return big.NewInt(0)
	}
	return new(big.Int).SetBytes(value.Bytes())
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

