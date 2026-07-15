#!/usr/bin/env node

import {
  getTestCaseInfo,
  prepareStage,
  validateOutput,
} from "./protocol.mjs";

async function readRequest() {
  let body = "";
  for await (const chunk of process.stdin) body += chunk;
  return JSON.parse(body);
}

try {
  const request = await readRequest();
  let response;
  switch (request.command) {
    case "case_info":
      response = getTestCaseInfo(request.test_case);
      break;
    case "prepare_stage":
      response = prepareStage(request);
      break;
    case "validate_output":
      response = validateOutput(request);
      break;
    default:
      throw new Error(`Unknown sdk-tests command: ${String(request.command)}`);
  }
  process.stdout.write(`${JSON.stringify(response)}\n`);
} catch (error) {
  process.stderr.write(`${error instanceof Error ? error.message : String(error)}\n`);
  process.exitCode = 1;
}
