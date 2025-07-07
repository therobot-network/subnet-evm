import { expect } from "chai";
import { Contract, Signer } from "ethers";
import fs from "fs";
import * as path from "path";
import yaml from "js-yaml";
import { setupTestEnvironment, TestEnv } from "../helpers/setupFixtures";

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
  let usdcContract: Contract;
  let usdcContractAddress: string;
  let owner: Signer;

  before(async function () {
    env = await setupTestEnvironment(["llm", "executor", "usdcContract"]);
    ({ executor, owner, usdcContractAddress, usdcContract } = env);
  });

  it("should pass transferHalf.yaml", async function () {
    const testFile = path.join(erc20PlansDir, "transferHalf.yaml");
    const raw = fs.readFileSync(testFile, "utf8");
    const data = yaml.load(raw) as {
      title: string;
      description: string;
      prompt: string;
      expected?: string;
      fails?: boolean;
    };

    const ownerAddress = await owner.getAddress();

    const balanceBefore = await usdcContract.balanceOf(ownerAddress);

    const payload = JSON.stringify({
      prompt: data.prompt,
      wallets: {
        signer: "0x8db97C7cEcE249c2b98bDC0226Cc4C2A57BF52FC",
        bob: "0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
      },
      contracts: {
        USDC: {
          primitive: "erc20",
          address: usdcContractAddress,
        },
      },
    });

    const tx = await executor.evalPrompt(payload);
    await tx.wait();

    const balanceAfter = await usdcContract.balanceOf(ownerAddress);

    expect(balanceAfter).to.equal(
      balanceBefore / 2n,
      "Balance should be halved",
    );
  });
});
