// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

contract RobotDemo {
    event PromptEvent(string promptString);
    /**
     * @param prompt Prompt for robot brain.
     */
    function execute(string calldata prompt) external {
        emit PromptEvent(prompt);
    }
}
