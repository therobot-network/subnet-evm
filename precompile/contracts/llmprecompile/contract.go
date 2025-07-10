package llmprecompile

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"

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
	ContinueEvaluationGasCost     uint64 = 3000   /* SET A GAS COST HERE */
	EvaluatePlanGasCost           uint64 = 4000   /* SET A GAS COST HERE */
	EvaluatePromptGasCost         uint64 = 200000 /* SET A GAS COST HERE */
	PublishRobotContractGasCost   uint64 = 1000   /* SET A GAS COST HERE */
	PublishPrimitiveGasCost       uint64 = 1500   /* SET A GAS COST HERE */
	PublishSystemPrimitiveGasCost uint64 = 1500   /* SET A GAS COST HERE */
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

// Singleton StatefulPrecompiledContract and signatures.
var (

	// LLMPrecompileRawABI contains the raw ABI of LLMPrecompile contract.
	//go:embed contract.abi
	LLMPrecompileRawABI string

	LLMPrecompileABI = contract.ParseABI(LLMPrecompileRawABI)

	// Prompt Counter for identification of evaluations
	addressToPrimitiveName = common.BytesToHash([]byte("addressToPrimitiveName"))
	lookupStorageKey       = crypto.Keccak256Hash([]byte("lookupStorage")) // Base slot key
	systemPrimitiveKey     = []byte("systemPrimitiveKey")

	LLMPrecompilePrecompile = createLLMPrecompilePrecompile()

	// backendUrl = "http://192.168.1.62:80"
	backendUrl = "https://brain-sprint.therobot.network"

	llmApiPromptURL   = backendUrl + "/eval_prompt"
	llmApiPlanURL     = backendUrl + "/eval_plan"
	llmApiContinueURL = backendUrl + "/continue_eval"
)

// sendToBackend sends a POST to the backend and returns the response directly
func sendToBackend(url string, requestBody map[string]interface{}) (map[string]interface{}, error) {
	reqBytes, err := json.Marshal(requestBody)
	if err != nil {
		log.Error("sendToBackend: failed to marshal request body", "error", err, "requestBody", requestBody)
		return nil, fmt.Errorf("failed to marshal request body: %w", err)
	}
	log.Info("sendToBackend: Sending HTTP POST", "url", url, "body", string(reqBytes))

	client := &http.Client{Timeout: 60 * time.Second}

	resp, err := client.Post(url, "application/json", bytes.NewBuffer(reqBytes))
	if err != nil {
		log.Error("sendToBackend: POST failed", "error", err, "url", url)
		return nil, fmt.Errorf("POST failed: %w", err)
	}
	defer resp.Body.Close()
	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Error("sendToBackend: failed to read response", "error", err, "url", url)
		return nil, fmt.Errorf("failed to read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		log.Error("sendToBackend: Non-200/202 response", "status", resp.StatusCode, "body", string(respBytes), "url", url)
		return nil, fmt.Errorf("POST returned status %d", resp.StatusCode)
	}
	var backendResp map[string]interface{}
	if err := json.Unmarshal(respBytes, &backendResp); err != nil {
		log.Error("sendToBackend: failed to unmarshal response", "error", err, "response", string(respBytes), "url", url)
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}
	log.Info("sendToBackend: received response", "response", backendResp)
	return backendResp, nil
}

func processBackendResponse(
	resp map[string]interface{},
	accessibleState contract.AccessibleState,
	addr common.Address,
	promptId *big.Int,
	suppliedGas uint64,
) (evaluationDone bool, contractMethodParams []ILLMContractMethodParams, err error) {
	log.Info("processBackendResponse called", "addr", addr.Hex(), "suppliedGas", suppliedGas)
	stateDB := accessibleState.GetStateDB()
	key := promptId.String() + "cache" // Use promptId as key prefix

	if runComplete, ok := resp["run_complete"].(bool); ok && runComplete {
		log.Info("processBackendResponse: run_complete", "promptId", promptId)
		if err := removeItemLocalState(stateDB, addr, key); err != nil {
			return false, nil, err
		}
		return true, nil, nil
	}
	backendCallId, ok := resp["id"].(string)
	if !ok || backendCallId == "" {
		log.Error("processBackendResponse: missing or invalid 'id' in response", "response", resp)
		return false, nil, errors.New("missing or invalid 'id' in response")
	}
	methodName, ok := resp["method_name"].(string)
	if !ok {
		return false, nil, errors.New("missing or invalid 'method_name'")
	}
	contractAddrStr, ok := resp["contract_address"].(string)
	if !ok {
		return false, nil, errors.New("missing or invalid 'contract_address'")
	}
	inputsRaw, ok := resp["args"].([]interface{})
	if !ok {
		return false, nil, errors.New("missing or invalid 'args'")
	}
	// Handle return_type as []interface{} or []string
	var returnTypes []string
	if rtRaw, ok := resp["return_type"]; ok {
		switch v := rtRaw.(type) {
		case []interface{}:
			for _, elem := range v {
				if s, ok := elem.(string); ok {
					returnTypes = append(returnTypes, s)
				}
			}
		case []string:
			returnTypes = v
		}
	}
	log.Info("Parsed method details", "methodName", methodName, "contractAddress", contractAddrStr, "inputCount", len(inputsRaw))
	contractAddr := common.HexToAddress(contractAddrStr)
	// Handle system primitive if contractAddr is zero address
	if contractAddr == (common.Address{}) {
		// Call system primitive, send result to backend, and recursively process response
		resultToSend, remainingGas, sysErr := systemPrimitiveMethod(methodName, inputsRaw, accessibleState, suppliedGas)
		if sysErr != nil {
			log.Error("processBackendResponse: systemPrimitiveMethod failed", "error", sysErr, "methodName", methodName, "inputsRaw", inputsRaw)
			return false, nil, fmt.Errorf("systemPrimitiveMethod failed: %w", sysErr)
		}
		log.Info("processBackendResponse: systemPrimitiveMethod executed successfully", "methodName", methodName, "inputsRaw", inputsRaw, "result", resultToSend)
		payload := map[string]interface{}{"run_id": promptId, "result": resultToSend, "id": backendCallId}
		log.Info("processBackendResponse: sending systemPrimitive result to backend", "payload", payload)
		respNew, err := sendToBackend(llmApiContinueURL, payload)
		if err != nil {
			log.Error("processBackendResponse: sendToBackend failed", "error", err)
			return false, nil, err
		}
		// Recursively process the new backend response
		return processBackendResponse(respNew, accessibleState, addr, promptId, remainingGas)
	}

	// --- Build ABI method signature ---
	abiInputs := make([]abi.Argument, len(inputsRaw))
	args := make([]interface{}, len(inputsRaw))
	for i, inputObj := range inputsRaw {
		inputMap, ok := inputObj.(map[string]interface{})
		if !ok {
			return false, nil, fmt.Errorf("invalid input format at index %d", i)
		}
		typeStr, ok := inputMap["type"].(string)
		if !ok {
			return false, nil, fmt.Errorf("missing 'type' in input at index %d", i)
		}
		typ, err := abi.NewType(typeStr, "", nil)
		if err != nil {
			return false, nil, fmt.Errorf("abi.NewType failed for input %d: %w", i, err)
		}
		abiInputs[i] = abi.Argument{Name: fmt.Sprintf("arg%d", i), Type: typ}

		data := inputMap["data"]
		arg, err := convertToABIType(data, typ)
		if err != nil {
			return false, nil, fmt.Errorf("failed to convert input %d to ABI type: %w", i, err)
		}
		args[i] = arg
	}

	// Calculate method selector (ID) and pack callData
	inputTypes := make([]string, len(abiInputs))
	for i, arg := range abiInputs {
		inputTypes[i] = arg.Type.String()
	}
	sig := fmt.Sprintf("%s(%s)", methodName, strings.Join(inputTypes, ","))
	methodID := crypto.Keccak256([]byte(sig))[:4]
	packedArgs, err := abi.Arguments(abiInputs).Pack(args...)
	if err != nil {
		return false, nil, fmt.Errorf("ABI pack failed: %w", err)
	}
	callData := append(methodID, packedArgs...)
	log.Info("processBackendResponse: method signature and selector", "sig", sig, "methodID", fmt.Sprintf("%x", methodID))
	log.Info("processBackendResponse: packed ABI args", "packedArgsHex", fmt.Sprintf("%x", packedArgs), "len", len(packedArgs))
	log.Info("processBackendResponse: final callData", "callDataHex", fmt.Sprintf("%x", callData), "len", len(callData))

	// --- Store context for continueEvaluation ---
	ctx := map[string]interface{}{
		"returnTypes":   returnTypes,
		"backendCallId": backendCallId,
	}

	if err := updatePlanLocalState(stateDB, addr, key, ctx); err != nil {
		return false, nil, err
	}

	log.Info("processBackendResponse complete", "contract", contractAddr.Hex(), "callDataLen", len(callData))
	return false, []ILLMContractMethodParams{{ContractAddress: contractAddr, MethodData: callData}}, nil
}

func continueEvaluation(accessibleState contract.AccessibleState, caller common.Address, addr common.Address, input []byte, suppliedGas uint64, readOnly bool) (ret []byte, remainingGas uint64, err error) {
	log.Info("continueEvaluation called", "caller", caller.Hex(), "addr", addr.Hex(), "inputLen", len(input), "suppliedGas", suppliedGas, "readOnly", readOnly)
	if remainingGas, err = contract.DeductGas(suppliedGas, ContinueEvaluationGasCost); err != nil {
		log.Info("Insufficient gas supplied", "Error", err)
		return nil, 0, err
	}
	if readOnly {
		return nil, remainingGas, vmerrs.ErrWriteProtection
	}
	in, err := UnpackContinueEvaluationInput(input)
	if err != nil {
		log.Error("continueEvaluation: UnpackContinueEvaluationInput failed", "error", err)
		return nil, suppliedGas, err
	}
	// Decode promptId (big.Int) back to the original run_id string (hex if possible, else fallback to hash string)
	promptIdBig := in.PromptId
	// Try to load context with hex string
	backendCache, err := loadEvalContext(accessibleState.GetStateDB(), addr, promptIdBig.String())
	if err != nil {
		log.Error("continueEvaluation: loadEvalContext failed", "error", err, "promptId", promptIdBig)
		return nil, remainingGas, err
	}

	results := in.ContractMethodResults
	log.Info("continueEvaluation unpacked input", "promptId", promptIdBig, "resultsLen", len(results), "results", results)

	// Robustly extract returnTypes as []string from backendCache
	var returnTypes []string
	if rtRaw, ok := backendCache["returnTypes"]; ok {
		switch v := rtRaw.(type) {
		case []interface{}:
			for _, elem := range v {
				if s, ok := elem.(string); ok {
					returnTypes = append(returnTypes, s)
				}
			}
		case []string:
			returnTypes = v
		}
	}
	backendCallId, _ := backendCache["backendCallId"].(string)
	log.Info("continueEvaluation context loaded", "returnTypes", returnTypes, "backendCallId", backendCallId)
	// Decode results (assume single call for now)
	var decoded []interface{}
	if len(results) > 0 {
		// Build ABI outputs
		abiOutputs := make([]abi.Argument, len(returnTypes))
		for i, typ := range returnTypes {
			t, _ := abi.NewType(typ, "", nil)
			abiOutputs[i] = abi.Argument{Name: fmt.Sprintf("ret%d", i), Type: t}
		}
		args := abi.Arguments(abiOutputs)
		decoded, err = args.Unpack(results[0])
		if err != nil {
			log.Error("continueEvaluation: ABI unpack failed", "error", err)
			return nil, remainingGas, err
		}
		log.Info("continueEvaluation decoded results", "decoded", decoded)
	}
	// Send decoded result to backend, include methodId as 'id'
	var resultToSend interface{}
	if len(decoded) == 1 {
		resultToSend = decoded[0]
	} else {
		resultToSend = decoded
	}
	payload := map[string]interface{}{"run_id": promptIdBig, "result": resultToSend, "id": backendCallId}
	log.Info("continueEvaluation sending to backend", "payload", payload)
	resp, err := sendToBackend(llmApiContinueURL, payload)
	if err != nil {
		log.Error("continueEvaluation: sendToBackend failed", "error", err)
		return nil, remainingGas, err
	}
	evaluationDone, contractMethodParams, err := processBackendResponse(resp, accessibleState, addr, promptIdBig, suppliedGas)
	if err != nil {
		log.Error("continueEvaluation: processBackendResponse failed", "error", err)
		return nil, remainingGas, err
	}

	var output ContinueEvaluationOutput
	output.EvaluationDone = evaluationDone
	output.ContractMethodParams = contractMethodParams

	packed, err := PackContinueEvaluationOutput(output)

	if err != nil {
		log.Error("continueEvaluation: ABI pack failed", "error", err)
		return nil, remainingGas, err
	}
	log.Info("continueEvaluation returning", "packedLen", len(packed), "promptId", promptIdBig)
	return packed, remainingGas, nil
}

// Helper: load context from plan-local state
func loadEvalContext(stateDB contract.StateDB, addr common.Address, promptId string) (map[string]interface{}, error) {
	log.Info("loadEvalContext called", "addr", addr.Hex(), "promptId", promptId)
	raw, err := getLargeState(stateDB, addr, lookupStorageKey)
	if err != nil {
		log.Error("loadEvalContext: getLargeState failed", "error", err)
		return nil, err
	}
	var jsonMap map[string]interface{}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &jsonMap); err != nil {
			log.Error("loadEvalContext: json.Unmarshal failed", "error", err)
			return nil, err
		}
	} else {
		return nil, fmt.Errorf("no context for promptId %s (empty state)", promptId)
	}

	// The cache is stored directly under the key promptId+"cache"
	ctxRaw, ok := jsonMap[promptId+"cache"]
	if !ok {
		log.Error("loadEvalContext: no context for promptId", "promptId", promptId)
		return nil, fmt.Errorf("no context for promptId %s", promptId)
	}
	ctx, ok := ctxRaw.(map[string]interface{})
	if !ok {
		log.Error("loadEvalContext: invalid context format", "ctxRaw", ctxRaw)
		return nil, fmt.Errorf("invalid context format")
	}
	log.Info("loadEvalContext loaded context", "keys", keysOfMap(ctx))
	return ctx, nil
}

// keysOfMap returns the keys of a map[string]interface{} as a []string for logging
func keysOfMap(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// Helper to get the SystemPrimitive address from state
func getSystemPrimitiveAddress(stateDB contract.StateDB, addr common.Address) (string, error) {
	key := "SystemPrimitive"
	fullKey := crypto.Keccak256Hash(append(systemPrimitiveKey, []byte(key)...))
	stored, err := getLargeState(stateDB, addr, fullKey)
	if err != nil {
		log.Error("getSystemPrimitiveAddress: getLargeState failed", "error", err)
		return "", fmt.Errorf("failed to get SystemPrimitive address: %w", err)
	}
	if len(stored) == 0 {
		return common.Address{}.Hex(), nil
	}
	// stored is []byte, convert to string (should be hex address)
	return string(stored), nil
}

// evaluatePlan expects a Python script and names, and runs evaluateSteps.
func evaluatePlan(accessibleState contract.AccessibleState, caller common.Address, addr common.Address, input []byte, suppliedGas uint64, readOnly bool) (ret []byte, remainingGas uint64, err error) {
	log.Info("evaluatePlan called", "caller", caller.Hex(), "addr", addr.Hex(), "inputLen", len(input), "suppliedGas", suppliedGas, "readOnly", readOnly)
	if remainingGas, err = contract.DeductGas(suppliedGas, EvaluatePlanGasCost); err != nil {
		log.Info("Insufficient gas supplied", "Error", err)
		return nil, 0, err
	}
	if readOnly {
		return nil, remainingGas, vmerrs.ErrWriteProtection
	}
	plan, contracts, wallets, _, unpackErr := UnpackEvaluatePlanInput(input)
	if unpackErr != nil {
		log.Error("evaluatePlan: UnpackEvaluatePlanInput failed", "error", unpackErr)
		return nil, remainingGas, unpackErr
	}
	log.Info("evaluatePlan unpacked input", "plan", plan, "contracts", contracts, "wallets", wallets)

	// Add SystemPrimitive to contracts
	systemAddr, err := getSystemPrimitiveAddress(accessibleState.GetStateDB(), addr)
	if err != nil {
		log.Error("evaluatePlan: could not get SystemPrimitive address", "error", err)
		return nil, remainingGas, err
	}
	contracts["systemPrimitive"] = map[string]interface{}{
		"primitive": "SystemPrimitive",
		"address":   systemAddr,
	}

	promptIdInt := getPromptIdFromInput(input)

	// Use promptIdInt for run_id and output
	payload := map[string]interface{}{"plan": plan, "contracts": contracts, "wallet_addresses": wallets, "run_id": promptIdInt, "localModel": false}
	resp, err := sendToBackend(llmApiPlanURL, payload)
	if err != nil {
		log.Error("evaluatePlan: sendToBackend failed", "error", err)
		return nil, remainingGas, err
	}

	evaluationDone, contractMethodParams, err := processBackendResponse(resp, accessibleState, addr, promptIdInt, suppliedGas)
	if err != nil {
		log.Error("evaluatePlan: processBackendResponse failed", "error", err)
		return nil, remainingGas, err
	}

	var output EvaluatePlanOutput
	output.PromptId = promptIdInt
	output.EvaluationDone = evaluationDone
	output.ContractMethodParams = contractMethodParams

	packed, err := PackEvaluatePlanOutput(output)
	if err != nil {
		log.Error("evaluatePlan: ABI pack failed", "error", err)
		return nil, remainingGas, err
	}
	log.Info("evaluatePlan returning", "packedLen", len(packed))
	return packed, remainingGas, nil
}

// evaluatePrompt expects a prompt, sends it to the LLM API, gets a Starlark script, and runs evaluateSteps.
func evaluatePrompt(accessibleState contract.AccessibleState, caller common.Address, addr common.Address, input []byte, suppliedGas uint64, readOnly bool) (ret []byte, remainingGas uint64, err error) {
	log.Info("evaluatePrompt called", "caller", caller.Hex(), "addr", addr.Hex(), "inputLen", len(input), "suppliedGas", suppliedGas, "readOnly", readOnly)

	if remainingGas, err = contract.DeductGas(suppliedGas, EvaluatePromptGasCost); err != nil {
		log.Info("Insufficient gas supplied", "Error", err)
		return nil, 0, err
	}
	if readOnly {
		return nil, remainingGas, vmerrs.ErrWriteProtection
	}

	prompt, contracts, wallets, _, unpackErr := UnpackEvaluatePromptInput(input)
	if unpackErr != nil {
		return nil, remainingGas, unpackErr
	}
	log.Info("evaluatePrompt unpacked input", "prompt", prompt, "contracts", contracts, "wallets", wallets)

	// Add SystemPrimitive to contracts
	systemAddr, err := getSystemPrimitiveAddress(accessibleState.GetStateDB(), addr)
	if err != nil {
		log.Error("evaluatePrompt: could not get SystemPrimitive address", "error", err)
		return nil, remainingGas, err
	}
	contracts["SystemPrimitive"] = map[string]interface{}{
		"primitive": "SystemPrimitive",
		"address":   systemAddr,
	}

	promptIdInt := getPromptIdFromInput(input)

	payload := map[string]interface{}{
		"user_prompt":      prompt,
		"contracts":        contracts,
		"wallet_addresses": wallets,
		"localModel":       false,
		"run_id":           promptIdInt,
	}

	resp, err := sendToBackend(llmApiPromptURL, payload)
	if err != nil {
		return nil, remainingGas, err
	}

	evaluationDone, contractMethodParams, err := processBackendResponse(resp, accessibleState, addr, promptIdInt, suppliedGas)

	if err != nil {
		log.Error("evaluatePrompt: processBackendResponse failed", "error", err)
		return nil, remainingGas, err
	}

	var output EvaluatePromptOutput
	output.PromptId = promptIdInt
	output.EvaluationDone = evaluationDone
	output.ContractMethodParams = contractMethodParams
	packed, err := PackEvaluatePromptOutput(output)
	if err != nil {
		log.Info("Failed to pack output", "Error", err)
		return nil, remainingGas, err
	}
	log.Info("evaluatePrompt returning", "packedLen", len(packed), "runIDBigInt", promptIdInt.String())
	return packed, remainingGas, nil
}

// getPromptIdFromInput computes a uint256 promptId as the sha256 hash of the input bytes
func getPromptIdFromInput(input []byte) *big.Int {
	hash := sha256.Sum256(input)
	return new(big.Int).SetBytes(hash[:])
}
