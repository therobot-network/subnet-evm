// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import {Math} from "@openzeppelin/contracts/utils/math/Math.sol";
import {SystemPrimitiveBase} from "./SystemPrimitiveBase.sol";
import {UserDecimalFormatting} from "../../libraries/UserDecimalFormatting.sol";

contract PythonPrimitive is SystemPrimitiveBase {
    uint8 public constant DECIMALS = 18;
    error DivisionByZero();
    error Overflow();
    error ArrayCannotBeEmpty();

    constructor(
        address llmPrecompile,
        string memory metadata
    ) SystemPrimitiveBase(llmPrecompile, "PythonPrimitive", metadata) {}

    function add(
        string memory a,
        string memory b
    ) public pure returns (string memory) {
        uint256 aUint = UserDecimalFormatting.userFormatToContractFormat(
            a,
            DECIMALS
        );
        uint256 bUint = UserDecimalFormatting.userFormatToContractFormat(
            b,
            DECIMALS
        );
        (bool success, uint256 sum) = Math.tryAdd(aUint, bUint);
        if (!success) revert Overflow();
        return UserDecimalFormatting.contractFormatToUserFormat(sum, DECIMALS);
    }

    function subtract(
        string memory a,
        string memory b
    ) public pure returns (string memory) {
        uint256 aUint = UserDecimalFormatting.userFormatToContractFormat(
            a,
            DECIMALS
        );
        uint256 bUint = UserDecimalFormatting.userFormatToContractFormat(
            b,
            DECIMALS
        );
        (bool success, uint256 result) = Math.trySub(aUint, bUint);
        if (!success) revert Overflow();
        return
            UserDecimalFormatting.contractFormatToUserFormat(result, DECIMALS);
    }

    function multiply(
        string memory a,
        string memory b
    ) public pure returns (string memory) {
        uint256 aUint = UserDecimalFormatting.userFormatToContractFormat(
            a,
            DECIMALS
        );
        uint256 bUint = UserDecimalFormatting.userFormatToContractFormat(
            b,
            DECIMALS
        );
        (bool success, uint256 result) = Math.tryMul(aUint, bUint);
        if (!success) revert Overflow();
        uint256 scaled = result / (10 ** DECIMALS);
        return
            UserDecimalFormatting.contractFormatToUserFormat(scaled, DECIMALS);
    }

    function divide(
        string memory a,
        string memory b
    ) public pure returns (string memory) {
        uint256 aUint = UserDecimalFormatting.userFormatToContractFormat(
            a,
            DECIMALS
        );
        uint256 bUint = UserDecimalFormatting.userFormatToContractFormat(
            b,
            DECIMALS
        );
        (bool success, uint256 result) = Math.tryDiv(aUint, bUint);
        result = result * (10 ** DECIMALS);
        if (!success) revert DivisionByZero();
        return
            UserDecimalFormatting.contractFormatToUserFormat(result, DECIMALS);
    }

    function mod(
        string memory a,
        string memory b
    ) public pure returns (string memory) {
        uint256 aUint = UserDecimalFormatting.userFormatToContractFormat(
            a,
            DECIMALS
        );
        uint256 bUint = UserDecimalFormatting.userFormatToContractFormat(
            b,
            DECIMALS
        );
        (bool success, uint256 result) = Math.tryMod(aUint, bUint);
        if (!success) revert DivisionByZero();
        return
            UserDecimalFormatting.contractFormatToUserFormat(result, DECIMALS);
    }

    function pow(
        string memory base,
        string memory exponent
    ) public pure returns (string memory) {
        uint256 baseUint = UserDecimalFormatting.userFormatToContractFormat(
            base,
            DECIMALS
        );
        uint256 expUint = UserDecimalFormatting.userFormatToContractFormat(
            exponent,
            DECIMALS
        ) / (10 ** DECIMALS);
        uint256 result = baseUint ** expUint;
        return
            UserDecimalFormatting.contractFormatToUserFormat(result, DECIMALS);
    }

    function max2(
        string memory a,
        string memory b
    ) public pure returns (string memory) {
        return
            UserDecimalFormatting.contractFormatToUserFormat(
                Math.max(
                    UserDecimalFormatting.userFormatToContractFormat(
                        a,
                        DECIMALS
                    ),
                    UserDecimalFormatting.userFormatToContractFormat(
                        b,
                        DECIMALS
                    )
                ),
                DECIMALS
            );
    }

    function min2(
        string memory a,
        string memory b
    ) public pure returns (string memory) {
        return
            UserDecimalFormatting.contractFormatToUserFormat(
                Math.min(
                    UserDecimalFormatting.userFormatToContractFormat(
                        a,
                        DECIMALS
                    ),
                    UserDecimalFormatting.userFormatToContractFormat(
                        b,
                        DECIMALS
                    )
                ),
                DECIMALS
            );
    }

    function max(string[] memory arr) public pure returns (string memory) {
        if (arr.length == 0) revert ArrayCannotBeEmpty();
        uint256 maxVal = UserDecimalFormatting.userFormatToContractFormat(
            arr[0],
            DECIMALS
        );
        for (uint256 i = 1; i < arr.length; i++) {
            uint256 val = UserDecimalFormatting.userFormatToContractFormat(
                arr[i],
                DECIMALS
            );
            if (val > maxVal) maxVal = val;
        }
        return
            UserDecimalFormatting.contractFormatToUserFormat(maxVal, DECIMALS);
    }

    function min(string[] memory arr) public pure returns (string memory) {
        if (arr.length == 0) revert ArrayCannotBeEmpty();
        uint256 minVal = UserDecimalFormatting.userFormatToContractFormat(
            arr[0],
            DECIMALS
        );
        for (uint256 i = 1; i < arr.length; i++) {
            uint256 val = UserDecimalFormatting.userFormatToContractFormat(
                arr[i],
                DECIMALS
            );
            if (val < minVal) minVal = val;
        }
        return
            UserDecimalFormatting.contractFormatToUserFormat(minVal, DECIMALS);
    }

    function abs(string memory x) public pure returns (string memory) {
        int256 val = int256(
            UserDecimalFormatting.userFormatToContractFormat(x, DECIMALS)
        );
        uint256 result = uint256(val < 0 ? -val : val);
        return
            UserDecimalFormatting.contractFormatToUserFormat(result, DECIMALS);
    }

    function ceilDiv(
        string memory a,
        string memory b
    ) public pure returns (string memory) {
        uint256 aUint = UserDecimalFormatting.userFormatToContractFormat(
            a,
            DECIMALS
        );
        uint256 bUint = UserDecimalFormatting.userFormatToContractFormat(
            b,
            DECIMALS
        );
        uint256 result = Math.ceilDiv(aUint, bUint);
        return UserDecimalFormatting.contractFormatToUserFormat(result, 0);
    }

    function sqrt(string memory a) public pure returns (string memory) {
        uint256 aUint = UserDecimalFormatting.userFormatToContractFormat(
            a,
            DECIMALS
        );
        uint256 result = Math.sqrt(aUint);
        return
            UserDecimalFormatting.contractFormatToUserFormat(
                result,
                DECIMALS / 2
            );
    }

    function mulDiv(
        string memory x,
        string memory y,
        string memory denominator
    ) public pure returns (string memory) {
        uint256 xUint = UserDecimalFormatting.userFormatToContractFormat(
            x,
            DECIMALS
        );
        uint256 yUint = UserDecimalFormatting.userFormatToContractFormat(
            y,
            DECIMALS
        );
        uint256 dUint = UserDecimalFormatting.userFormatToContractFormat(
            denominator,
            DECIMALS
        );
        uint256 result = Math.mulDiv(xUint, yUint, dUint);
        return
            UserDecimalFormatting.contractFormatToUserFormat(result, DECIMALS);
    }

    // Boolean comparison functions (unchanged)
    function greaterThan(
        string memory a,
        string memory b
    ) public pure returns (bool) {
        return
            UserDecimalFormatting.userFormatToContractFormat(a, DECIMALS) >
            UserDecimalFormatting.userFormatToContractFormat(b, DECIMALS);
    }

    function greaterThanOrEqual(
        string memory a,
        string memory b
    ) public pure returns (bool) {
        return
            UserDecimalFormatting.userFormatToContractFormat(a, DECIMALS) >=
            UserDecimalFormatting.userFormatToContractFormat(b, DECIMALS);
    }

    function lessThan(
        string memory a,
        string memory b
    ) public pure returns (bool) {
        return
            UserDecimalFormatting.userFormatToContractFormat(a, DECIMALS) <
            UserDecimalFormatting.userFormatToContractFormat(b, DECIMALS);
    }

    function lessThanOrEqual(
        string memory a,
        string memory b
    ) public pure returns (bool) {
        return
            UserDecimalFormatting.userFormatToContractFormat(a, DECIMALS) <=
            UserDecimalFormatting.userFormatToContractFormat(b, DECIMALS);
    }

    function equal(
        string memory a,
        string memory b
    ) public pure returns (bool) {
        return
            UserDecimalFormatting.userFormatToContractFormat(a, DECIMALS) ==
            UserDecimalFormatting.userFormatToContractFormat(b, DECIMALS);
    }

    function notEqual(
        string memory a,
        string memory b
    ) public pure returns (bool) {
        return
            UserDecimalFormatting.userFormatToContractFormat(a, DECIMALS) !=
            UserDecimalFormatting.userFormatToContractFormat(b, DECIMALS);
    }
}
