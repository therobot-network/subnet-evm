package llmprecompile

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"math/big"
	"net/http"
	"strings"
	"time"

	"github.com/ava-labs/subnet-evm/accounts/abi"
	"github.com/ava-labs/subnet-evm/precompile/contract"
	"github.com/ava-labs/subnet-evm/vmerrs"

	_ "embed"
	"reflect"

	"github.com/ethereum/go-ethereum/common"
)

const (
	// Gas costs for each function. These are set to 1 by default.
	// You should set a gas cost for each function in your contract.
	// Generally, you should not set gas costs very low as this may cause your network to be vulnerable to DoS attacks.
	// There are some predefined gas costs in contract/utils.go that you can use.
	ContinueEvaluationGasCost     uint64 = 1 /* SET A GAS COST HERE */
	EvaluatePlanGasCost           uint64 = 1 /* SET A GAS COST HERE */
	EvaluatePromptGasCost         uint64 = 1 /* SET A GAS COST HERE */
	PublishCustomPrimitiveGasCost uint64 = 1 /* SET A GAS COST HERE */
	PublishPrimitiveGasCost       uint64 = 1 /* SET A GAS COST HERE */
)

// CUSTOM CODE STARTS HERE
// Reference imports to suppress errors from unused imports. This code and any unnecessary imports can be removed.
var (
	_ = abi.JSON
	_ = errors.New
	_ = big.NewInt
	_ = vmerrs.ErrOutOfGas
	_ = common.Big0
)


type Arg struct {
	Value  string `json:"Value"`
	Lookup string    `json:"Lookup"`
    AbiType string `json:"AbiType"`
}

type Step struct {
	Method    string   `json:"Method,omitempty"`
	Contract  Arg      `json:"Contract,omitempty"`
	Args      []Arg    `json:"Args,omitempty"`
    Output    []string   `json:"Output,omitempty"`
}

func HTTPPostJSON(url string, requestBody interface{}) ([]byte, error) {
    reqBytes, err := json.Marshal(requestBody)
    if err != nil {
        return nil, fmt.Errorf("failed to marshal request body: %w", err)
    }
    log.Printf("Sending HTTP POST to %s with JSON body: %s", url, string(reqBytes))

    client := &http.Client{
        Timeout: 60 * time.Second,
    }

    req, err := http.NewRequest("POST", url, bytes.NewBuffer(reqBytes))
    if err != nil {
        return nil, fmt.Errorf("failed to create HTTP request: %w", err)
    }
    req.Header.Set("Content-Type", "application/json")

    resp, err := client.Do(req)
    if err != nil {
        return nil, fmt.Errorf("HTTP request failed: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        body, _ := ioutil.ReadAll(resp.Body)
        log.Printf("Non-200 response: %d, body: %s", resp.StatusCode, body)
        return nil, fmt.Errorf("HTTP request returned status %d", resp.StatusCode)
    }

    respBytes, err := ioutil.ReadAll(resp.Body)
    if err != nil {
        return nil, fmt.Errorf("failed to read HTTP response: %w", err)
    }

    log.Printf("respBytes: %s", respBytes)
    return respBytes, nil
}

func getLookupValue(arg Arg, stateDB contract.StateDB) (interface{}, error) {
    // If Value is provided, return it directly
    if arg.Value != "" {
        return arg.Value, nil
    }

    if arg.Lookup == "" {
        return nil, nil
    }

    // Convert lookup key to a hash
    lookupKey := common.BytesToHash([]byte(arg.Lookup))
    
    // Retrieve ABI-encoded data from storage
    lookupData, err := getLargeState(stateDB, ContractAddress, lookupKey)
    if err != nil {
        log.Printf(
            "Error: Failed to retrieve data from state | Key=%s | HashedKey=%s | Error=%v", 
            arg.Lookup, lookupKey.Hex(), err,
        )
        return nil, fmt.Errorf("failed to retrieve data from state for key %s (%s): %w", arg.Lookup, lookupKey.Hex(), err)
    }

    log.Printf(
        "Retrieved ABI-encoded data | Key=%s | HashedKey=%s | Data=%x",
        arg.Lookup, lookupKey.Hex(), lookupData,
    )

    // Define ABI structure for decoding
    abiType, err := abi.NewType(arg.AbiType, "", nil)
    if err != nil {
        log.Printf(
            "Error: Failed to create ABI type | Key=%s | HashedKey=%s | ABIType=%s | Error=%v",
            arg.Lookup, lookupKey.Hex(), arg.AbiType, err,
        )
        return nil, fmt.Errorf("failed to create ABI type: %w", err)
    }

    abiArgs := abi.Arguments{{Type: abiType}}

    // Handle Dynamic Arrays Explicitly
    var decodedValues []interface{}

        decodedValues, err = abiArgs.Unpack(lookupData)
        if err != nil {
            log.Printf(
                "Error: ABI decoding failed | Key=%s | HashedKey=%s | ABIType=%s | Error=%v",
                arg.Lookup, lookupKey.Hex(), arg.AbiType, err,
            )
            return nil, fmt.Errorf("ABI decoding failed for key %s (%s): %w", arg.Lookup, lookupKey.Hex(), err)
        }
        log.Printf(
            "Decoded ABI value | Key=%s | HashedKey=%s | DecodedValue=%+v",
            arg.Lookup, lookupKey.Hex(), decodedValues,
        )
        return decodedValues[0], nil
}

func decodeDynamicArray(data []byte, elementType string) ([]interface{}, error) {
    if len(data) < 32 {
        return nil, fmt.Errorf("invalid ABI data length for array decoding")
    }

    // Read the length of the array (first 32 bytes)
    arrayLength := new(big.Int).SetBytes(data[:32]).Int64()
    if arrayLength < 0 {
        return nil, fmt.Errorf("negative array length in ABI encoding")
    }

    log.Printf("Decoding dynamic array: elementType=%s, length=%d", elementType, arrayLength)

    // Slice to hold the decoded values
    values := make([]interface{}, arrayLength)

    // Start decoding from index 32
    offset := 32
    elementSize := 32 // Each ABI-encoded element is 32 bytes

    for i := int64(0); i < arrayLength; i++ {
        if offset+elementSize > len(data) {
            return nil, fmt.Errorf("ABI data length mismatch for array decoding")
        }

        rawValue := data[offset : offset+elementSize]

        switch elementType {
        case "uint256":
            values[i] = new(big.Int).SetBytes(rawValue)
        case "bool":
            values[i] = rawValue[31] != 0 // Last byte determines boolean value
        case "address":
            values[i] = common.BytesToAddress(rawValue)
        case "string":
            return nil, fmt.Errorf("string[] not supported in direct ABI decoding")
        default:
            return nil, fmt.Errorf("unsupported element type for ABI array: %s", elementType)
        }

        offset += elementSize
    }

    return values, nil
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
        case string:
            bigIntValue, success := new(big.Int).SetString(v, 10)
            if !success {
                return nil, fmt.Errorf("invalid integer value: %s", v)
            }
            return bigIntValue, nil
        default:
            return nil, fmt.Errorf("expected integer-compatible type, got %T", value)
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
            return nil, fmt.Errorf("invalid boolean value: %s", v)
        default:
            return nil, fmt.Errorf("expected boolean-compatible type, got %T", value)
        }

    case abi.StringTy:
        strValue, ok := value.(string)
        if !ok {
            return nil, fmt.Errorf("expected string, got %T", value)
        }
        return strValue, nil

    case abi.SliceTy, abi.ArrayTy:
        // Handle dynamic and static arrays
        slice := reflect.ValueOf(value)
        if slice.Kind() != reflect.Slice {
            return nil, fmt.Errorf("expected array type, got %T", value)
        }

        convertedSlice := reflect.MakeSlice(reflect.SliceOf(reflect.TypeOf(abiType.Elem)), slice.Len(), slice.Len())
        for i := 0; i < slice.Len(); i++ {
            convertedValue, err := convertToABIType(slice.Index(i).Interface(), *abiType.Elem)
            if err != nil {
                return nil, fmt.Errorf("failed to convert array element %d: %w", i, err)
            }
            convertedSlice.Index(i).Set(reflect.ValueOf(convertedValue))
        }
        return convertedSlice.Interface(), nil

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
        arg.AbiType = input.Type.String() // Ensure correct ABI type

        // Retrieve ABI-decoded value or raw string from state
        argValue, err := getLookupValue(arg, stateDB)
        if err != nil {
            log.Printf("Failed fetching argument value: %v", err)
            return nil, fmt.Errorf("failed to fetch argument value from lookup storage: %w", err)
        }

        // If argValue is a string, convert it dynamically. If arg.AbiType will be passed, we can skip this
        if strVal, ok := argValue.(string); ok {
            abiType, err := abi.NewType(arg.AbiType, "", nil)
            if err != nil {
                return nil, fmt.Errorf("failed to create ABI type: %w", err)
            }

            convertedValue, err := convertToABIType(strVal, abiType)
            if err != nil {
                return nil, fmt.Errorf("failed to convert string value: %w", err)
            }

            log.Printf("Converted string Arg[%d]: RawValue=%s, ConvertedValue=%v, ExpectedType=%s", i, strVal, convertedValue, arg.AbiType)
            packedArgs[i] = convertedValue
        } else {
            packedArgs[i] = argValue
        }

        log.Printf("Processed Arg[%d]: Value=%v, ExpectedType=%s", i, packedArgs[i], arg.AbiType)
    }

    return packedArgs, nil
}



func getContractAddress(contract Arg, stateDB contract.StateDB) (common.Address, error) {
    // Retrieve contract address using ABI decoding
    contract.AbiType = "address"
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


// Singleton StatefulPrecompiledContract and signatures.
var (

	// LLMPrecompileRawABI contains the raw ABI of LLMPrecompile contract.
	//go:embed contract.abi
	LLMPrecompileRawABI string

	LLMPrecompileABI = contract.ParseABI(LLMPrecompileRawABI)

	// Prompt Counter for identification of evaluations
	promptCounterKey = common.BytesToHash([]byte("promptCounter"))
	stepsKey = common.BytesToHash([]byte("steps"))
    pcKey    = common.BytesToHash([]byte("pc"))

	LLMPrecompilePrecompile = createLLMPrecompilePrecompile()

    llmApiURL = "http://robotbrain-v2-loadbalancer-2026683595.eu-west-1.elb.amazonaws.com/eval_prompt"
)

// ILLMContractMethodParams is an auto generated low-level Go binding around an user-defined struct.
type ILLMContractMethodParams struct {
	ContractAddress common.Address
	MethodData      []byte
}

type ContinueEvaluationInput struct {
	PromptId              *big.Int
	ContractMethodResults [][]byte
}

type ContinueEvaluationOutput struct {
	EvaluationDone       bool
	ContractMethodParams []ILLMContractMethodParams
}

type EvaluatePlanOutput struct {
	PromptId             *big.Int
	ContractMethodParams []ILLMContractMethodParams
}

type EvaluatePromptOutput struct {
	PromptId             *big.Int
	ContractMethodParams []ILLMContractMethodParams
}

type PublishCustomPrimitiveInput struct {
	ContractAddress  common.Address
	PrimitiveAddress common.Address
}

type PublishPrimitiveInput struct {
	ContractAddress common.Address
	Metadata        string
}

// UnpackContinueEvaluationInput attempts to unpack [input] as ContinueEvaluationInput
// assumes that [input] does not include selector (omits first 4 func signature bytes)
func UnpackContinueEvaluationInput(input []byte) (ContinueEvaluationInput, error) {
	inputStruct := ContinueEvaluationInput{}
	err := LLMPrecompileABI.UnpackInputIntoInterface(&inputStruct, "continueEvaluation", input, false)

	return inputStruct, err
}

// PackContinueEvaluation packs [inputStruct] of type ContinueEvaluationInput into the appropriate arguments for continueEvaluation.
func PackContinueEvaluation(inputStruct ContinueEvaluationInput) ([]byte, error) {
	return LLMPrecompileABI.Pack("continueEvaluation", inputStruct.PromptId, inputStruct.ContractMethodResults)
}

// PackContinueEvaluationOutput attempts to pack given [outputStruct] of type ContinueEvaluationOutput
// to conform the ABI outputs.
func PackContinueEvaluationOutput(outputStruct ContinueEvaluationOutput) ([]byte, error) {
	return LLMPrecompileABI.PackOutput("continueEvaluation",
		outputStruct.EvaluationDone,
		outputStruct.ContractMethodParams,
	)
}

// UnpackContinueEvaluationOutput attempts to unpack [output] as ContinueEvaluationOutput
// assumes that [output] does not include selector (omits first 4 func signature bytes)
func UnpackContinueEvaluationOutput(output []byte) (ContinueEvaluationOutput, error) {
	outputStruct := ContinueEvaluationOutput{}
	err := LLMPrecompileABI.UnpackIntoInterface(&outputStruct, "continueEvaluation", output)

	return outputStruct, err
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

// Utility function to decode results into an ordered array
func decodeResults(method abi.Method, result []byte) ([]interface{}, error) {
    log.Printf("Decoding results for method: %s", method.Name)
    log.Printf("Decoding result: %s", result)

    // Attempt to unpack the results
    decodedResults, err := method.Outputs.Unpack(result)
    if err != nil {
        log.Printf("Error: Failed to decode results: %v", err)
        return nil, fmt.Errorf("failed to decode results: %w", err)
    }

    // Log the decoded outputs
    if len(decodedResults) == 0 {
        log.Printf("No outputs were decoded for method: %s", method.Name)
    } else {
        for i, value := range decodedResults {
            log.Printf("Output decoded: Index=%d, Value=%v", i, value)
        }
    }

    return decodedResults, nil
}

// Temporary function. Later we will use a DB
func getContractPrimitive(address string) (contract string, primitive string) {
    contract, exists := contractsAddresses[address]
    if !exists {
        return "", "" // Return empty values instead of an error
    }

    primitive, exists = primitiveABI[contract]
    if !exists {
        return contract, "" // Return contract but empty primitive
    }

    return contract, primitive
}




// Utility function to prepare the next step's contract call
func prepareNextStep(step Step, contractAddress common.Address, stateDB contract.StateDB) ([]ILLMContractMethodParams, error) {
    log.Printf("Preparing next step: Method=%s, Contract=%s", step.Method, step.Contract)

    _,contractAbi := getContractPrimitive(contractAddress.Hex())
    if contractAbi == "" {
        log.Printf("Error: Failed to get contract primitive ABI.")
        return nil, fmt.Errorf("failed to get contract primitive abi")
    }

    // Parse the ABI
    // TODO: handle assignment/jump if primitives in go
    parsedABI, err := abi.JSON(strings.NewReader(contractAbi))
    if err != nil {
        log.Printf("Error: Failed to parse ABI for step Method=%s, Contract=%s. Error: %v", step.Method, step.Contract, err)
        return nil, fmt.Errorf("failed to parse ABI: %w", err)
    }
    log.Printf("Successfully parsed ABI for Method=%s, Contract=%s", step.Method, step.Contract)

    // Retrieve the method
    method, exists := parsedABI.Methods[step.Method]
    if !exists {
        log.Printf("Error: Method=%s not found in ABI for Contract=%s", step.Method, step.Contract)
        return nil, fmt.Errorf("method %s not found in ABI", step.Method)
    }
    log.Printf("Successfully retrieved method %s from ABI for Contract=%s", step.Method, step.Contract)

    // Process arguments
    packedArgs, err := ProcessArguments(method.Inputs, step.Args, stateDB)
    if err != nil {
        log.Printf("Error: Failed to process arguments for Method=%s, Contract=%s. Error: %v", step.Method, step.Contract, err)
        return nil, fmt.Errorf("failed to process arguments: %w", err)
    }
    log.Printf("Successfully processed arguments for Method=%s. Packed Arguments: %+v", step.Method, packedArgs)

    // Pack method data
    methodData, err := method.Inputs.Pack(packedArgs...)
    if err != nil {
        log.Printf("Error: Failed to pack method data for Method=%s, Contract=%s. Error: %v", step.Method, step.Contract, err)
        return nil, fmt.Errorf("failed to pack method data: %w", err)
    }
    log.Printf("Successfully packed method data for Method=%s. Method Data (hex): %x", step.Method, methodData)

    // Return the prepared contract method parameters
    contractParams := []ILLMContractMethodParams{
        {
            ContractAddress: contractAddress,
            MethodData:      append(method.ID, methodData...),
        },
    }
    log.Printf("Prepared contract method parameters for Method=%s, Contract=%s: %+v", step.Method, step.Contract, contractParams)

    return contractParams, nil
}


func updateMemoryInState(stateDB contract.StateDB, addr common.Address, storageKey string, storageData interface{}, typ string) error {
    if storageKey == "" {
        return nil
    }

    outputKeyHash := common.BytesToHash([]byte(storageKey))

    // Clear existing value if present
    if stateDB.GetState(addr, outputKeyHash) != (common.Hash{}) {
        log.Printf("Clearing existing value for key: %s", storageKey)
        stateDB.SetState(addr, outputKeyHash, common.Hash{}) // TODO: Clear large state if necessary
    }

    // Create ABI type
    abiType, err := abi.NewType(typ, "", nil)
    if err != nil {
        return fmt.Errorf("failed to create ABI type: %w", err)
    }

    // Define ABI structure for encoding
    abiArgs := abi.Arguments{{Type: abiType}}

    // Encode value using ABI
    encodedData, err := abiArgs.Pack(storageData)
    if err != nil {
        return fmt.Errorf("ABI encoding failed: %w", err)
    }

    log.Printf("Storing ABI-encoded data for Key=%s (%s): %x", storageKey, outputKeyHash, encodedData)
    setLargeState(stateDB, addr, outputKeyHash, encodedData)

    return nil
}



// continueEvaluation processes a given prompt ID and an array of contract method results,
// returning a structured response indicating the evaluation status and associated method parameters.
//
// The function performs the following:
// 1. Deducts gas for execution and verifies permissions using the allow list.
// 2. Accepts an input containing:
//    - PromptId: The ID of the evaluation prompt.
//    - ContractMethodResults: An array of byte arrays representing method data.
// 3. Constructs an output with:
//    - EvaluationDone: Always set to false.
//    - ContractMethodParams: An array where each entry includes:
//       - ContractAddress
//       - MethodData
//
// Output:
// The function returns a packed ABI-compliant byte array containing the constructed output.
func continueEvaluation(accessibleState contract.AccessibleState, caller common.Address, addr common.Address, input []byte, suppliedGas uint64, readOnly bool) (ret []byte, remainingGas uint64, err error) {
    log.Printf("Starting continueEvaluation function. Caller: %s, Contract Address: %s", caller.Hex(), addr.Hex())

    // Deduct gas
    if remainingGas, err = contract.DeductGas(suppliedGas, ContinueEvaluationGasCost); err != nil {
        log.Printf("Error: Insufficient gas supplied. Error: %v", err)
        return nil, 0, err
    }

    if readOnly {
        log.Printf("Error: Write protection violation. Function is not allowed in read-only mode.")
        return nil, remainingGas, vmerrs.ErrWriteProtection
    }

    stateDB := accessibleState.GetStateDB()

    // Retrieve steps from state using getLargeState
    encodedSteps, err := getLargeState(stateDB, addr, stepsKey)
    if err != nil {
        log.Printf("Error: Failed to retrieve steps from state. Error: %v", err)
        return nil, remainingGas, err
    }
    log.Printf("Encoded steps retrieved from state: %s", string(encodedSteps))

    // Decode the steps
    var steps []Step
    // Remove null bytes from the encoded steps
    sanitizedEncodedSteps := bytes.ReplaceAll(encodedSteps, []byte("\x00"), []byte{})
    log.Printf("Sanitized steps: %s", string(sanitizedEncodedSteps))
    if err := json.Unmarshal(sanitizedEncodedSteps, &steps); err != nil {
        log.Printf("Error: Failed to decode steps from state. Error: %v", err)
        return nil, remainingGas, fmt.Errorf("failed to decode steps: %w", err)
    }
    log.Printf("Successfully decoded %d steps from state.", len(steps))

    // Retrieve the program counter
    currentPC, err := getPCFromState(stateDB, addr)
    if err != nil {
        log.Printf("Error: Failed to retrieve the program counter. Error: %v", err)
        return nil, remainingGas, err
    }
    log.Printf("Current program counter: %d", currentPC.Int64())

    // unpacking input values
    inputStruct, err := UnpackContinueEvaluationInput(input)

    if err != nil {
        log.Printf("Error: Failed to unpack input. Error: %v", err)
        return nil, remainingGas, fmt.Errorf("failed to unpack input: %w", err)
    }
    log.Printf("Decoded input. Prompt ID: %d, ContractMethodResults count: %d", inputStruct.PromptId, len(inputStruct.ContractMethodResults))

    // Process the current step
    currentStep := steps[currentPC.Int64()]
    log.Printf("Processing step %d: Method=%s, Contract=%s", currentPC.Int64(), currentStep.Method, currentStep.Contract)

    contractAddress, err := getContractAddress(currentStep.Contract, stateDB)
    if err != nil {
        log.Printf("Error: Failed to parse contract address. Error: %v", err)
        return nil, remainingGas, fmt.Errorf("failed to parse contract address: %w", err)
    }

    _, contractAbi := getContractPrimitive(contractAddress.Hex())
    if contractAbi == "" {
        log.Printf("Error: Failed to get contract primitive ABI")
        return nil, remainingGas, fmt.Errorf("failed to get contract primitive abi")
    }

    parsedABI, err := abi.JSON(strings.NewReader(contractAbi))
    if err != nil {
        log.Printf("Error: Failed to parse ABI for step %d. Error: %v", currentPC.Int64(), err)
        return nil, remainingGas, fmt.Errorf("failed to parse ABI: %w", err)
    }
    log.Printf("Successfully parsed ABI for step %d.", currentPC.Int64())

    decodedResults, err := decodeResults(parsedABI.Methods[currentStep.Method], inputStruct.ContractMethodResults[0])
    if err != nil {
        log.Printf("Error: Failed to decode results for step %d. Error: %v", currentPC.Int64(), err)
        return nil, remainingGas, fmt.Errorf("failed to decode results: %w", err)
    }
    log.Printf("Decoded results for step %d: %+v", currentPC.Int64(), decodedResults)

    // Ensure output length matches decodedResults length
    if len(currentStep.Output) != len(decodedResults) {
        log.Printf("Warning: Mismatch between Output length (%d) and decodedResults length (%d) for step %d",
            len(currentStep.Output), len(decodedResults), currentPC.Int64())
    }

    // Store each decoded result using the corresponding output key
    for i := 0; i < len(decodedResults) && i < len(currentStep.Output); i++ {
        storageKey := currentStep.Output[i]
        expectedType := parsedABI.Methods[currentStep.Method].Outputs[i].Type.String()
        if err := updateMemoryInState(stateDB, addr, storageKey, decodedResults[i], expectedType); err != nil {
            log.Printf("Error: Failed to update memory in state for step %d, Output[%d]=%s. Error: %v",
                currentPC.Int64(), i, storageKey, err)
            return nil, remainingGas, err
        }
        log.Printf("Successfully updated memory in state for step %d, Output[%d]=%s.",
            currentPC.Int64(), i, storageKey)
    }

    // Increment the program counter
    nextPC := currentPC.Add(currentPC, big.NewInt(1))
    log.Printf("Updated program counter to %d.", nextPC.Int64())

    // Check if evaluation is done
    if nextPC.Int64() >= int64(len(steps)) {
        log.Printf("Evaluation completed. No more steps to process.")
        savePCToState(stateDB, addr, nextPC)
        output := ContinueEvaluationOutput{EvaluationDone: true}
        packedOutput, err := PackContinueEvaluationOutput(output)
        if err != nil {
            log.Printf("Error: Failed to pack final output. Error: %v", err)
            return nil, remainingGas, err
        }
        log.Printf("Successfully packed final output. Returning.")
        return packedOutput, remainingGas, nil
    }

    // Prepare the next step
    nextStep := steps[nextPC.Int64()]
    log.Printf("Preparing next step %d: Method=%s, Contract=%s", nextPC.Int64(), nextStep.Method, nextStep.Contract)

    contractAddress, err = getContractAddress(nextStep.Contract, stateDB)
    if err != nil {
        log.Printf("Error: Failed to parse contract address. Error: %v", err)
        return nil, remainingGas, fmt.Errorf("failed to parse contract address: %w", err)
    }

    for contractAddress == (common.Address{}) {
        nextPC, remainingGas, err = systemPrimitiveStep(nextPC, nextStep, addr, stateDB, accessibleState, remainingGas)
        if err != nil {
            log.Printf("Error: Failed to do system primitive step. Error: %v", err)
            return nil, remainingGas, err
        }
        if nextPC.Int64() >= int64(len(steps)) {
            log.Printf("Evaluation completed. No more steps to process.")
            savePCToState(stateDB, addr, nextPC)
            output := ContinueEvaluationOutput{EvaluationDone: true}
            packedOutput, err := PackContinueEvaluationOutput(output)
            if err != nil {
                log.Printf("Error: Failed to pack final output. Error: %v", err)
                return nil, remainingGas, err
            }
            log.Printf("Successfully packed final output. Returning.")
            return packedOutput, remainingGas, nil
        }
        nextStep = steps[nextPC.Int64()]
        contractAddress, err = getContractAddress(nextStep.Contract, stateDB)
        if err != nil {
            log.Printf("Error: Failed to parse contract address. Error: %v", err)
            return nil, remainingGas, fmt.Errorf("failed to parse contract address: %w", err)
        }
    }
    
    savePCToState(stateDB, addr, nextPC)
    
    contractMethodParams, err := prepareNextStep(nextStep, contractAddress, stateDB)
    if err != nil {
        log.Printf("Error: Failed to prepare next step %d. Error: %v", nextPC.Int64(), err)
        return nil, remainingGas, err
    }
    log.Printf("Successfully prepared contract method params for next step %d.", nextPC.Int64())

    // Pack the output for the next step
    output := ContinueEvaluationOutput{
        EvaluationDone:       false,
        ContractMethodParams: contractMethodParams,
    }

    packedOutput, err := PackContinueEvaluationOutput(output)
    if err != nil {
        log.Printf("Error: Failed to pack output for next step %d. Error: %v", nextPC.Int64(), err)
        return nil, remainingGas, err
    }

    log.Printf("Successfully packed output for next step %d. Returning.", nextPC.Int64())
    return packedOutput, remainingGas, nil
}

// Helper function to parse JSON and extract "prompt"/"plan" and "lookupTable"
func parseEvalInputJSON(input string, expectedKey string) (string, string, error) {
	// Parse the string into a JSON map
	var parsed map[string]string
	err := json.Unmarshal([]byte(input), &parsed)
	if err != nil {
		return "", "", errors.New("failed to parse JSON: " + err.Error())
	}

	// Extract the required key dynamically
	evalData, ok := parsed[expectedKey]
	if !ok {
		return "", "", fmt.Errorf("missing required key: '%s'", expectedKey)
	}

	// Extract lookupTable (optional)
	lookupTable, ok := parsed["lookupTable"]
	if !ok {
		lookupTable = "" // Default to empty string if not present
	}

	return evalData, lookupTable, nil
}

// UnpackEvaluatePlanInput attempts to unpack [input] into the string type argument
// assumes that [input] does not include selector (omits first 4 func signature bytes)
func UnpackEvaluatePlanInput(input []byte) (string, string, error) {
	// Unpack the input string
	res, err := LLMPrecompileABI.UnpackInput("evaluatePlan", input, false)
	if err != nil {
		return "", "", err
	}

	unpacked := *abi.ConvertType(res[0], new(string)).(*string)
    return parseEvalInputJSON(unpacked, "plan")
}

// PackEvaluatePlan packs [plan] of type string into the appropriate arguments for evaluatePlan.
// the packed bytes include selector (first 4 func signature bytes).
// This function is mostly used for tests.
func PackEvaluatePlan(plan string) ([]byte, error) {
	return LLMPrecompileABI.Pack("evaluatePlan", plan)
}

// PackEvaluatePlanOutput attempts to pack given [outputStruct] of type EvaluatePlanOutput
// to conform the ABI outputs.
func PackEvaluatePlanOutput(outputStruct EvaluatePlanOutput) ([]byte, error) {
	return LLMPrecompileABI.PackOutput("evaluatePlan",
		outputStruct.PromptId,
		outputStruct.ContractMethodParams,
	)
}

// UnpackEvaluatePlanOutput attempts to unpack [output] as EvaluatePlanOutput
// assumes that [output] does not include selector (omits first 4 func signature bytes)
func UnpackEvaluatePlanOutput(output []byte) (EvaluatePlanOutput, error) {
	outputStruct := EvaluatePlanOutput{}
	err := LLMPrecompileABI.UnpackIntoInterface(&outputStruct, "evaluatePlan", output)

	return outputStruct, err
}

func sanitizeSteps(input []byte) ([]byte, error) {
    // Remove null bytes
    sanitized := bytes.ReplaceAll(input, []byte("\x00"), []byte{})
    
    // Validate JSON structure
    var temp []Step
    if err := json.Unmarshal(sanitized, &temp); err != nil {
        return nil, fmt.Errorf("failed to validate sanitized steps: %w", err)
    }
    return sanitized, nil
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



// evaluateSteps contains the shared logic for processing steps in evaluatePlan and evaluatePrompt.
func evaluateSteps(accessibleState contract.AccessibleState, addr common.Address, inputSteps []Step, suppliedGas uint64, gasCost uint64) (ret []byte, remainingGas uint64, err error) {
    stateDB := accessibleState.GetStateDB()

    // Deduct gas
    if remainingGas, err = contract.DeductGas(suppliedGas, gasCost); err != nil {
        return nil, 0, err
    }

    // Increment and log the prompt counter
    currentPromptId := IncrementPromptCounter(stateDB)
    log.Printf("Current Prompt ID: %v", currentPromptId)

    // Encode the steps for storage
    encodedSteps, err := encodeSteps(inputSteps)
    if err != nil {
        log.Printf("Error: Failed to encode steps: %v", err)
        return nil, remainingGas, fmt.Errorf("failed to encode steps: %w", err)
    }
    log.Printf("Encoded steps before storing: %s", string(encodedSteps))

    sanitizedSteps, err := sanitizeSteps(encodedSteps)
    if err != nil {
        log.Printf("Error: Failed to sanitize steps before storage. Error: %v", err)
        return nil, remainingGas, err
    }

    // Delete any existing plan
    deletePlanFromState(stateDB, addr, stepsKey)

    // Store the encoded steps using setLargeState
    setLargeState(stateDB, addr, stepsKey, sanitizedSteps)
    log.Printf("Steps stored successfully in state.")

    // Initialize the program counter to 0
    currentPC := big.NewInt(0)
    log.Printf("Initialized program counter to: %v", currentPC)

    // Prepare the first step
    nextStep := inputSteps[0]
    log.Printf("Current step: %+v", nextStep)

    contractAddress, err := getContractAddress(nextStep.Contract, stateDB)
    if err != nil {
        log.Printf("Error: Failed to parse contract address. Error: %v", err)
        return nil, remainingGas, fmt.Errorf("failed to parse contract address: %w", err)
    }

    for contractAddress.Hex() == "" {
        currentPC, remainingGas, err = systemPrimitiveStep(currentPC, nextStep, addr, stateDB, accessibleState, remainingGas)
        if err != nil {
            log.Printf("Error: Failed to do system primitive step. Error: %v", err)
            return nil, remainingGas, err
        }
        if currentPC.Int64() >= int64(len(inputSteps)) {
            log.Printf("Evaluation completed. No more steps to process.")
            savePCToState(stateDB, addr, currentPC)
            output := ContinueEvaluationOutput{EvaluationDone: true}
            packedOutput, err := PackContinueEvaluationOutput(output)
            if err != nil {
                log.Printf("Error: Failed to pack final output. Error: %v", err)
                return nil, remainingGas, err
            }
            log.Printf("Successfully packed final output. Returning.")
            return packedOutput, remainingGas, nil
        }
        nextStep = inputSteps[currentPC.Int64()]
        contractAddress, err = getContractAddress(nextStep.Contract, stateDB)
        if err != nil {
            log.Printf("Error: Failed to parse contract address. Error: %v", err)
            return nil, remainingGas, fmt.Errorf("failed to parse contract address: %w", err)
        }
    }
    
    savePCToState(stateDB, addr, currentPC)

    contractMethodParams, err := prepareNextStep(nextStep, contractAddress, stateDB)
    if err != nil {
        log.Printf("Error: Failed to prepare next step. Error: %v", err)
        return nil, remainingGas, err
    }
    log.Printf("Contract Method Params: %+v", contractMethodParams)

    // Construct the output
    output := EvaluatePlanOutput{
        PromptId:             currentPromptId,
        ContractMethodParams: contractMethodParams,
    }

    // Pack the output for the next step
    packedOutput, err := PackEvaluatePlanOutput(output)
    if err != nil {
        log.Printf("Error: Failed to pack output. Error: %v", err)
        return nil, remainingGas, err
    }

    log.Printf("evaluateSteps completed successfully.")
    return packedOutput, remainingGas, nil
}

func storeLookupEntries(stateDB contract.StateDB, addr common.Address, lookupJsonString string) (map[string]string, error) {
    // If the JSON string is empty, return an empty map.
    if lookupJsonString == "" {
        log.Printf("No lookup entries")
        return map[string]string{}, nil
    }

    // Unmarshal the input JSON string into a map.
    var lookupMap map[string]string
    if err := json.Unmarshal([]byte(lookupJsonString), &lookupMap); err != nil {
        return nil, fmt.Errorf("failed to unmarshal lookup JSON: %w", err)
    }

    // Iterate over the map and store values as `common.Address`
    for key, val := range lookupMap {
        if !common.IsHexAddress(val) {
            log.Printf("Warning: Skipping invalid address for key %s: %s", key, val)
            continue // Skip invalid addresses
        }

        // Convert to `common.Address`
        addressValue := common.HexToAddress(val)

        // Store using ABI encoding (as an address)
        if err := updateMemoryInState(stateDB, addr, key, addressValue, "address"); err != nil {
            log.Printf("Error: Failed to store lookup entry for key %s. Error: %v", key, err)
            return nil, fmt.Errorf("failed to store lookup entry for key %s: %w", key, err)
        }

        log.Printf("Successfully stored lookup entry: Key=%s, Address=%s", key, addressValue.Hex())
    }

    return lookupMap, nil
}


    // signerKey := common.BytesToHash([]byte("signer"))
    // signerHex := caller.Hex()
    // signerArray := []interface{}{signerHex} // Store in an array to match other entries

    // signerBytes, err := json.Marshal(signerArray)
    // if err != nil {
    //     return fmt.Errorf("failed to encode signer array: %w", err)
    // }

    // setLargeState(stateDB, addr, signerKey, signerBytes)

    // log.Printf("Stored signer as array: %s under key: signer", signerHex)


// evaluatePlan uses evaluateSteps for its logic.
func evaluatePlan(accessibleState contract.AccessibleState, caller common.Address, addr common.Address, input []byte, suppliedGas uint64, readOnly bool) (ret []byte, remainingGas uint64, err error) {
    if readOnly {
        return nil, suppliedGas, vmerrs.ErrWriteProtection
    }

    // Unpack the input to retrieve the string argument
    plan, lookupTable, err := UnpackEvaluatePlanInput(input)
    if err != nil {
        log.Printf("Error: Failed to unpack input. Error: %v", err)
        return nil, suppliedGas, err
    }

    log.Printf("Plan: %s", plan)
    log.Printf("LookupTable: %s", lookupTable)

    stateDB := accessibleState.GetStateDB()
    // Store the lookup entries.
	_, err = storeLookupEntries(stateDB, addr, lookupTable)
	if err != nil {
		log.Fatalf("Error storing lookup entries: %v", err)
        return nil, suppliedGas, fmt.Errorf("Error storing lookup entries: %w", err)
	}
    
    // Parse input into steps
    var inputSteps []Step
    if err := json.Unmarshal([]byte(plan), &inputSteps); err != nil {
        log.Printf("Error: Failed to parse input string as steps. Error: %v", err)
        return nil, suppliedGas, fmt.Errorf("invalid input format: %w", err)
    }
    log.Printf("Parsed %d steps from input string.", len(inputSteps))

    if len(inputSteps) == 0 {
        return nil, suppliedGas, fmt.Errorf("evaluatePlan: input steps are empty")
    }

    return evaluateSteps(accessibleState, addr, inputSteps, suppliedGas, EvaluatePlanGasCost)
}

// evaluatePrompt uses evaluateSteps for its logic.
func evaluatePrompt(accessibleState contract.AccessibleState, caller common.Address, addr common.Address, input []byte, suppliedGas uint64, readOnly bool) (ret []byte, remainingGas uint64, err error) {
    if readOnly {
        return nil, suppliedGas, vmerrs.ErrWriteProtection
    }

    // Unpack the input to retrieve the string argument
    prompt, lookupTableString, err := UnpackEvaluatePromptInput(input)
    if err != nil {
        log.Printf("Error: Failed to unpack input. Error: %v", err)
        return nil, suppliedGas, err
    }
    log.Printf("Input string: %s", prompt)
    log.Printf("LookupTable: %s", lookupTableString)

    stateDB := accessibleState.GetStateDB()

	// Store the lookup entries.
	lookupTable , err := storeLookupEntries(stateDB, addr, lookupTableString)
	if err != nil {
		log.Printf("Error storing lookup entries: %v", err)
		return nil, suppliedGas, err
	}

    // Initialize primitiveMapping as a map
    primitiveMapping := make(map[string]string)

    // Iterate over lookupTable
    for key, value := range lookupTable {
        contract, _ := getContractPrimitive(value) // Check if value is a contract
        if contract != ""  { // Ensure both exist
            primitiveMapping[key] = contract // Store {lookupTable key: primitive value}
        }
    }

    // Define the API endpoint and the request payload.
	requestPayload := map[string]interface{}{
		"user_prompt": prompt,
        "primitives": primitiveMapping,
	}

	// Call the HTTP API.
	respBytes, err := HTTPPostJSON(llmApiURL, requestPayload)
    if err != nil {
        return nil, suppliedGas, fmt.Errorf("HTTP API call failed: %w", err)
    }

    log.Printf("API returned result: %s\n", respBytes)

    var inputSteps []Step
    if err := json.Unmarshal(respBytes, &inputSteps); err != nil {
        log.Printf("Error: Failed to parse response string as steps. Error: %v", err)
        return nil, suppliedGas, fmt.Errorf("invalid response format: %w", err)
    }
    log.Printf("Parsed %d steps from input string.", len(inputSteps))

    // Use pre-defined steps for evaluatePrompt
    if len(inputSteps) == 0 {
        return nil, suppliedGas, fmt.Errorf("evaluatePrompt: no predefined steps available")
    }

    return evaluateSteps(accessibleState, addr, inputSteps, suppliedGas, EvaluatePromptGasCost)
}


// UnpackEvaluatePromptInput attempts to unpack [input] into the string type argument
// assumes that [input] does not include selector (omits first 4 func signature bytes)
func UnpackEvaluatePromptInput(input []byte) (string, string, error) {
	res, err := LLMPrecompileABI.UnpackInput("evaluatePrompt", input, false)
	if err != nil {
		return "", "", err
	}
	unpacked := *abi.ConvertType(res[0], new(string)).(*string)
	return parseEvalInputJSON(unpacked, "prompt")
}

// PackEvaluatePrompt packs [prompt] of type string into the appropriate arguments for evaluatePrompt.
// the packed bytes include selector (first 4 func signature bytes).
// This function is mostly used for tests.
func PackEvaluatePrompt(prompt string) ([]byte, error) {
	return LLMPrecompileABI.Pack("evaluatePrompt", prompt)
}

// PackEvaluatePromptOutput attempts to pack given [outputStruct] of type EvaluatePromptOutput
// to conform the ABI outputs.
func PackEvaluatePromptOutput(outputStruct EvaluatePromptOutput) ([]byte, error) {
	return LLMPrecompileABI.PackOutput("evaluatePrompt",
		outputStruct.PromptId,
		outputStruct.ContractMethodParams,
	)
}

// UnpackEvaluatePromptOutput attempts to unpack [output] as EvaluatePromptOutput
// assumes that [output] does not include selector (omits first 4 func signature bytes)
func UnpackEvaluatePromptOutput(output []byte) (EvaluatePromptOutput, error) {
	outputStruct := EvaluatePromptOutput{}
	err := LLMPrecompileABI.UnpackIntoInterface(&outputStruct, "evaluatePrompt", output)

	return outputStruct, err
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

// Utility function to encode steps into bytes
func encodeSteps(steps []Step) ([]byte, error) {
    encoded, err := json.Marshal(steps)
    if err != nil {
        return nil, fmt.Errorf("failed to encode steps to JSON: %w", err)
    }
    return encoded, nil
}


// setLargeState stores [data] and includes its total length as an 8-byte prefix.
func setLargeState(stateDB contract.StateDB, addr common.Address, key common.Hash, data []byte) {
    // 1) Clear old data
    for i := 0; ; i++ {
        chunkKey := common.BytesToHash(append(key.Bytes(), byte(i)))
        oldChunk := stateDB.GetState(addr, chunkKey)
        if oldChunk == (common.Hash{}) {
            break
        }
        stateDB.SetState(addr, chunkKey, common.Hash{})
    }

    // 2) Write length + data
    // The first chunk stores the length in the first 8 bytes
    // The rest is data chunking
    totalLen := uint64(len(data))
    prefix := make([]byte, 8)
    binary.BigEndian.PutUint64(prefix, totalLen) // 8-byte length prefix

    fullData := append(prefix, data...) // Combine length prefix + actual data

    // 3) Store [fullData] in 32-byte chunks
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
        return nil, fmt.Errorf("no data found for key %s", key.Hex())
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



// UnpackPublishCustomPrimitiveInput attempts to unpack [input] as PublishCustomPrimitiveInput
// assumes that [input] does not include selector (omits first 4 func signature bytes)
func UnpackPublishCustomPrimitiveInput(input []byte) (PublishCustomPrimitiveInput, error) {
	inputStruct := PublishCustomPrimitiveInput{}
	err := LLMPrecompileABI.UnpackInputIntoInterface(&inputStruct, "publishCustomPrimitive", input, false)

	return inputStruct, err
}

// PackPublishCustomPrimitive packs [inputStruct] of type PublishCustomPrimitiveInput into the appropriate arguments for publishCustomPrimitive.
func PackPublishCustomPrimitive(inputStruct PublishCustomPrimitiveInput) ([]byte, error) {
	return LLMPrecompileABI.Pack("publishCustomPrimitive", inputStruct.ContractAddress, inputStruct.PrimitiveAddress)
}

func publishCustomPrimitive(accessibleState contract.AccessibleState, caller common.Address, addr common.Address, input []byte, suppliedGas uint64, readOnly bool) (ret []byte, remainingGas uint64, err error) {
	if remainingGas, err = contract.DeductGas(suppliedGas, PublishCustomPrimitiveGasCost); err != nil {
		return nil, 0, err
	}
	if readOnly {
		return nil, remainingGas, vmerrs.ErrWriteProtection
	}
	// attempts to unpack [input] into the arguments to the PublishCustomPrimitiveInput.
	// Assumes that [input] does not include selector
	// You can use unpacked [inputStruct] variable in your code
	inputStruct, err := UnpackPublishCustomPrimitiveInput(input)
	if err != nil {
		return nil, remainingGas, err
	}

	// CUSTOM CODE STARTS HERE
	_ = inputStruct // CUSTOM CODE OPERATES ON INPUT
	// this function does not return an output, leave this one as is
	packedOutput := []byte{}

	// Return the packed output and the remaining gas
	return packedOutput, remainingGas, nil
}

// UnpackPublishPrimitiveInput attempts to unpack [input] as PublishPrimitiveInput
// assumes that [input] does not include selector (omits first 4 func signature bytes)
func UnpackPublishPrimitiveInput(input []byte) (PublishPrimitiveInput, error) {
	inputStruct := PublishPrimitiveInput{}
	err := LLMPrecompileABI.UnpackInputIntoInterface(&inputStruct, "publishPrimitive", input, false)

	return inputStruct, err
}

// PackPublishPrimitive packs [inputStruct] of type PublishPrimitiveInput into the appropriate arguments for publishPrimitive.
func PackPublishPrimitive(inputStruct PublishPrimitiveInput) ([]byte, error) {
	return LLMPrecompileABI.Pack("publishPrimitive", inputStruct.ContractAddress, inputStruct.Metadata)
}

func publishPrimitive(accessibleState contract.AccessibleState, caller common.Address, addr common.Address, input []byte, suppliedGas uint64, readOnly bool) (ret []byte, remainingGas uint64, err error) {
	if remainingGas, err = contract.DeductGas(suppliedGas, PublishPrimitiveGasCost); err != nil {
		return nil, 0, err
	}
	if readOnly {
		return nil, remainingGas, vmerrs.ErrWriteProtection
	}
	// attempts to unpack [input] into the arguments to the PublishPrimitiveInput.
	// Assumes that [input] does not include selector
	// You can use unpacked [inputStruct] variable in your code
	inputStruct, err := UnpackPublishPrimitiveInput(input)
	if err != nil {
		return nil, remainingGas, err
	}

	// CUSTOM CODE STARTS HERE
	_ = inputStruct // CUSTOM CODE OPERATES ON INPUT
	// this function does not return an output, leave this one as is
	packedOutput := []byte{}

	// Return the packed output and the remaining gas
	return packedOutput, remainingGas, nil
}

// createLLMPrecompilePrecompile returns a StatefulPrecompiledContract with getters and setters for the precompile.

func createLLMPrecompilePrecompile() contract.StatefulPrecompiledContract {
	var functions []*contract.StatefulPrecompileFunction

	abiFunctionMap := map[string]contract.RunStatefulPrecompileFunc{
		"continueEvaluation":     continueEvaluation,
		"evaluatePlan":           evaluatePlan,
		"evaluatePrompt":         evaluatePrompt,
		"publishCustomPrimitive": publishCustomPrimitive,
		"publishPrimitive":       publishPrimitive,
	}

	for name, function := range abiFunctionMap {
		method, ok := LLMPrecompileABI.Methods[name]
		if !ok {
			panic(fmt.Errorf("given method (%s) does not exist in the ABI", name))
		}
		functions = append(functions, contract.NewStatefulPrecompileFunction(method.ID, function))
	}
	// Construct the contract with no fallback function.
	statefulContract, err := contract.NewStatefulPrecompileContract(nil, functions)
	if err != nil {
		panic(err)
	}
	return statefulContract
}
