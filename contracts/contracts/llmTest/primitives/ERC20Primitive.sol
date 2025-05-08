// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

// import "hardhat/console.sol";

import {ERC20Upgradeable} from "@openzeppelin/contracts-upgradeable/token/ERC20/ERC20Upgradeable.sol";
// import {ReentrancyGuardUpgradeable} from "@openzeppelin/contracts-upgradeable/utils/ReentrancyGuardUpgradeable.sol";
import {ContextUpgradeable} from "@openzeppelin/contracts-upgradeable/utils/ContextUpgradeable.sol";
import {Initializable} from "@openzeppelin/contracts-upgradeable/proxy/utils/Initializable.sol";
import {PrimitiveBase} from "./PrimitiveBase.sol";
import {IRobotStateEmitter} from "../interfaces/IExecutor.sol";
import {IERC20Primitive} from "./interfaces/IERC20Primitive.sol";
import {UserDecimalFormatting} from "../libraries/UserDecimalFormatting.sol";

contract ERC20Primitive is
    Initializable,
    PrimitiveBase,
    ERC20Upgradeable,
    IERC20Primitive
{
    uint256 public constant MINT_AMOUNT = 100 * 1e18; // 100 tokens
    // uint256 public constant INITIAL_MINT_AMOUNT = 1000 * 1e18; // 1000 tokens
    uint256 public constant MINT_TIME_LIMIT = 3600; // 1 hour

    mapping(address => uint256) private _requestLog;
    mapping(address => bool) private _hasBalance;
    uint256 private _holderCount;
    uint256 private _burned;
    uint256 private _transferCount;
    uint256 private _transferred;

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

    /**
     * @dev Initializes base contract data. Sets ownership to given address, name and custom rules
     * for custom primitives and proxy address.
     * @param owner_ custom primitive owner address
     * @param customRules custom primitive rules
     */
    // slither-disable-next-line naming-convention
    function robotContractInit(
        // solhint-disable-previous-line
        address owner_,
        string calldata customRules
    ) public override initializer {
        _robotContractBaseInit(owner_, customRules);
        __ERC20_init(
            string.concat("ROBOT_DEPLOYED_", _getContractName()),
            _getContractName()
        );
        // _mint(owner(), INITIAL_MINT_AMOUNT);
        // _updateHolder(owner());
    }

    // // keccak256(abi.encode(uint256(keccak256("openzeppelin.storage.ERC20")) - 1)) & ~bytes32(uint256(0xff))
    // bytes32 private constant _ERC20_STORAGE_LOCATION =
    //     0x52c63247e1f47db19d5ce0460030c497f067ca4cebf71ba98eeadabe20bace00;

    // function _getStorage() private pure returns (ERC20Storage storage $) {
    //     // slither-disable-next-line timestamp
    //     assembly {
    //         $.slot := _ERC20_STORAGE_LOCATION
    //     }
    // }

    // function _setDescriptor(string memory name_) internal {
    //     ERC20Storage storage $ = _getStorage();
    //     $._name = name_;
    // }

    function configure(
        // string memory descriptor,
        string calldata amount
    ) external onlyOwner {
        // _setDescriptor(descriptor);
        mintFloat(amount);
    }

    // users other than owner can mint only a max of 100 tokens in a request
    // users can request only once per hour
    function mint(uint256 amount) public onlyProxy {
        address sender = _msgSender();
        if (sender != owner()) {
            // slither-disable-next-line timestamp
            if (block.timestamp < _requestLog[sender] + MINT_TIME_LIMIT)
                revert RequestAfterSometime(MINT_TIME_LIMIT);

            _requestLog[sender] = block.timestamp;
            // users can mint only a max of MINT_AMOUNT
            amount = amount > MINT_AMOUNT ? MINT_AMOUNT : amount;
        }

        _mint(sender, amount);
        _emitMintStateChangeSupply(_updateHolder(sender));
    }

    function mintFloat(string calldata amount) public onlyProxy {
        if (bytes(amount).length == 0) revert EmptyInputString();
        uint256 amountInContractFormat = userFormatToContractFormat(amount);
        mint(amountInContractFormat);
    }

    function _emitMintStateChangeSupply(bool holderChanged) internal {
        IRobotStateEmitter.StateChangePayload
            memory payload = IRobotStateEmitter.StateChangePayload({
                uints: new IRobotStateEmitter.NamedUint[](0),
                floats: new IRobotStateEmitter.NamedFloat[](1),
                strings: new IRobotStateEmitter.NamedString[](0),
                addresses: new IRobotStateEmitter.NamedAddress[](0),
                bools: new IRobotStateEmitter.NamedBool[](0)
            });

        uint idx = 0;
        payload.uints = new IRobotStateEmitter.NamedUint[](
            holderChanged ? 1 : 0
        );
        if (holderChanged) {
            payload.uints[idx++] = IRobotStateEmitter.NamedUint(
                "numTokenHolders",
                _holderCount
            );
        }

        payload.floats[0] = IRobotStateEmitter.NamedFloat(
            "totalSupply",
            contractFormatToUserFormat(totalSupply())
        );

        _getRobotStateEmitter().emitStateChange(payload);
    }

    function burn(uint256 amount) public onlyProxy {
        address sender = _msgSender();

        _burn(sender, amount);

        // slither-disable-next-line events-maths
        _burned += amount;
        bool holderChanged = _updateHolder(sender);

        IRobotStateEmitter.StateChangePayload
            memory payload = IRobotStateEmitter.StateChangePayload({
                uints: new IRobotStateEmitter.NamedUint[](0),
                floats: new IRobotStateEmitter.NamedFloat[](2),
                strings: new IRobotStateEmitter.NamedString[](0),
                addresses: new IRobotStateEmitter.NamedAddress[](0),
                bools: new IRobotStateEmitter.NamedBool[](0)
            });

        if (holderChanged) {
            payload.uints = new IRobotStateEmitter.NamedUint[](1);
            payload.uints[0] = IRobotStateEmitter.NamedUint(
                "numTokenHolders",
                _holderCount
            );
        }

        payload.floats[0] = IRobotStateEmitter.NamedFloat(
            "totalSupply",
            contractFormatToUserFormat(totalSupply())
        );
        payload.floats[1] = IRobotStateEmitter.NamedFloat(
            "amountBurned",
            contractFormatToUserFormat(_burned)
        );

        _getRobotStateEmitter().emitStateChange(payload);
    }

    function burnFloat(string calldata amount) external onlyProxy {
        if (bytes(amount).length == 0) revert EmptyInputString();
        uint256 amountInContractFormat = userFormatToContractFormat(amount);
        burn(amountInContractFormat);
    }

    function transfer(
        address to,
        uint256 amount
    ) public override onlyProxy returns (bool) {
        address sender = _msgSender();
        bool success = super.transfer(to, amount);
        if (success) {
            _transferCount++;
            // slither-disable-start events-maths
            // solhint-disable-next-line reentrancy
            _transferred += amount;
            // slither-disable-end events-maths
            bool senderChanged = _updateHolder(sender);
            bool recipientChanged = _updateHolder(to);

            uint uintLen = 1 +
                (senderChanged ? 1 : 0) +
                (recipientChanged ? 1 : 0);

            IRobotStateEmitter.StateChangePayload
                memory payload = IRobotStateEmitter.StateChangePayload({
                    uints: new IRobotStateEmitter.NamedUint[](uintLen),
                    floats: new IRobotStateEmitter.NamedFloat[](1),
                    strings: new IRobotStateEmitter.NamedString[](0),
                    addresses: new IRobotStateEmitter.NamedAddress[](0),
                    bools: new IRobotStateEmitter.NamedBool[](0)
                });

            payload.uints[0] = IRobotStateEmitter.NamedUint(
                "numTransfers",
                _transferCount
            );

            uint idx = 1;
            if (senderChanged || recipientChanged) {
                payload.uints[idx++] = IRobotStateEmitter.NamedUint(
                    "numTokenHolders",
                    _holderCount
                );
            }

            payload.floats[0] = IRobotStateEmitter.NamedFloat(
                "amountTransferred",
                contractFormatToUserFormat(_transferred)
            );

            _getRobotStateEmitter().emitStateChange(payload);
        }
        return success;
    }

    function transferFloat(
        address to,
        string calldata amount
    ) external onlyProxy {
        if (bytes(amount).length == 0) revert EmptyInputString();
        uint256 amountInContractFormat = userFormatToContractFormat(amount);
        transfer(to, amountInContractFormat);
    }

    function _updateHolder(address account) internal returns (bool updated) {
        bool hadBalance = _hasBalance[account];
        bool hasBalanceNow = balanceOf(account) > 0;

        if (!hadBalance && hasBalanceNow) {
            _hasBalance[account] = true;
            _holderCount++;
            return true;
        } else if (hadBalance && !hasBalanceNow) {
            _hasBalance[account] = false;
            _holderCount--;
            return true;
        }
        return false;
    }

    function contractFormatToUserFormat(
        uint256 userInteger
    ) public view returns (string memory) {
        return
            UserDecimalFormatting.contractFormatToUserFormat(
                userInteger,
                decimals()
            );
    }

    // Converts a fixed-point string to an unsigned integer
    function userFormatToContractFormat(
        string memory userFixedPointString
    ) public view returns (uint256) {
        return
            UserDecimalFormatting.userFormatToContractFormat(
                userFixedPointString,
                decimals()
            );
    }

    function balanceOfFloat(
        address account
    ) external view returns (string memory) {
        return
            UserDecimalFormatting.contractFormatToUserFormat(
                balanceOf(account),
                decimals()
            );
    }

    function allowanceFloat(
        address owner,
        address spender
    ) external view returns (string memory) {
        return
            UserDecimalFormatting.contractFormatToUserFormat(
                allowance(owner, spender),
                decimals()
            );
    }
    function approveFloat(
        address spender,
        string calldata amount
    ) external onlyProxy returns (bool) {
        if (bytes(amount).length == 0) revert EmptyInputString();
        uint256 amountInContractFormat = userFormatToContractFormat(amount);
        return approve(spender, amountInContractFormat);
    }
    function transferFromFloat(
        address sender,
        address recipient,
        string calldata amount
    ) external onlyProxy returns (bool) {
        if (bytes(amount).length == 0) revert EmptyInputString();
        uint256 amountInContractFormat = userFormatToContractFormat(amount);
        return transferFrom(sender, recipient, amountInContractFormat);
    }

    function totalSupplyFloat() external view returns (string memory) {
        return
            UserDecimalFormatting.contractFormatToUserFormat(
                totalSupply(),
                decimals()
            );
    }

    function getRobotState()
        public
        view
        override
        returns (IRobotStateEmitter.StateChangePayload memory)
    {
        IRobotStateEmitter.StateChangePayload
            memory payload = IRobotStateEmitter.StateChangePayload({
                uints: new IRobotStateEmitter.NamedUint[](2),
                floats: new IRobotStateEmitter.NamedFloat[](3),
                strings: new IRobotStateEmitter.NamedString[](0),
                addresses: new IRobotStateEmitter.NamedAddress[](0),
                bools: new IRobotStateEmitter.NamedBool[](0)
            });

        payload.uints[0] = IRobotStateEmitter.NamedUint(
            "numTokenHolders",
            _holderCount
        );
        payload.uints[1] = IRobotStateEmitter.NamedUint(
            "numTransfers",
            _transferCount
        );

        payload.floats[0] = IRobotStateEmitter.NamedFloat(
            "amountTransferred",
            contractFormatToUserFormat(_transferred)
        );
        payload.floats[1] = IRobotStateEmitter.NamedFloat(
            "totalSupply",
            contractFormatToUserFormat(totalSupply())
        );
        payload.floats[2] = IRobotStateEmitter.NamedFloat(
            "amountBurned",
            contractFormatToUserFormat(_burned)
        );

        return payload;
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
