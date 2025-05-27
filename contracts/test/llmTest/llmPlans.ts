// (c) 2019-2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

import { expect } from "chai";
import { Contract, Signer } from "ethers";
import { ethers } from "hardhat";
import fs from "fs";
import * as path from "path";
import yaml from "js-yaml";
import { setupTestEnvironment, TestEnv } from "./helpers/setupFixtures";

const LLM_ADDRESS = "0x0300000000000000000000000000000000000000";

const binaryOpsRoot = path.resolve(
  __dirname,
  "planTests",
  "tests",
  "binary_operators",
);

const opDirs = fs
  .readdirSync(binaryOpsRoot)
  .filter((d) => fs.statSync(path.join(binaryOpsRoot, d)).isDirectory());

for (const opDir of opDirs) {
  // if (opDir != "minus") {
  //   continue; // Skip all but "minus" directory
  // }
  describe(`LLM Precompiled Contract - ${opDir}`, function () {
    let env: TestEnv;
    let executor: Contract;
    let llmContract: Contract;

    const testsDir = path.join(binaryOpsRoot, opDir);

    before(async function () {
      env = await setupTestEnvironment(["llm", "executor"]);
      ({ executor, llmContract } = env);
    });

    // Read all `.yaml` files under the "plus" directory
    const files = fs.readdirSync(testsDir).filter((f) => f.endsWith(".yaml"));

    for (const file of files) {
      const testName = path.basename(file, ".yaml");
      // if (testName != "text_minus_float") {
      //   continue;
      // }
      it(`should pass ${testName}`, async function () {
        // 1) load & parse the YAML
        const raw = fs.readFileSync(path.join(testsDir, file), "utf8");
        const data = yaml.load(raw) as {
          title: string;
          description: string;
          python: string;
          expected?: string;
          json: string;
          fails?: boolean;
        };

        const planObj = JSON.parse(data.json);
        const payload = JSON.stringify({
          plan: JSON.stringify(planObj),
          lookupTable: JSON.stringify({}),
        });

        if (data.fails) {
          let isFailed = false;
          await executor.evalPlan(payload).catch((err) => {
            isFailed = true;
          });
          expect(isFailed).to.be.true;
          return;
        }

        // 2) parse expected and assert event
        let expected: string[] = [];
        // Prefer YAML array of strings if possible
        if (
          Array.isArray(data.expected!) &&
          data.expected!.every((x) => typeof x === "string")
        ) {
          expected = data.expected;
        } else {
          try {
            const parsed = JSON.parse(data.expected!);
            if (Array.isArray(parsed)) {
              expected = parsed.map((x) => String(x));
            } else {
              expected = [String(parsed)];
            }
          } catch (e) {
            // Fallback: treat as string
            expected = [data.expected!.trim()];
          }
        }

        // 3) invoke evalPlan
        const tx = await executor.evalPlan(payload);
        await tx.wait();

        // 4) assert that each expected answer was emitted
        await expect(tx)
          .to.emit(llmContract, "QuestionAnswer")
          .withArgs(
            (_question) => true,
            (answer) => {
              const exp = expected[0];
              if (typeof exp === "object") {
                return answer === JSON.stringify(exp);
              }
              return answer === String(exp);
            },
          );
      });
    }
  });
}
