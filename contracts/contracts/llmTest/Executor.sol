// SPDX-License-Identifier: MIT

pragma solidity ^0.8.20;
// import "hardhat/console.sol";

import {Ownable} from "@openzeppelin/contracts/access/Ownable.sol";
import {ReentrancyGuard} from "@openzeppelin/contracts/utils/ReentrancyGuard.sol";

import {ILLM} from "./interfaces/ILLM.sol";
import {IExecutor, IRobotStorage, IRobotStateEmitter} from "./interfaces/IExecutor.sol";
import {RobotContract} from "./RobotContract.sol";

interface IRobotContract {
  function getInfo()
    external
    view
    returns (string memory contractName, string memory customRules, string memory primitiveName);

  function getRobotState() external view returns (IRobotStateEmitter.StateChangePayload memory);
}

contract Executor is Ownable, ReentrancyGuard, IExecutor, IRobotStorage, IRobotStateEmitter {
  struct PlanResults {
    address contractAddress;
    bool success;
    bytes resultData;
  }

  struct DeployedContract {
    string contractName;
    address contractAddress;
    string primitiveName;
    address primitiveAddress;
  }

  struct PrimitiveInfo {
    bool exists;
    address implementation;
    string metadata;
  }

  // LLM precompile contract address
  // slither-disable-next-line naming-convention
  ILLM public immutable LLM;

  address private _msgSigner;

  // list of all deployed custom primitive contracts
  DeployedContract[] public deployedContracts;

  // Mapping to track published primitives by name
  mapping(string => PrimitiveInfo) public primitives;

  // Mapping to track robot contracts by indeces in deployedContracts
  mapping(address => uint256) public deployedContractIndexByAddress;

  // Event to be emitted when a primitive is published
  event PrimitivePublished(
    address indexed publisher,
    address implementationAddress,
    string primitiveName,
    string metadata
    // uint256 publishedTimestamp
  );

  event RobotContractDeployed(
    string contractName,
    address indexed contractAddress,
    string primitiveName
    // uint256 deploymentTimestamp
  );

  event PromptCompleted(string prompt);
  event PlanCompleted(string plan);
  event Plan(uint promptId, ILLM.ContractMethodParams[] plan);
  event ActionFailed(uint256 step, bytes data);
  event StateChange(address robotContract, string primitiveName, StateChangePayload stateChangePayload);

  error InvalidPrecompileAddress();
  error InvalidImplementationAddress();
  error PrimitiveAlreadyPublished(string name);
  error PrimitiveNotPublished(string name);
  error InvalidRobotContract();

  constructor(address llmPrecompile) Ownable(msg.sender) {
    if (llmPrecompile == address(0)) revert InvalidPrecompileAddress();
    LLM = ILLM(llmPrecompile);
  }

  function getLlm() external view returns (ILLM) {
    return LLM;
  }

  function getPrimitive(string memory name) external view returns (address primitiveAddress, string memory metadata) {
    PrimitiveInfo storage primitive = primitives[name];
    if (!primitive.exists) {
      revert PrimitiveNotPublished(name);
    }
    return (primitive.implementation, primitive.metadata);
  }

  /**
   * @dev Deploys a new custom primitive contract for the given primitive address.
   * A new custom proxy contract is deployed using primitive address as implementation.
   * @param primitiveName primitive contract name
   * @param initData bytes calldata for initializing the proxy contract
   */
  function deployRobotContract(
    string calldata primitiveName,
    bytes calldata initData
  ) external nonReentrant returns (address customPrimitive) {
    if (!primitives[primitiveName].exists) {
      revert PrimitiveNotPublished(primitiveName);
    }
    /// @dev deploys a new custom primitive proxy contract
    /// with primitive address as implementation
    RobotContract customPrimitiveContract = new RobotContract(primitives[primitiveName].implementation, address(this));
    customPrimitiveContract.robotInitialize(initData);

    customPrimitive = address(customPrimitiveContract);

    // slither-disable-next-line unused-return
    (string memory name, , ) = IRobotContract(customPrimitive).getInfo();

    // slither-disable-next-line reentrancy-benign
    _newCustomContract(name, customPrimitive, primitiveName);
  }

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
  ) external nonReentrant {
    if (implementationAddress == address(0)) {
      revert InvalidImplementationAddress();
    }

    if (primitives[name].exists) {
      revert PrimitiveAlreadyPublished(name);
    }

    primitives[name] = PrimitiveInfo({implementation: implementationAddress, metadata: metadata, exists: true});

    emit PrimitivePublished(
      msg.sender,
      implementationAddress,
      name,
      metadata
      // block.timestamp
    );
  }

  /**
   * @dev Executes user prompt by getting the plan from LLM precompile and executing them.
   * @param prompt string user prompt
   */
  function evalPrompt(string calldata prompt) external nonReentrant {
    _eval(prompt, false);
    emit PromptCompleted(prompt);
  }

  /**
   * @dev Executes given plan json by sending it LLM and executing them.
   * @param plan string plan object
   */
  function evalPlan(string calldata plan) external nonReentrant {
    _eval(plan, true);
    emit PlanCompleted(plan);
  }

  /**
   * @dev Gets the msg signer from the storage.
   * @return msgSigner message signer address
   */
  function getMsgSigner() external view returns (address msgSigner) {
    return _msgSigner;
  }

  function getDeployedContracts() external view returns (DeployedContract[] memory) {
    return deployedContracts;
  }

  function _newCustomContract(string memory contractName, address contract_, string memory primitiveName) private {
    uint256 index = deployedContracts.length;

    DeployedContract storage deployed = deployedContracts.push();
    deployed.contractName = contractName;
    deployed.contractAddress = contract_;
    deployed.primitiveName = primitiveName;
    deployed.primitiveAddress = primitives[primitiveName].implementation;

    deployedContractIndexByAddress[contract_] = index;

    emit RobotContractDeployed(
      contractName,
      contract_,
      primitiveName
      // block.timestamp
    );
    IRobotStateEmitter.StateChangePayload memory iniitalState = IRobotContract(contract_).getRobotState();
    emit StateChange(contract_, primitiveName, iniitalState);
  }

  /**
   * @dev Executes given plan json or prompt by sending it LLM and executing them.
   * @param request plan/prompt string
   * @param isPlan bool flag indicating if string is plan or prompt
   */
  function _eval(string calldata request, bool isPlan) private {
    // save msg signer in storage
    _msgSigner = msg.sender;

    ILLM.ContractMethodParams[] memory plan;
    uint promptId;
    bool isEvaluationDone;
    // call LLM precompile to get the plan
    (promptId, isEvaluationDone, plan) = isPlan ? LLM.evaluatePlan(request) : LLM.evaluatePrompt(request);

    // execute plan and call LLM's continueEvaluation
    while (!isEvaluationDone) {
      emit Plan(promptId, plan);

      if (plan.length == 0) {
        break;
      }

      bytes[] memory results = _executePlan(plan);

      // send results back to LLM
      // slither-disable-next-line calls-loop
      (isEvaluationDone, plan) = LLM.continueEvaluation(promptId, results);
    }
  }

  /**
   * @dev Executes plan by calling contract functions using bytes call data and returns bytes results.
   * @param plan list of contract addresses and calldata
   * @return results list of function call results in bytes
   */
  function _executePlan(ILLM.ContractMethodParams[] memory plan) private returns (bytes[] memory results) {
    uint len = plan.length;
    results = new bytes[](len);

    for (uint i = 0; i < len; i++) {
      bool success;
      // call contract function using bytes method data
      // TODO: handle failure
      // slither-disable-next-line calls-loop
      (success, results[i]) = plan[i].contractAddress.call(
        // solhint-disable-previous-line
        plan[i].methodData
      );
      if (!success) emit ActionFailed(i, results[i]);
    }
  }

  /**
   * @dev Sends data regarding a state change in a contract.
   */
  function emitStateChange(StateChangePayload calldata stateChangePayload) external {
    uint256 index = deployedContractIndexByAddress[msg.sender];
    if (index >= deployedContracts.length || deployedContracts[index].contractAddress != msg.sender) {
      revert InvalidRobotContract();
    }

    emit StateChange(msg.sender, deployedContracts[index].primitiveName, stateChangePayload);
  }
}
