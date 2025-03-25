// SPDX-License-Identifier: MIT

pragma solidity ^0.8.20;

import {PrimitiveBase} from "./PrimitiveBase.sol";

contract EventPrimitive is PrimitiveBase {
  event uintArrayEvent(uint256[] uintArray);

  constructor(address llmPrecompile, string memory metadata) PrimitiveBase(llmPrecompile, metadata) {}

  function emitUintArrayEvent(uint256[] calldata uintArray) external {
    emit uintArrayEvent(uintArray);
  }
}
