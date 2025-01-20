// (c) 2019-2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

import { expect } from "chai";
import { Contract, Signer } from "ethers";
import { ethers } from "hardhat";
// import { test } from "./utils";

const ADMIN_ADDRESS = "0x8db97C7cEcE249c2b98bDC0226Cc4C2A57BF52FC";
const LLM_ADDRESS = "0x0300000000000000000000000000000000000000";

describe("ILLM", function () {
  let owner: Signer;
  let llmContract: Contract;
  let testContract: Contract;
  let erc20Contract: Contract;
  const erc20Address = "0x52C84043CD9c865236f11d9Fc9F56aa003c1f922";

  before(async function () {
    const erc20Code = await ethers.provider.getCode(erc20Address);
    if (erc20Code == "0x") {
      const ERC20 = await ethers.getContractFactory("ERC20ForTesting", owner);
      erc20Contract = await ERC20.deploy();
      await erc20Contract.waitForDeployment();
    } else {
      erc20Contract = await ethers.getContractAt(
        "ERC20ForTesting",
        erc20Address,
        owner,
      );
    }
  });

  beforeEach(async function () {
    //   const network = await ethers.provider.getNetwork();
    //   console.log(`Connected to chain ID: ${network.chainId}`);
    owner = await ethers.getSigner(ADMIN_ADDRESS);
    llmContract = await ethers.getContractAt("ILLM", LLM_ADDRESS, owner);

    let llmCode = await ethers.provider.getCode(LLM_ADDRESS);
    expect(llmCode).to.not.equal("0x");

    const ExampleLLM = await ethers.getContractFactory("ExampleLLMPrecompile", {
      owner,
    });
    testContract = await ExampleLLM.deploy();
    await testContract.waitForDeployment();
  });

  it.only("should test evaluatePrompt", async function () {
    // const recipientAddress = "0x000000000000000000000000000000000000dead";
    // const amount = ethers.parseUnits("10", 18).toString();

    const inputPrompt = `Hello World`;

    const tx = await testContract.evaluatePrompt(inputPrompt);
    await tx.wait();
    let methodData: string;
    let calleeContractAddress: string;
    await expect(tx)
      .to.emit(testContract, "EvaluatePromptEvent")
      .withArgs(
        (promptId) => true,
        (contractMethodParams) => {
          contractMethodParams[0].contractAddress == erc20Address;
          calleeContractAddress = contractMethodParams[0].contractAddress;
          methodData = contractMethodParams[0].methodData;
          return true;
        },
      );

    // Use the captured params to call the ERC20 method
    const result = await owner.call({
      to: calleeContractAddress,
      data: methodData,
    });
    const resultAsBigInt = BigInt(result);
    const balance = await erc20Contract.balanceOf(ADMIN_ADDRESS);
    expect(resultAsBigInt).to.equal(balance);
    // Verify the transfer occurred
    // const balance = await erc20Contract.balanceOf(recipientAddress);
    // expect(balance).to.equal(amount);
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
        (evaluationDone) => evaluationDone == true,
        (contractMethodParams) => {
          // Ensure the array length matches the input
          if (contractMethodParams.length !== 0) {
            return false;
          }
          return true;
        },
      );
  });
});
