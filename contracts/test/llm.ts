// (c) 2019-2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

import { expect } from "chai";
import { Contract, Signer } from "ethers";
import { ethers } from "hardhat";
import fs from "fs";
import * as path from "path";
// import { test } from "./utils";

const ADMIN_ADDRESS = "0x8db97C7cEcE249c2b98bDC0226Cc4C2A57BF52FC";
const LLM_ADDRESS = "0x0300000000000000000000000000000000000000";

describe("LLM Precompiled Contract", function () {
  let owner: Signer;
  let llmContract: Contract;
  let testContract: Contract;
  let counterAContract: Contract;
  let counterBContract: Contract;
  let erc20Contract: Contract;
  let mathContract: Contract;
  const counterAAddress = "0x17aB05351fC94a1a67Bf3f56DdbB941aE6c63E25";
  const counterBAddress = "0x5aa01B3b5877255cE50cc55e8986a7a5fe29C70e";
  const erc20Address = "0x5DB9A7629912EBF95876228C24A848de0bfB43A9";
  const mathAddress = "0x4Ac1d98D9cEF99EC6546dEd4Bd550b0b287aaD6D";

  // Read the JSON file containing the plans
  const planPath = path.resolve(__dirname, "llm_test_input_plans.json");
  const fileContent = fs.readFileSync(planPath, "utf8");
  const plans = JSON.parse(fileContent);

  before(async function () {
    owner = await ethers.getSigner(ADMIN_ADDRESS);
    // llmContract = await ethers.getContractAt("ILLM", LLM_ADDRESS, owner);

    let llmCode = await ethers.provider.getCode(LLM_ADDRESS);
    expect(llmCode).to.not.equal("0x");

    const ExampleLLM = await ethers.getContractFactory("ExampleLLMPrecompile", {
      owner,
    });
    testContract = await ExampleLLM.deploy();
    await testContract.waitForDeployment();

    const counterACode = await ethers.provider.getCode(counterAAddress);
    if (counterACode == "0x") {
      const Counter = await ethers.getContractFactory(
        "CounterPrimitive",
        owner,
      );
      const ERC20 = await ethers.getContractFactory("ERC20Primitive", owner);
      const Math = await ethers.getContractFactory("MathPrimitive", owner);
      counterAContract = await Counter.deploy();
      await counterAContract.waitForDeployment();
      counterBContract = await Counter.deploy();
      await counterBContract.waitForDeployment();
      erc20Contract = await ERC20.deploy();
      await erc20Contract.waitForDeployment();
      mathContract = await Math.deploy();
      await mathContract.waitForDeployment();
      const counterAAddressChain = await counterAContract.getAddress();
      const counterBAddressChain = await counterBContract.getAddress();
      const erc20AddressChain = await erc20Contract.getAddress();
      const mathAddressChain = await mathContract.getAddress();
      console.log("counterAAddress: ", counterAAddressChain);
      console.log("counterBAddress: ", counterBAddressChain);
      console.log("erc20AddressChain: ", erc20AddressChain);
      console.log("mathAddressChain: ", mathAddressChain);
    } else {
      counterAContract = await ethers.getContractAt(
        "CounterPrimitive",
        counterAAddress,
        owner,
      );
      counterBContract = await ethers.getContractAt(
        "CounterPrimitive",
        counterBAddress,
        owner,
      );
      erc20Contract = await ethers.getContractAt(
        "ERC20Primitive",
        erc20Address,
        owner,
      );
      mathContract = await ethers.getContractAt(
        "CounterPrimitive",
        mathAddress,
        owner,
      );
    }
  });

  it.skip("should test evaluatePrompt and continueEvaluation with lookup", async function () {
    const inputPrompt = `transfer 5 @USDC to @user1`;
    let promptIdRead: string;
    const user1Address = "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266";

    const adminBalanceStart = await erc20Contract.balanceOf(ADMIN_ADDRESS);
    const userBalanceStart = await erc20Contract.balanceOf(user1Address);

    // should fail when prompt key is not passed
    let isFailed = false;
    await testContract
      .evaluatePrompt(JSON.stringify({ plan: inputPrompt }))
      .catch((err) => {
        isFailed = true;
      });
    expect(isFailed).to.be.true;

    let tx = await testContract.evaluatePrompt(
      JSON.stringify({
        prompt: inputPrompt,
        lookupTable: JSON.stringify({
          USDC: erc20Address,
          user1: user1Address,
        }),
      }),
    );
    await tx.wait();
    let methodData: string;
    let calleeContractAddress: string;
    await expect(tx)
      .to.emit(testContract, "EvaluatePromptEvent")
      .withArgs(
        (promptId) => {
          promptIdRead = promptId;
          return true;
        },
        (contractMethodParams) => {
          calleeContractAddress = contractMethodParams[0].contractAddress;
          methodData = contractMethodParams[0].methodData;
          return true;
        },
      );

    // Update Counter A
    let result = await owner.sendTransaction({
      // let result = await owner.call({
      to: calleeContractAddress,
      data: methodData,
    });

    tx = await testContract.continueEvaluation(
      promptIdRead,
      ["0x0000000000000000000000000000000000000000000000000000000000000001"], //  'true'
      // contractMethodResults,
    );
    await tx.wait();
    await expect(tx)
      .to.emit(testContract, "ContinueEvaluationEvent")
      .withArgs(
        (evaluationDone) => evaluationDone == true,
        (contractMethodParams) => {
          return true;
        },
      );

    const adminBalanceEnd = await erc20Contract.balanceOf(ADMIN_ADDRESS);
    const userBalanceEnd = await erc20Contract.balanceOf(user1Address);

    expect(adminBalanceEnd).to.equal(adminBalanceStart - 5n);
    expect(userBalanceEnd).to.equal(userBalanceStart + 5n);
  });

  it("should test evaluatePlan and continueEvaluation basic", async function () {
    // should fail when plan is not passed
    let isFailed = false;
    await testContract
      .evaluatePlan(JSON.stringify({ prompt: "" }))
      .catch((err) => {
        isFailed = true;
      });
    expect(isFailed).to.be.true;

    // Read the JSON file containing the plans
    const withLookupPlan = JSON.stringify({
      plan: JSON.stringify(plans["basic"]),
      lookupTable: JSON.stringify({
        USDC: "0x17aB05351fC94a1a67Bf3f56DdbB941aE6c63E25",
        recipient: "0x000000000000000000000000000000000000dead",
      }),
    });

    const countAStart = await counterAContract.getCounter();
    const countBStart = await counterBContract.getCounter();

    let promptIdRead: string;

    let tx = await testContract.evaluatePlan(withLookupPlan);
    await tx.wait();
    let methodData: string;
    let calleeContractAddress: string;
    await expect(tx)
      .to.emit(testContract, "EvaluatePlanEvent")
      .withArgs(
        (promptId) => {
          promptIdRead = promptId;
          return true;
        },
        (contractMethodParams) => {
          calleeContractAddress = contractMethodParams[0].contractAddress;
          methodData = contractMethodParams[0].methodData;
          return true;
        },
      );

    // Update Counter A
    let result = await owner.sendTransaction({
      // let result = await owner.call({
      to: calleeContractAddress,
      data: methodData,
    });

    tx = await testContract.continueEvaluation(
      promptIdRead,
      ["0x000000000000000000000000000000000000000000000000000000000000000b"],
      // contractMethodResults,
    );
    await tx.wait();
    await expect(tx)
      .to.emit(testContract, "ContinueEvaluationEvent")
      .withArgs(
        (evaluationDone) => evaluationDone == false,
        (contractMethodParams) => {
          calleeContractAddress = contractMethodParams[0].contractAddress;
          methodData = contractMethodParams[0].methodData;
          return true;
        },
      );

    // Update Counter B
    result = await owner.sendTransaction({
      // let result = await owner.call({
      to: calleeContractAddress,
      data: methodData,
    });

    tx = await testContract.continueEvaluation(promptIdRead, [
      "0x0000000000000000000000000000000000000000000000000000000000000000",
    ]);
    await tx.wait();
    await expect(tx)
      .to.emit(testContract, "ContinueEvaluationEvent")
      .withArgs(
        (evaluationDone) => evaluationDone == true,
        (contractMethodParams) => true,
      );

    const countAEnd = await counterAContract.getCounter();
    const countBEnd = await counterBContract.getCounter();

    expect(countAEnd).to.equal(countAStart + 10n);
    expect(countBEnd).to.equal(countBStart + 20n);
  });

  it("should test evaluatePlan and continueEvaluation with lookup", async function () {
    // Read the JSON file containing the plans
    const withLookupPlan = JSON.stringify({
      plan: JSON.stringify(plans["withLookup"]),
    });

    const countAStart = await counterAContract.getCounter();
    const countBStart = await counterBContract.getCounter();

    let promptIdRead: string;

    let tx = await testContract.evaluatePlan(withLookupPlan);
    await tx.wait();
    let methodData: string;
    let calleeContractAddress: string;
    await expect(tx)
      .to.emit(testContract, "EvaluatePlanEvent")
      .withArgs(
        (promptId) => {
          promptIdRead = promptId;
          return true;
        },
        (contractMethodParams) => {
          calleeContractAddress = contractMethodParams[0].contractAddress;
          methodData = contractMethodParams[0].methodData;
          return true;
        },
      );

    // Update Counter A
    let result = await owner.sendTransaction({
      // let result = await owner.call({
      to: calleeContractAddress,
      data: methodData,
    });

    tx = await testContract.continueEvaluation(
      promptIdRead,
      ["0x000000000000000000000000000000000000000000000000000000000000000b"],
      // contractMethodResults,
    );
    await tx.wait();
    await expect(tx)
      .to.emit(testContract, "ContinueEvaluationEvent")
      .withArgs(
        (evaluationDone) => evaluationDone == false,
        (contractMethodParams) => {
          calleeContractAddress = contractMethodParams[0].contractAddress;
          methodData = contractMethodParams[0].methodData;
          return true;
        },
      );

    // Read counter A
    // result = await owner.sendTransaction({
    let resultTx = await owner.call({
      to: calleeContractAddress,
      data: methodData,
    });

    tx = await testContract.continueEvaluation(
      promptIdRead,
      [resultTx],
      // contractMethodResults,
    );
    await tx.wait();
    await expect(tx)
      .to.emit(testContract, "ContinueEvaluationEvent")
      .withArgs(
        (evaluationDone) => evaluationDone == false,
        (contractMethodParams) => {
          calleeContractAddress = contractMethodParams[0].contractAddress;
          methodData = contractMethodParams[0].methodData;
          return true;
        },
      );

    // Update Counter B
    result = await owner.sendTransaction({
      // let result = await owner.call({
      to: calleeContractAddress,
      data: methodData,
    });

    tx = await testContract.continueEvaluation(
      promptIdRead,
      ["0x000000000000000000000000000000000000000000000000000000000000001e"],
      // contractMethodResults,
    );
    await tx.wait();
    await expect(tx)
      .to.emit(testContract, "ContinueEvaluationEvent")
      .withArgs(
        (evaluationDone) => evaluationDone == true,
        (contractMethodParams) => true,
      );

    const countAEnd = await counterAContract.getCounter();
    const countBEnd = await counterBContract.getCounter();

    expect(countAEnd).to.equal(countAStart + 10n);
    expect(countBEnd).to.equal(countBStart + countAEnd);
  });

  it("should test evaluatePlan and continueEvaluation with erc20 and math", async function () {
    const withMathAndErc20Plan = JSON.stringify({
      plan: JSON.stringify(plans["withMathAndErc20"]),
      lookupTable: JSON.stringify({
        user1: "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266",
      }),
    });

    let adminBalanceStart = await erc20Contract.balanceOf(ADMIN_ADDRESS);
    let userBalanceStart = await erc20Contract.balanceOf(
      "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266",
    );

    let promptIdRead: string;

    let tx = await testContract.evaluatePlan(withMathAndErc20Plan);
    await tx.wait();
    let methodData: string;
    let calleeContractAddress: string;
    await expect(tx)
      .to.emit(testContract, "EvaluatePlanEvent")
      .withArgs(
        (promptId) => {
          promptIdRead = promptId;
          return true;
        },
        (contractMethodParams) => {
          calleeContractAddress = contractMethodParams[0].contractAddress;
          methodData = contractMethodParams[0].methodData;
          return true;
        },
      );

    // Transfer 6 to user
    let result = await owner.sendTransaction({
      // let result = await owner.call({
      to: calleeContractAddress,
      data: methodData,
    });
    await result.wait();

    let adminBalanceEnd = await erc20Contract.balanceOf(ADMIN_ADDRESS);
    let userBalanceEnd = await erc20Contract.balanceOf(
      "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266",
    );

    expect(adminBalanceEnd).to.equal(adminBalanceStart - 600n);
    expect(userBalanceEnd).to.equal(userBalanceStart + 600n);

    adminBalanceStart = adminBalanceEnd;
    userBalanceStart = userBalanceEnd;

    tx = await testContract.continueEvaluation(
      promptIdRead,
      ["0x0000000000000000000000000000000000000000000000000000000000000001"],
      // contractMethodResults,
    );
    await tx.wait();
    await expect(tx)
      .to.emit(testContract, "ContinueEvaluationEvent")
      .withArgs(
        (evaluationDone) => evaluationDone == false,
        (contractMethodParams) => {
          calleeContractAddress = contractMethodParams[0].contractAddress;
          methodData = contractMethodParams[0].methodData;
          return true;
        },
      );

    // Read admin balance
    // result = await owner.sendTransaction({
    let resultTx = await owner.call({
      to: calleeContractAddress,
      data: methodData,
    });

    tx = await testContract.continueEvaluation(
      promptIdRead,
      [resultTx],
      // contractMethodResults,
    );
    await tx.wait();
    await expect(tx)
      .to.emit(testContract, "ContinueEvaluationEvent")
      .withArgs(
        (evaluationDone) => evaluationDone == false,
        (contractMethodParams) => {
          calleeContractAddress = contractMethodParams[0].contractAddress;
          methodData = contractMethodParams[0].methodData;
          return true;
        },
      );

    // Divide by 2
    // result = await owner.sendTransaction({
    resultTx = await owner.call({
      to: calleeContractAddress,
      data: methodData,
    });

    tx = await testContract.continueEvaluation(
      promptIdRead,
      [resultTx],
      // contractMethodResults,
    );
    await tx.wait();
    await expect(tx)
      .to.emit(testContract, "ContinueEvaluationEvent")
      .withArgs(
        (evaluationDone) => evaluationDone == false,
        (contractMethodParams) => {
          calleeContractAddress = contractMethodParams[0].contractAddress;
          methodData = contractMethodParams[0].methodData;
          return true;
        },
      );

    // transfer result
    result = await owner.sendTransaction({
      // let result = await owner.call({
      to: calleeContractAddress,
      data: methodData,
    });
    await result.wait();

    tx = await testContract.continueEvaluation(
      promptIdRead,
      ["0x0000000000000000000000000000000000000000000000000000000000000001"],
      // contractMethodResults,
    );
    await tx.wait();
    await expect(tx)
      .to.emit(testContract, "ContinueEvaluationEvent")
      .withArgs(
        (evaluationDone) => evaluationDone == true,
        (contractMethodParams) => true,
      );

    adminBalanceEnd = await erc20Contract.balanceOf(ADMIN_ADDRESS);
    userBalanceEnd = await erc20Contract.balanceOf(
      "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266",
    );

    expect(adminBalanceEnd).to.equal(adminBalanceStart / 2n);
    expect(userBalanceEnd).to.equal(userBalanceStart + adminBalanceStart / 2n);
  });

  it("should test evaluatePlan and continueEvaluation with assign system primitive", async function () {
    const withLookupPlan = JSON.stringify({
      plan: JSON.stringify(plans["withAssign"]),
    });

    const countAStart = await counterAContract.getCounter();
    const countBStart = await counterBContract.getCounter();

    let promptIdRead: string;

    let tx = await testContract.evaluatePlan(withLookupPlan);
    await tx.wait();
    let methodData: string;
    let calleeContractAddress: string;
    await expect(tx)
      .to.emit(testContract, "EvaluatePlanEvent")
      .withArgs(
        (promptId) => {
          promptIdRead = promptId;
          return true;
        },
        (contractMethodParams) => {
          calleeContractAddress = contractMethodParams[0].contractAddress;
          methodData = contractMethodParams[0].methodData;
          return true;
        },
      );

    // Update Counter A
    let result = await owner.sendTransaction({
      // let result = await owner.call({
      to: calleeContractAddress,
      data: methodData,
    });

    tx = await testContract.continueEvaluation(
      promptIdRead,
      ["0x0000000000000000000000000000000000000000000000000000000000000001"],
      // contractMethodResults,
    );
    await tx.wait();
    await expect(tx)
      .to.emit(testContract, "ContinueEvaluationEvent")
      .withArgs(
        (evaluationDone) => evaluationDone == false,
        (contractMethodParams) => {
          calleeContractAddress = contractMethodParams[0].contractAddress;
          methodData = contractMethodParams[0].methodData;
          return true;
        },
      );

    // Read counter A
    // result = await owner.sendTransaction({
    let resultTx = await owner.call({
      to: calleeContractAddress,
      data: methodData,
    });

    tx = await testContract.continueEvaluation(
      promptIdRead,
      [resultTx],
      // contractMethodResults,
    );
    await tx.wait();
    await expect(tx)
      .to.emit(testContract, "ContinueEvaluationEvent")
      .withArgs(
        (evaluationDone) => evaluationDone == false,
        (contractMethodParams) => {
          calleeContractAddress = contractMethodParams[0].contractAddress;
          methodData = contractMethodParams[0].methodData;
          return true;
        },
      );

    // Update Counter B
    result = await owner.sendTransaction({
      // let result = await owner.call({
      to: calleeContractAddress,
      data: methodData,
    });

    tx = await testContract.continueEvaluation(
      promptIdRead,
      ["0x000000000000000000000000000000000000000000000000000000000000001e"],
      // contractMethodResults,
    );
    await tx.wait();
    await expect(tx)
      .to.emit(testContract, "ContinueEvaluationEvent")
      .withArgs(
        (evaluationDone) => evaluationDone == true,
        (contractMethodParams) => true,
      );

    const countAEnd = await counterAContract.getCounter();
    const countBEnd = await counterBContract.getCounter();

    expect(countAEnd).to.equal(countAStart + 10n);
    expect(countBEnd).to.equal(countBStart + countAEnd);
  });

  it("should test evaluatePlan and continueEvaluation with jumpIf system primitive", async function () {
    const withJumpIfPlan = JSON.stringify({
      plan: JSON.stringify(plans["withJumpIf"]),
    });

    let promptIdRead: string;

    let tx = await testContract.evaluatePlan(withJumpIfPlan);
    await tx.wait();
    let methodData: string;
    let calleeContractAddress: string;
    await expect(tx)
      .to.emit(testContract, "EvaluatePlanEvent")
      .withArgs(
        (promptId) => {
          promptIdRead = promptId;
          return true;
        },
        (contractMethodParams) => {
          calleeContractAddress = contractMethodParams[0].contractAddress;
          methodData = contractMethodParams[0].methodData;
          return true;
        },
      );

    // Reset Counter A
    let result = await owner.sendTransaction({
      // let result = await owner.call({
      to: calleeContractAddress,
      data: methodData,
    });

    let countATemp = await counterAContract.getCounter();

    tx = await testContract.continueEvaluation(
      promptIdRead,
      ["0x0000000000000000000000000000000000000000000000000000000000000001"],
      // contractMethodResults,
    );
    await tx.wait();
    await expect(tx)
      .to.emit(testContract, "ContinueEvaluationEvent")
      .withArgs(
        (evaluationDone) => evaluationDone == false,
        (contractMethodParams) => {
          calleeContractAddress = contractMethodParams[0].contractAddress;
          methodData = contractMethodParams[0].methodData;
          return true;
        },
      );

    // First JumpIf - should jump to 'increase 20'
    result = await owner.sendTransaction({
      to: calleeContractAddress,
      data: methodData,
    });

    countATemp = await counterAContract.getCounter();

    // Should not Jump to end
    tx = await testContract.continueEvaluation(
      promptIdRead,
      ["0x0000000000000000000000000000000000000000000000000000000000000001"],
      // contractMethodResults,
    );
    await tx.wait();
    await expect(tx)
      .to.emit(testContract, "ContinueEvaluationEvent")
      .withArgs(
        (evaluationDone) => evaluationDone == false,
        (contractMethodParams) => {
          calleeContractAddress = contractMethodParams[0].contractAddress;
          methodData = contractMethodParams[0].methodData;
          return true;
        },
      );

    // Increase 30
    result = await owner.sendTransaction({
      // let result = await owner.call({
      to: calleeContractAddress,
      data: methodData,
    });

    countATemp = await counterAContract.getCounter();

    tx = await testContract.continueEvaluation(
      promptIdRead,
      ["0x0000000000000000000000000000000000000000000000000000000000000001"],
      // contractMethodResults,
    );
    await tx.wait();
    await expect(tx)
      .to.emit(testContract, "ContinueEvaluationEvent")
      .withArgs(
        (evaluationDone) => evaluationDone == true,
        (contractMethodParams) => true,
      );

    const countAEnd = await counterAContract.getCounter();
    expect(countAEnd).to.equal(20n + 30n);
  });
});
