// (c) 2019-2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

import { expect } from "chai";
import { Contract, Signer } from "ethers";
import { ethers } from "hardhat";
import { test } from "./utils";

// make sure this is always an admin for hello world precompile
const ADMIN_ADDRESS = "0x8db97C7cEcE249c2b98bDC0226Cc4C2A57BF52FC";
const LLM_ADDRESS = "0x0300000000000000000000000000000000000000";

describe("ILLM", function () {
  let owner: Signer;
  let llmContract: Contract;
  let testContract: Contract;
  let ownerAddress: string;

  beforeEach(async function () {
    //   const network = await ethers.provider.getNetwork();
    //   console.log(`Connected to chain ID: ${network.chainId}`);
    owner = await ethers.getSigner(ADMIN_ADDRESS);
    ownerAddress = await owner.getAddress();
    llmContract = await ethers.getContractAt("ILLM", LLM_ADDRESS, owner);

    const code = await ethers.provider.getCode(LLM_ADDRESS);
    console.log("Contract code at address:", code);

    const WARP_ADDRESS = "0x0200000000000000000000000000000000000005";

    const warpCode = await ethers.provider.getCode(WARP_ADDRESS);
    console.log("Contract code at address:", warpCode);

    const ExampleLLM = await ethers.getContractFactory("ExampleLLM", {
      owner,
    });
    testContract = await ExampleLLM.deploy();
    await testContract.waitForDeployment();

    // const testContractAddress = await testContract.getAddress(); // Use the contract directly
    // const tx = await llmContract.setEnabled(testContractAddress);

    // // Wait for the transaction to be mined
    // await tx.wait();

    // const isHealthy = await testContract.healthCheck();
    // console.log("isHealthy: ", isHealthy);

    // const adminEnabled = await llmContract.readAllowList(ADMIN_ADDRESS);
    // console.log("isEnabled: ", adminEnabled);

    // const isEnabled = await llmContract.readAllowList(testContractAddress);
    // console.log("isEnabled: ", isEnabled);
  });

  it("should test evaluatePrompt", async function () {
    const tx = await testContract.evaluatePrompt("Hello World");
    await tx.wait();
    await expect(tx).to.emit(testContract, "EvaluatePromptEvent");
    // .withArgs(
    //   1,
    // );
  });
});

// describe("ExampleLLMTest", function () {
//   this.timeout("30s");

//   beforeEach("Setup DS-Test contract", async function () {
//     const signer = await ethers.getSigner(ADMIN_ADDRESS);
//     const helloWorldPromise = ethers.getContractAt("ILLM", LLM_ADDRESS, signer);

//     // return ethers
//     //   .getContractFactory("ExampleLLMTest", { signer })
//     //   .then((factory) => factory.deploy())
//     //   .then((contract) => {
//     //     this.testContract = contract;
//     //     return contract.waitForDeployment().then(() => contract);
//     //   })
//     //   .then(() => Promise.all([helloWorldPromise]))
//     //   .then(([helloWorld]) =>
//     //     helloWorld.setAdmin(await testContract.getAddress()),
//     //   )
//     //   .then((tx) => tx.wait());

//     const factory = await ethers.getContractFactory("ExampleLLMTest", {
//       signer,
//     });
//     const contract = await factory.deploy();
//     await contract.waitForDeployment(); // Wait for deployment to complete

//     this.testContract = contract; // Assign the deployed contract to `this.testContract`

//     // Wait for the promise from helloWorldPromise to resolve
//     const helloWorld = await helloWorldPromise;

//     // Call `setAdmin` on the LLM contract
//     const contractAddress = await contract.getAddress(); // Use the contract directly
//     const tx = await helloWorld.setAdmin(contractAddress);

//     // Wait for the transaction to be mined
//     const txFinal = await tx.wait();

//     // Return the final transaction receipt
//     return txFinal;
//   });

// test("should gets default hello world", ["step_getDefaultLLM"]);

// test(
//   "should not set greeting before enabled",
//   "step_doesNotSetGreetingBeforeEnabled",
// );

// test(
//   "should set and get greeting with enabled account",
//   "step_setAndGetGreeting",
// );
// });
