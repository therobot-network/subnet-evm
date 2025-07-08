// SPDX-License-Identifier: MIT

pragma solidity ^0.8.20;

import {ILLM} from "../../interfaces/ILLM.sol";

contract MockLLM is ILLM {
    ContractMethodParams[] public methodParams;

    event NewPrimitive(address contractAddress, string metadata);
    event RobotContractPublished(
        address contractAddress,
        address primitiveAddress
    );

    constructor() {} // solhint-disable-line

    function setContractMethodParams(
        ContractMethodParams[] memory contractMethodParams
    ) external {
        // reset method params
        delete methodParams;
        for (uint i = 0; i < contractMethodParams.length; i++) {
            methodParams.push(contractMethodParams[i]);
        }
    }

    function evaluatePrompt(
        string calldata /* prompt */
    )
        external
        view
        returns (
            uint promptId,
            bool evaluationDone,
            ContractMethodParams[] memory contractMethodParams
        )
    {
        return (1, false, methodParams);
    }

    function evaluatePlan(
        string calldata /* plan */
    )
        external
        view
        returns (
            uint promptId,
            bool evaluationDone,
            ContractMethodParams[] memory contractMethodParams
        )
    {
        return (1, false, methodParams);
    }

    // setGreeting  stores the greeting string
    function continueEvaluation(
        uint /* promptId  */,
        bytes[] calldata /* contractMethodResults  */
    )
        external
        pure
        returns (
            bool evaluationDone,
            ContractMethodParams[] memory contractMethodParams
        )
    {
        return (true, contractMethodParams);
    }

    function publishPrimitive(
        address contractAddress,
        string memory metadata
    ) external {
        emit NewPrimitive(contractAddress, metadata);
    }

    function publishRobotContract(
        address contractAddress,
        address primitiveAddress
    ) external {
        emit RobotContractPublished(contractAddress, primitiveAddress);
    }

    function publishSystemPrimitive(
        address /* contractAddress */,
        string memory /* name */,
        string memory /* metadata */
    ) external {} // solhint-disable-line
}
