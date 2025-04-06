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
const boolenTrueHash =
  "0x0000000000000000000000000000000000000000000000000000000000000001";

describe("LLM Precompiled Contract", function () {
  let owner: Signer;
  // let user1: Signer;
  let llmContract: Contract;
  let testContract: Contract;
  let counterAContract: Contract;
  let counterAContractAddress: String;
  let counterBContract: Contract;
  let counterBContractAddress: String;
  let mathContract: Contract;
  let mathContractAddress: String;
  let ammContract1: Contract;
  let ammContract1Address: string;
  let ammContract2: Contract;
  let ammContract2Address: string;
  let ammContract3: Contract;
  let ammContract3Address: string;
  let usdcContract: Contract;
  let usdcContractAddress: String;
  let jiriContract: Contract;
  let jiriContractAddress: String;

  const erc20PrimitiveAddress = "0x5aa01B3b5877255cE50cc55e8986a7a5fe29C70e";
  const ammPrimitiveAddress = "0x5DB9A7629912EBF95876228C24A848de0bfB43A9";
  const counterPrimitiveAddress = "0x4Ac1d98D9cEF99EC6546dEd4Bd550b0b287aaD6D";
  const mathPrimitiveAddress = "0xA4cD3b0Eb6E5Ab5d8CE4065BcCD70040ADAB1F00";
  const pythonPrimitiveAddress = "0xa4DfF80B4a1D748BF28BC4A271eD834689Ea3407";

  // Read the JSON file containing the plans
  const planPath = path.resolve(__dirname, "llm_test_input_plans.json");
  const fileContent = fs.readFileSync(planPath, "utf8");
  const plans = JSON.parse(fileContent);

  const abiCoder = new ethers.AbiCoder();

  function generateFunctionCallData(methodName, argTypes = [], args = []) {
    // Function signature and arguments
    const functionSignature = `${methodName}(${argTypes.join(",")})`;

    // Compute the function selector
    const functionSelector = ethers.id(functionSignature).substring(0, 10);
    if (!args.length) return functionSelector;

    // Encode the data
    const encodedData = abiCoder.encode(
      argTypes, // Parameter types
      args, // Arguments
    );

    // Combine function selector and encoded arguments
    const data = functionSelector + encodedData.substring(2); // Remove the "0x" from encodedData
    return data;
  }

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

  function pause(duration: number = 1000): Promise<void> {
    return new Promise((resolve) => setTimeout(resolve, duration));
  }

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
      ethers.parseEther(usdcAmount),
      jiriContractAddress,
      ethers.parseEther(jiriAmount),
    );
    await tx.wait();

    // console.log(`Activating AMM at ${ammAddress}`);
    await ammContract.activate();

    // console.log(`AMM ${ammAddress} setup complete.`);
  }

  before(async function () {
    owner = await ethers.getSigner(ADMIN_ADDRESS);

    // const signers = await ethers.getSigners();
    // user1 = signers[1];
    // const user1Address = await user1.getAddress();

    // // Get the current balance of the admin
    // const adminBalance = await ethers.provider.getBalance(ADMIN_ADDRESS);
    // const halfBalance = adminBalance / 2n; // Use BigInt division

    // console.log(`Admin balance: ${ethers.formatEther(adminBalance)} ETH`);
    // console.log(`Sending ${ethers.formatEther(halfBalance)} ETH to User1...`);

    // // Send transaction
    // const tx = await owner.sendTransaction({
    //   to: user1Address,
    //   value: halfBalance, // Send half of admin's balance
    // });

    // await tx.wait(); // Wait for the transaction to be mined

    // console.log(
    //   `Successfully transferred ${ethers.formatEther(halfBalance)} ETH to User1.`,
    // );

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

    // May not need executor
    const Executor = await ethers.getContractFactory("Executor");
    const executor = await Executor.deploy(LLM_ADDRESS);
    await executor.waitForDeployment();
    const executorAddr = await executor.getAddress();

    try {
      const ERC20Primitive = await ethers.getContractFactory("ERC20Primitive");
      const erc20Primitive = await ERC20Primitive.deploy(
        LLM_ADDRESS,
        "",
        executorAddr,
      );
      await erc20Primitive.waitForDeployment();
      const erc20PrimitiveAddr = await erc20Primitive.getAddress();
      console.log("ERC20Primitive deployed at:", erc20PrimitiveAddr);
    } catch (error) {
      console.log("Did not deploy ERC20Primitive");
    }
    try {
      const AmmPrimitive = await ethers.getContractFactory("AmmPrimitive");
      const ammPrimitive = await AmmPrimitive.deploy(
        LLM_ADDRESS,
        "",
        executorAddr,
      );
      await ammPrimitive.waitForDeployment();
      const ammPrimitiveAddr = await ammPrimitive.getAddress();
      console.log("AmmPrimitive deployed at:", ammPrimitiveAddr);
    } catch (error) {
      console.log("Did not deploy AmmPrimitive");
    }
    try {
      const CounterPrimitive =
        await ethers.getContractFactory("CounterPrimitive");
      const counterPrimitive = await CounterPrimitive.deploy(
        LLM_ADDRESS,
        "",
        executorAddr,
      );
      await counterPrimitive.waitForDeployment();
      const counterPrimitiveAddr = await counterPrimitive.getAddress();
      console.log("CounterPrimitive deployed at:", counterPrimitiveAddr);
    } catch (error) {
      console.log("Did not deploy CounterPrimitive");
    }
    try {
      const MathPrimitive = await ethers.getContractFactory("MathPrimitive");
      const mathPrimitive = await MathPrimitive.deploy(
        LLM_ADDRESS,
        "",
        executorAddr,
      );
      await mathPrimitive.waitForDeployment();
      const mathPrimitiveAddr = await mathPrimitive.getAddress();
      console.log("mathPrimitive deployed at:", mathPrimitiveAddr);
    } catch (error) {
      console.log("Did not deploy mathPrimitive");
    }
    try {
      const PythonPrimitive =
        await ethers.getContractFactory("PythonPrimitive");
      const pythonPrimitive = await PythonPrimitive.deploy(LLM_ADDRESS);
      await pythonPrimitive.waitForDeployment();
      const pythonPrimitiveAddr = await pythonPrimitive.getAddress();
      console.log("pythonPrimitive deployed at:", pythonPrimitiveAddr);
    } catch (error) {
      console.log("Did not deploy pythonPrimitive");
    }

    let initData = generateFunctionCallData(
      "initialize",
      ["address", "string", "string"],
      [ADMIN_ADDRESS, "calculator", ""],
    );

    let tx = await executor.deployRobotContract("math", initData);
    let receipt = await tx.wait();
    await expect(receipt)
      .to.emit(executor, "RobotContractDeployed")
      .withArgs(
        "calculator",
        (cloneAddr) => {
          mathContractAddress = cloneAddr;
          return true;
        },
        "math",
      );

    initData = generateFunctionCallData(
      "initialize",
      ["address", "string", "string"],
      [ADMIN_ADDRESS, "xCounter", ""],
    );
    tx = await executor.deployRobotContract("counter", initData);
    receipt = await tx.wait();
    await expect(receipt)
      .to.emit(executor, "RobotContractDeployed")
      .withArgs(
        "xCounter",
        (cloneAddr) => {
          counterAContractAddress = cloneAddr;
          return true;
        },
        "counter",
      );

    counterAContract = await ethers.getContractAt(
      "CounterPrimitive",
      counterAContractAddress,
      owner,
    );

    tx = await executor.deployRobotContract("counter", initData);
    receipt = await tx.wait();
    await expect(receipt)
      .to.emit(executor, "RobotContractDeployed")
      .withArgs(
        "xCounter",
        (cloneAddr) => {
          counterBContractAddress = cloneAddr;
          return true;
        },
        "counter",
      );

    counterBContract = await ethers.getContractAt(
      "CounterPrimitive",
      counterBContractAddress,
      owner,
    );

    initData = generateFunctionCallData(
      "initialize",
      ["address", "string", "uint256", "string", "string"],
      [ADMIN_ADDRESS, "USDC", ethers.parseEther("100000"), "USDC Token", ""],
    );

    tx = await executor.deployRobotContract("erc20", initData);
    receipt = await tx.wait();
    await expect(receipt)
      .to.emit(executor, "RobotContractDeployed")
      .withArgs(
        "USDC Token",
        (cloneAddr) => {
          usdcContractAddress = cloneAddr;
          return true;
        },
        "erc20",
      );

    usdcContract = await ethers.getContractAt(
      "ERC20Primitive",
      usdcContractAddress,
      owner,
    );

    initData = generateFunctionCallData(
      "initialize",
      ["address", "string", "uint256", "string", "string"],
      [ADMIN_ADDRESS, "JIRI", ethers.parseEther("100000"), "JIRI Token", ""],
    );

    tx = await executor.deployRobotContract("erc20", initData);
    receipt = await tx.wait();
    await expect(receipt)
      .to.emit(executor, "RobotContractDeployed")
      .withArgs(
        "JIRI Token",
        (cloneAddr) => {
          jiriContractAddress = cloneAddr;
          return true;
        },
        "erc20",
      );

    jiriContract = await ethers.getContractAt(
      "ERC20Primitive",
      jiriContractAddress,
      owner,
    );

    initData = generateFunctionCallData(
      "initialize",
      ["address", "address", "address", "string", "string"],
      [
        ADMIN_ADDRESS,
        usdcContractAddress,
        jiriContractAddress,
        "AMM USDC-JIRI",
        "",
      ],
    );

    tx = await executor.deployRobotContract("amm", initData);
    receipt = await tx.wait();
    await expect(receipt)
      .to.emit(executor, "RobotContractDeployed")
      .withArgs(
        "AMM USDC-JIRI",
        (cloneAddr) => {
          ammContract1Address = cloneAddr;
          return true;
        },
        "amm",
      );

    ammContract1 = await ethers.getContractAt(
      "AmmPrimitive",
      ammContract1Address,
      owner,
    );

    tx = await executor.deployRobotContract("amm", initData);
    receipt = await tx.wait();
    await expect(receipt)
      .to.emit(executor, "RobotContractDeployed")
      .withArgs(
        "AMM USDC-JIRI",
        (cloneAddr) => {
          ammContract2Address = cloneAddr;
          return true;
        },
        "amm",
      );

    ammContract2 = await ethers.getContractAt(
      "AmmPrimitive",
      ammContract2Address,
      owner,
    );

    tx = await executor.deployRobotContract("amm", initData);
    receipt = await tx.wait();
    await expect(receipt)
      .to.emit(executor, "RobotContractDeployed")
      .withArgs(
        "AMM USDC-JIRI",
        (cloneAddr) => {
          ammContract3Address = cloneAddr;
          return true;
        },
        "amm",
      );

    ammContract3 = await ethers.getContractAt(
      "AmmPrimitive",
      ammContract3Address,
      owner,
    );
  });

  it("Prompt: Transfer 5 #USDC to @user1", async function () {
    const inputPrompt = `transfer 5 #USDC to @user1`;
    let promptIdRead: string;
    const user1Address = "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266";

    const adminBalanceStart = await usdcContract.balanceOf(ADMIN_ADDRESS);
    const userBalanceStart = await usdcContract.balanceOf(user1Address);

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
          USDC: usdcContractAddress,
          user1: user1Address,
          signer: ADMIN_ADDRESS,
          txLogsId: "0b6d013d1a577c1f",
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

    // userFormatToContractFormat
    let resultTx = await owner.call({
      // let result = await owner.sendTransaction({
      to: calleeContractAddress,
      data: methodData,
    });

    tx = await testContract.continueEvaluation(
      promptIdRead,
      // ["0x0000000000000000000000000000000000000000000000000000000000000001"], //  'true'
      [resultTx],
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

    const adminBalanceEnd = await usdcContract.balanceOf(ADMIN_ADDRESS);
    const userBalanceEnd = await usdcContract.balanceOf(user1Address);

    const transferedAmount = ethers.parseUnits("5", 18);

    expect(adminBalanceEnd).to.equal(adminBalanceStart - transferedAmount);
    expect(userBalanceEnd).to.equal(userBalanceStart + transferedAmount);
  });

  it("Prompt: If I have more than 10 #USDC, transfer 5 #USDC to @alice", async function () {
    const inputPrompt = `If I have more than 10 #USDC, transfer 5 #USDC to @alice`;
    let promptIdRead: string;
    const user1Address = "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266";

    const adminBalanceStart = await usdcContract.balanceOf(ADMIN_ADDRESS);
    const userBalanceStart = await usdcContract.balanceOf(user1Address);

    let tx = await testContract.evaluatePrompt(
      JSON.stringify({
        prompt: inputPrompt,
        lookupTable: JSON.stringify({
          USDC: usdcContractAddress,
          alice: user1Address,
          signer: ADMIN_ADDRESS,
          calculator: mathContractAddress,
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

    // Read balance
    // let result = await owner.sendTransaction({
    let resultTx = await owner.call({
      to: calleeContractAddress,
      data: methodData,
    });

    tx = await testContract.continueEvaluation(
      promptIdRead,
      // ["0x0000000000000000000000000000000000000000000000000000000000000001"], //  'true'
      [resultTx],
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

    // userFormatToContractFormat
    resultTx = await owner.call({
      // let result = await owner.sendTransaction({
      to: calleeContractAddress,
      data: methodData,
    });

    tx = await testContract.continueEvaluation(
      promptIdRead,
      // ["0x0000000000000000000000000000000000000000000000000000000000000001"], //  'true'
      [resultTx],
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

    // Check if greater than 10
    resultTx = await owner.call({
      to: calleeContractAddress,
      data: methodData,
    });

    if (adminBalanceStart > 10n) {
      tx = await testContract.continueEvaluation(
        promptIdRead,
        // ["0x0000000000000000000000000000000000000000000000000000000000000001"], //  'true'
        [resultTx],
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

      // userFormatToContractFormat
      resultTx = await owner.call({
        // let result = await owner.sendTransaction({
        to: calleeContractAddress,
        data: methodData,
      });

      tx = await testContract.continueEvaluation(
        promptIdRead,
        // ["0x0000000000000000000000000000000000000000000000000000000000000001"], //  'true'
        [resultTx],
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

      // Send 5 USDC
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

      const adminBalanceEnd = await usdcContract.balanceOf(ADMIN_ADDRESS);
      const userBalanceEnd = await usdcContract.balanceOf(user1Address);

      const transferedAmount = ethers.parseUnits("5", 18);

      expect(adminBalanceEnd).to.equal(adminBalanceStart - transferedAmount);
      expect(userBalanceEnd).to.equal(userBalanceStart + transferedAmount);
    } else {
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

      const adminBalanceEnd = await usdcContract.balanceOf(ADMIN_ADDRESS);
      const userBalanceEnd = await usdcContract.balanceOf(user1Address);

      expect(adminBalanceEnd).to.equal(adminBalanceStart);
      expect(userBalanceEnd).to.equal(userBalanceStart);
    }
  });

  it("Prompt: How much #USDC do I have?", async function () {
    const inputPrompt = `How much #USDC do I have?`;
    let promptIdRead: string;

    const adminBalance = await usdcContract.balanceOf(ADMIN_ADDRESS);

    let tx = await testContract.evaluatePrompt(
      JSON.stringify({
        prompt: inputPrompt,
        lookupTable: JSON.stringify({
          USDC: usdcContractAddress,
          signer: ADMIN_ADDRESS,
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

    // Read balance
    // let result = await owner.sendTransaction({
    let resultTx = await owner.call({
      to: calleeContractAddress,
      data: methodData,
    });

    tx = await testContract.continueEvaluation(
      promptIdRead,
      // ["0x0000000000000000000000000000000000000000000000000000000000000001"], //  'true'
      [resultTx],
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

    // userFormatToContractFormat
    resultTx = await owner.call({
      // let result = await owner.sendTransaction({
      to: calleeContractAddress,
      data: methodData,
    });

    tx = await testContract.continueEvaluation(
      promptIdRead,
      // ["0x0000000000000000000000000000000000000000000000000000000000000001"], //  'true'
      [resultTx],
    );
    await tx.wait();
    await expect(tx)
      .to.emit(testContract, "ContinueEvaluationEvent")
      .withArgs(
        (evaluationDone) => evaluationDone == true,
        (contractMethodParams) => {
          return true;
        },
      )
      .and.to.emit(llmContract, "QuestionAnswer")
      .withArgs(
        (question) => question === inputPrompt,
        (answer) => ethers.parseUnits(answer, 18) === adminBalance,
      );
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
        CounterA: counterAContractAddress,
        CounterB: counterBContractAddress,
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
      lookupTable: JSON.stringify({
        CounterA: counterAContractAddress,
        CounterB: counterBContractAddress,
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

  it("should test evaluatePlan with erc20 and math", async function () {
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

    let adminBalanceEnd = await usdcContract.balanceOf(ADMIN_ADDRESS);
    let userBalanceEnd = await usdcContract.balanceOf(
      "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266",
    );

    expect(adminBalanceEnd).to.equal(adminBalanceStart - 600n);
    expect(userBalanceEnd).to.equal(userBalanceStart + 600n);

    adminBalanceStart = adminBalanceEnd;
    userBalanceStart = userBalanceEnd;

    let resultTx = await continueEvaluationAndCall(
      testContract,
      owner,
      promptIdRead,
    );

    resultTx = await continueEvaluationAndCall(
      testContract,
      owner,
      promptIdRead,
      resultTx,
    );

    await continueEvaluationAndSend(
      testContract,
      owner,
      promptIdRead,
      resultTx,
    );

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

    adminBalanceEnd = await usdcContract.balanceOf(ADMIN_ADDRESS);
    userBalanceEnd = await usdcContract.balanceOf(
      "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266",
    );

    expect(adminBalanceEnd).to.equal(adminBalanceStart / 2n);
    expect(userBalanceEnd).to.equal(userBalanceStart + adminBalanceStart / 2n);
  });

  it("should test evaluatePlan with assign system primitive", async function () {
    const withLookupPlan = JSON.stringify({
      plan: JSON.stringify(plans["withAssign"]),
      lookupTable: JSON.stringify({
        CounterA: counterAContractAddress,
        CounterB: counterBContractAddress,
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

    let resultTx = await continueEvaluationAndCall(
      testContract,
      owner,
      promptIdRead,
    );

    await continueEvaluationAndSend(
      testContract,
      owner,
      promptIdRead,
      resultTx,
    );

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

  it("should test evaluatePlan with assignArray system primitive", async function () {
    const withAssignArrayPlan = JSON.stringify({
      plan: JSON.stringify(plans["withAssignArray"]),
      lookupTable: JSON.stringify({
        CounterA: counterAContractAddress,
        CounterB: counterBContractAddress,
      }),
    });

    let promptIdRead: string;

    let tx = await testContract.evaluatePlan(withAssignArrayPlan);
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

    // Read counter B
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

    // Read counter B Again
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

    const countA = await counterAContract.getCounter();
    const countB = await counterBContract.getCounter();

    // Get Max
    // result = await owner.sendTransaction({
    resultTx = await owner.call({
      to: calleeContractAddress,
      data: methodData,
    });

    tx = await testContract.continueEvaluation(promptIdRead, [resultTx]);
    await tx.wait();
    await expect(tx)
      .to.emit(testContract, "ContinueEvaluationEvent")
      .withArgs(
        (evaluationDone) => evaluationDone == true,
        (contractMethodParams) => true,
      )
      .and.to.emit(llmContract, "QuestionAnswer")
      .withArgs(
        (question) => question == "What's the max value of the counters?",
        (answer) => {
          const max = countA > countB ? countA : countB;
          return answer == max.toString();
        },
      );
  });

  it("should test evaluatePlan with JumpIfNot system primitive", async function () {
    const withJumpIfNotPlan = JSON.stringify({
      plan: JSON.stringify(plans["withJumpIfNot"]),
      lookupTable: JSON.stringify({
        CounterA: counterAContractAddress,
        CounterB: counterBContractAddress,
      }),
    });

    let promptIdRead: string;

    let tx = await testContract.evaluatePlan(withJumpIfNotPlan);
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

    // First JumpIfNot - should jump to 'increase 20'
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

  it("should test evaluatePlan with answerUserQuestion system primitive", async function () {
    const adminBalance = await usdcContract.balanceOf(ADMIN_ADDRESS);

    const withAnswerUserQuestionPlan = JSON.stringify({
      plan: JSON.stringify(plans["withAnswerUserQuestion"]),
      lookupTable: JSON.stringify({
        signer: ADMIN_ADDRESS,
        USDC: usdcContractAddress,
      }),
    });

    let promptIdRead: string;

    let tx = await testContract.evaluatePlan(withAnswerUserQuestionPlan);
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

    // Read balance
    // let result = await owner.sendTransaction({
    let resultTx = await owner.call({
      to: calleeContractAddress,
      data: methodData,
    });

    tx = await testContract.continueEvaluation(
      promptIdRead,
      // ["0x0000000000000000000000000000000000000000000000000000000000000001"], //  'true'
      [resultTx],
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

    // userFormatToContractFormat
    resultTx = await owner.call({
      // let result = await owner.sendTransaction({
      to: calleeContractAddress,
      data: methodData,
    });

    tx = await testContract.continueEvaluation(
      promptIdRead,
      // ["0x0000000000000000000000000000000000000000000000000000000000000001"], //  'true'
      [resultTx],
    );
    await tx.wait();
    await expect(tx)
      .to.emit(testContract, "ContinueEvaluationEvent")
      .withArgs(
        (evaluationDone) => evaluationDone == true,
        (contractMethodParams) => {
          return true;
        },
      )
      .and.to.emit(llmContract, "QuestionAnswer")
      .withArgs(
        (question) => question == "How much USDC do I have?",
        (answer) =>
          ethers.parseUnits(answer, 18).toString() == adminBalance.toString(),
      );
  });

  it("should test evaluatePlan with AMM", async function () {
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

    const prices = [
      await ammContract1.price(jiriContractAddress),
      await ammContract2.price(jiriContractAddress),
      await ammContract3.price(jiriContractAddress),
    ];

    const lowestPrice = prices.reduce(
      (min, p) => (p < min ? p : min),
      prices[0],
    );
    const highestPrice = prices.reduce(
      (max, p) => (p > max ? p : max),
      prices[0],
    );

    // console.log(`Lowest Price: ${lowestPrice}`);
    // console.log(`Highest Price: ${highestPrice}`);

    const startJiriBalance = await jiriContract.balanceOf(ADMIN_ADDRESS);
    const startUsdcBalance = await usdcContract.balanceOf(ADMIN_ADDRESS);

    const sellJiriAmount = startJiriBalance / 2n;
    const receivedUsdc = sellJiriAmount * highestPrice;

    // Buy back JIRI at the lowest price
    const boughtJiri = receivedUsdc / lowestPrice;

    // Calculate final balances
    const endJiriBalanceExpected =
      startJiriBalance - sellJiriAmount + boughtJiri;
    const endUsdcBalanceExpected = startUsdcBalance; // Assuming no fees

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

    let promptIdRead: string;

    let tx = await testContract.evaluatePlan(withAMMPlan);
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

    let resultTx = await owner.call({
      to: calleeContractAddress,
      data: methodData,
    });

    resultTx = await continueEvaluationAndCall(
      testContract,
      owner,
      promptIdRead,
      resultTx,
    );

    resultTx = await continueEvaluationAndCall(
      testContract,
      owner,
      promptIdRead,
      resultTx,
    );

    await continueEvaluationAndSend(
      testContract,
      owner,
      promptIdRead,
      resultTx,
    );

    resultTx = await continueEvaluationAndCall(
      testContract,
      owner,
      promptIdRead,
      resultTx,
    );

    resultTx = await continueEvaluationAndCall(
      testContract,
      owner,
      promptIdRead,
      resultTx,
    );

    resultTx = await continueEvaluationAndCall(
      testContract,
      owner,
      promptIdRead,
      resultTx,
    );

    resultTx = await continueEvaluationAndCall(
      testContract,
      owner,
      promptIdRead,
      resultTx,
    );

    resultTx = await continueEvaluationAndCall(
      testContract,
      owner,
      promptIdRead,
      resultTx,
    );

    resultTx = await continueEvaluationAndCall(
      testContract,
      owner,
      promptIdRead,
      resultTx,
    );

    resultTx = await continueEvaluationAndCall(
      testContract,
      owner,
      promptIdRead,
      resultTx,
    );

    await continueEvaluationAndSend(
      testContract,
      owner,
      promptIdRead,
      resultTx,
    );

    await continueEvaluationAndSend(testContract, owner, promptIdRead);

    resultTx = await continueEvaluationAndCall(
      testContract,
      owner,
      promptIdRead,
    );

    await continueEvaluationAndSend(
      testContract,
      owner,
      promptIdRead,
      resultTx,
    );

    await continueEvaluationAndSend(testContract, owner, promptIdRead);

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
        () => true,
      );

    // const endJiriBalance = await jiriContract.balanceOf(ADMIN_ADDRESS);
    // const endUsdcBalance = await usdcContract.balanceOf(ADMIN_ADDRESS);
    // expect(endJiriBalance).to.equal(endJiriBalanceExpected);
    // expect(endUsdcBalance).to.equal(endUsdcBalanceExpected);

    // resultTx = await continueEvaluationAndCall(
    //   testContract,
    //   owner,
    //   promptIdRead,
    // );
  });

  it("Prompt: Arbitrage: Please check the price of #JIRI in #USDC on 3 exchanges: #AMM_1, #AMM_2, #AMM_3. On the most expensive exchange, sell half of my #JIRI for #USDC. Then on the least expensive exchange, buy that much #JIRI back. make sure to approve the swap amount before executing the swap action.", async function () {
    const inputPrompt =
      "Arbitrage: Please check the price of #JIRI in #USDC on 3 exchanges: #AMM_1, #AMM_2, #AMM_3. On the most expensive exchange, sell half of my #JIRI for #USDC. Then on the least expensive exchange, buy that much #JIRI back. make sure to approve the swap amount before executing the swap action.";
    let promptIdRead: string;
    const user1Address = "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266";

    const adminBalanceStart = await jiriContract.balanceOf(ADMIN_ADDRESS);
    const userBalanceStart = await jiriContract.balanceOf(user1Address);

    let isActive = await ammContract1.isActive();
    if (!isActive) {
      await setupAmmLiquidity(
        ammContract1,
        ammContract1Address,
        "20000",
        "10000",
      );
    }
    isActive = await ammContract2.isActive();
    if (!isActive) {
      await setupAmmLiquidity(
        ammContract2,
        ammContract2Address,
        "15000",
        "15000",
      );
    }
    isActive = await ammContract3.isActive();
    if (!isActive) {
      await setupAmmLiquidity(
        ammContract3,
        ammContract3Address,
        "10000",
        "20000",
      );
    }

    const prices = [
      await ammContract1.price(jiriContractAddress),
      await ammContract2.price(jiriContractAddress),
      await ammContract3.price(jiriContractAddress),
    ];

    const lowestPrice = prices.reduce(
      (min, p) => (p < min ? p : min),
      prices[0],
    );
    const highestPrice = prices.reduce(
      (max, p) => (p > max ? p : max),
      prices[0],
    );

    // console.log(`Lowest Price: ${lowestPrice}`);
    // console.log(`Highest Price: ${highestPrice}`);

    const startJiriBalance = await jiriContract.balanceOf(ADMIN_ADDRESS);
    const startUsdcBalance = await jiriContract.balanceOf(ADMIN_ADDRESS);

    const sellJiriAmount = startJiriBalance / 2n;
    const receivedUsdc = sellJiriAmount * highestPrice;

    // Buy back JIRI at the lowest price
    const boughtJiri = receivedUsdc / lowestPrice;

    // Calculate final balances
    const endJiriBalanceExpected =
      startJiriBalance - sellJiriAmount + boughtJiri;
    const endUsdcBalanceExpected = startUsdcBalance; // Assuming no fees

    let tx = await testContract.evaluatePrompt(
      JSON.stringify({
        prompt: inputPrompt,
        lookupTable: JSON.stringify({
          JIRI: jiriContractAddress,
          USDC: usdcContractAddress,
          AMM_1: ammContract1Address,
          AMM_2: ammContract2Address,
          AMM_3: ammContract3Address,
          calculator: mathContractAddress,
          signer: ADMIN_ADDRESS,
        }),
      }),
      { gasLimit: 5000000, timeout: 60000 },
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

    // Read balance
    // let result = await owner.sendTransaction({
    let resultTx = await owner.call({
      to: calleeContractAddress,
      data: methodData,
    });

    resultTx = await continueEvaluationAndCall(
      testContract,
      owner,
      promptIdRead,
      resultTx,
    );

    resultTx = await continueEvaluationAndCall(
      testContract,
      owner,
      promptIdRead,
      resultTx,
    );

    await continueEvaluationAndSend(
      testContract,
      owner,
      promptIdRead,
      resultTx,
    );

    resultTx = await continueEvaluationAndCall(
      testContract,
      owner,
      promptIdRead,
      resultTx,
    );

    resultTx = await continueEvaluationAndCall(
      testContract,
      owner,
      promptIdRead,
      resultTx,
    );

    resultTx = await continueEvaluationAndCall(
      testContract,
      owner,
      promptIdRead,
      resultTx,
    );

    resultTx = await continueEvaluationAndCall(
      testContract,
      owner,
      promptIdRead,
      resultTx,
    );

    resultTx = await continueEvaluationAndCall(
      testContract,
      owner,
      promptIdRead,
      resultTx,
    );

    resultTx = await continueEvaluationAndCall(
      testContract,
      owner,
      promptIdRead,
      resultTx,
    );

    resultTx = await continueEvaluationAndCall(
      testContract,
      owner,
      promptIdRead,
      resultTx,
    );

    await continueEvaluationAndSend(
      testContract,
      owner,
      promptIdRead,
      resultTx,
    );

    await continueEvaluationAndSend(testContract, owner, promptIdRead);

    resultTx = await continueEvaluationAndCall(
      testContract,
      owner,
      promptIdRead,
    );

    await continueEvaluationAndSend(
      testContract,
      owner,
      promptIdRead,
      resultTx,
    );

    await continueEvaluationAndSend(testContract, owner, promptIdRead);

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
        () => true,
      );

    const endJiriBalance = await jiriContract.balanceOf(ADMIN_ADDRESS);
    const endUsdcBalance = await jiriContract.balanceOf(ADMIN_ADDRESS);
    console.log(
      `Start JIRI Balance: ${ethers.formatEther(startJiriBalance)}`,
      `Start USDC Balance: ${ethers.formatEther(startUsdcBalance)}`,
    );
    console.log(
      `End JIRI Balance: ${ethers.formatEther(endJiriBalance)}`,
      `End USDC Balance: ${ethers.formatEther(endUsdcBalance)}`,
    );
    // expect(endJiriBalance).to.equal(endJiriBalanceExpected);
    // expect(endUsdcBalance).to.equal(endUsdcBalanceExpected);

    // resultTx = await continueEvaluationAndCall(
    //   testContract,
    //   owner,
    //   promptIdRead,
    // );
  });
});
