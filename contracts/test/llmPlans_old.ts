// (c) 2019-2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

import { expect } from "chai";
import { BaseContract, Contract, Signer } from "ethers";
import { ethers } from "hardhat";
import fs from "fs";
import * as path from "path";
// import { test } from "./utils";
import { setupTestEnvironment, TestEnv } from "./helpers/setupFixtures";
// import { setupAmmLiquidity } from "./helpers/utils";

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

  // Read the JSON file containing the plans
  const planPath = path.resolve(__dirname, "llm_test_input_plans_old.json");
  const fileContent = fs.readFileSync(planPath, "utf8");
  const plans = JSON.parse(fileContent);

  async function setupAmmLiquidity(
    ammContract: any,
    ammAddress: string,
    usdcAmount: string,
    jiriAmount: string,
  ) {
    // console.log(
    //   `Approving USDC (${usdcAmount}) and JIRI (${jiriAmount}) for AMM: ${ammAddress}`,
    // );

    let tx = await usdcContract.approve(
      ammAddress,
      ethers.parseEther(usdcAmount),
    );
    await tx.wait();

    tx = await jiriContract.approve(ammAddress, ethers.parseEther(jiriAmount));
    await tx.wait();

    // console.log(`Adding Liquidity: USDC=${usdcAmount}, JIRI=${jiriAmount}`);

    tx = await ammContract.addLiquidity(
      usdcContractAddress,
      usdcAmount,
      jiriContractAddress,
      jiriAmount,
    );
    await tx.wait();

    // console.log(`Activating AMM at ${ammAddress}`);
    await ammContract.activate();

    // console.log(`AMM ${ammAddress} setup complete.`);
  }

  before(async function () {
    env = await setupTestEnvironment();
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

  it("should test evalPlan and continueEvaluation basic", async function () {
    // should fail when plan is not passed
    let isFailed = false;
    await executor.evalPlan(JSON.stringify({ prompt: "" })).catch((err) => {
      isFailed = true;
    });
    expect(isFailed).to.be.true;

    // Read the JSON file containing the plans
    const withLookupPlan = JSON.stringify({
      plan: JSON.stringify(plans["basic"]),
      lookupTable: JSON.stringify({
        CounterA: counterAContractAddress,
        CounterB: counterBContractAddress,
        recipient: "0x000000000000000000000000000000000000dead",
      }),
    });

    const countAStart = await counterAContract.getCounter();
    const countBStart = await counterBContract.getCounter();

    let tx = await executor.evalPlan(withLookupPlan);
    await tx.wait();

    const countAEnd = await counterAContract.getCounter();
    const countBEnd = await counterBContract.getCounter();

    expect(countAEnd).to.equal(countAStart + 10n);
    expect(countBEnd).to.equal(countBStart + 20n);
  });

  it("should test evalPlan and continueEvaluation with lookup", async function () {
    // Read the JSON file containing the plans
    const withLookupPlan = JSON.stringify({
      plan: JSON.stringify(plans["withLookup"]),
      lookupTable: JSON.stringify({
        CounterA: counterAContractAddress,
        CounterB: counterBContractAddress,
      }),
    });

    const countAStart = await counterAContract.getCounter();
    const countBStart = await counterBContract.getCounter();

    let tx = await executor.evalPlan(withLookupPlan);
    await tx.wait();

    const countAEnd = await counterAContract.getCounter();
    const countBEnd = await counterBContract.getCounter();

    expect(countAEnd).to.equal(countAStart + 10n);
    expect(countBEnd).to.equal(countBStart + countAEnd);
  });

  it("should test evalPlan with erc20 and math", async function () {
    const withMathAndErc20Plan = JSON.stringify({
      plan: JSON.stringify(plans["withMathAndErc20"]),
      lookupTable: JSON.stringify({
        USDC: usdcContractAddress,
        signer: ADMIN_ADDRESS,
        user1: "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266",
      }),
    });

    let adminBalanceStart = await usdcContract.balanceOf(ADMIN_ADDRESS);
    let userBalanceStart = await usdcContract.balanceOf(
      "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266",
    );

    let tx = await executor.evalPlan(withMathAndErc20Plan);
    await tx.wait();

    const adminBalanceEnd = await usdcContract.balanceOf(ADMIN_ADDRESS);
    const userBalanceEnd = await usdcContract.balanceOf(
      "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266",
    );

    expect(adminBalanceEnd).to.equal((adminBalanceStart - 600n) / 2n);
    expect(userBalanceEnd).to.equal(
      (userBalanceStart + adminBalanceStart + 600n) / 2n,
    );
  });

  it("should test evalPlan with assign system primitive", async function () {
    const withLookupPlan = JSON.stringify({
      plan: JSON.stringify(plans["withAssign"]),
      lookupTable: JSON.stringify({
        CounterA: counterAContractAddress,
        CounterB: counterBContractAddress,
      }),
    });

    const countAStart = await counterAContract.getCounter();
    const countBStart = await counterBContract.getCounter();

    let tx = await executor.evalPlan(withLookupPlan);
    await tx.wait();

    const countAEnd = await counterAContract.getCounter();
    const countBEnd = await counterBContract.getCounter();

    expect(countAEnd).to.equal(countAStart + 10n);
    expect(countBEnd).to.equal(countBStart + countAEnd);
  });

  it("should test evalPlan with assignArray system primitive", async function () {
    const withAssignArrayPlan = JSON.stringify({
      plan: JSON.stringify(plans["withAssignArray"]),
      lookupTable: JSON.stringify({
        CounterA: counterAContractAddress,
        CounterB: counterBContractAddress,
      }),
    });

    let tx = await executor.evalPlan(withAssignArrayPlan);
    await tx.wait();

    const countA = await counterAContract.getCounter();
    const countB = await counterBContract.getCounter();

    await expect(tx)
      .to.emit(llmContract, "QuestionAnswer")
      .withArgs(
        (question) => question == "What's the max value of the counters?",
        (answer) => {
          const max = countA > countB ? countA : countB;
          return answer == max.toString();
        },
      );
  });

  it("should test evalPlan with JumpIfNot system primitive", async function () {
    const withJumpIfNotPlan = JSON.stringify({
      plan: JSON.stringify(plans["withJumpIfNot"]),
      lookupTable: JSON.stringify({
        CounterA: counterAContractAddress,
        CounterB: counterBContractAddress,
      }),
    });

    let tx = await executor.evalPlan(withJumpIfNotPlan);
    await tx.wait();

    const countAEnd = await counterAContract.getCounter();
    expect(countAEnd).to.equal(20n + 30n);
  });

  it("should test evalPlan with answerUserQuestion system primitive", async function () {
    const adminBalance = await usdcContract.balanceOf(ADMIN_ADDRESS);

    const withAnswerUserQuestionPlan = JSON.stringify({
      plan: JSON.stringify(plans["withAnswerUserQuestion"]),
      lookupTable: JSON.stringify({
        signer: ADMIN_ADDRESS,
        USDC: usdcContractAddress,
      }),
    });

    let tx = await executor.evalPlan(withAnswerUserQuestionPlan);
    await tx.wait();

    await expect(tx)
      .to.emit(llmContract, "QuestionAnswer")
      .withArgs(
        (question) => question == "How much USDC do I have?",
        (answer) =>
          ethers.parseUnits(answer, 18).toString() == adminBalance.toString(),
      );
  });

  it("should test evalPlan with AMM", async function () {
    let isActive = await ammContract1.isActive();
    // const usdcOwned = ethers.formatEther(
    //   await usdcContract.balanceOf(ADMIN_ADDRESS),
    // );
    // const jiriOwned = ethers.formatEther(
    //   await usdcContract.balanceOf(ADMIN_ADDRESS),
    // );
    // console.log(`USDC Owned: ${usdcOwned}`);
    // console.log(`JIRI Owned: ${jiriOwned}`);

    if (!isActive) {
      await setupAmmLiquidity(
        ammContract1,
        ammContract1Address,
        "2000",
        "1000",
      );
    }
    isActive = await ammContract2.isActive();
    if (!isActive) {
      await setupAmmLiquidity(
        ammContract2,
        ammContract2Address,
        "1500",
        "1500",
      );
    }
    isActive = await ammContract3.isActive();
    if (!isActive) {
      await setupAmmLiquidity(
        ammContract3,
        ammContract3Address,
        "1000",
        "2000",
      );
    }

    // const prices = [
    //   await ammContract1.price(jiriContractAddress),
    //   await ammContract2.price(jiriContractAddress),
    //   await ammContract3.price(jiriContractAddress),
    // ];

    // const lowestPrice = prices.reduce(
    //   (min, p) => (p < min ? p : min),
    //   prices[0],
    // );
    // const highestPrice = prices.reduce(
    //   (max, p) => (p > max ? p : max),
    //   prices[0],
    // );

    // console.log(`Lowest Price: ${lowestPrice}`);
    // console.log(`Highest Price: ${highestPrice}`);

    // const startJiriBalance = await jiriContract.balanceOf(ADMIN_ADDRESS);
    // const startUsdcBalance = await usdcContract.balanceOf(ADMIN_ADDRESS);

    // const sellJiriAmount = startJiriBalance / 2n;
    // const receivedUsdc = sellJiriAmount * highestPrice;

    // // Buy back JIRI at the lowest price
    // const boughtJiri = receivedUsdc / lowestPrice;

    // // Calculate final balances
    // const endJiriBalanceExpected =
    //   startJiriBalance - sellJiriAmount + boughtJiri;
    // const endUsdcBalanceExpected = startUsdcBalance; // Assuming no fees

    const withAMMPlan = JSON.stringify({
      plan: JSON.stringify(plans["withAMM"]),
      lookupTable: JSON.stringify({
        JIRI: jiriContractAddress,
        USDC: usdcContractAddress,
        AMM_1: ammContract1Address,
        AMM_2: ammContract2Address,
        AMM_3: ammContract3Address,
        calculator: mathContractAddress,
        signer: ADMIN_ADDRESS,
      }),
    });

    let tx = await executor.evalPlan(withAMMPlan);
    await tx.wait();
  });

  it("should test evalPlan with ERC20 Deploy", async function () {
    const withRobotContractDeployPlan = JSON.stringify({
      plan: JSON.stringify(plans["withRobotContractDeploy"]),
      lookupTable: JSON.stringify({
        sender: ADMIN_ADDRESS,
      }),
    });

    let tx = await executor.evalPlan(withRobotContractDeployPlan);
    await tx.wait();

    const robotContracts = await executor.getDeployedContracts();

    // const tstEntries = robotContracts.filter((c) => c.contractName === "TST");
    const tstEntries = robotContracts.filter((c) => c[0] === "TST");

    expect(tstEntries.length).to.be.greaterThan(0);

    // grab the last one
    const tstContractAddress =
      tstEntries[tstEntries.length - 1].contractAddress;

    await expect(tx)
      .to.emit(llmContract, "QuestionAnswer")
      .withArgs(
        (question) => question == "TST address is:",
        (answer) => tstContractAddress == answer,
      );
  });

  it("should test evalPlan with Amm Deploy", async function () {
    const withAmmDeployPlan = JSON.stringify({
      plan: JSON.stringify(plans["withAmmDeploy"]),
      lookupTable: JSON.stringify({
        sender: ADMIN_ADDRESS,
      }),
    });

    let tx = await executor.evalPlan(withAmmDeployPlan);
    await tx.wait();
  });
});
