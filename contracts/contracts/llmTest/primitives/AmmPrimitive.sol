// SPDX-License-Identifier: MIT

pragma solidity ^0.8.20;

import {PrimitiveBase} from "./PrimitiveBase.sol";

import {Initializable} from "@openzeppelin/contracts-upgradeable/proxy/utils/Initializable.sol";
import {IERC20} from "@openzeppelin/contracts/token/ERC20/IERC20.sol";

contract AmmPrimitive is Initializable, PrimitiveBase {
  bool public isActive;

  address private _token1;
  address private _token2;

  uint256 private _reserves1;
  uint256 private _reserves2;

  uint256 private _fee;

  mapping(address => uint256) public liquidityShare;
  uint256 public totalLiquidityShares;

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

  constructor(address llmPrecompile, string memory metadata) PrimitiveBase(llmPrecompile, metadata) {}

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
  }

  function setFee(uint256 newFee) external onlyOwner {
    if (newFee > 1000) revert FeeTooHigh(); // Max 10% (1000 bps)
    _fee = newFee;
    emit FeeSet(newFee);
  }

  function getFee() external view returns (uint256 fee) {
    return _fee;
  }

  function activate() external onlyOwner onlyWhenInactive {
    if (!(_areTokensSet())) revert TokensNotSet();
    if (_reserves1 == 0 || _reserves2 == 0) revert LiquidityNotAdded();

    isActive = true;
  }

  function setTokens(address token1Addr, address token2Addr) public onlyOwner onlyWhenInactive {
    _setTokens(token1Addr, token2Addr);
    emit TokensSet(token1Addr, token2Addr);
  }

  function addLiquidity(address token1, uint256 amount1, address token2, uint256 amount2) external {
    if (!(_areTokensSet())) revert TokensNotSet();
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

    emit LiquidityAdded(msgSender, _amount1, _amount2);

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

    emit LiquidityRemoved(msgSender, tokenAmountFirst, tokenAmountSecond);

    _transferTokenOut(_token1, msgSender, tokenAmountFirst);
    _transferTokenOut(_token2, msgSender, tokenAmountSecond);
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

    if (isToken1) {
      _reserves1 += amountIn;
      _reserves2 -= amountOut;
    } else {
      _reserves2 += amountIn;
      _reserves1 -= amountOut;
    }

    emit TokensSwapped(msg.sender, amountIn, amountOut);

    // Transfer input tokens in
    _transferTokenIn(inTokenAddr, msg.sender, amountIn);
    // Transfer output tokens to user
    _transferTokenOut(outTokenAddr, msg.sender, amountOut);
  }

  function price(address token) external view returns (uint256 tokenPrice) {
    if (_reserves1 == 0 || _reserves2 == 0) revert LiquidityNotAdded();

    if (token == _token2) {
      tokenPrice = (_reserves2 * 1e18) / _reserves1;
      return tokenPrice;
    } else if (token != _token1) {
      revert InvalidToken();
    }

    tokenPrice = (_reserves1 * 1e18) / _reserves2;
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
    if (_token1 == address(0)) return false;
    if (_token2 == address(0)) return false;
    return true;
  }
}
