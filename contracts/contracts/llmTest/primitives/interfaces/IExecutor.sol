// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

interface IExecutor {
    /**
     * @dev Gets the msg signer from the storage.
     * @return msgSigner message signer address
     */
    function getMsgSigner() external view returns (address msgSigner);
}

interface IRobotStorage {
    /**
     * @dev Publishes a primitive to the Executor.
     * @param implementationAddress address of the primitive implementation
     * @param name name of the primitive
     * @param metadata metadata of the primitive
     */
    function publishPrimitive(
        address implementationAddress,
        string memory name,
        string memory metadata
    ) external;
}
