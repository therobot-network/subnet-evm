// SPDX-License-Identifier: MIT

pragma solidity ^0.8.20;

// import "hardhat/console.sol";

import {PrimitiveBase} from "./PrimitiveBase.sol";

import {Initializable} from "@openzeppelin/contracts-upgradeable/proxy/utils/Initializable.sol";
import {IERC20} from "@openzeppelin/contracts/token/ERC20/IERC20.sol";
import {IERC20Metadata} from "@openzeppelin/contracts/token/ERC20/extensions/IERC20Metadata.sol";

import {IRobotStateEmitter} from "../interfaces/IExecutor.sol";

import {IERC20Primitive} from "./interfaces/IERC20Primitive.sol";

import {UserDecimalFormatting} from "../libraries/UserDecimalFormatting.sol";

contract AmmPrimitive is Initializable, PrimitiveBase {
  //Constants
  string private constant _L_TOKEN1 = "token1Liquidity";
  string private constant _L_TOKEN2 = "token2Liquidity";
  string private constant _TOT_LIQ = "totalLiquidityShares";
  uint8 private constant _FEE_DECIMALS = 4;

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

  function initialize(
    address owner,
    address token1Addr,
    address token2Addr,
    string calldata name_,
    string calldata customRules_
  ) external initializer {
    // set token pair
    _setTokens(token1Addr, token2Addr);

    __Primitive_init(owner, name_, customRules_);
  }

  function _setTokens(address token1Addr, address token2Addr) private {
    if (token1Addr == address(0) || token2Addr == address(0)) revert InvalidTokenAddress();

    _token1 = token1Addr;
    _token2 = token2Addr;
    _tokenOnePrimitive = IERC20Primitive(token1Addr);
    _tokenTwoPrimitive = IERC20Primitive(token2Addr);
  }

  function setFee(uint256 newFee) external onlyOwner {
    if (newFee > 1000) revert FeeTooHigh(); // Max 10% (1000 bps)
    _fee = newFee;
    emit FeeSet(newFee);
    IRobotStateEmitter.StateChangePayload memory payload = _initStateChangePayload(0, 1, 0, 0, 0);
    payload.floats[0] = IRobotStateEmitter.NamedFloat(
      "fee",
      UserDecimalFormatting.contractFormatToUserFormat(_fee, _FEE_DECIMALS)
    );
    _getRobotStateEmitter().emitStateChange(payload);
  }

  function getFee() external view returns (uint256 fee) {
    return _fee;
  }

  function activate() external onlyOwner onlyWhenInactive {
    if (!_areTokensSet()) revert TokensNotSet();
    if (_reserves1 == 0 || _reserves2 == 0) revert LiquidityNotAdded();

    isActive = true;
    IRobotStateEmitter.StateChangePayload memory payload = _initStateChangePayload(0, 0, 0, 0, 1);
    payload.bools[0] = IRobotStateEmitter.NamedBool("active", isActive);

    _getRobotStateEmitter().emitStateChange(payload);
  }

  function setTokens(address token1Addr, address token2Addr) public onlyOwner onlyWhenInactive {
    _setTokens(token1Addr, token2Addr);
    emit TokensSet(token1Addr, token2Addr);
    IRobotStateEmitter.StateChangePayload memory payload = _initStateChangePayload(0, 0, 2, 2, 0);
    payload.strings[0] = IRobotStateEmitter.NamedString("token1Name", IERC20Metadata(_token1).name());
    payload.strings[1] = IRobotStateEmitter.NamedString("token2Name", IERC20Metadata(_token2).name());

    payload.addresses[0] = IRobotStateEmitter.NamedAddress("token1Address", _token1);
    payload.addresses[1] = IRobotStateEmitter.NamedAddress("token2Address", _token2);
    _getRobotStateEmitter().emitStateChange(payload);
  }

  function addLiquidity(address token1, uint256 amount1, address token2, uint256 amount2) external {
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
      uint256 liquiditySharesToMint = (_amount1 * totalLiquidityShares) / _reserves1;
      liquidityShare[msgSender] += liquiditySharesToMint;
      totalLiquidityShares += liquiditySharesToMint;

      // Update reserves
      _reserves1 += _amount1;
      _reserves2 += _amount2;
    }
    bool holderChanged = _updateLiquidityHolder(msgSender);
    emit LiquidityAdded(msgSender, _amount1, _amount2);

    IRobotStateEmitter.StateChangePayload memory payload = _initStateChangePayload(0, 3, 0, 0, 0);

    if (holderChanged) {
      payload.uints = new IRobotStateEmitter.NamedUint[](1);
      payload.uints[0] = IRobotStateEmitter.NamedUint("totalLiquidityHolders", _liquidityHolderCount);
    }
    payload.floats[0] = IRobotStateEmitter.NamedFloat(
      _TOT_LIQ,
      _tokenOnePrimitive.contractFormatToUserFormat(totalLiquidityShares)
    );
    payload.floats[1] = IRobotStateEmitter.NamedFloat(
      _L_TOKEN1,
      _tokenOnePrimitive.contractFormatToUserFormat(_reserves1)
    );
    payload.floats[2] = IRobotStateEmitter.NamedFloat(
      _L_TOKEN2,
      _tokenTwoPrimitive.contractFormatToUserFormat(_reserves2)
    );
    _getRobotStateEmitter().emitStateChange(payload);

    // Transfer tokens from sender to contract
    _transferTokenIn(_token1, msgSender, _amount1);
    _transferTokenIn(_token2, msgSender, _amount2);
  }

  function getReserves() external view returns (uint256, uint256) {
    return (_reserves1, _reserves2);
  }

  function removeLiquidity(uint256 amount) external {
    if (amount == 0) revert InvalidAmount();
    address msgSender = _msgSender();
    if (amount > liquidityShare[msgSender]) revert InsufficientLiquidityShare();

    // Compute token amounts using proper proportional scaling
    uint256 tokenAmountFirst = (_reserves1 * amount) / totalLiquidityShares;
    uint256 tokenAmountSecond = (_reserves2 * amount) / totalLiquidityShares;

    // Deduct liquidity shares
    liquidityShare[msgSender] -= amount;
    totalLiquidityShares -= amount;

    // Update reserves
    _reserves1 -= tokenAmountFirst;
    _reserves2 -= tokenAmountSecond;

    bool holderChanged = _updateLiquidityHolder(msgSender);
    emit LiquidityRemoved(msgSender, tokenAmountFirst, tokenAmountSecond);

    IRobotStateEmitter.StateChangePayload memory payload = _initStateChangePayload(holderChanged ? 1 : 0, 3, 0, 0, 0);
    payload.floats[0] = IRobotStateEmitter.NamedFloat(
      _TOT_LIQ,
      _tokenOnePrimitive.contractFormatToUserFormat(totalLiquidityShares)
    );
    payload.floats[1] = IRobotStateEmitter.NamedFloat(
      _L_TOKEN1,
      _tokenOnePrimitive.contractFormatToUserFormat(_reserves1)
    );
    payload.floats[2] = IRobotStateEmitter.NamedFloat(
      _L_TOKEN2,
      _tokenTwoPrimitive.contractFormatToUserFormat(_reserves2)
    );
    if (holderChanged) {
      payload.uints[0] = IRobotStateEmitter.NamedUint("totalLiquidityHolders", _liquidityHolderCount);
    }

    _getRobotStateEmitter().emitStateChange(payload);
    _transferTokenOut(_token1, msgSender, tokenAmountFirst);
    _transferTokenOut(_token2, msgSender, tokenAmountSecond);
  }

  function _updateLiquidityHolder(address account) internal returns (bool updated) {
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

  function swap(address token, uint256 amountIn) external onlyWhenActive {
    if (amountIn == 0) revert InvalidAmount();

    address inTokenAddr = _token1;
    address outTokenAddr = _token2;
    uint256 inReserve = _reserves1;
    uint256 outReserve = _reserves2;
    bool isToken1 = true;

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
    uint256 amountOut = (product * outReserve) / ((inReserve * 10000) + product);

    IRobotStateEmitter.StateChangePayload memory payload = _initStateChangePayload(1, 3, 0, 0, 0);

    if (isToken1) {
      _reserves1 += amountIn;
      _reserves2 -= amountOut;
      _amountOneSwapped += amountIn;
      payload.floats[0] = IRobotStateEmitter.NamedFloat(
        "amount1Swapped",
        _tokenOnePrimitive.contractFormatToUserFormat(_amountOneSwapped)
      );
    } else {
      _reserves2 += amountIn;
      _reserves1 -= amountOut;
      _amountTwoSwapped += amountIn;
      payload.floats[0] = IRobotStateEmitter.NamedFloat(
        "amount2Swapped",
        _tokenTwoPrimitive.contractFormatToUserFormat(_amountTwoSwapped)
      );
    }

    payload.floats[1] = IRobotStateEmitter.NamedFloat(
      _L_TOKEN1,
      _tokenOnePrimitive.contractFormatToUserFormat(_reserves1)
    );
    payload.floats[2] = IRobotStateEmitter.NamedFloat(
      _L_TOKEN2,
      _tokenTwoPrimitive.contractFormatToUserFormat(_reserves2)
    );

    _numSwaps++;
    payload.uints[0] = IRobotStateEmitter.NamedUint("numSwaps", _numSwaps);

    emit TokensSwapped(msg.sender, amountIn, amountOut);

    _getRobotStateEmitter().emitStateChange(payload);
    // Transfer input tokens in
    _transferTokenIn(inTokenAddr, msg.sender, amountIn);
    // Transfer output tokens to user
    _transferTokenOut(outTokenAddr, msg.sender, amountOut);
  }

  function price(address token) external view returns (uint256) {
    if (_reserves1 == 0 || _reserves2 == 0) revert LiquidityNotAdded();

    if (token == _token2) {
      return (_reserves2 * 1e18) / _reserves1;
    } else if (token != _token1) {
      revert InvalidToken();
    }
    return (_reserves1 * 1e18) / _reserves2;
  }

  function status()
    external
    view
    returns (address[2] memory tokens, uint256 liquidity1, uint256 liquidity2, bool active, uint256 fee)
  {
    return ([_token1, _token2], _reserves1, _reserves2, isActive, _fee);
  }

  function _transferTokenIn(address tokenAddress, address from, uint256 amount) private {
    bool success = IERC20(tokenAddress).transferFrom(from, address(this), amount);
    if (!success) revert TokenTransferFailed();
  }

  function _transferTokenOut(address tokenAddress, address recipient, uint256 amount) private {
    bool success = IERC20(tokenAddress).transfer(recipient, amount);
    if (!success) revert TokenTransferFailed();
  }

  function _areTokensSet() private view returns (bool) {
    return _token1 != address(0) && _token2 != address(0);
  }

  function getRobotState() external view override returns (IRobotStateEmitter.StateChangePayload memory) {
    IRobotStateEmitter.StateChangePayload memory payload = _initStateChangePayload(2, 6, 2, 2, 1);

    payload.uints[0] = IRobotStateEmitter.NamedUint("numSwaps", _numSwaps);

    payload.uints[1] = IRobotStateEmitter.NamedUint("totalLiquidityHolders", _liquidityHolderCount);

    payload.floats[0] = IRobotStateEmitter.NamedFloat(
      "amount1Swapped",
      _tokenOnePrimitive.contractFormatToUserFormat(_amountOneSwapped)
    );
    payload.floats[1] = IRobotStateEmitter.NamedFloat(
      "amount2Swapped",
      _tokenTwoPrimitive.contractFormatToUserFormat(_amountTwoSwapped)
    );
    payload.floats[2] = IRobotStateEmitter.NamedFloat(
      _TOT_LIQ,
      _tokenOnePrimitive.contractFormatToUserFormat(totalLiquidityShares)
    );
    payload.floats[3] = IRobotStateEmitter.NamedFloat(
      _L_TOKEN1,
      _tokenOnePrimitive.contractFormatToUserFormat(_reserves1)
    );
    payload.floats[4] = IRobotStateEmitter.NamedFloat(
      _L_TOKEN2,
      _tokenTwoPrimitive.contractFormatToUserFormat(_reserves2)
    );
    payload.floats[5] = IRobotStateEmitter.NamedFloat(
      "fee",
      UserDecimalFormatting.contractFormatToUserFormat(_fee, _FEE_DECIMALS)
    );

    payload.strings[0] = IRobotStateEmitter.NamedString("token1Name", IERC20Metadata(_token1).name());
    payload.strings[1] = IRobotStateEmitter.NamedString("token2Name", IERC20Metadata(_token2).name());

    payload.addresses[0] = IRobotStateEmitter.NamedAddress("token1Address", _token1);
    payload.addresses[1] = IRobotStateEmitter.NamedAddress("token2Address", _token2);

    payload.bools[0] = IRobotStateEmitter.NamedBool("active", isActive);

    return payload;
  }

  function _initStateChangePayload(
    uint uintN,
    uint floatN,
    uint stringN,
    uint addressN,
    uint boolN
  ) internal pure returns (IRobotStateEmitter.StateChangePayload memory payload) {
    payload = IRobotStateEmitter.StateChangePayload({
      uints: new IRobotStateEmitter.NamedUint[](uintN),
      floats: new IRobotStateEmitter.NamedFloat[](floatN),
      strings: new IRobotStateEmitter.NamedString[](stringN),
      addresses: new IRobotStateEmitter.NamedAddress[](addressN),
      bools: new IRobotStateEmitter.NamedBool[](boolN)
    });
  }
}
