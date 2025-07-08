// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

// import "hardhat/console.sol";

import {ERC20Upgradeable} from "@openzeppelin/contracts-upgradeable/token/ERC20/ERC20Upgradeable.sol";
import {ContextUpgradeable} from "@openzeppelin/contracts-upgradeable/utils/ContextUpgradeable.sol";
import {Initializable} from "@openzeppelin/contracts-upgradeable/proxy/utils/Initializable.sol";
import {PrimitiveBase} from "./PrimitiveBase.sol";
import {IRobotStateEmitter, Operation} from "../interfaces/IExecutor.sol";
import {IERC20Primitive} from "./interfaces/IERC20Primitive.sol";
import {UserDecimalFormatting} from "../libraries/UserDecimalFormatting.sol";

contract ERC20Primitive is
    Initializable,
    PrimitiveBase,
    ERC20Upgradeable,
    IERC20Primitive
{
    uint256 public constant MINT_AMOUNT = 100 * 1e18; // 100 tokens
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
    function robotContractInit(
        address owner_,
        string calldata customRules
    ) public override initializer {
        _robotContractBaseInit(owner_, customRules);
        __ERC20_init(
            string.concat("ROBOT_DEPLOYED_", _getContractName()),
            _getContractName()
        );
    }

    function configure(string calldata amount) external onlyOwner {
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
    }

    function mintFloat(string calldata amount) public onlyProxy {
        if (bytes(amount).length == 0) revert EmptyInputString();
        uint256 amountInContractFormat = userFormatToContractFormat(amount);
        mint(amountInContractFormat);
    }

    function burn(uint256 amount) public onlyProxy {
        address sender = _msgSender();
        _burn(sender, amount);
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
        bool success = super.transfer(to, amount);
        return success;
    }

    function transferFloat(
        address to,
        string calldata amount
    ) external onlyProxy returns (bool) {
        if (bytes(amount).length == 0) revert EmptyInputString();
        uint256 amountInContractFormat = userFormatToContractFormat(amount);
        return transfer(to, amountInContractFormat);
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

    function _update(
        address from,
        address to,
        uint256 value
    ) internal override {
        // call into PrimitiveBase (or ERC20Upgradeable hook, if any)
        super._update(from, to, value);

        // Mint case (from == address(0))
        if (from == address(0)) {
            bool toHolderChangedMint = _updateHolder(to);
            IRobotStateEmitter.StateChangePayload
                memory payloadMint = IRobotStateEmitter.StateChangePayload({
                    uints: new IRobotStateEmitter.NamedUint[](0),
                    floats: new IRobotStateEmitter.NamedFloat[](2),
                    strings: new IRobotStateEmitter.NamedString[](0),
                    addresses: new IRobotStateEmitter.NamedAddress[](0),
                    bools: new IRobotStateEmitter.NamedBool[](0)
                });

            uint idx = 0;
            payloadMint.uints = new IRobotStateEmitter.NamedUint[](
                toHolderChangedMint ? 1 : 0
            );
            if (toHolderChangedMint) {
                payloadMint.uints[idx++] = IRobotStateEmitter.NamedUint(
                    "numTokenHolders",
                    _holderCount,
                    address(0),
                    Operation.Set
                );
            }
            payloadMint.floats[0] = IRobotStateEmitter.NamedFloat(
                "totalSupply",
                contractFormatToUserFormat(totalSupply()),
                address(0),
                Operation.Set
            );
            payloadMint.floats[1] = IRobotStateEmitter.NamedFloat(
                "balance",
                contractFormatToUserFormat(balanceOf(to)),
                to,
                Operation.Set
            );

            _getRobotStateEmitter().emitStateChange(payloadMint);
            return;
        }

        // Burn case (to == address(0))
        if (to == address(0)) {
            _burned += value;
            // slither-disable-next-line events-maths
            bool holderChangedBurn = _updateHolder(from);

            IRobotStateEmitter.StateChangePayload
                memory payloadBurn = IRobotStateEmitter.StateChangePayload({
                    uints: new IRobotStateEmitter.NamedUint[](0),
                    floats: new IRobotStateEmitter.NamedFloat[](3),
                    strings: new IRobotStateEmitter.NamedString[](0),
                    addresses: new IRobotStateEmitter.NamedAddress[](0),
                    bools: new IRobotStateEmitter.NamedBool[](0)
                });

            if (holderChangedBurn) {
                payloadBurn.uints = new IRobotStateEmitter.NamedUint[](1);
                payloadBurn.uints[0] = IRobotStateEmitter.NamedUint(
                    "numTokenHolders",
                    _holderCount,
                    address(0),
                    Operation.Set
                );
            }
            payloadBurn.floats[0] = IRobotStateEmitter.NamedFloat(
                "totalSupply",
                contractFormatToUserFormat(totalSupply()),
                address(0),
                Operation.Set
            );
            payloadBurn.floats[1] = IRobotStateEmitter.NamedFloat(
                "amountBurned",
                contractFormatToUserFormat(_burned),
                address(0),
                Operation.Set
            );
            payloadBurn.floats[2] = IRobotStateEmitter.NamedFloat(
                "balance",
                contractFormatToUserFormat(balanceOf(from)),
                from,
                Operation.Set
            );

            _getRobotStateEmitter().emitStateChange(payloadBurn);
            return;
        }

        // Transfer case
        _transferCount++;
        _transferred += value;
        bool fromHolderChanged = _updateHolder(from);
        bool toHolderChanged = _updateHolder(to);

        uint totalUints = 2 + (fromHolderChanged || toHolderChanged ? 1 : 0);

        IRobotStateEmitter.StateChangePayload
            memory payload = IRobotStateEmitter.StateChangePayload({
                uints: new IRobotStateEmitter.NamedUint[](totalUints),
                floats: new IRobotStateEmitter.NamedFloat[](3),
                strings: new IRobotStateEmitter.NamedString[](0),
                addresses: new IRobotStateEmitter.NamedAddress[](0),
                bools: new IRobotStateEmitter.NamedBool[](0)
            });

        uint idxUint = 0;
        payload.uints[idxUint++] = IRobotStateEmitter.NamedUint(
            "numTransfers",
            _transferCount,
            address(0),
            Operation.Set
        );
        payload.uints[idxUint++] = IRobotStateEmitter.NamedUint(
            "numAccountTransfers",
            1,
            from,
            Operation.Add
        );
        if (fromHolderChanged || toHolderChanged) {
            payload.uints[idxUint++] = IRobotStateEmitter.NamedUint(
                "numTokenHolders",
                _holderCount,
                address(0),
                Operation.Set
            );
        }

        payload.floats[0] = IRobotStateEmitter.NamedFloat(
            "amountTransferred",
            contractFormatToUserFormat(_transferred),
            address(0),
            Operation.Set
        );
        payload.floats[1] = IRobotStateEmitter.NamedFloat(
            "balance",
            contractFormatToUserFormat(balanceOf(from)),
            from,
            Operation.Set
        );
        payload.floats[2] = IRobotStateEmitter.NamedFloat(
            "balance",
            contractFormatToUserFormat(balanceOf(to)),
            to,
            Operation.Set
        );

        _getRobotStateEmitter().emitStateChange(payload);
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
            _holderCount,
            address(0),
            Operation.Set
        );
        payload.uints[1] = IRobotStateEmitter.NamedUint(
            "numTransfers",
            _transferCount,
            address(0),
            Operation.Set
        );

        payload.floats[0] = IRobotStateEmitter.NamedFloat(
            "amountTransferred",
            contractFormatToUserFormat(_transferred),
            address(0),
            Operation.Set
        );
        payload.floats[1] = IRobotStateEmitter.NamedFloat(
            "totalSupply",
            contractFormatToUserFormat(totalSupply()),
            address(0),
            Operation.Set
        );
        payload.floats[2] = IRobotStateEmitter.NamedFloat(
            "amountBurned",
            contractFormatToUserFormat(_burned),
            address(0),
            Operation.Set
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
