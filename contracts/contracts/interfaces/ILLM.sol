// (c) 2022-2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

interface ILLM {
  event QuestionAnswer(string question, string answer);

  struct ContractMethodParams {
    address contractAddress;
    bytes methodData;
  }

  function evaluatePrompt(
    string calldata prompt
  ) external returns (uint promptId, bool evaluationDone, ContractMethodParams[] calldata contractMethodParams);

  function evaluatePlan(
    string calldata prompt
  ) external returns (uint promptId, bool evaluationDone, ContractMethodParams[] calldata contractMethodParams);

  function continueEvaluation(
    uint promptId,
    bytes[] calldata contractMethodResults
  ) external returns (bool evaluationDone, ContractMethodParams[] calldata contractMethodParams);

  function publishPrimitive(address contractAddress, string memory primitiveName) external;

  function publishRobotContract(address contractAddress, address primitiveAddress) external;

  function publishSystemPrimitive(address contractAddress, string memory name, string memory metadata) external;
}
