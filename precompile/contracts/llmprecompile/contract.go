package llmprecompile

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/big"
	"strings"

	"github.com/ava-labs/subnet-evm/accounts/abi"
	"github.com/ava-labs/subnet-evm/precompile/contract"
	"github.com/ava-labs/subnet-evm/vmerrs"

	_ "embed"

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
	Lookup bool    `json:"Lookup"`
    LookupKey string `json:"LookupKey"`
    ReturnArgKey    int `json:"ReturnArgKey"` 
}

type Step struct {
	Method    string   `json:"Method,omitempty"`
	Contract  string   `json:"Contract,omitempty"`
	Primitive string   `json:"Primitive,omitempty"`
	Args      []Arg    `json:"Args,omitempty"`
    Output    string   `json:"Output,omitempty"`
	PcStep    bool     `json:"PcStep,omitempty"` // GoStep
	Condition string   `json:"Condition,omitempty"`
	SkipTo    int      `json:"SkipTo,omitempty"`
}

var	steps = demoPlans["withLookup"]

// isAllZeroBytes checks if a byte slice contains only zero bytes.
func isAllZeroBytes(data []byte) bool {
	for _, b := range data {
		if b != 0 {
			return false
		}
	}
	return true
}


// ProcessArguments converts arguments based on the expected types from the Primitive.
func ProcessArguments(inputs abi.Arguments, args []Arg, stateDB contract.StateDB) ([]interface{}, error) {
	if len(inputs) != len(args) {
		return nil, fmt.Errorf("mismatch between expected input count (%d) and provided arguments (%d)", len(inputs), len(args))
	}

	packedArgs := make([]interface{}, len(args))
	for i, input := range inputs {
		expectedType := input.Type.String()
		arg := args[i]
		argValue := arg.Value

		if arg.Lookup {
            // Perform lookup in blockchain storage
            lookupKey := common.BytesToHash([]byte(arg.LookupKey))
            lookupData := stateDB.GetState(ContractAddress, lookupKey).Bytes()
        
            if len(lookupData) == 0 || isAllZeroBytes(lookupData) {
                log.Printf("No valid data found for key=%s, returning error", lookupKey.Hex())
                return nil, fmt.Errorf("no valid data found for lookup key %s", arg.LookupKey)
            }
        
            // Sanitize stepData: Remove leading and trailing null bytes
            sanitizedStepData := bytes.Trim(lookupData, "\x00")
            log.Printf("Sanitized step data: %s", sanitizedStepData)
        
            // Decode the sanitized step data
            var stepResults []interface{}
            if err := json.Unmarshal(sanitizedStepData, &stepResults); err != nil {
                log.Printf("Error unmarshaling sanitized step data: %v", err)
                log.Printf("Raw sanitized step data causing issue: %s", sanitizedStepData)
                return nil, fmt.Errorf("failed to decode step results from storage: %w", err)
            }
        
            // Check bounds for the ReturnArgKey
            if arg.ReturnArgKey >= len(stepResults) {
                log.Printf("Index '%d' out of bounds for step results, length=%d", arg.ReturnArgKey, len(stepResults))
                return nil, fmt.Errorf("key '%d' not found in storage at key %s", arg.ReturnArgKey, arg.LookupKey)
            }
        
            // Retrieve and process the value
            retrievedValue := fmt.Sprintf("%v", stepResults[arg.ReturnArgKey])
        
            // Attempt Base64 decoding
            decodedValue, err := base64.StdEncoding.DecodeString(retrievedValue)
            if err == nil {
                retrievedValue = string(decodedValue) // Use the decoded value if successful
                log.Printf("Base64-decoded value: %s", retrievedValue)
            } else {
                log.Printf("Base64 decoding failed: %v", err)
            }
        
            argValue = retrievedValue
        }
        
		switch expectedType {
		case "address":
			packedArgs[i] = common.HexToAddress(argValue) // Convert to Ethereum address
		case "uint256":
			bigIntValue := new(big.Int)
			if _, success := bigIntValue.SetString(argValue, 10); !success {
				return nil, fmt.Errorf("invalid uint256 value: %s", argValue)
			}
			packedArgs[i] = bigIntValue
		case "bool":
			if argValue == "true" {
				packedArgs[i] = true
			} else if argValue == "false" {
				packedArgs[i] = false
			} else {
				return nil, fmt.Errorf("invalid boolean value: %s", argValue)
			}
		case "string":
			packedArgs[i] = argValue // Use string as-is
		default:
			return nil, fmt.Errorf("unsupported type: %s", expectedType)
		}

		// Log the conversion
		log.Printf("Arg %d: Value=%v, ConvertedValue=%v, ExpectedType=%s", i, argValue, packedArgs[i], expectedType)
	}

	return packedArgs, nil
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

// Utility function to prepare the next step's contract call
func prepareNextStep(step Step, stateDB contract.StateDB) ([]ILLMContractMethodParams, error) {
    log.Printf("Preparing next step: Method=%s, Contract=%s", step.Method, step.Contract)

    // Parse the ABI
    parsedABI, err := abi.JSON(strings.NewReader(primitiveABI[step.Primitive]))
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
            ContractAddress: common.HexToAddress(step.Contract),
            MethodData:      append(method.ID, methodData...),
        },
    }
    log.Printf("Prepared contract method parameters for Method=%s, Contract=%s: %+v", step.Method, step.Contract, contractParams)

    return contractParams, nil
}


func updateMemoryInState(stateDB contract.StateDB, addr common.Address, outputKey string, decodedResults []interface{}) error {
    if outputKey == "" {
        return nil
    }

    // Convert decodedResults to a memory representation
    memory := make([][]byte, len(decodedResults))
    for i, value := range decodedResults {
        encodedValue, err := json.Marshal(value)
        if err != nil {
            return fmt.Errorf("failed to encode result at index %d: %w", i, err)
        }
        memory[i] = encodedValue
    }

    // Serialize the memory array
    encodedMemory, err := json.Marshal(memory)
    if err != nil {
        return fmt.Errorf("failed to encode memory array: %w", err)
    }

    // Write to state using the outputKey
    log.Printf("Writing to state: Key=%s, EncodedMemory=%x", outputKey, encodedMemory)
    stateDB.SetState(addr, common.BytesToHash([]byte(outputKey)), common.BytesToHash(encodedMemory))
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

    parsedABI, err := abi.JSON(strings.NewReader(primitiveABI[currentStep.Primitive]))
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

    // Update memory in state using the Output field as the key
    if err := updateMemoryInState(stateDB, addr, currentStep.Output, decodedResults); err != nil {
        log.Printf("Error: Failed to update memory in state for step %d. Error: %v", currentPC.Int64(), err)
        return nil, remainingGas, err
    }
    log.Printf("Successfully updated memory in state for step %d under key: %s.", currentPC.Int64(), currentStep.Output)

    // Increment the program counter
    nextPC := currentPC.Add(currentPC, big.NewInt(1))
    savePCToState(stateDB, addr, nextPC)
    log.Printf("Updated program counter to %d.", nextPC.Int64())

    // Check if evaluation is done
    if nextPC.Int64() >= int64(len(steps)) {
        log.Printf("Evaluation completed. No more steps to process.")
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

    contractMethodParams, err := prepareNextStep(nextStep, stateDB)
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
func parseEvalInputJSON(input string) (string, string, error) {
	// Parse the string into a JSON map
	var parsed map[string]string
	err := json.Unmarshal([]byte(input), &parsed)
	if err != nil {
		return "", "", errors.New("failed to parse JSON: " + err.Error())
	}

	// Extract the required keys
	evalData, ok1 := parsed["prompt"]
	if !ok1 {
		evalData, ok1 = parsed["plan"] // Fallback to "plan"
	}
	if !ok1 {
		return "", "", errors.New("missing required key: 'prompt' or 'plan'")
	}

	lookupTable, ok2 := parsed["lookupTable"]
	if !ok2 {
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
    return parseEvalInputJSON(unpacked)
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
    savePCToState(stateDB, addr, currentPC)
    log.Printf("Initialized program counter to: %v", currentPC)

    // Prepare the first step
    nextStep := inputSteps[0]
    log.Printf("Current step: %+v", nextStep)

    contractMethodParams, err := prepareNextStep(nextStep, stateDB)
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
    fmt.Println("Plan:", plan)
    fmt.Println("LookupTable:", lookupTable)
    
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
    prompt, lookupTable, err := UnpackEvaluatePromptInput(input)
    if err != nil {
        log.Printf("Error: Failed to unpack input. Error: %v", err)
        return nil, suppliedGas, err
    }
    log.Printf("Input string: %s", prompt)
    fmt.Println("LookupTable:", lookupTable)

    // Use pre-defined steps for evaluatePrompt
    if len(steps) == 0 {
        return nil, suppliedGas, fmt.Errorf("evaluatePrompt: no predefined steps available")
    }

    return evaluateSteps(accessibleState, addr, steps, suppliedGas, EvaluatePromptGasCost)
}


// UnpackEvaluatePromptInput attempts to unpack [input] into the string type argument
// assumes that [input] does not include selector (omits first 4 func signature bytes)
func UnpackEvaluatePromptInput(input []byte) (string, string, error) {
	res, err := LLMPrecompileABI.UnpackInput("evaluatePrompt", input, false)
	if err != nil {
		return "", "", err
	}
	unpacked := *abi.ConvertType(res[0], new(string)).(*string)
	return parseEvalInputJSON(unpacked)
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

func setLargeState(stateDB contract.StateDB, addr common.Address, key common.Hash, data []byte) {
    chunks := len(data) / common.HashLength
    if len(data)%common.HashLength != 0 {
        chunks++
    }

    for i := 0; i < chunks; i++ {
        start := i * common.HashLength
        end := (i + 1) * common.HashLength
        if end > len(data) {
            end = len(data)
        }
        chunkKey := common.BytesToHash(append(key.Bytes(), byte(i)))
        stateDB.SetState(addr, chunkKey, common.BytesToHash(data[start:end]))
    }
}

func getLargeState(stateDB contract.StateDB, addr common.Address, key common.Hash) ([]byte, error) {
    var data []byte
    for i := 0; ; i++ {
        chunkKey := common.BytesToHash(append(key.Bytes(), byte(i)))
        chunk := stateDB.GetState(addr, chunkKey).Bytes()

        // checks for empty chunk
        isEmptyChunk := true
		for _, b := range chunk {
			if b != 0 {
				isEmptyChunk = false
				break
			}
		}

		// Break the loop if no valid data is found
		if isEmptyChunk {
			break
		}

        data = append(data, chunk...)
    }
    if len(data) == 0 {
        return nil, errors.New("no data found in state")
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
