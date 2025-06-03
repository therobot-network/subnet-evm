package llmprecompile

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"

	"math/big"
	"net/http"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/log"

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
	PublishRobotContractGasCost uint64 = 1000 /* SET A GAS COST HERE */
	PublishPrimitiveGasCost       uint64 = 1500 /* SET A GAS COST HERE */
	PublishSystemPrimitiveGasCost uint64 = 1500 /* SET A GAS COST HERE */
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


// type Arg struct {
// 	Value  string `json:"Value"`
// 	Lookup string    `json:"Lookup"`
//     // AbiType string `json:"AbiType"`
// }

type Arg struct {
	Value  *string `json:"Value"`
	Lookup *string  `json:"Lookup"`
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

    llmApiURL = "https://brain-sprint.therobot.network/eval_prompt"
)

func HTTPPostJSON(url string, requestBody interface{}) ([]byte, error) {
	reqBytes, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request body: %w", err)
	}
	log.Info("Sending HTTP POST", "url", url, "body", string(reqBytes))

	client := &http.Client{
		Timeout: 60 * time.Second,
	}

	var respBytes []byte
	maxRetries := 3

	for attempt := 1; attempt <= maxRetries; attempt++ {
		req, err := http.NewRequest("POST", url, bytes.NewBuffer(reqBytes))
		if err != nil {
			return nil, fmt.Errorf("failed to create HTTP request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			if osErr, ok := err.(net.Error); ok && osErr.Timeout() {
				log.Warn("HTTP request timed out", "attempt", attempt, "maxRetries", maxRetries)
				if attempt == maxRetries {
					return nil, fmt.Errorf("HTTP request failed after %d attempts: %w", maxRetries, err)
				}
				continue
			}
			return nil, fmt.Errorf("HTTP request failed: %w", err)
		}

		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			log.Info("Non-200 response", "status", resp.StatusCode, "body", string(body))
			return nil, fmt.Errorf("HTTP request returned status %d", resp.StatusCode)
		}

		respBytes, err = io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read HTTP response: %w", err)
		}

		log.Info("Received response", "body", string(respBytes))
		return respBytes, nil
	}

	return nil, fmt.Errorf("unexpected failure after retries")
}

// Utility function to prepare the next step's contract call
func prepareNextStep(step Step, contractAddress common.Address, addr common.Address, stateDB contract.StateDB) ([]ILLMContractMethodParams, error) {
	log.Info("Preparing next step", "Method", step.Method, "Contract", step.Contract)

	_, contractAbi := getContractPrimitive(stateDB, addr, contractAddress.Hex())
	if contractAbi == "" {
		log.Info("Failed to get contract primitive ABI")
		return nil, fmt.Errorf("failed to get contract primitive abi")
	}

	parsedABI, err := abi.JSON(strings.NewReader(contractAbi))
	if err != nil {
		log.Info("Failed to parse ABI", "Method", step.Method, "Contract", step.Contract, "Error", err)
		return nil, fmt.Errorf("failed to parse ABI: %w", err)
	}
	log.Info("Parsed ABI", "Method", step.Method, "Contract", step.Contract)

	method, exists := parsedABI.Methods[step.Method]
	if !exists {
		log.Info("Method not found in ABI", "Method", step.Method, "Contract", step.Contract)
		return nil, fmt.Errorf("method %s not found in ABI", step.Method)
	}
	log.Info("Retrieved method from ABI", "Method", step.Method, "Contract", step.Contract)

	packedArgs, err := ProcessArguments(method.Inputs, step.Args, stateDB)
	if err != nil {
		log.Info("Failed to process arguments", "Method", step.Method, "Contract", step.Contract, "Error", err)
		return nil, fmt.Errorf("failed to process arguments: %w", err)
	}
	log.Info("Processed arguments", "Method", step.Method, "PackedArguments", packedArgs)

	methodData, err := method.Inputs.Pack(packedArgs...)
	if err != nil {
		log.Info("Failed to pack method data", "Method", step.Method, "Contract", step.Contract, "Error", err)
		return nil, fmt.Errorf("failed to pack method data: %w", err)
	}
	log.Info("Packed method data", "Method", step.Method, "DataHex", fmt.Sprintf("%x", methodData))

	contractParams := []ILLMContractMethodParams{
		{
			ContractAddress: contractAddress,
			MethodData:      append(method.ID, methodData...),
		},
	}
	log.Info("Prepared contract method parameters", "Method", step.Method, "Contract", step.Contract, "Params", contractParams)

	return contractParams, nil
}


func continueEvaluation(accessibleState contract.AccessibleState, caller common.Address, addr common.Address, input []byte, suppliedGas uint64, readOnly bool) (ret []byte, remainingGas uint64, err error) {
	log.Info("Starting continueEvaluation function", "Caller", caller.Hex(), "ContractAddress", addr.Hex())

	if remainingGas, err = contract.DeductGas(suppliedGas, ContinueEvaluationGasCost); err != nil {
		log.Info("Insufficient gas supplied", "Error", err)
		return nil, 0, err
	}

	if readOnly {
		log.Info("Write protection violation")
		return nil, remainingGas, vmerrs.ErrWriteProtection
	}

	stateDB := accessibleState.GetStateDB()

	encodedSteps, err := getLargeState(stateDB, addr, stepsKey)
	if err != nil {
		log.Info("Failed to retrieve steps from state", "Error", err)
		return nil, remainingGas, err
	}

	var steps []Step
	sanitizedEncodedSteps := bytes.ReplaceAll(encodedSteps, []byte("\x00"), []byte{})
	if err := json.Unmarshal(sanitizedEncodedSteps, &steps); err != nil {
		log.Info("Failed to decode steps from state", "Error", err)
		return nil, remainingGas, fmt.Errorf("failed to decode steps: %w", err)
	}
	log.Info("Decoded steps", "Count", len(steps))

	currentPC, err := getPCFromState(stateDB, addr)
	if err != nil {
		log.Info("Failed to retrieve program counter", "Error", err)
		return nil, remainingGas, err
	}
	log.Info("Program counter", "Value", currentPC.Int64())

	inputStruct, err := UnpackContinueEvaluationInput(input)
	if err != nil {
		log.Info("Failed to unpack input", "Error", err)
		return nil, remainingGas, fmt.Errorf("failed to unpack input: %w", err)
	}
	log.Info("Decoded input", "PromptID", inputStruct.PromptId, "ResultCount", len(inputStruct.ContractMethodResults))

	currentStep := steps[currentPC.Int64()]
	stepJson, _ := json.MarshalIndent(currentStep, "", "  ")
	log.Info("Processing step", "PC", currentPC.Int64(), "Method", currentStep.Method, "Contract", currentStep.Contract, "Step", string(stepJson))

	contractAddress, err := getContractAddress(currentStep.Contract, stateDB)
	if err != nil {
		log.Info("Failed to parse contract address", "Error", err)
		return nil, remainingGas, fmt.Errorf("failed to parse contract address: %w", err)
	}

	_, contractAbi := getContractPrimitive(stateDB, addr, contractAddress.Hex())
	if contractAbi == "" {
		log.Info("Failed to get contract primitive ABI")
		return nil, remainingGas, fmt.Errorf("failed to get contract primitive abi")
	}

	parsedABI, err := abi.JSON(strings.NewReader(contractAbi))
	if err != nil {
		log.Info("Failed to parse ABI", "PC", currentPC.Int64(), "Error", err)
		return nil, remainingGas, fmt.Errorf("failed to parse ABI: %w", err)
	}
	log.Info("Parsed ABI", "PC", currentPC.Int64())

	decodedResults, err := parsedABI.Methods[currentStep.Method].Outputs.Unpack(inputStruct.ContractMethodResults[0])
	if err != nil {
		log.Info("Failed to decode results", "PC", currentPC.Int64(), "Error", err)
		return nil, remainingGas, fmt.Errorf("failed to decode results: %w", err)
	}
	log.Info("Decoded results", "PC", currentPC.Int64(), "Results", decodedResults)

	if len(currentStep.Output) != len(decodedResults) {
		log.Info("Output mismatch", "Expected", len(currentStep.Output), "Actual", len(decodedResults), "PC", currentPC.Int64())
	}

	for i := 0; i < len(decodedResults) && i < len(currentStep.Output); i++ {
		storageKey := currentStep.Output[i]
		result := decodedResults[i]

		var strValue string
		switch v := result.(type) {
		case string:
			strValue = v
		case common.Address:
			strValue = v.Hex()
		case *big.Int:
			strValue = v.String()
		default:
			jsonValue, err := json.Marshal(v)
			if err != nil {
				log.Info("Failed to encode result", "Index", i, "Key", storageKey, "Error", err)
				return nil, remainingGas, fmt.Errorf("failed to encode result to JSON: %w", err)
			}
			strValue = string(jsonValue)
		}
		

		if err := updatePlanLocalState(stateDB, addr, storageKey, strValue); err != nil {
			log.Info("Failed to update state", "PC", currentPC.Int64(), "Index", i, "Key", storageKey, "Error", err)
			return nil, remainingGas, err
		}

		log.Info("Stored result", "Index", i, "Key", storageKey, "Value", strValue)
	}

	nextPC := currentPC.Add(currentPC, big.NewInt(1))
	log.Info("Updated program counter", "NextPC", nextPC.Int64())

	if nextPC.Int64() >= int64(len(steps)) {
		log.Info("Evaluation complete")
		savePCToState(stateDB, addr, nextPC)
		output := ContinueEvaluationOutput{EvaluationDone: true}
		packedOutput, err := PackContinueEvaluationOutput(output)
		if err != nil {
			log.Info("Failed to pack final output", "Error", err)
			return nil, remainingGas, err
		}
		log.Info("Packed final output")
		return packedOutput, remainingGas, nil
	}

	nextStep := steps[nextPC.Int64()]
	log.Info("Preparing next step", "PC", nextPC.Int64(), "Method", nextStep.Method, "Contract", nextStep.Contract)

	contractAddress, err = getContractAddress(nextStep.Contract, stateDB)
	if err != nil {
		log.Info("Failed to parse contract address", "Error", err)
		return nil, remainingGas, fmt.Errorf("failed to parse contract address: %w", err)
	}

	for contractAddress == (common.Address{}) {
		nextPC, remainingGas, err = systemPrimitiveStep(nextPC, nextStep, addr, stateDB, accessibleState, remainingGas)
		if err != nil {
			log.Info("Failed system primitive step", "Error", err)
			return nil, remainingGas, err
		}
		if nextPC.Int64() >= int64(len(steps)) {
			log.Info("Evaluation complete after system step")
			savePCToState(stateDB, addr, nextPC)
			output := ContinueEvaluationOutput{EvaluationDone: true}
			packedOutput, err := PackContinueEvaluationOutput(output)
			if err != nil {
				log.Info("Failed to pack final output", "Error", err)
				return nil, remainingGas, err
			}
			log.Info("Packed final output")
			return packedOutput, remainingGas, nil
		}
		nextStep = steps[nextPC.Int64()]
		contractAddress, err = getContractAddress(nextStep.Contract, stateDB)
		if err != nil {
			log.Info("Failed to parse contract address", "Error", err)
			return nil, remainingGas, fmt.Errorf("failed to parse contract address: %w", err)
		}
	}

	savePCToState(stateDB, addr, nextPC)

	contractMethodParams, err := prepareNextStep(nextStep, contractAddress, addr, stateDB)
	if err != nil {
		log.Info("Failed to prepare next step", "PC", nextPC.Int64(), "Error", err)
		return nil, remainingGas, err
	}
	log.Info("Prepared method params for next step", "PC", nextPC.Int64())

	output := ContinueEvaluationOutput{
		EvaluationDone:       false,
		ContractMethodParams: contractMethodParams,
	}

	packedOutput, err := PackContinueEvaluationOutput(output)
	if err != nil {
		log.Info("Failed to pack output for next step", "PC", nextPC.Int64(), "Error", err)
		return nil, remainingGas, err
	}

	log.Info("Packed output for next step", "PC", nextPC.Int64())
	return packedOutput, remainingGas, nil
}


// Helper function to parse JSON and extract "prompt"/"plan" and "lookupTable"
func parseEvalInputJSON(input string, expectedKey string) (string, string, string, error) {
	var parsed map[string]string
	err := json.Unmarshal([]byte(input), &parsed)
	if err != nil {
		log.Info("Failed to parse JSON", "Error", err)
		return "", "", "", errors.New("failed to parse JSON: " + err.Error())
	}

	evalData, ok := parsed[expectedKey]
	if !ok {
		log.Info("Missing expected key", "Key", expectedKey)
		return "", "", "", fmt.Errorf("missing required key: '%s'", expectedKey)
	}

	lookupTable, ok := parsed["lookupTable"]
	if !ok {
		lookupTable = ""
	}
	primitives, ok := parsed["primitives"]
	if !ok {
		primitives = ""
	}

	log.Info("Parsed eval input JSON", "EvalKey", expectedKey, "EvalData", evalData, "LookupTable", lookupTable, "Primitives", primitives)
	return evalData, lookupTable, primitives, nil
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

func evaluateSteps(accessibleState contract.AccessibleState, addr common.Address, inputSteps []Step, suppliedGas uint64, gasCost uint64) (ret []byte, remainingGas uint64, err error) {
	stateDB := accessibleState.GetStateDB()

	if remainingGas, err = contract.DeductGas(suppliedGas, gasCost); err != nil {
		return nil, 0, err
	}

	currentPromptId := IncrementPromptCounter(stateDB)
	log.Info("Incremented prompt counter", "PromptId", currentPromptId)

	encodedSteps, err := json.Marshal(inputSteps)
	if err != nil {
		log.Info("Failed to encode steps", "Error", err)
		return nil, remainingGas, fmt.Errorf("failed to encode steps: %w", err)
	}
	log.Info("Encoded steps", "Length", len(encodedSteps))

	sanitizedSteps, err := sanitizeSteps(encodedSteps)
	if err != nil {
		log.Info("Failed to sanitize steps", "Error", err)
		return nil, remainingGas, err
	}

	deletePlanFromState(stateDB, addr, stepsKey)
	setLargeState(stateDB, addr, stepsKey, sanitizedSteps)
	log.Info("Stored sanitized steps in state")

	currentPC := big.NewInt(0)
	log.Info("Initialized program counter", "PC", currentPC)

	nextStep := inputSteps[0]
	log.Info("Preparing first step", "Step", nextStep)

	contractAddress, err := getContractAddress(nextStep.Contract, stateDB)
	if err != nil {
		log.Info("Failed to get contract address", "Error", err)
		return nil, remainingGas, fmt.Errorf("failed to parse contract address: %w", err)
	}

	for {
		if contractAddress != (common.Address{}) && contractAddress != common.HexToAddress("0x0000000000000000000000000000000000000000") {
			break
		}
	
		if currentPC.Int64() >= int64(len(inputSteps)) {
			log.Info("All steps completed")
			savePCToState(stateDB, addr, currentPC)
			output := EvaluatePlanOutput{
				PromptId:             currentPromptId,
				EvaluationDone:       true,
				ContractMethodParams: []ILLMContractMethodParams{},
			}
			packedOutput, err := PackEvaluatePlanOutput(output)
			if err != nil {
				log.Info("Failed to pack final output", "Error", err)
				return nil, remainingGas, err
			}
			log.Info("Returning packed output for completed plan")
			return packedOutput, remainingGas, nil
		}
	
		nextStep = inputSteps[currentPC.Int64()]
		contractAddress, err = getContractAddress(nextStep.Contract, stateDB)
		if err != nil {
			log.Info("Failed to get contract address", "Error", err)
			return nil, remainingGas, fmt.Errorf("failed to parse contract address: %w", err)
		}
	
		currentPC, remainingGas, err = systemPrimitiveStep(currentPC, nextStep, addr, stateDB, accessibleState, remainingGas)
		if err != nil {
			log.Info("System primitive step failed", "Error", err)
			return nil, remainingGas, err
		}
	}
	

	savePCToState(stateDB, addr, currentPC)
	contractMethodParams, err := prepareNextStep(nextStep, contractAddress, addr, stateDB)
	if err != nil {
		log.Info("Failed to prepare next step", "Error", err)
		return nil, remainingGas, err
	}
	log.Info("Prepared contract method params", "Params", contractMethodParams)

	output := EvaluatePlanOutput{
		PromptId:             currentPromptId,
		EvaluationDone:       false,
		ContractMethodParams: contractMethodParams,
	}

	packedOutput, err := PackEvaluatePlanOutput(output)
	if err != nil {
		log.Info("Failed to pack output", "Error", err)
		return nil, remainingGas, err
	}

	log.Info("Returning packed output for next step")
	return packedOutput, remainingGas, nil
}


// evaluatePlan uses evaluateSteps for its logic.
func evaluatePlan(accessibleState contract.AccessibleState, caller common.Address, addr common.Address, input []byte, suppliedGas uint64, readOnly bool) (ret []byte, remainingGas uint64, err error) {
	if readOnly {
		return nil, suppliedGas, vmerrs.ErrWriteProtection
	}

	plan, lookupTable, _, err := UnpackEvaluatePlanInput(input)
	if err != nil {
		log.Info("Failed to unpack evaluatePlan input", "Error", err)
		return nil, suppliedGas, err
	}

	log.Info("Unpacked plan input", "Plan", plan, "LookupTable", lookupTable)

	stateDB := accessibleState.GetStateDB()
	_, err = storeLookupEntries(stateDB, addr, lookupTable)
	if err != nil {
		log.Info("Failed to store lookup entries", "Error", err)
		return nil, suppliedGas, fmt.Errorf("error storing lookup entries: %w", err)
	}

	var inputSteps []Step
	if err := json.Unmarshal([]byte(plan), &inputSteps); err != nil {
		log.Info("Failed to unmarshal plan into steps", "Error", err)
		return nil, suppliedGas, fmt.Errorf("invalid input format: %w", err)
	}
	log.Info("Parsed steps from plan", "StepCount", len(inputSteps))

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

	prompt, lookupTableString, userPrimitivesString, err := UnpackEvaluatePromptInput(input)
	if err != nil {
		log.Info("Failed to unpack evaluatePrompt input", "Error", err)
		return nil, suppliedGas, err
	}
	log.Info("Unpacked prompt input", "Prompt", prompt, "LookupTable", lookupTableString, "UserPrimitives", userPrimitivesString)	

	stateDB := accessibleState.GetStateDB()
	lookupTable, err := storeLookupEntries(stateDB, addr, lookupTableString)
	if err != nil {
		log.Info("Failed to store lookup entries", "Error", err)
		return nil, suppliedGas, err
	}

	contractToPrimitiveMapping := make(map[string]string)
	var txLogsId string

	for key, value := range lookupTable {
		strVal, ok := value.(string)
		if !ok {
			log.Info("Skipping non-string lookup entry", "Key", key, "Value", value)
			continue
		}
		if key == "txLogsId" {
			txLogsId = strVal
			log.Info("Captured txLogsId", "Value", strVal)
			continue
		}
		contract, _ := getContractPrimitive(stateDB, addr, strVal)
		if contract != "" {
			contractToPrimitiveMapping[key] = contract
		}
	}

		// Parse userPrimitivesString (must be a JSON list of strings)
		var userPrimitives []string
		if strings.TrimSpace(userPrimitivesString) == "" {
			userPrimitives = []string{} // Default to empty slice if string is empty or whitespace
		} else {
			if err := json.Unmarshal([]byte(userPrimitivesString), &userPrimitives); err != nil {
				log.Info("Failed to unmarshal userPrimitives", "Error", err)
				return nil, suppliedGas, fmt.Errorf("invalid userPrimitives format: %w", err)
			}
		}
	
		validPrimitives := []string{}
		for _, primitiveName := range userPrimitives {
			primitiveNameHash := common.BytesToHash([]byte(primitiveName))
			fullKey := crypto.Keccak256Hash(append(lookupStorageKey.Bytes(), primitiveNameHash.Bytes()...))
	
			existingValue, err := getLargeState(stateDB, addr, fullKey)
			if err != nil {
				log.Info("Failed to check userPrimitive in state", "primitiveName", primitiveName, "Error", err)
				return nil, suppliedGas, fmt.Errorf("failed to check userPrimitive in state: %w", err)
			}
			if len(existingValue) > 0 {
				log.Info("Valid userPrimitive found", "primitiveName", primitiveName)
				validPrimitives = append(validPrimitives, primitiveName)
			} else {
				log.Info("userPrimitive not found in state", "primitiveName", primitiveName)
				return nil, suppliedGas, fmt.Errorf("userPrimitive not found in state: %w", err)
			}
		}
	

	requestPayload := map[string]interface{}{
		"user_prompt": prompt,
		"contracts": contractToPrimitiveMapping,
		"primitives": validPrimitives,
		"txLogsId": txLogsId,
		"localModel": false,
	}

	// if txLogsId != "" {
	// 	requestPayload["txLogsId"] = txLogsId
	// }

	respBytes, err := HTTPPostJSON(llmApiURL, requestPayload)
	if err != nil {
		log.Info("HTTP API call failed", "Error", err)
		return nil, suppliedGas, fmt.Errorf("HTTP API call failed: %w", err)
	}
	log.Info("Received API response", "Response", string(respBytes))

	var inputSteps []Step
	if err := json.Unmarshal(respBytes, &inputSteps); err != nil {
		log.Info("Failed to unmarshal API response into steps", "Error", err)
		return nil, suppliedGas, fmt.Errorf("invalid response format: %w", err)
	}
	log.Info("Parsed steps from API response", "StepCount", len(inputSteps))

	if len(inputSteps) == 0 {
		return nil, suppliedGas, fmt.Errorf("evaluatePrompt: no predefined steps available")
	}

	return evaluateSteps(accessibleState, addr, inputSteps, suppliedGas, EvaluatePromptGasCost)
}
