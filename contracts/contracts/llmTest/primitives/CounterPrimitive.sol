// SPDX-License-Identifier: MIT

pragma solidity ^0.8.20;

// import {PrimitiveBase} from "./PrimitiveBase.sol";

//  is PrimitiveBase
contract CounterPrimitive {
  uint256 public counter;

  constructor() {}

  function increase(uint256 number) external returns (uint256) {
    counter += number;
    return counter;
  }

  function decrease(uint256 number) external returns (uint256) {
    counter -= number;
    return counter;
  }

  function reset() external returns (uint256) {
    counter = 0;
    return counter;
  }

  function getCounter() external view returns (uint256) {
    return counter;
  }
}
