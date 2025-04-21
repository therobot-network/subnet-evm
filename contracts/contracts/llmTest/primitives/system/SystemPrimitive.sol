// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import {SystemPrimitiveBase} from "./SystemPrimitiveBase.sol";
import {ReentrancyGuard} from "@openzeppelin/contracts/utils/ReentrancyGuard.sol";
import {RobotContract} from "../../RobotContract.sol";

contract SystemPrimitive is SystemPrimitiveBase, ReentrancyGuard {
    address public immutable executor;

    event RobotContractDeployed(
        address indexed contractAddress,
        address indexed primitiveContract
    );

    error InvalidExecutorAddress();

    constructor(
        address _llmPrecompile,
        string memory metadata,
        address _executor
    ) SystemPrimitiveBase(_llmPrecompile, "SystemPrimitive", metadata) {
        if (_executor == address(0)) revert InvalidExecutorAddress();
        executor = _executor;
    }

    function deployRobotContract(
        address primitiveAddress
    ) external nonReentrant returns (address customPrimitive) {
        RobotContract customPrimitiveContract = new RobotContract(
            primitiveAddress,
            executor
        );

        customPrimitive = address(customPrimitiveContract);
        emit RobotContractDeployed(customPrimitive, primitiveAddress);
    }
}
