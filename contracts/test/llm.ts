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

describe("ILLM", function () {
  let owner: Signer;
  let llmContract: Contract;
  let testContract: Contract;
  let counterAContract: Contract;
  let counterBContract: Contract;
  const counterAAddress = "0x17aB05351fC94a1a67Bf3f56DdbB941aE6c63E25";
  const counterBAddress = "0x5aa01B3b5877255cE50cc55e8986a7a5fe29C70e";

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
  });

  beforeEach(async function () {
    const counterACode = await ethers.provider.getCode(counterAAddress);
    if (counterACode == "0x") {
      const Counter = await ethers.getContractFactory(
        "CounterPrimitive",
        owner,
      );
      counterAContract = await Counter.deploy();
      await counterAContract.waitForDeployment();
      counterBContract = await Counter.deploy();
      await counterBContract.waitForDeployment();
      const counterAAddressChain = await counterAContract.getAddress();
      const counterBAddressChain = await counterBContract.getAddress();
      console.log("counterAAddress: ", counterAAddressChain);
      console.log("counterBAddress: ", counterBAddressChain);
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
    }
  });

  it("should test evaluatePrompt and continueEvaluation with lookup", async function () {
    // const recipientAddress = "0x000000000000000000000000000000000000dead";
    // const amount = ethers.parseUnits("10", 18).toString();

    const countAStart = await counterAContract.getCounter();
    const countBStart = await counterBContract.getCounter();

    const inputPrompt = `Hello World`;
    let promptIdRead: string;

    let tx = await testContract.evaluatePrompt(JSON.stringify({
      prompt: inputPrompt
    }));
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

  it("should test evaluatePlan and continueEvaluation basic", async function () {
    const planPath = path.resolve(__dirname, "llm_test_input_plans.json");

    // Read the JSON file containing the plans
    const fileContent = fs.readFileSync(planPath, "utf8");
    const plans = JSON.parse(fileContent);
    const withLookupPlan = JSON.stringify({
      plan: plans["basic"]
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
    const planPath = path.resolve(__dirname, "llm_test_input_plans.json");

    // Read the JSON file containing the plans
    const fileContent = fs.readFileSync(planPath, "utf8");
    const plans = JSON.parse(fileContent);
    const withLookupPlan = JSON.stringify({ plan: plans["withLookup"] });

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
});
