// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import {ILLM} from "../../interfaces/ILLM.sol";

interface IExecutor {
  /**
   * @dev Gets the msg signer from the storage.
   * @return msgSigner message signer address
   */
  function getMsgSigner() external view returns (address msgSigner);

  // solhint-disable-next-line func-name-mixedcase
  function getLlm() external view returns (ILLM);
}

interface IRobotStorage {
  /**
   * @dev Publishes a primitive to the Executor.
   * @param implementationAddress address of the primitive implementation
   * @param name name of the primitive
   * @param metadata metadata of the primitive
   */
  function publishPrimitive(address implementationAddress, string memory name, string memory metadata) external;

  function getPrimitive(string memory name) external view returns (address primitiveAddress, string memory metadata);
}
interface IRobotStateEmitter {
  struct NamedFloat {
    string name;
    string value;
  }

  struct NamedUint {
    string name;
    uint256 value;
  }

  struct NamedString {
    string name;
    string value;
  }

  struct NamedAddress {
    string name;
    address value;
  }

  struct NamedBool {
    string name;
    bool value;
  }

  struct StateChangePayload {
    NamedFloat[] floats;
    NamedUint[] uints;
    NamedString[] strings;
    NamedAddress[] addresses;
    NamedBool[] bools;
  }
  /**
   * @dev Sends data regarding a state change in a contract.
   * @param stateChangePayload The payload describing the state change
   */
  function emitStateChange(StateChangePayload calldata stateChangePayload) external;
}
