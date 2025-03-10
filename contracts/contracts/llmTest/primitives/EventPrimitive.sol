// SPDX-License-Identifier: MIT

pragma solidity ^0.8.20;

// import {PrimitiveBase} from "./PrimitiveBase.sol";

//  is PrimitiveBase
contract EventPrimitive {
  event uintArrayEvent(uint256[] uintArray);

  constructor() {}

  function emitUintArrayEvent(uint256[] calldata uintArray) external {
    emit uintArrayEvent(uintArray);
  }
}
