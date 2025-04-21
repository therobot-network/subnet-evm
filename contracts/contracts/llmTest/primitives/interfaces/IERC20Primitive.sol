// (c) 2022-2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

interface IERC20Primitive {
    function contractFormatToUserFormat(
        uint256 userInteger
    ) external view returns (string memory);
}
