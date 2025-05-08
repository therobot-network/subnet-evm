import { ethers } from "hardhat";
import fs from "fs";
import path from "path";
import {
  BaseContract,
  Contract,
  Signer,
  Interface,
  LogDescription,
} from "ethers";
import { generateFunctionCallData } from "./utils";

export const ADMIN_ADDRESS = "0x8db97C7cEcE249c2b98bDC0226Cc4C2A57BF52FC";
export const LLM_ADDRESS = "0x0300000000000000000000000000000000000000";

export interface TestEnv {
  executor: Contract;
  executorAddress: string;
  owner: Signer;
  llmContract: Contract;
  // testContract: Contract;
  counterAContract: Contract;
  counterAContractAddress: string;
  counterBContract: Contract;
  counterBContractAddress: string;
  mathContractAddress: string;
  ammContract1: Contract;
  ammContract1Address: string;
  ammContract2: Contract;
  ammContract2Address: string;
  ammContract3: Contract;
  ammContract3Address: string;
  usdcContract: Contract;
  usdcContractAddress: string;
  jiriContract: Contract;
  jiriContractAddress: string;
}

/**
 * Deploys or attaches all precompile + primitives + clones via Executor
 * and returns an object with all of your test‑level instances & addresses.
 */
export async function setupTestEnvironment(): Promise<TestEnv> {
  const [owner] = await ethers.getSigners();

  // 1) Attach to LLM precompile (must already exist on‑chain)
  const llmContract = await ethers.getContractAt("ILLM", LLM_ADDRESS, owner);
  const code = await ethers.provider.getCode(LLM_ADDRESS);
  if (code === "0x") throw new Error("LLM precompile not deployed");

  // // 2) Deploy ExampleLLM test contract
  // const ExampleLLM = await ethers.getContractFactory("ExampleLLMPrecompile", {
  //   signer: owner,
  // });
  // const testContract = await ExampleLLM.deploy();
  // await testContract.waitForDeployment();

  // 3) Deploy or attach Executor
  const EXEC_ADDR = "0x52C84043CD9c865236f11d9Fc9F56aa003c1f922";
  let executorAddress = EXEC_ADDR;
  const existingCode = await ethers.provider.getCode(EXEC_ADDR);
  if (existingCode === "0x") {
    const ExecutorFactory = await ethers.getContractFactory("Executor", {
      signer: owner,
    });
    const deployed = await ExecutorFactory.deploy(LLM_ADDRESS);
    await deployed.waitForDeployment();
    executorAddress = await deployed.getAddress();
    console.log(
      `Deployed Executor at ${executorAddress} (LLM: ${LLM_ADDRESS})`,
    );
  }
  // get executor contract instance
  const executor = await ethers.getContractAt(
    "Executor",
    executorAddress,
    owner,
  );

  // 0) Deploy and link UserDecimalFormatting library
  const UDFFactory = await ethers.getContractFactory("UserDecimalFormatting", {
    signer: owner,
  });
  const udf = await UDFFactory.deploy();
  await udf.waitForDeployment();
  const udfAddress = await udf.getAddress();

  // 4) Deploy core primitives (no library linking here, excluding PythonPrimitive)
  const [CounterFactory, MathFactory, SystemFactory] = await Promise.all([
    ethers.getContractFactory("CounterPrimitive"),
    ethers.getContractFactory("MathPrimitive"),
    ethers.getContractFactory("SystemPrimitive"),
  ]);

  try {
    const counterImpl = await CounterFactory.deploy(
      LLM_ADDRESS,
      "",
      executorAddress,
    );
    await counterImpl.waitForDeployment();
  } catch (err) {
    console.warn("Did not deploy CounterPrimitive");
  }

  try {
    const mathImpl = await MathFactory.deploy(LLM_ADDRESS, "", executorAddress);
    await mathImpl.waitForDeployment();
  } catch (err) {
    console.warn("Did not deploy MathPrimitive");
  }

  try {
    const systemImpl = await SystemFactory.deploy(
      LLM_ADDRESS,
      "",
      executorAddress,
    );
    await systemImpl.waitForDeployment();
  } catch (err) {
    console.warn("Did not deploy SystemPrimitive deployment");
  }

  // 5) Prepare factories for ERC20, AMM and Python with library linkage
  const [ERC20Factory, AmmFactory, PythonFactory] = await Promise.all([
    ethers.getContractFactory("ERC20Primitive", {
      signer: owner,
      libraries: { UserDecimalFormatting: udfAddress },
    }),
    ethers.getContractFactory("AmmPrimitive", {
      signer: owner,
      libraries: { UserDecimalFormatting: udfAddress },
    }),
    ethers.getContractFactory("PythonPrimitive", {
      signer: owner,
      libraries: { UserDecimalFormatting: udfAddress },
    }),
  ]);

  // Deploy ERC20, AMM and Python implementations (linked)
  try {
    const ercImpl = await ERC20Factory.deploy(LLM_ADDRESS, "", executorAddress);
    await ercImpl.waitForDeployment();
  } catch {
    console.warn("Did not deploy ERC20Primitive");
  }

  try {
    const ammImpl = await AmmFactory.deploy(LLM_ADDRESS, "", executorAddress);
    await ammImpl.waitForDeployment();
  } catch {
    console.warn("Did not deploy AmmPrimitive");
  }

  try {
    const pyImpl = await PythonFactory.deploy(LLM_ADDRESS, "");
    await pyImpl.waitForDeployment();
  } catch {
    console.warn("Did not deploy PythonPrimitive");
  }

  // Helper to deploy via Executor
  async function deployClone(
    name: string,
    label: string,
    initArgs: any[] = [],
  ): Promise<string> {
    const sigTypes = initArgs.map((a) =>
      typeof a === "string" && a.startsWith("0x")
        ? "address"
        : typeof a === "string"
          ? "string"
          : "uint256",
    );
    const initData = generateFunctionCallData("configure", sigTypes, initArgs);
    const ownerAddress = await owner.getAddress();
    const tx = await executor.deployRobotContract(
      name,
      label,
      ownerAddress,
      "",
      initData,
    );
    // const rcpt = await tx.wait();

    // const iface = executor.interface as unknown as Interface;
    // const event = iface.getEvent("RobotContractDeployed");
    // if (!event) {
    //   throw new Error(`Event ${"RobotContractDeployed"} not found in the ABI`);
    // }

    // // Filter logs for the specific event topic
    // const logs = rcpt.logs.filter((log) => {
    //   const parsedLog = iface.parseLog(log);
    //   return parsedLog?.name === "RobotContractDeployed";
    // });

    // return logs[0].args[1] as string;

    const rcpt = await tx.wait();
    const iface = executor.interface as unknown as Interface;

    let cloneAddr: string | undefined;

    for (const rawLog of rcpt.logs) {
      let parsed: LogDescription;
      try {
        parsed = iface.parseLog(rawLog);
      } catch {
        // this log isn't our event
        continue;
      }
      if (parsed.name === "RobotContractDeployed") {
        // grab the clone address (2nd indexed param)
        cloneAddr = parsed.args.contractAddress as string;
        break;
      }
    }

    if (!cloneAddr) {
      throw new Error("RobotContractDeployed event not found");
    }

    return cloneAddr;
  }

  // 5) Clone out each primitive
  const counterAContractAddress = await deployClone("counter", "xCounter");
  const counterBContractAddress = await deployClone("counter", "xCounter");
  const mathContractAddress = await deployClone("math", "calculator");
  const usdcContractAddress = await deployClone("erc20", "USDC", ["100000"]);
  const jiriContractAddress = await deployClone("erc20", "JIRI", ["100000"]);
  const ammContract1Address = await deployClone("amm", "AMM USDC-JIRI", [
    usdcContractAddress,
    jiriContractAddress,
    "0",
  ]);
  const ammContract2Address = await deployClone("amm", "AMM USDC-JIRI", [
    usdcContractAddress,
    jiriContractAddress,
    "0",
  ]);
  const ammContract3Address = await deployClone("amm", "AMM USDC-JIRI", [
    usdcContractAddress,
    jiriContractAddress,
    "0",
  ]);

  // 6) Attach to those clones
  const [
    counterAContract,
    counterBContract,
    ammContract1,
    ammContract2,
    ammContract3,
    usdcContract,
    jiriContract,
  ] = await Promise.all([
    ethers.getContractAt("CounterPrimitive", counterAContractAddress, owner),
    ethers.getContractAt("CounterPrimitive", counterBContractAddress, owner),
    ethers.getContractAt("AmmPrimitive", ammContract1Address, owner),
    ethers.getContractAt("AmmPrimitive", ammContract2Address, owner),
    ethers.getContractAt("AmmPrimitive", ammContract3Address, owner),
    ethers.getContractAt("ERC20Primitive", usdcContractAddress, owner),
    ethers.getContractAt("ERC20Primitive", jiriContractAddress, owner),
  ]);

  return {
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
  };
}
