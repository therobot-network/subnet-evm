// (c) 2022-2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

interface ILLM {
  struct ContractMethodParams {
    address contractAddress;
    bytes methodData;
  }

  // sayHello returns the stored greeting string
  function evaluatePrompt(
    string calldata prompt
  ) external returns (uint promptId, ContractMethodParams[] calldata contractMethodParams);

  function evaluatePlan(
    string calldata plan
  ) external returns (uint promptId, ContractMethodParams[] calldata contractMethodParams);

  // setGreeting  stores the greeting string
  function continueEvaluation(
    uint promptId,
    bytes[] calldata contractMethodResults
  ) external returns (bool evaluationDone, ContractMethodParams[] calldata contractMethodParams);

  function publishPrimitive(address contractAddress, string memory metadata) external;

  function publishCustomPrimitive(address contractAddress, address primitiveAddress) external;

  function healthCheck() external view returns (bool healthy);
}
