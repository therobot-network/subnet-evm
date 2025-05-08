// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import {SystemPrimitiveBase} from "./SystemPrimitiveBase.sol";
import {ReentrancyGuard} from "@openzeppelin/contracts/utils/ReentrancyGuard.sol";
import {RobotContract} from "../../RobotContract.sol";

import {IRobotStorage} from "../../interfaces/IExecutor.sol";

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
        string memory primitiveName,
        string memory contractName,
        address ownerAddress,
        string memory customRules
    ) external nonReentrant returns (address customPrimitive) {
        address primitiveAddress = IRobotStorage(executor)
            .getPrimitiveImplementation(primitiveName);

        RobotContract customPrimitiveContract = new RobotContract(
            primitiveAddress,
            executor,
            contractName
        );
        customPrimitive = address(customPrimitiveContract);

        IRobotStorage(executor).publishRobotContract(
            contractName,
            customPrimitive,
            primitiveName
        );
        customPrimitiveContract.robotContractInit(ownerAddress, customRules);
    }
}
