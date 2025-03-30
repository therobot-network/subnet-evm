// (c) 2019-2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

import { expect } from "chai";
import { Contract, Signer } from "ethers";
import { ethers } from "hardhat";
import fs from "fs";
import * as path from "path";
import { maxHeaderSize } from "http";
// import { test } from "./utils";

const ADMIN_ADDRESS = "0x8db97C7cEcE249c2b98bDC0226Cc4C2A57BF52FC";
const user1Address = "0x8db97C7cEcE249c2b98bDC0226Cc4C2A57BF52FC";
const LLM_ADDRESS = "0x0300000000000000000000000000000000000000";
const boolenTrueHash =
  "0x0000000000000000000000000000000000000000000000000000000000000001";

describe("LLM Precompiled Contract", function () {
  let owner: Signer;
  let user1: Signer;
  let llmContract: Contract;
  let testContract: Contract;

  // Read the JSON file containing the plans
  const planPath = path.resolve(__dirname, "llm_test_input_plans.json");
  const fileContent = fs.readFileSync(planPath, "utf8");
  const plans = JSON.parse(fileContent);

  async function continueEvaluationAndCall(
    testContract: any,
    owner: any,
    promptIdRead: string,
    contractMethodResult: string = boolenTrueHash,
  ) {
    // Call continueEvaluation with the provided result
    let tx = await testContract.continueEvaluation(promptIdRead, [
      contractMethodResult,
    ]);
    await tx.wait();

    let calleeContractAddress: string;
    let methodData: string;

    // Expect event to be emitted and extract data
    await expect(tx)
      .to.emit(testContract, "ContinueEvaluationEvent")
      .withArgs(
        (evaluationDone: boolean) => evaluationDone === false,
        (contractMethodParams: any[]) => {
          calleeContractAddress = contractMethodParams[0].contractAddress;
          methodData = contractMethodParams[0].methodData;
          return true;
        },
      );

    // Call the next contract with retrieved data
    return await owner.call({
      to: calleeContractAddress,
      data: methodData,
    });
  }

  async function continueEvaluationAndSend(
    testContract: any,
    owner: any,
    promptIdRead: string,
    contractMethodResult: string = boolenTrueHash,
  ) {
    // Call continueEvaluation with the provided result
    let tx = await testContract.continueEvaluation(promptIdRead, [
      contractMethodResult,
    ]);
    await tx.wait();

    let calleeContractAddress: string;
    let methodData: string;

    // Expect event to be emitted and extract data
    await expect(tx)
      .to.emit(testContract, "ContinueEvaluationEvent")
      .withArgs(
        (evaluationDone: boolean) => evaluationDone === false,
        (contractMethodParams: any[]) => {
          calleeContractAddress = contractMethodParams[0].contractAddress;
          methodData = contractMethodParams[0].methodData;
          return true;
        },
      );

    // Call the next contract with retrieved data
    const result = await owner.sendTransaction({
      to: calleeContractAddress,
      data: methodData,
    });

    await result.wait();
  }

  before(async function () {
    owner = await ethers.getSigner(ADMIN_ADDRESS);

    llmContract = (await ethers.getContractAt(
      "ILLM",
      LLM_ADDRESS,
      owner,
    )) as unknown as Contract;

    let llmCode = await ethers.provider.getCode(LLM_ADDRESS);
    expect(llmCode).to.not.equal("0x");

    const ExampleLLM = await ethers.getContractFactory("ExampleLLMPrecompile", {
      owner,
    });
    testContract = (await ExampleLLM.deploy()) as unknown as Contract;
    await testContract.waitForDeployment();
  });

  it("should test dictionary creation system primitives", async function () {
    const dictionaryPlan = JSON.stringify({
      plan: JSON.stringify(plans["withAssignDict"]),
    });

    let promptIdRead: string;

    let tx = await testContract.evaluatePlan(dictionaryPlan);
    const receipt = await tx.wait();
    let methodData: string;
    let calleeContractAddress: string;
    await expect(tx)
      .to.emit(testContract, "EvaluatePlanEvent")
      .withArgs(
        (promptId) => {
          promptIdRead = promptId;
          return true;
        },
        (contractMethodParams) => contractMethodParams.length === 0,
      );

    const questionEvents = receipt.logs
      .map((log) => {
        try {
          return llmContract.interface.parseLog(log);
        } catch {
          return null;
        }
      })
      .filter((log) => log?.name === "QuestionAnswer");

    expect(questionEvents.length).to.equal(3);
    expect(questionEvents[0].args.question).to.equal("Adam's balance is");
    expect(questionEvents[0].args.answer).to.equal("100");
    expect(questionEvents[1].args.question).to.equal("Karen's balance is");
    expect(questionEvents[1].args.answer).to.equal("Default Value");
    expect(questionEvents[2].args.question).to.equal("Karen's new balance is");
    expect(questionEvents[2].args.answer).to.equal("300");
  });

  it("should test dictionary manipulation system primitives", async function () {
    const dictionaryPlan = JSON.stringify({
      plan: JSON.stringify(plans["toArray"]),
    });

    let promptIdRead: string;

    let tx = await testContract.evaluatePlan(dictionaryPlan);
    const receipt = await tx.wait();
    let methodData: string;
    let calleeContractAddress: string;
    await expect(tx)
      .to.emit(testContract, "EvaluatePlanEvent")
      .withArgs(
        (promptId) => {
          promptIdRead = promptId;
          return true;
        },
        (contractMethodParams) => contractMethodParams.length === 0,
      );

    const questionEvents = receipt.logs
      .map((log) => {
        try {
          return llmContract.interface.parseLog(log);
        } catch {
          return null;
        }
      })
      .filter((log) => log?.name === "QuestionAnswer");

    expect(questionEvents.length).to.equal(5);
    expect(questionEvents[0].args.question).to.equal("keysArray is");
    expect(questionEvents[0].args.answer).to.equal("[adam bill]");
    expect(questionEvents[1].args.question).to.equal("valuesArray is");
    expect(questionEvents[1].args.answer).to.equal("[100 200]");
    expect(questionEvents[2].args.question).to.equal("dictArray is");
    expect(questionEvents[2].args.answer).to.equal("[adam bill]");
    expect(questionEvents[3].args.question).to.equal("forItemsKeysArray is");
    expect(questionEvents[3].args.answer).to.equal("[adam bill]");
    expect(questionEvents[4].args.question).to.equal("forItemsValuesArray is");
    expect(questionEvents[4].args.answer).to.equal("[100 200]");
  });

  it("should test length system primitives", async function () {
    const dictionaryPlan = JSON.stringify({
      plan: JSON.stringify(plans["withLen"]),
    });

    let promptIdRead: string;

    let tx = await testContract.evaluatePlan(dictionaryPlan);
    const receipt = await tx.wait();
    let methodData: string;
    let calleeContractAddress: string;
    await expect(tx)
      .to.emit(testContract, "EvaluatePlanEvent")
      .withArgs(
        (promptId) => {
          promptIdRead = promptId;
          return true;
        },
        (contractMethodParams) => contractMethodParams.length === 0,
      );

    const questionEvents = receipt.logs
      .map((log) => {
        try {
          return llmContract.interface.parseLog(log);
        } catch {
          return null;
        }
      })
      .filter((log) => log?.name === "QuestionAnswer");

    expect(questionEvents.length).to.equal(2);
    expect(questionEvents[0].args.question).to.equal("dictionary length is");
    expect(questionEvents[0].args.answer).to.equal("2");
    expect(questionEvents[1].args.question).to.equal("array length is");
    expect(questionEvents[1].args.answer).to.equal("3");
  });

  it("should test index system primitives", async function () {
    const dictionaryPlan = JSON.stringify({
      plan: JSON.stringify(plans["withIndex"]),
    });

    let promptIdRead: string;

    let tx = await testContract.evaluatePlan(dictionaryPlan);
    const receipt = await tx.wait();
    let methodData: string;
    let calleeContractAddress: string;
    await expect(tx)
      .to.emit(testContract, "EvaluatePlanEvent")
      .withArgs(
        (promptId) => {
          promptIdRead = promptId;
          return true;
        },
        (contractMethodParams) => contractMethodParams.length === 0,
      );

    const questionEvents = receipt.logs
      .map((log) => {
        try {
          return llmContract.interface.parseLog(log);
        } catch {
          return null;
        }
      })
      .filter((log) => log?.name === "QuestionAnswer");

    expect(questionEvents.length).to.equal(1);
    expect(questionEvents[0].args.question).to.equal("index 1 is");
    expect(questionEvents[0].args.answer).to.equal("a");
  });
});
