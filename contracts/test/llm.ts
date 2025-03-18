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
  let counterAContract: Contract;
  let counterBContract: Contract;
  let jiri: Contract;
  let mathContract: Contract;
  let eventContract: Contract;
  let ammContract1: Contract;
  let ammContract2: Contract;
  let ammContract3: Contract;
  let usdc: Contract;
  const counterAAddress = "0x17aB05351fC94a1a67Bf3f56DdbB941aE6c63E25";
  const counterBAddress = "0x5aa01B3b5877255cE50cc55e8986a7a5fe29C70e";
  const jiriAddress = "0x5DB9A7629912EBF95876228C24A848de0bfB43A9";
  const mathAddress = "0x4Ac1d98D9cEF99EC6546dEd4Bd550b0b287aaD6D";
  const eventAddress = "0xA4cD3b0Eb6E5Ab5d8CE4065BcCD70040ADAB1F00";
  const usdcAddress = "0xa4DfF80B4a1D748BF28BC4A271eD834689Ea3407";
  const amm1Address = "0x95CA0a568236fC7413Cd2b794A7da24422c2BBb6";
  const amm2Address = "0x789a5FDac2b37FCD290fb2924382297A6AE65860";
  const amm3Address = "0xE3573540ab8A1C4c754Fd958Dc1db39BBE81b208";
  const pythonPrimitiveAddress = "0xe336d36FacA76840407e6836d26119E1EcE0A2b4";

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

  function pause(duration: number = 1000): Promise<void> {
    return new Promise((resolve) => setTimeout(resolve, duration));
  }

  async function setupAmmLiquidity(
    ammContract: any,
    ammAddress: string,
    usdcAmount: string,
    jiriAmount: string,
  ) {
    console.log(
      `Approving USDC (${usdcAmount}) and JIRI (${jiriAmount}) for AMM: ${ammAddress}`,
    );

    await usdc.approve(ammAddress, ethers.parseEther(usdcAmount));
    await jiri.approve(ammAddress, ethers.parseEther(jiriAmount));

    await pause(2000);
    console.log(`Adding Liquidity: USDC=${usdcAmount}, JIRI=${jiriAmount}`);

    await ammContract.addLiquidity(
      usdcAddress,
      ethers.parseEther(usdcAmount),
      jiriAddress,
      ethers.parseEther(jiriAmount),
    );

    await pause();
    console.log(`Activating AMM at ${ammAddress}`);
    await ammContract.activate();

    console.log(`AMM ${ammAddress} setup complete.`);
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

    const counterACode = await ethers.provider.getCode(counterAAddress);
    // if (true) {
    if (counterACode == "0x") {
      const Counter = await ethers.getContractFactory(
        "CounterPrimitive",
        owner,
      );
      const ERC20 = await ethers.getContractFactory("ERC20Primitive", owner);
      const Math = await ethers.getContractFactory("MathPrimitive", owner);
      const Event = await ethers.getContractFactory("EventPrimitive", owner);
      const Amm = await ethers.getContractFactory("AmmPrimitive", owner);
      const PythonPrimitive = await ethers.getContractFactory(
        "PythonPrimitive",
        owner,
      );

      counterAContract = (await Counter.deploy()) as unknown as Contract;
      await counterAContract.waitForDeployment();
      counterBContract = (await Counter.deploy()) as unknown as Contract;
      await counterBContract.waitForDeployment();
      jiri = (await ERC20.deploy()) as unknown as Contract;
      await jiri.waitForDeployment();
      mathContract = (await Math.deploy()) as unknown as Contract;
      await mathContract.waitForDeployment();
      eventContract = (await Event.deploy()) as unknown as Contract;
      await eventContract.waitForDeployment();
      usdc = (await ERC20.deploy()) as unknown as Contract;
      await usdc.waitForDeployment();
      const pythonPrimitive = await PythonPrimitive.deploy();
      await pythonPrimitive.waitForDeployment();

      // // Get balances of user1
      // const jiriBalance = await jiri.balanceOf(user1Address);
      // const usdcBalance = await usdc.balanceOf(user1Address);

      // // Calculate 1/10th of their balances
      // const jiriAmount = jiriBalance / 10n;
      // const usdcAmount = usdcBalance / 10n;

      // console.log(
      //   `Transferring ${jiriAmount} JIRI and ${usdcAmount} USDC from User1 to Admin.`,
      // );

      // // Transfer to ADMIN_ADDRESS
      // const txJiri = await jiri
      //   .connect(user1)
      //   .transfer(ADMIN_ADDRESS, jiriAmount);
      // await txJiri.wait();

      // const txUsdc = await usdc
      //   .connect(user1)
      //   .transfer(ADMIN_ADDRESS, usdcAmount);
      // await txUsdc.wait();

      // console.log(`Successfully transferred JIRI and USDC to Admin.`);

      const counterAAddressChain = await counterAContract.getAddress();
      const counterBAddressChain = await counterBContract.getAddress();
      const jiriAddressChain = await jiri.getAddress();
      const usdcAddressChain = await usdc.getAddress();
      const mathAddressChain = await mathContract.getAddress();
      const eventAddressChain = await eventContract.getAddress();
      const pythonPrimitiveChain = await pythonPrimitive.getAddress();

      ammContract1 = (await Amm.deploy(
        usdcAddressChain,
        jiriAddressChain,
      )) as unknown as Contract;
      await ammContract1.waitForDeployment();
      ammContract2 = (await Amm.deploy(
        usdcAddressChain,
        jiriAddressChain,
      )) as unknown as Contract;
      await ammContract2.waitForDeployment();
      ammContract3 = (await Amm.deploy(
        usdcAddressChain,
        jiriAddressChain,
      )) as unknown as Contract;
      await ammContract3.waitForDeployment();

      const ammContract1Chain = await ammContract1.getAddress();
      const ammContract2Chain = await ammContract2.getAddress();
      const ammContract3Chain = await ammContract3.getAddress();

      console.log("counterAAddress: ", counterAAddressChain);
      console.log("counterBAddress: ", counterBAddressChain);
      console.log("jiriAddressChain: ", jiriAddressChain);
      console.log("usdcAddressChain: ", usdcAddressChain);
      console.log("mathAddressChain: ", mathAddressChain);
      console.log("eventAddressChain: ", eventAddressChain);
      console.log("pythonPrimitiveChain: ", pythonPrimitiveChain);
      console.log("ammContract1Chain: ", ammContract1Chain);
      console.log("ammContract2Chain: ", ammContract2Chain);
      console.log("ammContract3Chain: ", ammContract3Chain);

      // await usdc.approve(ammContract1Chain, ethers.parseEther("100"));
      // await jiri.approve(ammContract1Chain, ethers.parseEther("10"));
      // await ammContract1.addLiquidity(
      //   usdcAddressChain,
      //   ethers.parseEther("100"),
      //   jiriAddressChain,
      //   ethers.parseEther("10"),
      // );
      // await ammContract1.activate();

      // // let testResult = await ammContract1.price(jiriAddressChain);

      // await usdc.approve(ammContract2Chain, ethers.parseEther("50"));
      // await jiri.approve(ammContract2Chain, ethers.parseEther("50"));
      // await ammContract2.addLiquidity(
      //   usdcAddressChain,
      //   ethers.parseEther("50"),
      //   jiriAddressChain,
      //   ethers.parseEther("50"),
      // );
      // await ammContract2.activate();

      // await usdc.approve(ammContract3Chain, ethers.parseEther("10"));
      // await jiri.approve(ammContract3Chain, ethers.parseEther("100"));
      // await ammContract3.addLiquidity(
      //   usdcAddressChain,
      //   ethers.parseEther("10"),
      //   jiriAddressChain,
      //   ethers.parseEther("100"),
      // );
      // await ammContract3.activate();
    } else {
      counterAContract = (await ethers.getContractAt(
        "CounterPrimitive",
        counterAAddress,
        owner,
      )) as unknown as Contract;
      counterBContract = (await ethers.getContractAt(
        "CounterPrimitive",
        counterBAddress,
        owner,
      )) as unknown as Contract;
      jiri = (await ethers.getContractAt(
        "ERC20Primitive",
        jiriAddress,
        owner,
      )) as unknown as Contract;
      usdc = (await ethers.getContractAt(
        "ERC20Primitive",
        usdcAddress,
        owner,
      )) as unknown as Contract;
      mathContract = (await ethers.getContractAt(
        "CounterPrimitive",
        mathAddress,
        owner,
      )) as unknown as Contract;
      eventContract = (await ethers.getContractAt(
        "EventPrimitive",
        eventAddress,
        owner,
      )) as unknown as Contract;
      ammContract1 = (await ethers.getContractAt(
        "AmmPrimitive",
        amm1Address,
        owner,
      )) as unknown as Contract;
      ammContract2 = (await ethers.getContractAt(
        "AmmPrimitive",
        amm2Address,
        owner,
      )) as unknown as Contract;
      ammContract3 = (await ethers.getContractAt(
        "AmmPrimitive",
        amm3Address,
        owner,
      )) as unknown as Contract;
    }
  });

  it("Prompt: Transfer 5 #USDC to @user1", async function () {
    const inputPrompt = `transfer 5 #USDC to @user1`;
    // const inputPrompt = `Please transfer half my #USDC to @j`;
    let promptIdRead: string;
    const user1Address = "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266";

    const adminBalanceStart = await jiri.balanceOf(ADMIN_ADDRESS);
    const userBalanceStart = await jiri.balanceOf(user1Address);

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
          USDC: jiriAddress,
          user1: user1Address,
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

    const adminBalanceEnd = await jiri.balanceOf(ADMIN_ADDRESS);
    const userBalanceEnd = await jiri.balanceOf(user1Address);

    const transferedAmount = ethers.parseUnits("5", 18);

    expect(adminBalanceEnd).to.equal(adminBalanceStart - transferedAmount);
    expect(userBalanceEnd).to.equal(userBalanceStart + transferedAmount);
  });

  it("Prompt: If I have more than 10 #USDC, transfer 5 #USDC to @alice", async function () {
    const inputPrompt = `If I have more than 10 #USDC, transfer 5 #USDC to @alice`;
    let promptIdRead: string;
    const user1Address = "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266";

    const adminBalanceStart = await jiri.balanceOf(ADMIN_ADDRESS);
    const userBalanceStart = await jiri.balanceOf(user1Address);

    let tx = await testContract.evaluatePrompt(
      JSON.stringify({
        prompt: inputPrompt,
        lookupTable: JSON.stringify({
          USDC: jiriAddress,
          alice: user1Address,
          signer: ADMIN_ADDRESS,
          calculator: mathAddress,
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

      const adminBalanceEnd = await jiri.balanceOf(ADMIN_ADDRESS);
      const userBalanceEnd = await jiri.balanceOf(user1Address);

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

      const adminBalanceEnd = await jiri.balanceOf(ADMIN_ADDRESS);
      const userBalanceEnd = await jiri.balanceOf(user1Address);

      expect(adminBalanceEnd).to.equal(adminBalanceStart);
      expect(userBalanceEnd).to.equal(userBalanceStart);
    }
  });

  it("Prompt: How much #USDC do I have?", async function () {
    const inputPrompt = `How much #USDC do I have?`;
    let promptIdRead: string;

    const adminBalance = await jiri.balanceOf(ADMIN_ADDRESS);

    let tx = await testContract.evaluatePrompt(
      JSON.stringify({
        prompt: inputPrompt,
        lookupTable: JSON.stringify({
          USDC: jiriAddress,
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
        (question) => question == inputPrompt,
        (answer) => answer == String(adminBalance),
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

  it("should test evaluatePlan with erc20 and math", async function () {
    const withMathAndErc20Plan = JSON.stringify({
      plan: JSON.stringify(plans["withMathAndErc20"]),
      lookupTable: JSON.stringify({
        user1: "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266",
      }),
    });

    let adminBalanceStart = await jiri.balanceOf(ADMIN_ADDRESS);
    let userBalanceStart = await jiri.balanceOf(
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

    let adminBalanceEnd = await jiri.balanceOf(ADMIN_ADDRESS);
    let userBalanceEnd = await jiri.balanceOf(
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

    adminBalanceEnd = await jiri.balanceOf(ADMIN_ADDRESS);
    userBalanceEnd = await jiri.balanceOf(
      "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266",
    );

    expect(adminBalanceEnd).to.equal(adminBalanceStart / 2n);
    expect(userBalanceEnd).to.equal(userBalanceStart + adminBalanceStart / 2n);
  });

  it("should test evaluatePlan with assign system primitive", async function () {
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
    const adminBalance = await jiri.balanceOf(ADMIN_ADDRESS);

    const withAnswerUserQuestionPlan = JSON.stringify({
      plan: JSON.stringify(plans["withAnswerUserQuestion"]),
      lookupTable: JSON.stringify({
        signer: ADMIN_ADDRESS,
        USDC: jiriAddress,
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
    if (!isActive) {
      await setupAmmLiquidity(ammContract1, amm1Address, "20000", "10000");
    }
    isActive = await ammContract2.isActive();
    if (!isActive) {
      await setupAmmLiquidity(ammContract2, amm2Address, "15000", "15000");
    }
    isActive = await ammContract3.isActive();
    if (!isActive) {
      await setupAmmLiquidity(ammContract3, amm3Address, "10000", "20000");
    }

    const prices = [
      await ammContract1.price(jiriAddress),
      await ammContract2.price(jiriAddress),
      await ammContract3.price(jiriAddress),
    ];

    const lowestPrice = prices.reduce(
      (min, p) => (p < min ? p : min),
      prices[0],
    );
    const highestPrice = prices.reduce(
      (max, p) => (p > max ? p : max),
      prices[0],
    );

    console.log(`Lowest Price: ${lowestPrice}`);
    console.log(`Highest Price: ${highestPrice}`);

    const startJiriBalance = await jiri.balanceOf(ADMIN_ADDRESS);
    const startUsdcBalance = await jiri.balanceOf(ADMIN_ADDRESS);

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
        JIRI: jiriAddress,
        USDC: usdcAddress,
        AMM_1: amm1Address,
        AMM_2: amm1Address,
        AMM_3: amm1Address,
        PythonPrimitive: pythonPrimitiveAddress,
        calculator: mathAddress,
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

    // const endJiriBalance = await jiri.balanceOf(ADMIN_ADDRESS);
    // const endUsdcBalance = await jiri.balanceOf(ADMIN_ADDRESS);
    // expect(endJiriBalance).to.equal(endJiriBalanceExpected);
    // expect(endUsdcBalance).to.equal(endUsdcBalanceExpected);

    // resultTx = await continueEvaluationAndCall(
    //   testContract,
    //   owner,
    //   promptIdRead,
    // );
  });

  it.only("Prompt: Arbitrage: Please check the price of #JIRI in #USDC on 3 exchanges: #AMM_1, #AMM_2, #AMM_3. On the most expensive exchange, sell half of my #JIRI for #USDC. Then on the least expensive exchange, buy that much #JIRI back. make sure to approve the swap amount before executing the swap action.", async function () {
    const inputPrompt =
      "Arbitrage: Please check the price of #JIRI in #USDC on 3 exchanges: #AMM_1, #AMM_2, #AMM_3. On the most expensive exchange, sell half of my #JIRI for #USDC. Then on the least expensive exchange, buy that much #JIRI back. make sure to approve the swap amount before executing the swap action.";
    let promptIdRead: string;
    const user1Address = "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266";

    const adminBalanceStart = await jiri.balanceOf(ADMIN_ADDRESS);
    const userBalanceStart = await jiri.balanceOf(user1Address);

    let tx = await testContract.evaluatePrompt(
      JSON.stringify({
        prompt: inputPrompt,
        lookupTable: JSON.stringify({
          JIRI: jiriAddress,
          USDC: usdcAddress,
          AMM_1: amm1Address,
          AMM_2: amm1Address,
          AMM_3: amm1Address,
          PythonPrimitive: pythonPrimitiveAddress,
          calculator: mathAddress,
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

    let isActive = await ammContract1.isActive();
    if (!isActive) {
      await setupAmmLiquidity(ammContract1, amm1Address, "20000", "10000");
    }
    isActive = await ammContract2.isActive();
    if (!isActive) {
      await setupAmmLiquidity(ammContract2, amm2Address, "15000", "15000");
    }
    isActive = await ammContract3.isActive();
    if (!isActive) {
      await setupAmmLiquidity(ammContract3, amm3Address, "10000", "20000");
    }

    const prices = [
      await ammContract1.price(jiriAddress),
      await ammContract2.price(jiriAddress),
      await ammContract3.price(jiriAddress),
    ];

    const lowestPrice = prices.reduce(
      (min, p) => (p < min ? p : min),
      prices[0],
    );
    const highestPrice = prices.reduce(
      (max, p) => (p > max ? p : max),
      prices[0],
    );

    console.log(`Lowest Price: ${lowestPrice}`);
    console.log(`Highest Price: ${highestPrice}`);

    const startJiriBalance = await jiri.balanceOf(ADMIN_ADDRESS);
    const startUsdcBalance = await jiri.balanceOf(ADMIN_ADDRESS);

    const sellJiriAmount = startJiriBalance / 2n;
    const receivedUsdc = sellJiriAmount * highestPrice;

    // Buy back JIRI at the lowest price
    const boughtJiri = receivedUsdc / lowestPrice;

    // Calculate final balances
    const endJiriBalanceExpected =
      startJiriBalance - sellJiriAmount + boughtJiri;
    const endUsdcBalanceExpected = startUsdcBalance; // Assuming no fees

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

    // const endJiriBalance = await jiri.balanceOf(ADMIN_ADDRESS);
    // const endUsdcBalance = await jiri.balanceOf(ADMIN_ADDRESS);
    // expect(endJiriBalance).to.equal(endJiriBalanceExpected);
    // expect(endUsdcBalance).to.equal(endUsdcBalanceExpected);

    // resultTx = await continueEvaluationAndCall(
    //   testContract,
    //   owner,
    //   promptIdRead,
    // );
  });
});
