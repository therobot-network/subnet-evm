// (c) 2019-2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

import { expect } from "chai";
import { Contract, Signer } from "ethers";
import { ethers } from "hardhat";
// import { test } from "./utils";

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

    let code = await ethers.provider.getCode(LLM_ADDRESS);
    expect(code).to.not.equal("0x");

    const ExampleLLM = await ethers.getContractFactory("ExampleLLMPrecompile", {
      owner,
    });
    testContract = await ExampleLLM.deploy();
    await testContract.waitForDeployment();
  });

  it("should test evaluatePrompt", async function () {
    const inputPrompt = "Hello World";
    // let health = await llmContract.healthCheck();
    // console.log("health: ", health);
    // health = await testContract.healthCheck();
    // console.log("health: ", health);

    // let expectedPromptId = 1n;
    const expectedAddress = ethers.ZeroAddress;
    const expectedBytes = ethers.toUtf8Bytes(inputPrompt);
    const expectedHex = ethers.hexlify(expectedBytes);

    const tx = await testContract.evaluatePrompt(inputPrompt);
    await tx.wait();
    await expect(tx).to.emit(testContract, "HealthCheck").withArgs(true);
    await expect(tx)
      .to.emit(testContract, "EvaluatePromptEvent")
      .withArgs(
        (promptId) => true,
        (contractMethodParams) => {
          const firstContractMethodParams = contractMethodParams[0];
          return (
            firstContractMethodParams.contractAddress == expectedAddress &&
            firstContractMethodParams.methodData == expectedHex
          );
        },
      );
  });

  it("should test continueEvaluation", async function () {
    const expectedAddress = ethers.ZeroAddress;
    const firstResultBytes = ethers.toUtf8Bytes("Hello World");
    const firstResultHex = ethers.hexlify(firstResultBytes);
    const secondResultBytes = ethers.toUtf8Bytes("Hello Mars");
    const secondResultHex = ethers.hexlify(secondResultBytes);
    const promptId = 1;
    const contractMethodResults = [firstResultHex, secondResultHex];

    const tx = await testContract.continueEvaluation(
      promptId,
      contractMethodResults,
    );
    await tx.wait();
    await expect(tx)
      .to.emit(testContract, "ContinueEvaluationEvent")
      .withArgs(
        (evaluationDone) => evaluationDone == false,
        (contractMethodParams) => {
          // Ensure the array length matches the input
          if (contractMethodParams.length !== contractMethodResults.length) {
            return false;
          }

          // Compare each parameter in contractMethodParams
          for (let i = 0; i < contractMethodParams.length; i++) {
            const param = contractMethodParams[i];
            if (
              param.contractAddress !== expectedAddress || // Ensure address matches
              param.methodData !== contractMethodResults[i] // Ensure methodData matches
            ) {
              return false;
            }
          }
          return true;
        },
      );
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
