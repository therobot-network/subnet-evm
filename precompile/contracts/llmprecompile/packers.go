package llmprecompile

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/big"

	"github.com/ava-labs/subnet-evm/accounts/abi"
	"github.com/ava-labs/subnet-evm/precompile/contract"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
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
	EvaluationDone       bool
	ContractMethodParams []ILLMContractMethodParams
}

type EvaluatePromptOutput struct {
	PromptId             *big.Int
	EvaluationDone       bool
	ContractMethodParams []ILLMContractMethodParams
}

type PublishPrimitiveInput struct {
	ContractAddress common.Address
	PrimitiveName   string
}

type PublishRobotContractInput struct {
	ContractAddress  common.Address
	PrimitiveAddress common.Address
}

type PublishSystemPrimitiveInput struct {
	ContractAddress common.Address
	Name            string
	Metadata        string
}

// --- Packing/Unpacking Functions ---

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

// Helper function to parse JSON and extract "prompt"/"plan", "contracts", "wallets", and "txLogsId"
func parseEvalInputJSON(input string, expectedKey string) (string, map[string]interface{}, map[string]string, string, error) {
	log.Info("parseEvalInputJSON called", "input", input, "expectedKey", expectedKey)
	var parsed map[string]interface{}
	err := json.Unmarshal([]byte(input), &parsed)
	if err != nil {
		log.Info("Failed to parse JSON", "Error", err)
		return "", nil, nil, "", errors.New("failed to parse JSON: " + err.Error())
	}

	// Extract the main string (prompt or plan)
	evalDataRaw, ok := parsed[expectedKey]
	if !ok {
		log.Info("Missing expected key", "Key", expectedKey)
		return "", nil, nil, "", fmt.Errorf("missing required key: '%s'", expectedKey)
	}
	evalData, ok := evalDataRaw.(string)
	if !ok {
		return "", nil, nil, "", fmt.Errorf("expected '%s' to be a string", expectedKey)
	}

	// Extract contracts (map[string]interface{})
	contracts := make(map[string]interface{})
	if contractsRaw, ok := parsed["contracts"]; ok {
		if contractsMap, ok := contractsRaw.(map[string]interface{}); ok {
			for k, v := range contractsMap {
				contracts[k] = v
			}
		}
	}

	// Extract wallets (map[string]string)
	wallets := make(map[string]string)
	if walletsRaw, ok := parsed["wallets"]; ok {
		if walletsMap, ok := walletsRaw.(map[string]interface{}); ok {
			for k, v := range walletsMap {
				if str, ok := v.(string); ok {
					wallets[k] = str
				}
			}
		}
	}

	planIdRaw, ok := parsed["planId"]
	if !ok {
		return evalData, contracts, wallets, "", nil
	}
	planId, ok := planIdRaw.(string)
	if !ok {
		return "", nil, nil, "", fmt.Errorf("expected '%s' to be a string", expectedKey)
	}

	log.Info("Parsed eval input JSON", "EvalKey", expectedKey, "EvalData", evalData, "Contracts", contracts, "Wallets", wallets)
	return evalData, contracts, wallets, planId, nil
}

// UnpackEvaluatePlanInput attempts to unpack [input] into the string type argument
// assumes that [input] does not include selector (omits first 4 func signature bytes)
func UnpackEvaluatePlanInput(input []byte) (string, map[string]interface{}, map[string]string, string, error) {
	res, err := LLMPrecompileABI.UnpackInput("evaluatePlan", input, false)
	if err != nil {
		return "", nil, nil, "", err
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
		outputStruct.EvaluationDone,
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

// UnpackEvaluatePromptInput attempts to unpack [input] into the string type argument
// assumes that [input] does not include selector (omits first 4 func signature bytes)
func UnpackEvaluatePromptInput(input []byte) (string, map[string]interface{}, map[string]string, string, error) {
	res, err := LLMPrecompileABI.UnpackInput("evaluatePrompt", input, false)
	if err != nil {
		return "", nil, nil, "", err
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
		outputStruct.EvaluationDone,
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

// UnpackPublishPrimitiveInput attempts to unpack [input] as PublishPrimitiveInput
// assumes that [input] does not include selector (omits first 4 func signature bytes)
func UnpackPublishPrimitiveInput(input []byte) (PublishPrimitiveInput, error) {
	inputStruct := PublishPrimitiveInput{}
	err := LLMPrecompileABI.UnpackInputIntoInterface(&inputStruct, "publishPrimitive", input, false)
	return inputStruct, err
}

// PackPublishPrimitive packs [inputStruct] of type PublishPrimitiveInput into the appropriate arguments for publishPrimitive.
func PackPublishPrimitive(inputStruct PublishPrimitiveInput) ([]byte, error) {
	return LLMPrecompileABI.Pack("publishPrimitive", inputStruct.ContractAddress, inputStruct.PrimitiveName)
}

// UnpackPublishRobotContractInput attempts to unpack [input] as PublishRobotContractInput
// assumes that [input] does not include selector (omits first 4 func signature bytes)
func UnpackPublishRobotContractInput(input []byte) (PublishRobotContractInput, error) {
	inputStruct := PublishRobotContractInput{}
	err := LLMPrecompileABI.UnpackInputIntoInterface(&inputStruct, "publishRobotContract", input, false)
	return inputStruct, err
}

// PackPublishRobotContract packs [inputStruct] of type PublishRobotContractInput into the appropriate arguments for publishRobotContract.
func PackPublishRobotContract(inputStruct PublishRobotContractInput) ([]byte, error) {
	return LLMPrecompileABI.Pack("publishRobotContract", inputStruct.ContractAddress, inputStruct.PrimitiveAddress)
}

// UnpackPublishSystemPrimitiveInput attempts to unpack [input] as PublishSystemPrimitiveInput
// assumes that [input] does not include selector (omits first 4 func signature bytes)
func UnpackPublishSystemPrimitiveInput(input []byte) (PublishSystemPrimitiveInput, error) {
	inputStruct := PublishSystemPrimitiveInput{}
	err := LLMPrecompileABI.UnpackInputIntoInterface(&inputStruct, "publishSystemPrimitive", input, false)
	return inputStruct, err
}

// PackPublishSystemPrimitive packs [inputStruct] of type PublishSystemPrimitiveInput into the appropriate arguments for publishSystemPrimitive.
func PackPublishSystemPrimitive(inputStruct PublishSystemPrimitiveInput) ([]byte, error) {
	return LLMPrecompileABI.Pack("publishSystemPrimitive", inputStruct.ContractAddress, inputStruct.Name, inputStruct.Metadata)
}

func createLLMPrecompilePrecompile() contract.StatefulPrecompiledContract {
	var functions []*contract.StatefulPrecompileFunction

	abiFunctionMap := map[string]contract.RunStatefulPrecompileFunc{
		"continueEvaluation":     continueEvaluation,
		"evaluatePlan":           evaluatePlan,
		"evaluatePrompt":         evaluatePrompt,
		"publishPrimitive":       publishPrimitive,
		"publishRobotContract":   publishRobotContract,
		"publishSystemPrimitive": publishSystemPrimitive,
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
