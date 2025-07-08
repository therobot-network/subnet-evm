// SPDX-License-Identifier: MIT

pragma solidity ^0.8.20;

// import "hardhat/console.sol";

import {PrimitiveBase} from "./PrimitiveBase.sol";

import {Initializable} from "@openzeppelin/contracts-upgradeable/proxy/utils/Initializable.sol";
import {IERC20} from "@openzeppelin/contracts/token/ERC20/IERC20.sol";
import {IERC20Metadata} from "@openzeppelin/contracts/token/ERC20/extensions/IERC20Metadata.sol";

import {IRobotStateEmitter, Operation} from "../interfaces/IExecutor.sol";

import {IERC20Primitive} from "./interfaces/IERC20Primitive.sol";

import {UserDecimalFormatting} from "../libraries/UserDecimalFormatting.sol";

import {ReentrancyGuardUpgradeable} from "@openzeppelin/contracts-upgradeable/utils/ReentrancyGuardUpgradeable.sol";

contract AmmPrimitive is
    Initializable,
    PrimitiveBase,
    ReentrancyGuardUpgradeable
{
    //Constants
    string private constant _L_TOKEN1 = "token1Liquidity";
    string private constant _L_TOKEN2 = "token2Liquidity";
    string private constant _TOT_LIQ = "totalLiquidityShares";
    uint8 private constant _FEE_DECIMALS = 2;

    bool public isActive;

    address private _token1;
    address private _token2;

    IERC20Primitive private _tokenOnePrimitive;
    IERC20Primitive private _tokenTwoPrimitive;

    uint256 private _reserves1;
    uint256 private _reserves2;

    uint256 private _fee;

    mapping(address => uint256) public liquidityShare;
    mapping(address => bool) private _hasLiquidity;
    uint256 private _liquidityHolderCount;
    uint256 public totalLiquidityShares;

    uint256 private _numSwaps;
    uint256 private _amountOneSwapped;
    uint256 private _amountTwoSwapped;

    event TokensSet(address token1Addr, address token2Addr);
    event LiquidityAdded(address account, uint256 amount1, uint256 amount2);
    event LiquidityRemoved(address account, uint256 amount1, uint256 amount2);
    event TokensSwapped(address account, uint256 amountIn, uint256 amountOut);
    event FeeSet(uint256 newFee);

    error AmmActive();
    error AmmNotActive();
    error TokensNotSet();
    error LiquidityNotAdded();
    error LiquidityNotZero();
    error TokenTransferFailed();
    error InsufficientLiquidityShare();
    error InvalidToken();
    error InvalidAmount();
    error InvalidTokenAddress();
    error FeeTooHigh();

    // Modifiers to restrict function calls
    modifier onlyWhenActive() {
        if (!isActive) revert AmmNotActive();
        _;
    }

    modifier onlyWhenInactive() {
        if (isActive) revert AmmActive();
        _;
    }

    constructor(
        address llmPrecompile,
        string memory metadata,
        address primitiveStorageAddress
    ) PrimitiveBase(llmPrecompile, "amm", metadata, primitiveStorageAddress) {}

    function configure(
        address token1Addr,
        address token2Addr,
        string calldata feeFloat
    ) external onlyOwner onlyWhenInactive nonReentrant {
        // slither-disable-next-line reentrancy-benign
        setTokens(token1Addr, token2Addr);
        setFee(feeFloat);
    }

    function getLiquidityShare(
        address account
    ) external view returns (string memory) {
        return
            UserDecimalFormatting.contractFormatToUserFormat(
                liquidityShare[account],
                IERC20Metadata(_token1).decimals()
            );
    }

    function getTotalLiquidityShares() external view returns (string memory) {
        return
            UserDecimalFormatting.contractFormatToUserFormat(
                totalLiquidityShares,
                IERC20Metadata(_token1).decimals()
            );
    }

    function _setTokens(address token1Addr, address token2Addr) private {
        if (
            token1Addr == address(0) ||
            token2Addr == address(0) ||
            token1Addr == token2Addr
        ) revert InvalidTokenAddress();

        _token1 = token1Addr;
        _token2 = token2Addr;
        _tokenOnePrimitive = IERC20Primitive(token1Addr);
        _tokenTwoPrimitive = IERC20Primitive(token2Addr);
    }

    function setFee(string calldata feeFloat) public onlyOwner {
        uint256 newFee = UserDecimalFormatting.userFormatToContractFormat(
            feeFloat,
            _FEE_DECIMALS
        );
        setFeeUint(newFee);
    }

    function setFeeUint(uint256 newFee) public onlyOwner {
        if (newFee > 1000) revert FeeTooHigh(); // Max 10% (1000 bps)
        _fee = newFee;
        emit FeeSet(newFee);
        IRobotStateEmitter.StateChangePayload
            memory payload = _initStateChangePayload(0, 1, 0, 0, 0);
        payload.floats[0] = IRobotStateEmitter.NamedFloat(
            "fee",
            UserDecimalFormatting.contractFormatToUserFormat(
                _fee,
                _FEE_DECIMALS
            ),
            address(0),
            Operation.Set
        );
        _getRobotStateEmitter().emitStateChange(payload);
    }

    function getFee() external view returns (string memory) {
        return
            UserDecimalFormatting.contractFormatToUserFormat(
                _fee,
                _FEE_DECIMALS
            );
    }

    function activate() external onlyOwner onlyWhenInactive {
        if (!_areTokensSet()) revert TokensNotSet();
        if (_reserves1 == 0 || _reserves2 == 0) revert LiquidityNotAdded();

        isActive = true;
        IRobotStateEmitter.StateChangePayload
            memory payload = _initStateChangePayload(0, 0, 0, 0, 1);
        payload.bools[0] = IRobotStateEmitter.NamedBool(
            "active",
            isActive,
            address(0)
        );

        _getRobotStateEmitter().emitStateChange(payload);
    }

    function setTokens(
        address token1Addr,
        address token2Addr
    ) public onlyOwner onlyWhenInactive {
        if (_reserves1 != 0 || _reserves2 != 0) {
            revert LiquidityNotZero();
        }
        _setTokens(token1Addr, token2Addr);
        emit TokensSet(token1Addr, token2Addr);
        IRobotStateEmitter.StateChangePayload
            memory payload = _initStateChangePayload(0, 0, 2, 2, 0);
        payload.strings[0] = IRobotStateEmitter.NamedString(
            "token1Name",
            IERC20Metadata(_token1).symbol(),
            address(0)
        );
        payload.strings[1] = IRobotStateEmitter.NamedString(
            "token2Name",
            IERC20Metadata(_token2).symbol(),
            address(0)
        );

        payload.addresses[0] = IRobotStateEmitter.NamedAddress(
            "token1Address",
            _token1,
            address(0)
        );
        payload.addresses[1] = IRobotStateEmitter.NamedAddress(
            "token2Address",
            _token2,
            address(0)
        );
        _getRobotStateEmitter().emitStateChange(payload);
    }

    function addLiquidity(
        address token1,
        string calldata amount1,
        address token2,
        string calldata amount2
    ) external {
        if (!_areTokensSet()) revert TokensNotSet();
        if (token1 == token2) revert InvalidToken();

        uint256 _amount1 = UserDecimalFormatting.userFormatToContractFormat(
            amount1,
            IERC20Metadata(_token1).decimals()
        );
        uint256 _amount2 = UserDecimalFormatting.userFormatToContractFormat(
            amount2,
            IERC20Metadata(_token2).decimals()
        );
        addLiquidityUint(token1, _amount1, token2, _amount2);
    }

    function addLiquidityUint(
        address token1,
        uint256 amount1,
        address token2,
        uint256 amount2
    ) public {
        if (!_areTokensSet()) revert TokensNotSet();
        if (token1 == token2) revert InvalidToken();

        address msgSender = _msgSender();

        uint256 _amount1 = amount1;
        uint256 _amount2 = amount2;

        if (token1 == _token2) {
            _amount1 = amount2;
        } else if (token1 != _token1) {
            revert InvalidToken();
        }

        if (token2 == _token1) {
            _amount2 = amount1;
        } else if (token2 != _token2) {
            revert InvalidToken();
        }

        // If first liquidity addition, set reserves directly
        // Bug here? What about the case where just _amount2 is added?
        if (_reserves1 == 0 || _reserves2 == 0) {
            _reserves1 = _amount1;
            _reserves2 = _amount2;
            totalLiquidityShares = _amount1; // Initialize liquidity shares
            liquidityShare[msgSender] = totalLiquidityShares;
        } else {
            // Ensure the correct ratio is maintained
            // slither-disable-next-line divide-before-multiply
            uint256 expectedAmount2 = (_reserves2 * _amount1) / _reserves1;
            if (_amount2 < expectedAmount2) {
                _amount1 = (_reserves1 * _amount2) / _reserves2; // Adjust _amount1 to match ratio
            } else {
                _amount2 = expectedAmount2; // Adjust amount2 to maintain ratio
            }

            // Calculate liquidity shares
            uint256 liquiditySharesToMint = (_amount1 * totalLiquidityShares) /
                _reserves1;
            liquidityShare[msgSender] += liquiditySharesToMint;
            totalLiquidityShares += liquiditySharesToMint;

            // Update reserves
            _reserves1 += _amount1;
            _reserves2 += _amount2;
        }
        bool holderChanged = _updateLiquidityHolder(msgSender);
        emit LiquidityAdded(msgSender, _amount1, _amount2);

        IRobotStateEmitter.StateChangePayload
            memory payload = _initStateChangePayload(0, 4, 0, 0, 0);

        if (holderChanged) {
            payload.uints = new IRobotStateEmitter.NamedUint[](1);
            payload.uints[0] = IRobotStateEmitter.NamedUint(
                "totalLiquidityHolders",
                _liquidityHolderCount,
                address(0),
                Operation.Set
            );
        }
        payload.floats[0] = IRobotStateEmitter.NamedFloat(
            _TOT_LIQ,
            _tokenOnePrimitive.contractFormatToUserFormat(totalLiquidityShares),
            address(0),
            Operation.Set
        );
        payload.floats[1] = IRobotStateEmitter.NamedFloat(
            _L_TOKEN1,
            _tokenOnePrimitive.contractFormatToUserFormat(_reserves1),
            address(0),
            Operation.Set
        );
        payload.floats[2] = IRobotStateEmitter.NamedFloat(
            _L_TOKEN2,
            _tokenTwoPrimitive.contractFormatToUserFormat(_reserves2),
            address(0),
            Operation.Set
        );
        payload.floats[3] = IRobotStateEmitter.NamedFloat(
            "liquidityTokens",
            _tokenOnePrimitive.contractFormatToUserFormat(
                liquidityShare[msgSender]
            ),
            msgSender,
            Operation.Set
        );
        _getRobotStateEmitter().emitStateChange(payload);

        // Transfer tokens from sender to contract
        _transferTokenIn(_token1, msgSender, _amount1);
        _transferTokenIn(_token2, msgSender, _amount2);
    }

    function getReserves()
        external
        view
        returns (string memory, string memory)
    {
        return (
            UserDecimalFormatting.contractFormatToUserFormat(
                _reserves1,
                IERC20Metadata(_token1).decimals()
            ),
            UserDecimalFormatting.contractFormatToUserFormat(
                _reserves2,
                IERC20Metadata(_token2).decimals()
            )
        );
    }

    function removeLiquidity(string calldata amount) external {
        uint256 _amount = UserDecimalFormatting.userFormatToContractFormat(
            amount,
            IERC20Metadata(_token1).decimals()
        );
        removeLiquidityUint(_amount);
    }

    function removeLiquidityUint(uint256 amount) public {
        if (amount == 0) revert InvalidAmount();
        address msgSender = _msgSender();
        if (amount > liquidityShare[msgSender])
            revert InsufficientLiquidityShare();

        // Compute token amounts using proper proportional scaling
        uint256 tokenAmountFirst = (_reserves1 * amount) / totalLiquidityShares;
        uint256 tokenAmountSecond = (_reserves2 * amount) /
            totalLiquidityShares;

        // Deduct liquidity shares
        liquidityShare[msgSender] -= amount;
        totalLiquidityShares -= amount;

        // Update reserves
        _reserves1 -= tokenAmountFirst;
        _reserves2 -= tokenAmountSecond;

        bool holderChanged = _updateLiquidityHolder(msgSender);
        emit LiquidityRemoved(msgSender, tokenAmountFirst, tokenAmountSecond);

        IRobotStateEmitter.StateChangePayload
            memory payload = _initStateChangePayload(
                holderChanged ? 1 : 0,
                4,
                0,
                0,
                0
            );
        payload.floats[0] = IRobotStateEmitter.NamedFloat(
            _TOT_LIQ,
            _tokenOnePrimitive.contractFormatToUserFormat(totalLiquidityShares),
            address(0),
            Operation.Set
        );
        payload.floats[1] = IRobotStateEmitter.NamedFloat(
            _L_TOKEN1,
            _tokenOnePrimitive.contractFormatToUserFormat(_reserves1),
            address(0),
            Operation.Set
        );
        payload.floats[2] = IRobotStateEmitter.NamedFloat(
            _L_TOKEN2,
            _tokenTwoPrimitive.contractFormatToUserFormat(_reserves2),
            address(0),
            Operation.Set
        );
        payload.floats[3] = IRobotStateEmitter.NamedFloat(
            "liquidityTokens",
            _tokenOnePrimitive.contractFormatToUserFormat(
                liquidityShare[msgSender]
            ),
            msgSender,
            Operation.Set
        );
        if (holderChanged) {
            payload.uints[0] = IRobotStateEmitter.NamedUint(
                "totalLiquidityHolders",
                _liquidityHolderCount,
                address(0),
                Operation.Set
            );
        }

        _getRobotStateEmitter().emitStateChange(payload);
        _transferTokenOut(_token1, msgSender, tokenAmountFirst);
        _transferTokenOut(_token2, msgSender, tokenAmountSecond);
    }

    function _updateLiquidityHolder(
        address account
    ) internal returns (bool updated) {
        bool hadBalance = _hasLiquidity[account];
        bool hasBalanceNow = liquidityShare[account] > 0;

        if (!hadBalance && hasBalanceNow) {
            _hasLiquidity[account] = true;
            _liquidityHolderCount++;
            return true;
        } else if (hadBalance && !hasBalanceNow) {
            _hasLiquidity[account] = false;
            _liquidityHolderCount--;
            return true;
        }
        return false;
    }

    function swap(
        address token,
        string calldata amountIn
    ) external onlyWhenActive {
        uint256 _amountIn = UserDecimalFormatting.userFormatToContractFormat(
            amountIn,
            IERC20Metadata(token).decimals()
        );
        swapUint(token, _amountIn);
    }

    function swapUint(address token, uint256 amountIn) public onlyWhenActive {
        address inTokenAddr = _token1;
        address outTokenAddr = _token2;
        uint256 inReserve = _reserves1;
        uint256 outReserve = _reserves2;
        bool isToken1 = true;
        address msgSender = _msgSender();

        if (amountIn == 0) revert InvalidAmount();
        if (token == _token2) {
            inTokenAddr = _token2;
            outTokenAddr = _token1;
            inReserve = _reserves2;
            outReserve = _reserves1;
            isToken1 = false;
        } else if (token != _token1) {
            revert InvalidToken();
        }

        uint256 feeMultiplier = 10000 - _fee; // _fee is in basis points
        uint256 product = amountIn * feeMultiplier; // no division yet
        uint256 amountOut = (product * outReserve) /
            ((inReserve * 10000) + product);

        IRobotStateEmitter.StateChangePayload
            memory payload = _initStateChangePayload(2, 3, 0, 0, 0);

        if (isToken1) {
            _reserves1 += amountIn;
            _reserves2 -= amountOut;
            _amountOneSwapped += amountIn;
            payload.floats[0] = IRobotStateEmitter.NamedFloat(
                "amount1Swapped",
                _tokenOnePrimitive.contractFormatToUserFormat(
                    _amountOneSwapped
                ),
                address(0),
                Operation.Set
            );
        } else {
            _reserves2 += amountIn;
            _reserves1 -= amountOut;
            _amountTwoSwapped += amountIn;
            payload.floats[0] = IRobotStateEmitter.NamedFloat(
                "amount2Swapped",
                _tokenTwoPrimitive.contractFormatToUserFormat(
                    _amountTwoSwapped
                ),
                address(0),
                Operation.Set
            );
        }

        payload.floats[1] = IRobotStateEmitter.NamedFloat(
            _L_TOKEN1,
            _tokenOnePrimitive.contractFormatToUserFormat(_reserves1),
            address(0),
            Operation.Set
        );
        payload.floats[2] = IRobotStateEmitter.NamedFloat(
            _L_TOKEN2,
            _tokenTwoPrimitive.contractFormatToUserFormat(_reserves2),
            address(0),
            Operation.Set
        );

        _numSwaps++;
        payload.uints[0] = IRobotStateEmitter.NamedUint(
            "numSwaps",
            _numSwaps,
            address(0),
            Operation.Set
        );
        payload.uints[1] = IRobotStateEmitter.NamedUint(
            "numSwapsPerAccount",
            1,
            msgSender,
            Operation.Add
        );

        emit TokensSwapped(msgSender, amountIn, amountOut);

        _getRobotStateEmitter().emitStateChange(payload);
        // Transfer input tokens in
        _transferTokenIn(inTokenAddr, msgSender, amountIn);
        // Transfer output tokens to user
        _transferTokenOut(outTokenAddr, msgSender, amountOut);
    }

    function priceUint(address token) external view returns (uint256) {
        if (_reserves1 == 0 || _reserves2 == 0) revert LiquidityNotAdded();

        if (token == _token2) {
            return (_reserves2 * 1e18) / _reserves1;
        } else if (token != _token1) {
            revert InvalidToken();
        }
        return (_reserves1 * 1e18) / _reserves2;
    }

    function price(address token) external view returns (string memory) {
        if (_reserves1 == 0 || _reserves2 == 0) revert LiquidityNotAdded();

        uint8 d1 = IERC20Metadata(_token1).decimals();
        uint8 d2 = IERC20Metadata(_token2).decimals();

        uint256 priceWad = 0;

        if (token == _token2) {
            // price of token2 in token1 → token1 / token2
            priceWad =
                (_reserves2 * (10 ** d1) * 1e18) /
                (_reserves1 * (10 ** d2));
        } else if (token == _token1) {
            // price of token1 in token2 → token2 / token1
            priceWad =
                (_reserves1 * (10 ** d2) * 1e18) /
                (_reserves2 * (10 ** d1));
        } else {
            revert InvalidToken();
        }

        // Format the WAD price to user-friendly string (18 decimal places)
        return UserDecimalFormatting.contractFormatToUserFormat(priceWad, 18);
    }

    function status()
        external
        view
        returns (
            address[2] memory tokens,
            string memory liquidity1,
            string memory liquidity2,
            bool active,
            string memory fee
        )
    {
        return (
            [_token1, _token2],
            _tokenOnePrimitive.contractFormatToUserFormat(_reserves1),
            _tokenTwoPrimitive.contractFormatToUserFormat(_reserves2),
            isActive,
            UserDecimalFormatting.contractFormatToUserFormat(
                _fee,
                _FEE_DECIMALS
            )
        );
    }

    function _transferTokenIn(
        address tokenAddress,
        address from,
        uint256 amount
    ) private {
        bool success = IERC20(tokenAddress).transferFrom(
            from,
            address(this),
            amount
        );
        if (!success) revert TokenTransferFailed();
    }

    function _transferTokenOut(
        address tokenAddress,
        address recipient,
        uint256 amount
    ) private {
        bool success = IERC20(tokenAddress).transfer(recipient, amount);
        if (!success) revert TokenTransferFailed();
    }

    function _areTokensSet() private view returns (bool) {
        return _token1 != address(0) && _token2 != address(0);
    }

    function getRobotState()
        public
        view
        override
        returns (IRobotStateEmitter.StateChangePayload memory)
    {
        bool tokenOneSet = _token1 != address(0);
        bool tokenTwoSet = _token2 != address(0);

        IRobotStateEmitter.StateChangePayload
            memory payload = _initStateChangePayload(2, 6, 2, 2, 1);

        payload.uints[0] = IRobotStateEmitter.NamedUint(
            "numSwaps",
            _numSwaps,
            address(0),
            Operation.Set
        );

        payload.uints[1] = IRobotStateEmitter.NamedUint(
            "totalLiquidityHolders",
            _liquidityHolderCount,
            address(0),
            Operation.Set
        );

        payload.floats[0] = IRobotStateEmitter.NamedFloat(
            "amount1Swapped",
            tokenOneSet
                ? _tokenOnePrimitive.contractFormatToUserFormat(
                    _amountOneSwapped
                )
                : "0",
            address(0),
            Operation.Set
        );
        payload.floats[1] = IRobotStateEmitter.NamedFloat(
            "amount2Swapped",
            tokenTwoSet
                ? _tokenTwoPrimitive.contractFormatToUserFormat(
                    _amountTwoSwapped
                )
                : "0",
            address(0),
            Operation.Set
        );
        payload.floats[2] = IRobotStateEmitter.NamedFloat(
            _TOT_LIQ,
            tokenOneSet
                ? _tokenOnePrimitive.contractFormatToUserFormat(
                    totalLiquidityShares
                )
                : "0",
            address(0),
            Operation.Set
        );
        payload.floats[3] = IRobotStateEmitter.NamedFloat(
            _L_TOKEN1,
            tokenOneSet
                ? _tokenOnePrimitive.contractFormatToUserFormat(_reserves1)
                : "0",
            address(0),
            Operation.Set
        );
        payload.floats[4] = IRobotStateEmitter.NamedFloat(
            _L_TOKEN2,
            tokenTwoSet
                ? _tokenTwoPrimitive.contractFormatToUserFormat(_reserves2)
                : "0",
            address(0),
            Operation.Set
        );
        payload.floats[5] = IRobotStateEmitter.NamedFloat(
            "fee",
            UserDecimalFormatting.contractFormatToUserFormat(
                _fee,
                _FEE_DECIMALS
            ),
            address(0),
            Operation.Set
        );

        payload.strings[0] = IRobotStateEmitter.NamedString(
            "token1Name",
            tokenOneSet ? IERC20Metadata(_token1).symbol() : "",
            address(0)
        );
        payload.strings[1] = IRobotStateEmitter.NamedString(
            "token2Name",
            tokenTwoSet ? IERC20Metadata(_token2).symbol() : "",
            address(0)
        );

        payload.addresses[0] = IRobotStateEmitter.NamedAddress(
            "token1Address",
            _token1,
            address(0)
        );
        payload.addresses[1] = IRobotStateEmitter.NamedAddress(
            "token2Address",
            _token2,
            address(0)
        );

        payload.bools[0] = IRobotStateEmitter.NamedBool(
            "active",
            isActive,
            address(0)
        );

        return payload;
    }

    function _initStateChangePayload(
        uint uintN,
        uint floatN,
        uint stringN,
        uint addressN,
        uint boolN
    )
        internal
        pure
        returns (IRobotStateEmitter.StateChangePayload memory payload)
    {
        payload = IRobotStateEmitter.StateChangePayload({
            uints: new IRobotStateEmitter.NamedUint[](uintN),
            floats: new IRobotStateEmitter.NamedFloat[](floatN),
            strings: new IRobotStateEmitter.NamedString[](stringN),
            addresses: new IRobotStateEmitter.NamedAddress[](addressN),
            bools: new IRobotStateEmitter.NamedBool[](boolN)
        });
    }
}
