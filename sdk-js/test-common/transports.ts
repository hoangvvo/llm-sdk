import { spawn } from "node:child_process";
import { once } from "node:events";
import { fileURLToPath } from "node:url";
import { createInterface } from "node:readline";
import type { TestContext } from "node:test";
import { getTestCasesByGroup } from "../../sdk-tests/protocol.ts";
import type { LanguageModel } from "../src/language-model.ts";
import { runTestCase } from "./cases.ts";

interface ReplayStart {
  base_url: string;
}

interface ReplayVerification {
  ok: boolean;
  error?: string;
}

async function startReplay(t: TestContext, testCaseName: string) {
  const script = fileURLToPath(
    new URL("../../sdk-tests/transport-server.mjs", import.meta.url),
  );
  const child = spawn(process.execPath, [script, testCaseName], {
    stdio: ["ignore", "pipe", "pipe"],
  });
  const exit = once(child, "exit") as Promise<
    [code: number | null, signal: NodeJS.Signals | null]
  >;
  let stderr = "";
  child.stderr.setEncoding("utf8");
  child.stderr.on("data", (chunk: string) => {
    stderr += chunk;
  });
  const lines = createInterface({ input: child.stdout });
  const iterator = lines[Symbol.asyncIterator]();
  const first = await iterator.next();
  if (first.done) {
    const [code] = await exit;
    throw new Error(
      `Transport replay exited before startup (${String(code)}): ${stderr}`,
    );
  }
  const start = JSON.parse(first.value) as ReplayStart;
  let finished = false;

  t.after(async () => {
    if (finished) return;
    child.kill();
    await exit;
  });

  return {
    baseURL: start.base_url,
    async verify() {
      const result = await iterator.next();
      const [code, signal] = await exit;
      finished = true;
      if (result.done) {
        throw new Error(
          `Transport replay exited without verification (${String(code)}, ${String(signal)}): ${stderr}`,
        );
      }
      const verification = JSON.parse(result.value) as ReplayVerification;
      if (code !== 0 || !verification.ok) {
        throw new Error(
          verification.error ??
            `Transport replay failed (${String(code)}, ${String(signal)}): ${stderr}`,
        );
      }
    },
  };
}

export async function runTransportTestGroup(
  t: TestContext,
  group: string,
  createModel: (baseURL: string) => LanguageModel,
) {
  for (const testCaseName of getTestCasesByGroup(group)) {
    await t.test(testCaseName, { timeout: 20_000 }, async (child) => {
      const replay = await startReplay(child, testCaseName);
      try {
        await runTestCase(child, createModel(replay.baseURL), testCaseName);
      } finally {
        await replay.verify();
      }
    });
  }
}
