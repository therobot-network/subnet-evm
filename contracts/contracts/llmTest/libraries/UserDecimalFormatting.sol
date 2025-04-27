// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import {Strings} from "@openzeppelin/contracts/utils/Strings.sol";

library UserDecimalFormatting {
    error EmptyInputString();
    error InvalidString(string reason);

    function contractFormatToUserFormat(
        uint256 userInteger,
        uint8 decimals
    ) internal pure returns (string memory) {
        if (userInteger == 0) return "0";

        uint256 factor = 10 ** uint256(decimals);
        uint256 intPart = userInteger / factor;
        uint256 decPart = userInteger % factor;

        string memory intString = Strings.toString(intPart);
        string memory decString = Strings.toString(decPart);

        while (bytes(decString).length < decimals) {
            decString = string(abi.encodePacked("0", decString));
        }

        return string(abi.encodePacked(intString, ".", decString));
    }

    // Converts a fixed-point string to an unsigned integer
    function userFormatToContractFormat(
        string memory userFixedPointString,
        uint8 decimals
    ) internal pure returns (uint256) {
        bytes memory strBytes = bytes(userFixedPointString);
        if (strBytes.length == 0) revert EmptyInputString();

        uint256 integerPart = 0;
        uint256 decimalPart = 0;
        uint256 factor = 10 ** uint256(decimals);
        bool hasDecimal = false;
        uint256 decimalDigits = 0;

        for (uint256 i = 0; i < strBytes.length; i++) {
            if (strBytes[i] == ".") {
                if (hasDecimal)
                    revert InvalidString("Multiple decimal points found");
                hasDecimal = true;
                continue;
            }

            if (strBytes[i] < "0" || strBytes[i] > "9")
                revert InvalidString("Invalid character");

            if (!hasDecimal) {
                integerPart =
                    integerPart *
                    10 +
                    (uint256(uint8(strBytes[i])) - 48);
            } else {
                if (decimalDigits < decimals) {
                    decimalPart =
                        decimalPart *
                        10 +
                        (uint256(uint8(strBytes[i])) - 48);
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
