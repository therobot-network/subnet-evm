// SPDX-License-Identifier: MIT

pragma solidity ^0.8.20;

// import "hardhat/console.sol";

import {PrimitiveBase} from "./PrimitiveBase.sol";
import {Initializable} from "@openzeppelin/contracts-upgradeable/proxy/utils/Initializable.sol";

contract CounterPrimitive is Initializable, PrimitiveBase {
    uint256 public counter;

    constructor(
        address llmPrecompile,
        string memory metadata,
        address primitiveStorageAddress
    )
        PrimitiveBase(
            llmPrecompile,
            "counter",
            metadata,
            primitiveStorageAddress
        )
    {}

    function initialize(
        address owner,
        string calldata name_,
        string calldata customRules_
    ) external initializer {
        __Primitive_init(owner, name_, customRules_);
    }

    function increase(uint256 number) external onlyProxy returns (uint256) {
        counter += number;
        return counter;
    }

    function decrease(uint256 number) external onlyProxy returns (uint256) {
        counter -= number;
        return counter;
    }

    function reset() external onlyProxy returns (uint256) {
        counter = 0;
        return counter;
    }

    function getCounter() external view onlyProxy returns (uint256) {
        return counter;
    }
}
