// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import {Math} from "@openzeppelin/contracts/utils/math/Math.sol";
import {SystemPrimitiveBase} from "./SystemPrimitiveBase.sol";

contract PythonPrimitive is SystemPrimitiveBase {
    error DivisionByZero();
    error Overflow();
    error ArrayCannotBeEmpty();

    constructor(
        address llmPrecompile,
        string memory metadata
    ) SystemPrimitiveBase(llmPrecompile, "PythonPrimitive", metadata) {}

    // Add two numbers with overflow protection
    function add(uint256 a, uint256 b) public pure returns (uint256) {
        (bool success, uint256 sum) = Math.tryAdd(a, b);
        if (!success) revert Overflow();
        return sum;
    }

    // Subtract two numbers with underflow protection
    function subtract(uint256 a, uint256 b) public pure returns (uint256) {
        (bool success, uint256 difference) = Math.trySub(a, b);
        if (!success) revert Overflow();
        return difference;
    }

    // Multiply two numbers with overflow protection
    function multiply(uint256 a, uint256 b) public pure returns (uint256) {
        (bool success, uint256 product) = Math.tryMul(a, b);
        if (!success) revert Overflow();
        return product;
    }

    // Divide two numbers with zero-division protection
    function divide(uint256 a, uint256 b) public pure returns (uint256) {
        (bool success, uint256 quotient) = Math.tryDiv(a, b);
        if (!success) revert DivisionByZero();
        return quotient;
    }

    // Divide two numbers with zero-division protection
    function mod(uint256 a, uint256 b) public pure returns (uint256) {
        (bool success, uint256 result) = Math.tryMod(a, b);
        if (!success) revert DivisionByZero();
        return result;
    }

    // Power function (equivalent to Python's pow)
    function pow(uint256 base, uint256 exponent) public pure returns (uint256) {
        return base ** exponent;
    }

    // check if number a is greater than number b
    function greaterThan(uint256 a, uint256 b) public pure returns (bool) {
        return a > b;
    }

    // check if number a is greater than or equal number b
    function greaterThanOrEqual(
        uint256 a,
        uint256 b
    ) public pure returns (bool) {
        return a >= b;
    }

    // check if number a is less than number b
    function lessThan(uint256 a, uint256 b) public pure returns (bool) {
        return a < b;
    }

    // check if number a is less than or equal number b
    function lessThanOrEqual(uint256 a, uint256 b) public pure returns (bool) {
        return a <= b;
    }

    // check if number a equals number b
    function equal(uint256 a, uint256 b) public pure returns (bool) {
        return a == b;
    }

    // check if number a is not equal number b
    function notEqual(uint256 a, uint256 b) public pure returns (bool) {
        return a != b;
    }

    // Find the maximum of two numbers
    function max2(uint256 a, uint256 b) public pure returns (uint256) {
        return Math.max(a, b);
    }

    // Find the minimum of two numbers
    function min2(uint256 a, uint256 b) public pure returns (uint256) {
        return Math.min(a, b);
    }

    // Find the maximum in an array
    function max(uint256[] memory arr) public pure returns (uint256) {
        if (arr.length == 0) revert ArrayCannotBeEmpty();
        uint256 maxValue = arr[0];
        for (uint256 i = 1; i < arr.length; i++) {
            if (arr[i] > maxValue) maxValue = arr[i];
        }
        return maxValue;
    }

    // Find the minimum in an array
    function min(uint256[] memory arr) public pure returns (uint256) {
        if (arr.length == 0) revert ArrayCannotBeEmpty();
        uint256 minValue = arr[0];
        for (uint256 i = 1; i < arr.length; i++) {
            if (arr[i] < minValue) minValue = arr[i];
        }
        return minValue;
    }

    // Absolute value
    function abs(int256 x) public pure returns (uint256) {
        return x < 0 ? uint256(-x) : uint256(x);
    }

    // Divide two numbers and round up
    function ceilDiv(uint256 a, uint256 b) public pure returns (uint256) {
        return Math.ceilDiv(a, b);
    }

    // Compute the square root of a number
    function sqrt(uint256 a) public pure returns (uint256) {
        return Math.sqrt(a);
    }

    // Multiply two numbers and divide by a denominator with full precision
    function mulDiv(
        uint256 x,
        uint256 y,
        uint256 denominator
    ) public pure returns (uint256) {
        return Math.mulDiv(x, y, denominator);
    }
}
