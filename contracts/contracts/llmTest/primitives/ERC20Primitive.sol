// SPDX-License-Identifier: MIT

pragma solidity ^0.8.20;

import {ERC20Upgradeable} from "@openzeppelin/contracts-upgradeable/token/ERC20/ERC20Upgradeable.sol";
import {ContextUpgradeable} from "@openzeppelin/contracts-upgradeable/utils/ContextUpgradeable.sol";
import {Initializable} from "@openzeppelin/contracts-upgradeable/proxy/utils/Initializable.sol";
import {Strings} from "@openzeppelin/contracts/utils/Strings.sol";

import {PrimitiveBase} from "./PrimitiveBase.sol";

contract ERC20Primitive is Initializable, PrimitiveBase, ERC20Upgradeable {
    uint256 public constant MINT_AMOUNT = 100 * 1e18; // 10 tokens
    uint256 public constant MINT_TIME_LIMIT = 3600; // 1 hour in seconds

    mapping(address => uint256) private _requestLog;

    error EmptyInputString();
    error InvalidString(string reason);
    error RequestAfterSometime(uint256 timestamp);

    constructor(
        address llmPrecompile,
        string memory metadata,
        address primitiveStorageAddress
    )
        PrimitiveBase(llmPrecompile, "erc20", metadata, primitiveStorageAddress)
    {}

    function initialize(
        address owner,
        string calldata symbol,
        uint256 amount,
        string calldata name_,
        string calldata customRules_
    ) external initializer {
        __Primitive_init(owner, name_, customRules_);
        __ERC20_init(name_, symbol);
        _mint(owner, amount);
    }

    // users other than owner can mint only a max of 100 tokens in a request
    // users can request only once per hour
    function mint(uint256 amount) external onlyProxy {
        address sender = _msgSender();
        if (sender != owner()) {
            // temp faucet for testnet
            // slither-disable-next-line timestamp
            if (block.timestamp < _requestLog[sender] + MINT_TIME_LIMIT)
                revert RequestAfterSometime(MINT_TIME_LIMIT);

            _requestLog[sender] = block.timestamp;
            // users can mint only a max of MINT_AMOUNT
            amount = amount > MINT_AMOUNT ? MINT_AMOUNT : amount;
        }

        _mint(sender, amount);
    }

    // Converts an unsigned integer to a fixed-point string
    function contractFormatToUserFormat(
        uint256 userInteger
    ) public view returns (string memory) {
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
    function userFormatToContractFormat(
        string memory userFixedPointString
    ) public view returns (uint256) {
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

    function _msgSender()
        internal
        view
        override(ContextUpgradeable, PrimitiveBase)
        returns (address)
    {
        return PrimitiveBase._msgSender();
    }
}
