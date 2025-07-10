package llmprecompile

import (
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
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
)

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

// removeItemLocalState removes a key from the plan-local state JSON object stored in lookupStorageKey.
func removeItemLocalState(stateDB contract.StateDB, addr common.Address, key string) error {
	if key == "" {
		return nil
	}

	raw, err := getLargeState(stateDB, addr, lookupStorageKey)
	if err != nil {
		return fmt.Errorf("failed to get existing state: %w", err)
	}

	if len(raw) == 0 {
		// Nothing to remove
		return nil
	}

	var jsonMap map[string]interface{}
	if err := json.Unmarshal(raw, &jsonMap); err != nil {
		return fmt.Errorf("failed to parse existing JSON state: %w", err)
	}

	if _, exists := jsonMap[key]; !exists {
		// Key not present, nothing to do
		return nil
	}
	delete(jsonMap, key)

	updated, err := json.Marshal(jsonMap)
	if err != nil {
		return fmt.Errorf("failed to marshal updated JSON: %w", err)
	}

	setLargeState(stateDB, addr, lookupStorageKey, updated)
	log.Info("Removed item from plan-local state", "key", key)

	return nil
}

// setLargeState stores [data] and includes its total length as an 8-byte prefix. It also removes any leftover chunks from previous larger values.
func setLargeState(stateDB contract.StateDB, addr common.Address, key common.Hash, data []byte) {
	// 1) Write length + data
	totalLen := uint64(len(data))
	prefix := make([]byte, 8)
	binary.BigEndian.PutUint64(prefix, totalLen)
	fullData := append(prefix, data...)

	chunkSize := common.HashLength // 32
	chunks := (len(fullData) + chunkSize - 1) / chunkSize

	// 2) Store [fullData] in 32-byte chunks
	for i := 0; i < chunks; i++ {
		start := i * chunkSize
		end := start + chunkSize
		if end > len(fullData) {
			end = len(fullData)
		}
		chunkData := make([]byte, chunkSize)
		copy(chunkData, fullData[start:end])
		chunkKey := common.BytesToHash(append(key.Bytes(), byte(i)))
		stateDB.SetState(addr, chunkKey, common.BytesToHash(chunkData))
	}

	// 3) Remove any leftover chunks from previous larger values
	// Read the previous length (if any) to determine if cleanup is needed
	firstChunkKey := common.BytesToHash(append(key.Bytes(), byte(0)))
	firstChunk := stateDB.GetState(addr, firstChunkKey).Bytes()
	var prevTotalLen uint64
	if len(firstChunk) >= 8 {
		prevTotalLen = binary.BigEndian.Uint64(firstChunk[:8])
	}
	prevChunks := int((prevTotalLen + 8 + uint64(chunkSize) - 1) / uint64(chunkSize))
	for i := chunks; i < prevChunks; i++ {
		chunkKey := common.BytesToHash(append(key.Bytes(), byte(i)))
		stateDB.SetState(addr, chunkKey, common.Hash{}) // zero out leftover chunk
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
