import { ethers } from "hardhat";
import { Contract, Signer, Interface, LogDescription } from "ethers";
import { generateFunctionCallData } from "./utils";

export const ADMIN_ADDRESS = "0x8db97C7cEcE249c2b98bDC0226Cc4C2A57BF52FC";
export const LLM_ADDRESS = "0x0300000000000000000000000000000000000000";
const EXEC_ADDR = "0x52C84043CD9c865236f11d9Fc9F56aa003c1f922";

export interface TestEnv {
  executor: Contract;
  executorAddress: string;
  owner: Signer;
  llmContract: Contract;

  // these might be undefined if you didn’t request them
  counterAContract?: Contract;
  counterAContractAddress?: string;

  counterBContract?: Contract;
  counterBContractAddress?: string;

  mathContractAddress?: string;
  mathContract?: Contract;

  usdcContractAddress?: string;
  usdcContract?: Contract;

  jiriContractAddress?: string;
  jiriContract?: Contract;

  ammContract1Address?: string;
  ammContract1?: Contract;

  ammContract2Address?: string;
  ammContract2?: Contract;

  ammContract3Address?: string;
  ammContract3?: Contract;
}

export async function setupTestEnvironment(
  contractsToDeploy: string[],
): Promise<TestEnv> {
  const toDeploy = new Set(contractsToDeploy);
  const [owner] = await ethers.getSigners();

  // 1) always attach to LLM
  const llmContract = await ethers.getContractAt("ILLM", LLM_ADDRESS, owner);
  if ((await ethers.provider.getCode(LLM_ADDRESS)) === "0x") {
    throw new Error("LLM precompile not deployed");
  }

  // 2) deploy or attach Executor
  let executorAddress = EXEC_ADDR;
  if ((await ethers.provider.getCode(EXEC_ADDR)) === "0x") {
    const ExecutorFactory = await ethers.getContractFactory("Executor", {
      signer: owner,
    });
    const deployed = await ExecutorFactory.deploy(LLM_ADDRESS);
    await deployed.waitForDeployment();
    executorAddress = await deployed.getAddress();
  }
  const executor = await ethers.getContractAt(
    "Executor",
    executorAddress,
    owner,
  );

  // 3) link UserDecimalFormatting if needed
  const UDFFactory = await ethers.getContractFactory("UserDecimalFormatting", {
    signer: owner,
  });
  const udf = await UDFFactory.deploy();
  await udf.waitForDeployment();
  const udfAddress = await udf.getAddress();

  // 4) Decide which primitives to deploy
  //    if you have at least one "counter" clone, deploy CounterPrimitive, etc.
  const needCounter =
    contractsToDeploy.includes("counterAContract") ||
    contractsToDeploy.includes("counterBContract");
  const needMath = contractsToDeploy.includes("mathContract");
  const needERC20 =
    contractsToDeploy.includes("usdcContract") ||
    contractsToDeploy.includes("jiriContract");
  const needAMM =
    contractsToDeploy.includes("ammContract1") ||
    contractsToDeploy.includes("ammContract2") ||
    contractsToDeploy.includes("ammContract3");
  const needSystem = contractsToDeploy.includes("systemPrimitive");
  const needPython = contractsToDeploy.includes("pythonPrimitive");

  // 5) deploy each primitive only if needed
  if (needCounter) {
    const CounterFactory = await ethers.getContractFactory("CounterPrimitive");
    await CounterFactory.deploy(LLM_ADDRESS, "", executorAddress).catch(() =>
      console.warn("Skipping CounterPrimitive"),
    );
  }
  if (needMath) {
    const MathFactory = await ethers.getContractFactory("MathPrimitive");
    await MathFactory.deploy(LLM_ADDRESS, "", executorAddress).catch(() =>
      console.warn("Skipping MathPrimitive"),
    );
  }
  if (needERC20) {
    const ERC20Factory = await ethers.getContractFactory("ERC20Primitive", {
      libraries: { UserDecimalFormatting: udfAddress },
    });
    await ERC20Factory.deploy(LLM_ADDRESS, "", executorAddress).catch(() =>
      console.warn("Skipping ERC20Primitive"),
    );
  }
  if (needAMM) {
    const AmmFactory = await ethers.getContractFactory("AmmPrimitive", {
      libraries: { UserDecimalFormatting: udfAddress },
    });
    await AmmFactory.deploy(LLM_ADDRESS, "", executorAddress).catch(() =>
      console.warn("Skipping AmmPrimitive"),
    );
  }
  if (needSystem) {
    const SystemFactory = await ethers.getContractFactory("SystemPrimitive", {
      libraries: { UserDecimalFormatting: udfAddress },
    });
    await SystemFactory.deploy(LLM_ADDRESS, "", executorAddress).catch(() =>
      console.warn("Skipping SystemPrimitive"),
    );
  }
  if (needPython) {
    const PythonFactory = await ethers.getContractFactory("PythonPrimitive", {
      libraries: { UserDecimalFormatting: udfAddress },
    });
    await PythonFactory.deploy(LLM_ADDRESS, "").catch(() =>
      console.warn("Skipping PythonPrimitive"),
    );
  }

  // 6) helper to clone via executor
  async function deployClone(
    name: string,
    label: string,
    initArgs: any[] = [],
  ) {
    const sigTypes = initArgs.map((a) =>
      typeof a === "string" && a.startsWith("0x")
        ? "address"
        : typeof a === "string"
          ? "string"
          : "uint256",
    );
    const initData = generateFunctionCallData("configure", sigTypes, initArgs);
    const tx = await executor.deployRobotContract(
      name,
      label,
      await owner.getAddress(),
      "",
      initData,
    );
    const rcpt = await tx.wait();
    const iface = executor.interface as unknown as Interface;

    for (const rawLog of rcpt.logs) {
      try {
        const parsed = iface.parseLog(rawLog);
        if (parsed.name === "RobotContractDeployed") {
          return parsed.args.contractAddress as string;
        }
      } catch {
        /* not our event */
      }
    }
    throw new Error("RobotContractDeployed event not found");
  }

  // 7) Clone only what you asked for
  const env: Partial<TestEnv> = {
    executor,
    executorAddress,
    owner,
    llmContract,
  };

  if (toDeploy.has("counterAContract")) {
    const addr = await deployClone("counter", "counterA");
    env.counterAContractAddress = addr;
    env.counterAContract = await ethers.getContractAt(
      "CounterPrimitive",
      addr,
      owner,
    );
  }
  if (toDeploy.has("counterBContract")) {
    const addr = await deployClone("counter", "counterB");
    env.counterBContractAddress = addr;
    env.counterBContract = await ethers.getContractAt(
      "CounterPrimitive",
      addr,
      owner,
    );
  }
  if (toDeploy.has("mathContract")) {
    const addr = await deployClone("math", "calculator");
    env.mathContractAddress = addr;
    env.mathContract = await ethers.getContractAt("MathPrimitive", addr, owner);
  }
  if (toDeploy.has("usdcContract")) {
    const addr = await deployClone("erc20", "USDC", ["100000"]);
    env.usdcContractAddress = addr;
    env.usdcContract = await ethers.getContractAt(
      "ERC20Primitive",
      addr,
      owner,
    );
  }
  if (toDeploy.has("jiriContract")) {
    const addr = await deployClone("erc20", "JIRI", ["100000"]);
    env.jiriContractAddress = addr;
    env.jiriContract = await ethers.getContractAt(
      "ERC20Primitive",
      addr,
      owner,
    );
  }

  // for AMM we depend on having both token addresses in env
  if (
    toDeploy.has("ammContract1") &&
    env.usdcContractAddress &&
    env.jiriContractAddress
  ) {
    const addr1 = await deployClone("amm", "amm1", [
      env.usdcContractAddress,
      env.jiriContractAddress,
      "0",
    ]);
    env.ammContract1Address = addr1;
    env.ammContract1 = await ethers.getContractAt("AmmPrimitive", addr1, owner);
  }
  if (
    toDeploy.has("ammContract2") &&
    env.usdcContractAddress &&
    env.jiriContractAddress
  ) {
    const addr2 = await deployClone("amm", "amm2", [
      env.usdcContractAddress,
      env.jiriContractAddress,
      "0",
    ]);
    env.ammContract2Address = addr2;
    env.ammContract2 = await ethers.getContractAt("AmmPrimitive", addr2, owner);
  }
  if (
    toDeploy.has("ammContract3") &&
    env.usdcContractAddress &&
    env.jiriContractAddress
  ) {
    const addr3 = await deployClone("amm", "amm3", [
      env.usdcContractAddress,
      env.jiriContractAddress,
      "0",
    ]);
    env.ammContract3Address = addr3;
    env.ammContract3 = await ethers.getContractAt("AmmPrimitive", addr3, owner);
  }

  return env as TestEnv;
}
