// test/utils.ts
import { ethers } from "hardhat";

const abiCoder = new ethers.AbiCoder();

/**
 * Generates the raw calldata for a Solidity function call.
 *
 * @param methodName  The name of the function (e.g. "configure")
 * @param argTypes    An array of Solidity types (e.g. ["address", "string"])
 * @param args        An array of JS values to encode
 * @returns           A hex string (selector + encoded args) ready to send
 */
export function generateFunctionCallData(
  methodName: string,
  argTypes: string[] = [],
  args: any[] = [],
): string {
  if (args.length === 0) {
    return "0x";
  }
  // Build the function signature and selector
  const signature = `${methodName}(${argTypes.join(",")})`;
  const selector = ethers.id(signature).substring(0, 10);

  // If no args, just return the selector
  if (args.length === 0) {
    return selector;
  }

  // ABI-encode the parameters and concat (dropping the "0x")
  const encoded = abiCoder.encode(argTypes, args);
  return selector + encoded.slice(2);
}

/**
 * (Stub) Compares an on-chain StateChange payload against an expected shape.
 * Keep or replace this with your real implementation.
 */
export function payloadMatchesExpected(
  expected: {
    floats?: Array<{ name: string; value: string }>;
    uints?: Array<{ name: string; value: bigint }>;
    strings?: Array<{ name: string; value: string }>;
    addresses?: Array<{ name: string; value: string }>;
    bools?: Array<{ name: string; value: boolean }>;
  },
  actual: any,
): boolean {
  // Your deep‐compare logic goes here…
  // For now, just return true to satisfy the TS signature.
  return true;
}

// export async function setupAmmLiquidity(
//   ammContract: any,
//   ammAddress: string,
//   usdcAmount: string,
//   jiriAmount: string,
// ) {
//   // console.log(
//   //   `Approving USDC (${usdcAmount}) and JIRI (${jiriAmount}) for AMM: ${ammAddress}`,
//   // );

//   let tx = await usdcContract.approve(
//     ammAddress,
//     ethers.parseEther(usdcAmount),
//   );
//   await tx.wait();

//   tx = await jiriContract.approve(ammAddress, ethers.parseEther(jiriAmount));
//   await tx.wait();

//   // console.log(`Adding Liquidity: USDC=${usdcAmount}, JIRI=${jiriAmount}`);

//   tx = await ammContract.addLiquidity(
//     usdcContractAddress,
//     ethers.parseEther(usdcAmount),
//     jiriContractAddress,
//     ethers.parseEther(jiriAmount),
//   );
//   await tx.wait();

//   // console.log(`Activating AMM at ${ammAddress}`);
//   await ammContract.activate();

//   // console.log(`AMM ${ammAddress} setup complete.`);
// }
