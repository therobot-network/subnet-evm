// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

import {Math} from "@openzeppelin/contracts/utils/math/Math.sol";
import {Initializable} from "@openzeppelin/contracts-upgradeable/proxy/utils/Initializable.sol";

import {PrimitiveBase} from "./PrimitiveBase.sol";

contract MathPrimitive is Initializable, PrimitiveBase {
    error DivisionByZero();
    error Overflow();

    constructor(
        address llmPrecompile,
        string memory metadata,
        address primitiveStorageAddress
    ) PrimitiveBase(llmPrecompile, "math", metadata, primitiveStorageAddress) {}

    function initialize(
        address owner,
        string calldata customRules_
    ) external initializer {
        __Primitive_init(owner, customRules_);
    }
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
    function max(uint256 a, uint256 b) public pure returns (uint256) {
        return Math.max(a, b);
    }

    // Find the minimum of two numbers
    function min(uint256 a, uint256 b) public pure returns (uint256) {
        return Math.min(a, b);
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
