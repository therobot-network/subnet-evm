// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

contract MathForTesting {
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

  function modulo(uint256 a, uint256 b) public pure returns (uint256) {
    require(b > 0, "Math: modulo by zero");
    return a % b;
  }

  function maximum(uint256 a, uint256 b) public pure returns (uint256) {
    return a >= b ? a : b;
  }

  function minimum(uint256 a, uint256 b) public pure returns (uint256) {
    return a <= b ? a : b;
  }

  function isGreaterThan(uint256 a, uint256 b) public pure returns (bool) {
    return a > b;
  }

  function isLessThan(uint256 a, uint256 b) public pure returns (bool) {
    return a < b;
  }

  function isGreaterThanOrEqual(uint256 a, uint256 b) public pure returns (bool) {
    return a >= b;
  }

  function isLessThanOrEqual(uint256 a, uint256 b) public pure returns (bool) {
    return a <= b;
  }

  function isEqual(uint256 a, uint256 b) public pure returns (bool) {
    return a == b;
  }

  function isNotEqual(uint256 a, uint256 b) public pure returns (bool) {
    return a != b;
  }
}
