package llmprecompile

import (
	"bytes"
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

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

const (
	// Gas costs for each function. These are set to 1 by default.
	// You should set a gas cost for each function in your contract.
	// Generally, you should not set gas costs very low as this may cause your network to be vulnerable to DoS attacks.
	// There are some predefined gas costs in contract/utils.go that you can use.
	ContinueEvaluationGasCost     uint64 = 3000 /* SET A GAS COST HERE */
	EvaluatePlanGasCost           uint64 = 4000 /* SET A GAS COST HERE */
	EvaluatePromptGasCost         uint64 = 200000 /* SET A GAS COST HERE */
	PublishCustomPrimitiveGasCost uint64 = 1000 /* SET A GAS COST HERE */
	PublishPrimitiveGasCost       uint64 = 1500 /* SET A GAS COST HERE */
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
    // AbiType string `json:"AbiType"`
}

type Step struct {
	Method    string   `json:"Method"`
	Contract  Arg      `json:"Contract"`
	Args      []Arg    `json:"Args"`
    Output    []string   `json:"Output"`
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
    lookupStorageKey = crypto.Keccak256Hash([]byte("lookupStorage")) // Base slot key
    // utilNameToAddressKey = common.BytesToHash([]byte("primitiveNameToAddress"))
    addressToPrimitiveName = common.BytesToHash([]byte("addressToPrimitiveName"))


	LLMPrecompilePrecompile = createLLMPrecompilePrecompile()

    llmApiURL = "http://robotbrain-v2-loadbalancer-2026683595.eu-west-1.elb.amazonaws.com/eval_prompt"
)

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


// Utility function to prepare the next step's contract call
func prepareNextStep(step Step, contractAddress common.Address, addr common.Address, stateDB contract.StateDB) ([]ILLMContractMethodParams, error) {
    log.Printf("Preparing next step: Method=%s, Contract=%s", step.Method, step.Contract)

    _,contractAbi := getContractPrimitive(stateDB, addr, contractAddress.Hex())
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
    // log.Printf("Encoded steps retrieved from state: %s", string(encodedSteps))

    // Decode the steps
    var steps []Step
    // Remove null bytes from the encoded steps
    sanitizedEncodedSteps := bytes.ReplaceAll(encodedSteps, []byte("\x00"), []byte{})
    // log.Printf("Sanitized steps: %s", string(sanitizedEncodedSteps))
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
    stepJson, _ := json.MarshalIndent(currentStep, "", "  ")
    log.Printf("Processing step %d: Method=%s, Contract=%s\nFull Step:\n%s",
	currentPC.Int64(), currentStep.Method, currentStep.Contract, string(stepJson))


    contractAddress, err := getContractAddress(currentStep.Contract, stateDB)
    if err != nil {
        log.Printf("Error: Failed to parse contract address. Error: %v", err)
        return nil, remainingGas, fmt.Errorf("failed to parse contract address: %w", err)
    }

    _, contractAbi := getContractPrimitive(stateDB, addr, contractAddress.Hex())
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

    decodedResults, err := parsedABI.Methods[currentStep.Method].Outputs.Unpack(inputStruct.ContractMethodResults[0])
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

    // // Store each decoded result using the corresponding output key
    // for i := 0; i < len(decodedResults) && i < len(currentStep.Output); i++ {
    //     storageKey := currentStep.Output[i]
    //     if err := updatePlanLocalState(stateDB, addr, storageKey, decodedResults[i]); err != nil {
    //         log.Printf("Error: Failed to update memory in state for step %d, Output[%d]=%s. Error: %v",
    //             currentPC.Int64(), i, storageKey, err)
    //         return nil, remainingGas, err
    //     }
    //     log.Printf("Successfully updated memory in state for step %d, Output[%d]=%s.",
    //         currentPC.Int64(), i, storageKey)
    // }

    for i := 0; i < len(decodedResults) && i < len(currentStep.Output); i++ {
        storageKey := currentStep.Output[i]
        result := decodedResults[i]
        
        var strValue string
        switch v := result.(type) {
        case string:
            strValue = v // Use raw string directly, no json.Marshal
        default:
            // JSON encode everything else (e.g. *big.Int, address, arrays, etc.)
            jsonValue, err := json.Marshal(v)
            if err != nil {
                log.Printf("Error: Failed to JSON-encode result for Output[%d]=%s: %v", i, storageKey, err)
                return nil, remainingGas, fmt.Errorf("failed to encode result to JSON: %w", err)
            }
            strValue = string(jsonValue)
        }
    
        if err := updatePlanLocalState(stateDB, addr, storageKey, strValue); err != nil {
            log.Printf("Error: Failed to update memory in state for step %d, Output[%d]=%s. Error: %v",
                currentPC.Int64(), i, storageKey, err)
            return nil, remainingGas, err
        }
    
        log.Printf("Stored Output[%d]=%s as JSON string: %s", i, storageKey, strValue)
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
    
    contractMethodParams, err := prepareNextStep(nextStep, contractAddress, addr, stateDB)
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
    encodedSteps, err := json.Marshal(inputSteps)
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

    for contractAddress == (common.Address{}) || contractAddress == common.HexToAddress("0x0000000000000000000000000000000000000000") {
        currentPC, remainingGas, err = systemPrimitiveStep(currentPC, nextStep, addr, stateDB, accessibleState, remainingGas)
        if err != nil {
            log.Printf("Error: Failed to do system primitive step. Error: %v", err)
            return nil, remainingGas, err
        }
        if currentPC.Int64() >= int64(len(inputSteps)) {
            log.Printf("Evaluation completed. No more steps to process.")
            savePCToState(stateDB, addr, currentPC)
            // Construct the output
            output := EvaluatePlanOutput{
                PromptId:             currentPromptId,
                ContractMethodParams: []ILLMContractMethodParams{},
            }
            packedOutput, err := PackEvaluatePlanOutput(output)
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

    contractMethodParams, err := prepareNextStep(nextStep, contractAddress, addr, stateDB)
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

    log.Printf("evaluation completed successfully.")
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
        strVal, ok := value.(string)
        if !ok {
            log.Printf("Warning: Lookup value for key '%s' is not a string, skipping: %+v", key, value)
            continue
        }
        contract, _ := getContractPrimitive(stateDB, addr, strVal) // Check if value is a contract
        if contract != ""  { // Ensure both exist
            primitiveMapping[key] = contract // Store {lookupTable key: primitive value}
        }
    }

    // Define the API endpoint and the request payload.
	requestPayload := map[string]interface{}{
		"user_prompt": prompt,
        "primitives": primitiveMapping,
        // "txId": ...
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

	// Unpack input values
	inputStruct, err := UnpackPublishCustomPrimitiveInput(input)
	if err != nil {
		log.Printf("Error: Failed to unpack input. Error: %v", err)
		return nil, remainingGas, err
	}

	stateDB := accessibleState.GetStateDB()

	contractAddress := inputStruct.ContractAddress
	primitiveAddress := inputStruct.PrimitiveAddress

	// Generate key to retrieve the name using primitive address
	primitiveAddressHash := common.BytesToHash([]byte(primitiveAddress.Hex()))
	fullKey := crypto.Keccak256Hash(append(addressToPrimitiveName.Bytes(), primitiveAddressHash.Bytes()...))

	// Retrieve name from storage using the primitive address
	storedName, err := getLargeState(stateDB, addr, fullKey)
	if err != nil {
		log.Printf("Error retrieving primitive name from state: %v", err)
		return nil, remainingGas, err
	}

	if len(storedName) == 0 {
		log.Printf("Error: No primitive name found for address: %s", primitiveAddress.Hex())
		return nil, remainingGas, fmt.Errorf("primitive name not found for address %s", primitiveAddress.Hex())
	}

	// Generate key to store the name under the contractAddress
	contractAddressHash := common.BytesToHash(contractAddress.Bytes())
	fullKey = crypto.Keccak256Hash(append(addressToPrimitiveName.Bytes(), contractAddressHash.Bytes()...))

	// Store the retrieved name under contractAddress
	setLargeState(stateDB, addr, fullKey, storedName)

    log.Printf(
        "Stored primitive mapping: Name=%s, ContractAddress=%s, ContractAddressHash=%s, FullKey=%s, StorageAddr=%s",
        string(storedName),
        contractAddress.Hex(),
        contractAddressHash.Hex(),
        fullKey.Hex(),
        addr.Hex(), // ← log the address you're storing into
    )
    
	// No output is expected for this function, so return an empty byte array
	return []byte{}, remainingGas, nil
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

    stateDB := accessibleState.GetStateDB()

    // Unpack input values
    inputStruct, err := UnpackPublishPrimitiveInput(input)
    if err != nil {
        log.Printf("Error: Failed to unpack input. Error: %v", err)
        return nil, remainingGas, err
    }

    contractAddress := inputStruct.ContractAddress
    metadata := inputStruct.Metadata

    // Store using ABI encoding (as an address)
    if err := updatePlanLocalState(stateDB, addr, metadata, contractAddress); err != nil {
        log.Printf("Error: Failed to store permanent lookup entry for key %s. Error: %v", metadata, err)
        return nil, remainingGas, fmt.Errorf("failed to permanent store lookup entry for key %s: %w", metadata, err)
    }

    log.Printf("Successfully stored permanent lookup entry: Key=%s, Address=%s", metadata, contractAddress.Hex())

    // Compute the metadata hash
    metadataHash := common.BytesToHash([]byte(metadata))

    // Generate the final key as keccak256(baseKey || metadataHash)
    fullKey := crypto.Keccak256Hash(append(lookupStorageKey.Bytes(), metadataHash.Bytes()...))    
    // fullKey := common.BytesToHash(append(utilNameToAddressKey.Bytes(), metadataHash.Bytes()...))

    // Check if the metadata key already exists
    existingValue, err := getLargeState(stateDB, addr, fullKey)
    if err != nil {
        log.Printf("Error retrieving metadata key from state: %v", err)
        return nil, remainingGas, err
    }
    if len(existingValue) > 0 {
        log.Printf("Error: Metadata key already exists in state: %s", metadata)
        return nil, remainingGas, fmt.Errorf("util name already registered")
    }

    // Store the mapping (metadata -> contractAddress)
    setLargeState(stateDB, addr, fullKey, contractAddress.Bytes())
    log.Printf("Stored primitive mapping name -> address: Metadata=%s, ContractAddress=%s", metadata, contractAddress.Hex())

    addressHash := common.BytesToHash([]byte(contractAddress.Hex()))
    fullKey = crypto.Keccak256Hash(append(addressToPrimitiveName.Bytes(), addressHash.Bytes()...))
    metadataBytes := []byte(metadata)  // Convert string to []byte
    setLargeState(stateDB, addr, fullKey, metadataBytes)

    log.Printf("Stored primitive mapping address -> name: Metadata=%s, ContractAddress=%s", metadata, contractAddress.Hex())

    contractAddressHash := common.BytesToHash(contractAddress.Bytes())
	fullKey = crypto.Keccak256Hash(append(addressToPrimitiveName.Bytes(), contractAddressHash.Bytes()...))

	// Store the retrieved name under contractAddress
	setLargeState(stateDB, addr, fullKey, metadataBytes)

    log.Printf(
        "Stored primitive mapping: Name=%s, ContractAddress=%s, ContractAddressHash=%s, FullKey=%s, StorageAddr=%s",
        string(metadataBytes),
        contractAddress.Hex(),
        contractAddressHash.Hex(),
        fullKey.Hex(),
        addr.Hex(), // ← log the address you're storing into
    )


    // No output is expected for this function, so return an empty byte array
    return []byte{}, remainingGas, nil
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
