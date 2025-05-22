// (c) 2019-2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

import { expect } from "chai";
import { BaseContract, Contract, Signer } from "ethers";
import { ethers } from "hardhat";
import fs from "fs";
import * as path from "path";
import yaml from "js-yaml";
import { setupTestEnvironment, TestEnv } from "./helpers/setupFixtures";

const ADMIN_ADDRESS = "0x8db97C7cEcE249c2b98bDC0226Cc4C2A57BF52FC";
const LLM_ADDRESS = "0x0300000000000000000000000000000000000000";
const boolenTrueHash =
  "0x0000000000000000000000000000000000000000000000000000000000000001";

describe("LLM Precompiled Contract", function () {
  let env: TestEnv;
  let executor: Contract;
  let executorAddress: string;
  let owner: Signer;
  let llmContract: Contract;
  let counterAContract: Contract;
  let counterAContractAddress: string;
  let counterBContract: Contract;
  let counterBContractAddress: string;
  let mathContractAddress: string;
  let ammContract1: Contract;
  let ammContract1Address: string;
  let ammContract2: Contract;
  let ammContract2Address: string;
  let ammContract3: Contract;
  let ammContract3Address: string;
  let usdcContract: Contract;
  let usdcContractAddress: string;
  let jiriContract: Contract;
  let jiriContractAddress: string;

  const yamlPath = path.resolve(__dirname, "planTests", "tests");

  // Read the JSON file containing the plans
  // const planPath = path.resolve(__dirname, "llm_test_input_plans.json");
  // const fileContent = fs.readFileSync(planPath, "utf8");
  // const plans = JSON.parse(fileContent);

  before(async function () {
    env = await setupTestEnvironment(["llm", "executor"]);
    ({
      executor,
      executorAddress,
      owner,
      llmContract,
      counterAContract,
      counterAContractAddress,
      counterBContract,
      counterBContractAddress,
      mathContractAddress,
      ammContract1,
      ammContract1Address,
      ammContract2,
      ammContract2Address,
      ammContract3,
      ammContract3Address,
      usdcContract,
      usdcContractAddress,
      jiriContract,
      jiriContractAddress,
    } = env);
  });

  it("should test int addition", async function () {
    const planPath = path.resolve(
      yamlPath,
      "binary_operators",
      "plus",
      "int_add.yaml",
    );

    const raw = fs.readFileSync(planPath, "utf8");

    // parse the YAML into a JS object
    const data = yaml.load(raw) as {
      title: string;
      description: string;
      prompt: string;
      python: string;
      json: string;
    };

    const plan = JSON.parse(data.json);

    // Read the JSON file containing the plans
    const simplest_math_plan = JSON.stringify({
      plan: JSON.stringify(plan.script),
      lookupTable: JSON.stringify({}),
    });

    let tx = await executor.evalPlan(simplest_math_plan);
    await tx.wait();

    await expect(tx)
      .to.emit(llmContract, "QuestionAnswer")
      .withArgs(
        (question) => true,
        (answer) => answer == plan.expected[0],
      );
  });
});
