// SPDX-License-Identifier: MIT

pragma solidity ^0.8.20;

// import "hardhat/console.sol";

import {Proxy} from "@openzeppelin/contracts/proxy/Proxy.sol";

import {StorageSlot} from "@openzeppelin/contracts/utils/StorageSlot.sol";

import {ILLM} from "../interfaces/ILLM.sol";
import {IExecutor} from "./interfaces/IExecutor.sol";

contract RobotContract is Proxy {
    /**
     * @dev Storage slot with the address of the current implementation.
     * This is the keccak-256 hash of "eip1967.proxy.implementation" subtracted by 1.
     */
    // solhint-disable-next-line private-vars-leading-underscore
    bytes32 internal constant IMPLEMENTATION_SLOT =
        0x360894a13ba1a3210667c828492db98dca3e2076cc3735a920a3ca505d382bbc;

    // solhint-disable-next-line private-vars-leading-underscore
    bytes32 internal constant EXECUTOR_SLOT =
        0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80;

    /**
     * @dev Storage slot with the name of the robot contract.
     * This is the keccak-256 hash of "robotContractName" subtracted by 1.
     */
    // solhint-disable-next-line private-vars-leading-underscore
    bytes32 internal constant CONTRACT_NAME_SLOT =
        0x9f2cbf14d45099f51bf985adb3c53b5e52f3a7db48a42b9fe7e952864c20df65;

    error InvalidPrimitiveAddress(address primitive);
    error InitializationFailed();
    error FetchFailed();
    error InvalidExecutorAddress(address executor);
    error InvalidContractName(string contractName);

    /**
     * @dev Sets implementation address and calls initialize function in implementation
     * through delegate call.
     * @param _primitive primitive contract address
     * @param _executor executor contract address
     */
    constructor(
        address _primitive,
        address _executor,
        string memory contractName
    ) {
        if (_primitive == address(0))
            revert InvalidPrimitiveAddress(_primitive);
        if (_executor == address(0)) revert InvalidExecutorAddress(_executor);
        if (bytes(contractName).length == 0)
            revert InvalidContractName(contractName);
        _setImplementation(_primitive);
        _setExecutor(_executor);
        StorageSlot.getStringSlot(CONTRACT_NAME_SLOT).value = contractName;

        ILLM llmPrecompile = IExecutor(_executor).getLlm();
        llmPrecompile.publishRobotContract(address(this), _primitive);
    }

    /**
     * @dev Calls initialize function in implementation through delegate call.
     * This function can be skipped over by directly calling the proxy fallback with initData.
     * @param ownerAddress address of the owner of the robot contract
     * @param customRules custom rules for the robot contract
     */
    function robotContractInit(
        address ownerAddress,
        string memory customRules
    ) external {
        bytes memory initData = abi.encodeWithSignature(
            "robotContractInit(address,string)",
            ownerAddress,
            customRules
        );

        // Call the initialize function in the implementation
        // slither-disable-next-line controlled-delegatecall
        (bool success, ) = _getImplementation().delegatecall(initData); // solhint-disable-line
        if (!success) revert InitializationFailed();
    }

    /**
     * @dev Fetches metadata from implementation contract storage using static call.
     */
    function getMetadata() external view returns (string memory) {
        // slither-disable-next-line low-level-calls
        (bool success, bytes memory data) = _getImplementation().staticcall(
            abi.encodeWithSignature("getMetadata()")
        );
        if (!success) revert FetchFailed();
        return abi.decode(data, (string));
    }

    /**
     * @dev Returns the current implementation address.
     */
    function _getImplementation() internal view returns (address impl) {
        // slither-disable-next-line assembly
        assembly {
            // solhint-disable-previous-line
            impl := sload(IMPLEMENTATION_SLOT)
        }
    }

    /**
     * @dev Sets the implementation address in the designated storage slot.
     */
    function _setImplementation(address impl) internal {
        // slither-disable-next-line assembly
        assembly {
            // solhint-disable-previous-line
            sstore(IMPLEMENTATION_SLOT, impl)
        }
    }

    /**
     * @dev Sets the implementation address in the designated storage slot.
     */
    function _setExecutor(address executor) internal {
        // slither-disable-next-line assembly
        assembly {
            // solhint-disable-previous-line
            sstore(EXECUTOR_SLOT, executor)
        }
    }

    /**
     * @dev Override function to return implementation address. Used by Proxy base contract
     */
    function _implementation()
        internal
        view
        virtual
        override
        returns (address)
    {
        return _getImplementation();
    }

    // Receive function to accept plain Ether transfers
    receive() external payable {}
}
