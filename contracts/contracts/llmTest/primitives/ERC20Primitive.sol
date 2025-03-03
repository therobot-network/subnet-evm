//SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

import "@openzeppelin/contracts/token/ERC20/ERC20.sol";
import {Strings} from "@openzeppelin/contracts/utils/Strings.sol";

contract ERC20Primitive is ERC20 {
  error EmptyInputString();
  error InvalidString(string reason);

  constructor() ERC20("Test Token", "TT") {
    _mint(msg.sender, 10000 * 10 ** 18);
  }

  // Converts an unsigned integer to a fixed-point string
  function contractFormatToUserFormat(uint256 userInteger) public view returns (string memory) {
    if (userInteger == 0) return "0";

    uint8 decimals = decimals();
    uint256 factor = 10 ** uint256(decimals);

    // Integer and decimal parts
    uint256 intPart = userInteger / factor;
    uint256 decPart = userInteger % factor;

    // Convert integer and decimal parts to strings using OpenZeppelin's `Strings.toString`
    string memory intString = Strings.toString(intPart);
    string memory decString = Strings.toString(decPart);

    // Ensure decimal part has leading zeros if needed
    while (bytes(decString).length < decimals) {
      decString = string(abi.encodePacked("0", decString));
    }

    return string(abi.encodePacked(intString, ".", decString));
  }

  // Converts a fixed-point string to an unsigned integer
  function userFormatToContractFormat(string memory userFixedPointString) public view returns (uint256) {
    bytes memory strBytes = bytes(userFixedPointString);
    if (strBytes.length == 0) revert EmptyInputString();

    uint8 decimals = decimals();
    uint256 integerPart = 0;
    uint256 decimalPart = 0;
    uint256 factor = 10 ** uint256(decimals);
    bool hasDecimal = false;
    uint256 decimalDigits = 0;

    for (uint256 i = 0; i < strBytes.length; i++) {
      if (strBytes[i] == ".") {
        if (hasDecimal) revert InvalidString("Multiple decimal points found");
        hasDecimal = true;
        continue;
      }

      if (strBytes[i] < "0" || strBytes[i] > "9") revert InvalidString("Invalid character");

      if (!hasDecimal) {
        integerPart = integerPart * 10 + (uint256(uint8(strBytes[i])) - 48);
      } else {
        if (decimalDigits < decimals) {
          decimalPart = decimalPart * 10 + (uint256(uint8(strBytes[i])) - 48);
          decimalDigits++;
        }
      }
    }

    while (decimalDigits < decimals) {
      decimalPart *= 10;
      decimalDigits++;
    }

    return (integerPart * factor) + decimalPart;
  }
}
