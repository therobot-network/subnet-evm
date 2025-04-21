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

contract ERC20Primitive is Initializable, PrimitiveBase, ERC20Upgradeable, IERC20Primitive {
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
  ) PrimitiveBase(llmPrecompile, "erc20", metadata, primitiveStorageAddress) {}

  function initialize(
    address owner,
    string calldata symbol,
    uint256 amount,
    string calldata name_,
    string calldata customRules_
  ) external initializer {
    __ERC20_init(name_, symbol);
    _mint(owner, amount);
    _updateHolder(owner);
    __Primitive_init(owner, name_, customRules_);
  }

  // users other than owner can mint only a max of 100 tokens in a request
  // users can request only once per hour
  function mint(uint256 amount) external onlyProxy {
    address sender = _msgSender();
    if (sender != owner()) {
      // slither-disable-next-line timestamp
      if (block.timestamp < _requestLog[sender] + MINT_TIME_LIMIT) revert RequestAfterSometime(MINT_TIME_LIMIT);

      _requestLog[sender] = block.timestamp;
      // users can mint only a max of MINT_AMOUNT
      amount = amount > MINT_AMOUNT ? MINT_AMOUNT : amount;
    }

    _mint(sender, amount);
    _emitMintStateChangeSupply(_updateHolder(sender));
  }

  function _emitMintStateChangeSupply(bool holderChanged) internal {
    IRobotStateEmitter.StateChangePayload memory payload = IRobotStateEmitter.StateChangePayload({
      uints: new IRobotStateEmitter.NamedUint[](0),
      floats: new IRobotStateEmitter.NamedFloat[](0),
      strings: new IRobotStateEmitter.NamedString[](0),
      addresses: new IRobotStateEmitter.NamedAddress[](0),
      bools: new IRobotStateEmitter.NamedBool[](0)
    });

    uint idx = 0;
    payload.uints = new IRobotStateEmitter.NamedUint[](holderChanged ? 1 : 0);
    if (holderChanged) {
      payload.uints[idx++] = IRobotStateEmitter.NamedUint("numTokenHolders", _holderCount);
    }

    payload.floats = new IRobotStateEmitter.NamedFloat[](1);
    payload.floats[0] = IRobotStateEmitter.NamedFloat("totalSupply", contractFormatToUserFormat(totalSupply()));

    _getRobotStateEmitter().emitStateChange(payload);
  }

  function burn(uint256 amount) external onlyProxy {
    address sender = _msgSender();

    _burn(sender, amount);

    // slither-disable-next-line events-maths
    _burned += amount;
    bool holderChanged = _updateHolder(sender);

    IRobotStateEmitter.StateChangePayload memory payload = IRobotStateEmitter.StateChangePayload({
      uints: new IRobotStateEmitter.NamedUint[](0),
      floats: new IRobotStateEmitter.NamedFloat[](2),
      strings: new IRobotStateEmitter.NamedString[](0),
      addresses: new IRobotStateEmitter.NamedAddress[](0),
      bools: new IRobotStateEmitter.NamedBool[](0)
    });

    if (holderChanged) {
      payload.uints = new IRobotStateEmitter.NamedUint[](1);
      payload.uints[0] = IRobotStateEmitter.NamedUint("numTokenHolders", _holderCount);
    }

    payload.floats[0] = IRobotStateEmitter.NamedFloat("totalSupply", contractFormatToUserFormat(totalSupply()));
    payload.floats[1] = IRobotStateEmitter.NamedFloat("amountBurned", contractFormatToUserFormat(_burned));

    _getRobotStateEmitter().emitStateChange(payload);
  }

  function transfer(address to, uint256 amount) public override onlyProxy returns (bool) {
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

      uint uintLen = 1 + (senderChanged ? 1 : 0) + (recipientChanged ? 1 : 0);

      IRobotStateEmitter.StateChangePayload memory payload = IRobotStateEmitter.StateChangePayload({
        uints: new IRobotStateEmitter.NamedUint[](uintLen),
        floats: new IRobotStateEmitter.NamedFloat[](1),
        strings: new IRobotStateEmitter.NamedString[](0),
        addresses: new IRobotStateEmitter.NamedAddress[](0),
        bools: new IRobotStateEmitter.NamedBool[](0)
      });

      payload.uints[0] = IRobotStateEmitter.NamedUint("numTransfers", _transferCount);

      uint idx = 1;
      if (senderChanged || recipientChanged) {
        payload.uints[idx++] = IRobotStateEmitter.NamedUint("numTokenHolders", _holderCount);
      }

      payload.floats[0] = IRobotStateEmitter.NamedFloat("amountTransferred", contractFormatToUserFormat(_transferred));

      _getRobotStateEmitter().emitStateChange(payload);
    }
    return success;
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

  function contractFormatToUserFormat(uint256 userInteger) public view returns (string memory) {
    return UserDecimalFormatting.contractFormatToUserFormat(userInteger, decimals());
  }

  // Converts a fixed-point string to an unsigned integer
  function userFormatToContractFormat(string memory userFixedPointString) public view returns (uint256) {
    return UserDecimalFormatting.userFormatToContractFormat(userFixedPointString, decimals());
  }

  function getRobotState() external view override returns (IRobotStateEmitter.StateChangePayload memory) {
    IRobotStateEmitter.StateChangePayload memory payload = IRobotStateEmitter.StateChangePayload({
      uints: new IRobotStateEmitter.NamedUint[](2),
      floats: new IRobotStateEmitter.NamedFloat[](3),
      strings: new IRobotStateEmitter.NamedString[](0),
      addresses: new IRobotStateEmitter.NamedAddress[](0),
      bools: new IRobotStateEmitter.NamedBool[](0)
    });

    payload.uints[0] = IRobotStateEmitter.NamedUint("numTokenHolders", _holderCount);
    payload.uints[1] = IRobotStateEmitter.NamedUint("numTransfers", _transferCount);

    payload.floats[0] = IRobotStateEmitter.NamedFloat("amountTransferred", contractFormatToUserFormat(_transferred));
    payload.floats[1] = IRobotStateEmitter.NamedFloat("totalSupply", contractFormatToUserFormat(totalSupply()));
    payload.floats[2] = IRobotStateEmitter.NamedFloat("amountBurned", contractFormatToUserFormat(_burned));

    return payload;
  }

  function _msgSender() internal view override(ContextUpgradeable, PrimitiveBase) returns (address) {
    return PrimitiveBase._msgSender();
  }
}
