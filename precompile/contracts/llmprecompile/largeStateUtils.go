package llmprecompile

import (
	"encoding/binary"
	"fmt"

	"github.com/ava-labs/subnet-evm/precompile/contract"
	"github.com/ethereum/go-ethereum/common"
)

// setLargeState stores [data] and includes its total length as an 8-byte prefix. TODO: remove extra byes from previous
func setLargeState(stateDB contract.StateDB, addr common.Address, key common.Hash, data []byte) error{

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
	return nil
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

func deleteLargeState(stateDB contract.StateDB, addr common.Address, key common.Hash) {
	// 1) Read the first chunk for length prefix
	firstChunkKey := common.BytesToHash(append(key.Bytes(), byte(0)))
	firstChunk := stateDB.GetState(addr, firstChunkKey).Bytes()
	if len(firstChunk) == 0 {
		// No data at all
		return
	}

	// The first 8 bytes store the total length
	if len(firstChunk) < 8 {
		return
	}
	totalLen := binary.BigEndian.Uint64(firstChunk[:8])

	// 2) Delete all chunks
	chunkIndex := 0
	for uint64(chunkIndex)*common.HashLength < totalLen {
		chunkKey := common.BytesToHash(append(key.Bytes(), byte(chunkIndex)))
		stateDB.SetState(addr, chunkKey, common.Hash{}) // Delete the chunk
		chunkIndex++
	}

	// 3) Delete the first chunk (length prefix)
	stateDB.SetState(addr, firstChunkKey, common.Hash{})
}