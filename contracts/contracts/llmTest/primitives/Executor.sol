// SPDX-License-Identifier: MIT

pragma solidity ^0.8.20;

import {Ownable} from "@openzeppelin/contracts/access/Ownable.sol";
import {ReentrancyGuard} from "@openzeppelin/contracts/utils/ReentrancyGuard.sol";

import {ILLM} from "../../interfaces/ILLM.sol";
import {IExecutor} from "./interfaces/IExecutor.sol";
import {CustomPrimitive} from "./CustomPrimitive.sol";

interface ICustomPrimitive {
  function getInfo() external view returns (string memory name, string memory customRules);
}

contract Executor is Ownable, ReentrancyGuard, IExecutor {
  struct PlanResults {
    address contractAddress;
    bool success;
    bytes resultData;
  }

  struct DeployedContract {
    string name;
    address contractAddress;
  }

  // LLM precompile contract address
  // slither-disable-next-line naming-convention
  ILLM public immutable LLM;

  address private _msgSigner;

  // list of all deployed custom primitive contracts
  DeployedContract[] public deployedContracts;

  event CustomPrimitiveDeployed(string name, address indexed contractAddress, address indexed primitiveContract);
  event PromptCompleted(string prompt);
  event PlanCompleted(string plan);
  event Plan(uint promptId, ILLM.ContractMethodParams[] plan);
  event ActionFailed(uint256 step, bytes data);

  error InvalidPrecompileAddress();

  constructor(address llmPrecompile) Ownable(msg.sender) {
    if (llmPrecompile == address(0)) revert InvalidPrecompileAddress();
    LLM = ILLM(llmPrecompile);
  }

  /**
   * @dev Deploys a new custom primitive contract for the given primitive address.
   * A new custom proxy contract is deployed using primitive address as implementation.
   * @param primitiveAddress primitive contract address
   * @param initData bytes calldata for initializing the proxy contract
   */
  function deployCustomPrimitive(
    address primitiveAddress,
    bytes calldata initData
  ) external nonReentrant returns (address customPrimitive) {
    /// @dev deploys a new custom primitive proxy contract
    /// with primitive address as implementation
    CustomPrimitive customPrimitiveContract = new CustomPrimitive(primitiveAddress, initData);

    customPrimitive = address(customPrimitiveContract);

    // slither-disable-next-line unused-return
    (string memory name, ) = ICustomPrimitive(customPrimitive).getInfo();
    // slither-disable-next-line reentrancy-benign
    _newCustomContract(name, primitiveAddress, customPrimitive);
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

  // /**
  //  * @dev Executes given plan json by sending it LLM and executing them.
  //  * @param plan string plan object
  //  */
  // function evalFunction(
  //     string calldata plan,
  //     string[] calldata types,
  //     bytes[] calldata lookupValues
  // ) external nonReentrant {
  //     _eval(plan, true);
  //     emit PlanCompleted(plan);
  // }

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

  function _newCustomContract(string memory name, address primitive, address contract_) private {
    DeployedContract storage deployed = deployedContracts.push();
    deployed.name = name;
    deployed.contractAddress = contract_;

    emit CustomPrimitiveDeployed(name, contract_, primitive);
  }

  /**
   * @dev Executes given plan json or prompt by sending it LLM and executing them.
   * @param request plan/prompt string
   * @param isPlan bool flag indicating if string is plan or prompt
   */
  function _eval(
    string calldata request,
    bool isPlan // returns (bytes memory)
  ) private {
    // save msg signer in storage
    _msgSigner = msg.sender;

    ILLM.ContractMethodParams[] memory plan;
    uint promptId;
    // call LLM precompile to get the plan
    (promptId, plan) = isPlan ? LLM.evaluatePlan(request) : LLM.evaluatePrompt(request);

    bool isEvaluationDone = false;
    // execute plan and call LLM's continueEvaluation
    while (!isEvaluationDone) {
      emit Plan(promptId, plan);
      bytes[] memory results = _executePlan(plan);

      // send results back to LLM
      // slither-disable-next-line calls-loop
      (isEvaluationDone, plan) = LLM.continueEvaluation(promptId, results);
    }
    // return plan[0].methodData;
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
}
