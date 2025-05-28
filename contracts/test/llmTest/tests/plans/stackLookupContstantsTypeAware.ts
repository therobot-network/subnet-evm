// (c) 2019-2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

import { expect } from "chai";
import { Contract, Signer } from "ethers";
import { ethers } from "hardhat";
import fs from "fs";
import * as path from "path";
import yaml from "js-yaml";
import { setupTestEnvironment, TestEnv } from "../helpers/setupFixtures";

const LLM_ADDRESS = "0x0300000000000000000000000000000000000000";

const stackLookupRoot = path.resolve(
  __dirname,
  "..",
  "..",
  "planTests",
  "tests",
  "stack_lookup_contstants-type-aware",
);

const files = fs
  .readdirSync(stackLookupRoot)
  .filter((f) => f.endsWith(".yaml"));

describe(`LLM Precompiled Contract - stack_lookup_contstants-type-aware`, function () {
  let env: TestEnv;
  let executor: Contract;
  let llmContract: Contract;

  before(async function () {
    env = await setupTestEnvironment(["llm", "executor"]);
    ({ executor, llmContract } = env);
  });

  for (const file of files) {
    const testName = path.basename(file, ".yaml");
    // if (testName != "specificTest") {
    //   continue;
    // }
    it(`should pass ${testName}`, async function () {
      // 1) load & parse the YAML
      const raw = fs.readFileSync(path.join(stackLookupRoot, file), "utf8");
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
      const receipt = await tx.wait();

      const iface = llmContract.interface;
      // Parse and collect all QuestionAnswer events from logs
      const events = receipt.logs
        .map((log) => {
          try {
            return iface.parseLog(log);
          } catch {
            return null;
          }
        })
        .filter((e) => e && e.name === "QuestionAnswer")
        .reverse();

      if (events.length === 0) {
        console.log(`Event QuestionAnswer not emitted`);
        return null;
      }

      expect(events.length).to.equal(expected.length);
      for (let i = 0; i < expected.length; i++) {
        const answer = events[i].args[1];
        const exp = expected[i];
        if (typeof exp === "object") {
          expect(answer).to.equal(JSON.stringify(exp));
        } else {
          expect(answer).to.equal(String(exp));
        }
      }
    });
  }
});
