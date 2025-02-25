// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

contract MathPrimitive {
  function add(uint256 a, uint256 b) public pure returns (uint256) {
    return a + b;
  }

  function subtract(uint256 a, uint256 b) public pure returns (uint256) {
    require(b <= a, "Math: subtraction underflow");
    return a - b;
  }

  function multiply(uint256 a, uint256 b) public pure returns (uint256) {
    return a * b;
  }

  function divide(uint256 a, uint256 b) public pure returns (uint256) {
    require(b > 0, "Math: division by zero");
    return a / b;
  }

  function mod(uint256 a, uint256 b) public pure returns (uint256) {
    require(b > 0, "Math: modulo by zero");
    return a % b;
  }

  function max(uint256 a, uint256 b) public pure returns (uint256) {
    return a >= b ? a : b;
  }

  function min(uint256 a, uint256 b) public pure returns (uint256) {
    return a <= b ? a : b;
  }

  function greaterThan(uint256 a, uint256 b) public pure returns (bool) {
    return a > b;
  }

  function lessThan(uint256 a, uint256 b) public pure returns (bool) {
    return a < b;
  }

  function greaterThanOrEqual(uint256 a, uint256 b) public pure returns (bool) {
    return a >= b;
  }

  function lessThanOrEqual(uint256 a, uint256 b) public pure returns (bool) {
    return a <= b;
  }

  function equal(uint256 a, uint256 b) public pure returns (bool) {
    return a == b;
  }

  function notEqual(uint256 a, uint256 b) public pure returns (bool) {
    return a != b;
  }
}
