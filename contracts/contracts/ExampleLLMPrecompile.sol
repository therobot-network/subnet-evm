//SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;
import "./interfaces/ILLM.sol";

// ExampleLLM shows how the HelloWorld precompile can be used in a smart contract.
contract ExampleLLMPrecompile {
  address constant LLM_ADDRESS = 0x0300000000000000000000000000000000000000;
  ILLM llm = ILLM(LLM_ADDRESS);

  event EvaluatePromptEvent(uint promptId, ILLM.ContractMethodParams[] contractMethodParams);
  event EvaluatePlanEvent(uint promptId, ILLM.ContractMethodParams[] contractMethodParams);
  event ContinueEvaluationEvent(bool evaluationDone, ILLM.ContractMethodParams[] contractMethodParams);

  function evaluatePrompt(
    string calldata prompt
  ) external returns (uint promptId, ILLM.ContractMethodParams[] memory contractMethodParams) {
    (promptId, contractMethodParams) = llm.evaluatePrompt(prompt);
    emit EvaluatePromptEvent(promptId, contractMethodParams);
    return (promptId, contractMethodParams);
  }

  function evaluatePlan(
    string calldata plan
  ) external returns (uint promptId, ILLM.ContractMethodParams[] memory contractMethodParams) {
    (promptId, contractMethodParams) = llm.evaluatePlan(plan);
    emit EvaluatePlanEvent(promptId, contractMethodParams);
    return (promptId, contractMethodParams);
  }

  function continueEvaluation(
    uint promptId,
    bytes[] calldata contractMethodResults
  ) external returns (bool evaluationDone, ILLM.ContractMethodParams[] memory contractMethodParams) {
    (evaluationDone, contractMethodParams) = llm.continueEvaluation(promptId, contractMethodResults);
    emit ContinueEvaluationEvent(evaluationDone, contractMethodParams);
    return (evaluationDone, contractMethodParams);
  }
}
