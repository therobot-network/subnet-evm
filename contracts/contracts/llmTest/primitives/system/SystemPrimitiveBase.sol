// SPDX-License-Identifier: MIT

pragma solidity ^0.8.20;

import {Ownable} from "@openzeppelin/contracts/access/Ownable.sol";

import {ILLM} from "../../../interfaces/ILLM.sol";

abstract contract SystemPrimitiveBase is Ownable {
  // llm precompile contract address for publishing new primitive
  // slither-disable-next-line naming-convention
  ILLM public immutable LLM_PRECOMPILE_BASE;
  // implementation contract address used for restricting direct initialization

  /**
   * @dev Initializes data and publishes primitive to LLM precompile.
   * @param llmPrecompile LLM precompile address (TODO: hardcode address)
   * @param metadata ipfs hash
   */
  constructor(address llmPrecompile, string memory name, string memory metadata) Ownable(msg.sender) {
    LLM_PRECOMPILE_BASE = ILLM(llmPrecompile);

    // publish system primitive to PCC
    LLM_PRECOMPILE_BASE.publishSystemPrimitive(address(this), name, metadata);
  }
}
