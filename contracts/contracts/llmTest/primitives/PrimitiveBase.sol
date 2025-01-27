// SPDX-License-Identifier: MIT

pragma solidity ^0.8.20;

import {Initializable} from "@openzeppelin/contracts/proxy/utils/Initializable.sol";
import {Ownable} from "@openzeppelin/contracts/access/Ownable.sol";

import {ILLM} from "./ILLM.sol";

abstract contract PrimitiveBase is Ownable, Initializable {
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
  string public name;
  string public customRules;

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
  constructor(address llmPrecompile, string memory metadata) Ownable() {
    LLM_PRECOMPILE = ILLM(llmPrecompile);
    _metadata = metadata;

    _IMPLEMENTATION_ADDRESS = address(this);

    // publish new primitive to llm
    // LLM_PRECOMPILE.publishPrimitive(_IMPLEMENTATION_ADDRESS, metadata);
  }

  /**
   * @dev Initializes base contract data. Sets ownership to given address, name and custom rules
   * for custom primitives and proxy address.
   * @param owner custom primitive owner address
   * @param name_ custom primitive name
   * @param customRules_ custom primitive rules
   */
  // slither-disable-next-line naming-convention
  function __Primitive_init(
    // solhint-disable-previous-line
    address owner,
    string calldata name_,
    string calldata customRules_
  ) internal initializer {
    if (address(this) == _IMPLEMENTATION_ADDRESS) revert OnlyProxy();
    _proxy = address(this);
    _transferOwnership(owner);

    name = name_;
    customRules = customRules_;

    _publishCustomPrimitive();
  }

  /**
   * @dev Publishes new custom primitive to LLM precompile.
   */
  function _publishCustomPrimitive() private {
    LLM_PRECOMPILE.publishCustomPrimitive(address(this), _IMPLEMENTATION_ADDRESS);
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
  function getInfo() external view returns (string memory, string memory) {
    return (name, customRules);
  }
}
