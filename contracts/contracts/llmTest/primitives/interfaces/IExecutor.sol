// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

interface IExecutor {
    /**
     * @dev Gets the msg signer from the storage.
     * @return msgSigner message signer address
     */
    function getMsgSigner() external view returns (address msgSigner);
}
