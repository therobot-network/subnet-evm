package llmprecompile

import (
	"fmt"

	"github.com/ava-labs/subnet-evm/precompile/contract"
	"github.com/ava-labs/subnet-evm/vmerrs"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"
)

func publishPrimitive(accessibleState contract.AccessibleState, caller common.Address, addr common.Address, input []byte, suppliedGas uint64, readOnly bool) (ret []byte, remainingGas uint64, err error) {
if remainingGas, err = contract.DeductGas(suppliedGas, PublishPrimitiveGasCost); err != nil {
	log.Info("Insufficient gas supplied", "Error", err)
	return nil, 0, err
}
if readOnly {
	return nil, remainingGas, vmerrs.ErrWriteProtection
}

	stateDB := accessibleState.GetStateDB()

	inputStruct, err := UnpackPublishPrimitiveInput(input)
	if err != nil {
		log.Info("Failed to unpack publishPrimitive input", "Error", err)
		return nil, remainingGas, err
	}

	contractAddress := inputStruct.ContractAddress
	metadata := inputStruct.PrimitiveName

	if err := updatePlanLocalState(stateDB, addr, metadata, contractAddress); err != nil {
		log.Info("Failed to store permanent lookup entry", "Key", metadata, "Error", err)
		return nil, remainingGas, fmt.Errorf("failed to permanent store lookup entry for key %s: %w", metadata, err)
	}

	log.Info("Stored permanent lookup entry", "Key", metadata, "Address", contractAddress.Hex())

	metadataHash := common.BytesToHash([]byte(metadata))
	fullKey := crypto.Keccak256Hash(append(lookupStorageKey.Bytes(), metadataHash.Bytes()...))

	existingValue, err := getLargeState(stateDB, addr, fullKey)
	if err != nil {
		log.Info("Failed to retrieve metadata key from state", "Error", err)
		return nil, remainingGas, err
	}
	if len(existingValue) > 0 {
		log.Info("Metadata key already exists in state", "Metadata", metadata)
		return nil, remainingGas, fmt.Errorf("util name already registered")
	}

	setLargeState(stateDB, addr, fullKey, contractAddress.Bytes())
	log.Info("Stored mapping metadata -> address", "Metadata", metadata, "ContractAddress", contractAddress.Hex())

	addressHash := common.BytesToHash([]byte(contractAddress.Hex()))
	fullKey = crypto.Keccak256Hash(append(addressToPrimitiveName.Bytes(), addressHash.Bytes()...))
	metadataBytes := []byte(metadata)
	setLargeState(stateDB, addr, fullKey, metadataBytes)

	log.Info("Stored mapping address -> metadata", "Metadata", metadata, "ContractAddress", contractAddress.Hex())

	contractAddressHash := common.BytesToHash(contractAddress.Bytes())
	fullKey = crypto.Keccak256Hash(append(addressToPrimitiveName.Bytes(), contractAddressHash.Bytes()...))
	setLargeState(stateDB, addr, fullKey, metadataBytes)

	log.Info("Stored primitive mapping",
		"Name", string(metadataBytes),
		"ContractAddress", contractAddress.Hex(),
		"ContractAddressHash", contractAddressHash.Hex(),
		"FullKey", fullKey.Hex(),
		"StorageAddr", addr.Hex(),
	)

	return []byte{}, remainingGas, nil
}

func publishRobotContract(accessibleState contract.AccessibleState, caller common.Address, addr common.Address, input []byte, suppliedGas uint64, readOnly bool) (ret []byte, remainingGas uint64, err error) {
	// log.Info("Made it to publishRobotContract function", "Caller", caller.Hex(), "ContractAddress", addr.Hex())

	if remainingGas, err = contract.DeductGas(suppliedGas, PublishRobotContractGasCost); err != nil {
		log.Info("Insufficient gas supplied", "Error", err)
		return nil, 0, err
	}
	if readOnly {
		return nil, remainingGas, vmerrs.ErrWriteProtection
	}

	inputStruct, err := UnpackPublishRobotContractInput(input)
	if err != nil {
		log.Info("Failed to unpack PublishRobotContractInput input", "Error", err)
		return nil, remainingGas, err
	}

	stateDB := accessibleState.GetStateDB()

	contractAddress := inputStruct.ContractAddress
	primitiveAddress := inputStruct.PrimitiveAddress

	primitiveAddressHash := common.BytesToHash([]byte(primitiveAddress.Hex()))
	fullKey := crypto.Keccak256Hash(append(addressToPrimitiveName.Bytes(), primitiveAddressHash.Bytes()...))

	storedName, err := getLargeState(stateDB, addr, fullKey)
	if err != nil {
		log.Info("Failed to retrieve primitive name from state", "Error", err)
		return nil, remainingGas, err
	}

	if len(storedName) == 0 {
		log.Info("Primitive name not found for address", "Address", primitiveAddress.Hex())
		return nil, remainingGas, fmt.Errorf("primitive name not found for address %s", primitiveAddress.Hex())
	}

	contractAddressHash := common.BytesToHash(contractAddress.Bytes())
	fullKey = crypto.Keccak256Hash(append(addressToPrimitiveName.Bytes(), contractAddressHash.Bytes()...))

	setLargeState(stateDB, addr, fullKey, storedName)

	log.Info("Stored primitive mapping",
		"Name", string(storedName),
		"ContractAddress", contractAddress.Hex(),
		"ContractAddressHash", contractAddressHash.Hex(),
		"FullKey", fullKey.Hex(),
		"StorageAddr", addr.Hex(),
	)

	return []byte{}, remainingGas, nil
}


func publishSystemPrimitive(accessibleState contract.AccessibleState, caller common.Address, addr common.Address, input []byte, suppliedGas uint64, readOnly bool) (ret []byte, remainingGas uint64, err error) {
if remainingGas, err = contract.DeductGas(suppliedGas, PublishPrimitiveGasCost); err != nil {
	log.Info("Insufficient gas supplied", "Error", err)
	return nil, 0, err
}
if readOnly {
	return nil, remainingGas, vmerrs.ErrWriteProtection
}

	stateDB := accessibleState.GetStateDB()

	inputStruct, err := UnpackPublishSystemPrimitiveInput(input)
	if err != nil {
		log.Info("Failed to unpack publishPrimitive input", "Error", err)
		return nil, remainingGas, err
	}

	contractAddress := inputStruct.ContractAddress
	metadata := inputStruct.Name

	if err := updatePlanLocalState(stateDB, addr, metadata, contractAddress); err != nil {
		log.Info("Failed to store permanent lookup entry", "Key", metadata, "Error", err)
		return nil, remainingGas, fmt.Errorf("failed to permanent store lookup entry for key %s: %w", metadata, err)
	}

	log.Info("Stored permanent lookup entry", "Key", metadata, "Address", contractAddress.Hex())

	metadataHash := common.BytesToHash([]byte(metadata))
	fullKey := crypto.Keccak256Hash(append(lookupStorageKey.Bytes(), metadataHash.Bytes()...))

	existingValue, err := getLargeState(stateDB, addr, fullKey)
	if err != nil {
		log.Info("Failed to retrieve metadata key from state", "Error", err)
		return nil, remainingGas, err
	}
	if len(existingValue) > 0 {
		log.Info("Metadata key already exists in state", "Metadata", metadata)
		return nil, remainingGas, fmt.Errorf("util name already registered")
	}

	setLargeState(stateDB, addr, fullKey, contractAddress.Bytes())
	log.Info("Stored mapping metadata -> address", "Metadata", metadata, "ContractAddress", contractAddress.Hex())

	addressHash := common.BytesToHash([]byte(contractAddress.Hex()))
	fullKey = crypto.Keccak256Hash(append(addressToPrimitiveName.Bytes(), addressHash.Bytes()...))
	metadataBytes := []byte(metadata)
	setLargeState(stateDB, addr, fullKey, metadataBytes)

	log.Info("Stored mapping address -> metadata", "Metadata", metadata, "ContractAddress", contractAddress.Hex())

	contractAddressHash := common.BytesToHash(contractAddress.Bytes())
	fullKey = crypto.Keccak256Hash(append(addressToPrimitiveName.Bytes(), contractAddressHash.Bytes()...))
	setLargeState(stateDB, addr, fullKey, metadataBytes)

	log.Info("Stored primitive mapping",
		"Name", string(metadataBytes),
		"ContractAddress", contractAddress.Hex(),
		"ContractAddressHash", contractAddressHash.Hex(),
		"FullKey", fullKey.Hex(),
		"StorageAddr", addr.Hex(),
	)

	return []byte{}, remainingGas, nil
}