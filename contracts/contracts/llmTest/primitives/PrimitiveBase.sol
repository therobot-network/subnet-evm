// SPDX-License-Identifier: MIT

pragma solidity ^0.8.20;

// import "hardhat/console.sol";
import {Initializable} from "@openzeppelin/contracts-upgradeable/proxy/utils/Initializable.sol";
import {OwnableUpgradeable} from "@openzeppelin/contracts-upgradeable/access/OwnableUpgradeable.sol";

import {ILLM} from "../../interfaces/ILLM.sol";
import {IExecutor, IRobotStorage} from "./interfaces/IExecutor.sol";

abstract contract PrimitiveBase is Initializable, OwnableUpgradeable {
  // llm precompile contract address for publishing new primitive
  // slither-disable-next-line naming-convention
  ILLM public immutable LLM_PRECOMPILE;
  // Primitive metadata. Set in constructor and cannot be modified.
  string private _metadata;
  // implementation contract address used for restricting direct initialization
  // slither-disable-next-line naming-convention
  address private immutable _IMPLEMENTATION_ADDRESS;

  // Address of the proxy contract
  // only proxy contract can call the functions and it should be directly callable
  address private _proxy;

  // variables for defining custom primitives
  string private _contractName;
  string private _customRules;
  string private _primitiveName;

  // solhint-disable-next-line private-vars-leading-underscore
  bytes32 internal constant EXECUTOR_SLOT = 0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80;

  error OnlyProxy();

  modifier onlyProxy() {
    if (_proxy != address(this)) revert OnlyProxy();
    _;
  }

  /**
   * @dev Initializes data and publishes primitive to LLM precompile.
   * @param llmPrecompile LLM precompile address (TODO: hardcode address)
   * @param metadata ipfs hash
   */
  constructor(address llmPrecompile, string memory name, string memory metadata, address primitiveStorageContract) {
    _initImplementation(msg.sender);
    LLM_PRECOMPILE = ILLM(llmPrecompile);
    _metadata = metadata;

    _IMPLEMENTATION_ADDRESS = address(this);

    IRobotStorage(primitiveStorageContract).publishPrimitive(_IMPLEMENTATION_ADDRESS, name, metadata);
    // publish new primitive to llm
    LLM_PRECOMPILE.publishPrimitive(_IMPLEMENTATION_ADDRESS, name);
  }

  /**
   * @dev Initialize implementation contract.
   */
  function _initImplementation(address owner) private initializer {
    __Ownable_init(owner);
  }

  /**
   * @dev Initializes base contract data. Sets ownership to given address, name and custom rules
   * for custom primitives and proxy address.
   * @param owner custom primitive owner address
   * @param name_ custom primitive name
   * @param customRules custom primitive rules
   */
  // slither-disable-next-line naming-convention
  function __Primitive_init(
    // solhint-disable-previous-line
    address owner,
    string calldata name_,
    string calldata customRules
  ) internal initializer {
    if (address(this) == _IMPLEMENTATION_ADDRESS) revert OnlyProxy();
    _proxy = address(this);
    __Ownable_init(owner);

    _contractName = name_;
    _customRules = customRules;

    _publishRobotContract();
  }

  /**
   * @dev Publishes new custom primitive to LLM precompile.
   */
  function _publishRobotContract() private {
    LLM_PRECOMPILE.publishRobotContract(address(this), _IMPLEMENTATION_ADDRESS);
  }

  /**
   * @dev Gets metadata of primitive.
   */
  function getMetadata() external view virtual returns (string memory) {
    return _metadata;
  }

  /**
   * @dev Gets primitive address.
   */
  function getPrimitiveAddress() external view returns (address) {
    return _IMPLEMENTATION_ADDRESS;
  }

  /**
   * @dev Gets custom primitive name and rules.
   */
  function getInfo()
    external
    view
    returns (string memory contractName, string memory customRules, string memory primitiveName)
  {
    return (_contractName, _customRules, _primitiveName);
  }

  /**
   * @dev Override openzeppelin's _msgSender function to get transaction signer address from the Executor
   * contract if transaction is called by executor else use the msg sender address.
   */
  function _msgSender() internal view virtual override returns (address) {
    // modifying msg sender (Temp fix)
    address executor = _getExecutor();
    return executor == msg.sender ? IExecutor(executor).getMsgSigner() : msg.sender;
  }

  /**
   * @dev Returns the current implementation address.
   */
  function _getExecutor() internal view returns (address impl) {
    // slither-disable-next-line assembly
    assembly {
      // solhint-disable-previous-line
      impl := sload(EXECUTOR_SLOT)
    }
  }
}
