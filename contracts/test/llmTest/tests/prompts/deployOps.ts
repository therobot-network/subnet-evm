import { expect } from "chai";
import { Contract, Signer } from "ethers";
import fs from "fs";
import * as path from "path";
import yaml from "js-yaml";
import { setupTestEnvironment, TestEnv } from "../helpers/setupFixtures";
import { ethers } from "hardhat";
import { sign } from "crypto";

// const originalSend = network.provider.send;

// before(function () {
//   network.provider.send = function (method: string, params: any[]) {
//     console.log(`📡 RPC Call → ${method}`, params);
//     // Use .apply to preserve the correct 'this'
//     return originalSend
//       .apply(this, [method, params])
//       .then((result: any) => {
//         console.log(`✅ RPC Response for ${method}:`, result);
//         return result;
//       })
//       .catch((error: any) => {
//         console.error(`❌ RPC Error on ${method}:`, error);
//         throw error;
//       });
//   };
// });

const erc20PlansDir = path.resolve(
  __dirname,
  "..",
  "..",
  "pythonBasedPlans",
  "erc20Plans",
);

describe("LLM Precompiled Contract - Prompt - erc20Plans", function () {
  let env: TestEnv;
  let executor: Contract;
  let owner: Signer;
  let llmContract: Contract;

  before(async function () {
    env = await setupTestEnvironment([
      "llm",
      "executor",
      "erc20Primitive",
      "systemPrimitive",
    ]);
    ({ executor, owner, llmContract } = env);
  });

  it("should pass deploy.yaml", async function () {
    const testFile = path.join(erc20PlansDir, "deploy.yaml");
    const raw = fs.readFileSync(testFile, "utf8");
    const data = yaml.load(raw) as {
      title: string;
      description: string;
      prompt: string;
      expected?: string;
      fails?: boolean;
    };

    const payload = JSON.stringify({
      prompt: data.prompt,
      wallets: {
        signer: await owner.getAddress(),
      },
      contracts: {
        AMM1: {
          primitive: "amm",
          address: ethers.ZeroAddress,
        },
      },
    });

    const tx = await executor.evalPrompt(payload);
    await tx.wait();

    const receipt = await tx.wait();
    for (const log of receipt.logs) {
      try {
        const parsed = executor.interface.parseLog(log);
        console.log("Event:", parsed.name, parsed.args);
      } catch (e) {
        // Not all logs are from this contract, so ignore parse errors
      }
    }

    let jiriContractAddress: string;

    await expect(tx)
      .to.emit(llmContract, "QuestionAnswer")
      .withArgs(
        (question) => true,
        (answer) => {
          jiriContractAddress = answer;
          return true;
        },
      );

    await expect(tx)
      .to.emit(executor, "RobotContractDeployed")
      .withArgs(
        (contractName) => contractName === "JIRI",
        (robotContractAddress) => {
          jiriContractAddress = robotContractAddress;
          return true;
        },
        (primitiveName) => primitiveName === "erc20",
      );

    const jiriContract = await ethers.getContractAt(
      "ERC20Primitive",
      jiriContractAddress,
      owner,
    );
    // const balance = await jiriContract.balanceOf(await owner.getAddress());
    // expect(balance).to.equal(500n);
    expect(await jiriContract.name()).to.equal("ROBOT_DEPLOYED_JIRI");
  });
});
