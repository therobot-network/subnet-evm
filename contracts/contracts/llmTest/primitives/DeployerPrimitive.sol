// SPDX-License-Identifier: MIT

pragma solidity ^0.8.20;

import {Initializable} from "@openzeppelin/contracts-upgradeable/proxy/utils/Initializable.sol";
import {ReentrancyGuard} from "@openzeppelin/contracts/utils/ReentrancyGuard.sol";

import {PrimitiveBase} from "./PrimitiveBase.sol";
import {RobotContract} from "./RobotContract.sol";

contract DeployerPrimitive is Initializable, PrimitiveBase, ReentrancyGuard {
  event RobotContractDeployed(address indexed contractAddress, address indexed primitiveContract);

  constructor(
    address llmPrecompile,
    string memory metadata,
    address primitiveStorageAddress
  ) PrimitiveBase(llmPrecompile, "deployer", metadata, primitiveStorageAddress) {}

  function initialize(address owner, string calldata name_, string calldata customRules_) external initializer {
    __Primitive_init(owner, name_, customRules_);
  }

  function deployRobotContract(
    address primitiveAddress,
    bytes calldata initData
  ) external nonReentrant returns (address customPrimitive) {
    RobotContract customPrimitiveContract = new RobotContract(primitiveAddress, initData);

    customPrimitive = address(customPrimitiveContract);
    emit RobotContractDeployed(customPrimitive, primitiveAddress);
  }
}
