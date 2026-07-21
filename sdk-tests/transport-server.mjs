#!/usr/bin/env node

import { createServer } from "node:http";
import { prepareTransportStage, validateTransportRequest } from "./protocol.ts";

function fail(message) {
  throw new Error(message);
}

async function readBody(request) {
  let body = "";
  for await (const chunk of request) body += chunk;
  if (body.length === 0) return undefined;
  try {
    return JSON.parse(body);
  } catch {
    return body;
  }
}

function frameEvent(event) {
  const lines = [];
  if (event.event !== undefined) lines.push(`event: ${event.event}`);
  const data =
    event.raw_data !== undefined
      ? event.raw_data
      : JSON.stringify(event.data ?? {});
  lines.push(`data: ${data}`);
  return `${lines.join("\n")}\n\n`;
}

async function writeFragments(response, value, splitAt = []) {
  let offset = 0;
  for (const end of [...splitAt, value.length]) {
    response.write(value.slice(offset, end));
    offset = end;
    await new Promise((resolve) => setImmediate(resolve));
  }
}

const testCaseName = process.argv[2];
if (!testCaseName) fail("Usage: transport-server.mjs <test-case-name>");

const prepared = prepareTransportStage({
  test_case: testCaseName,
  stage: 0,
  context: { stages: [] },
});
const fixture = prepared.transport;
let handled = false;
let verificationTimeout;

const server = createServer(async (request, response) => {
  if (handled) {
    response.statusCode = 503;
    response.end("transport fixture already consumed");
    return;
  }
  handled = true;

  let verification;
  try {
    const recording = {
      method: request.method,
      path: request.url,
      headers: Object.fromEntries(
        Object.entries(request.headers).map(([name, value]) => [
          name.toLowerCase(),
          Array.isArray(value) ? value.join(", ") : value,
        ]),
      ),
      body: await readBody(request),
    };
    validateTransportRequest(fixture.request, recording);
    verification = { ok: true };
  } catch (error) {
    verification = {
      ok: false,
      error: error instanceof Error ? error.message : String(error),
    };
  }

  const configured = fixture.response;
  response.statusCode = configured.status ?? 200;
  for (const [name, value] of Object.entries(configured.headers ?? {})) {
    response.setHeader(name, value);
  }

  response.once("finish", () => {
    clearTimeout(verificationTimeout);
    server.close(() => {
      process.stdout.write(`${JSON.stringify(verification)}\n`);
    });
  });

  if (configured.body !== undefined) {
    response.end(
      typeof configured.body === "string"
        ? configured.body
        : JSON.stringify(configured.body),
    );
  } else if (Array.isArray(configured.events)) {
    for (const event of configured.events) {
      await writeFragments(response, frameEvent(event), event.split_at);
    }
    response.end();
  } else if (Array.isArray(configured.chunks)) {
    for (const chunk of configured.chunks) {
      await writeFragments(response, chunk);
    }
    response.end();
  } else {
    response.end();
  }
});

server.listen(0, "127.0.0.1", () => {
  const address = server.address();
  if (!address || typeof address === "string") {
    fail("Failed to determine transport replay address");
  }
  process.stdout.write(
    `${JSON.stringify({ base_url: `http://127.0.0.1:${address.port}` })}\n`,
  );
  verificationTimeout = setTimeout(() => {
    server.close(() => {
      process.stdout.write(
        `${JSON.stringify({ ok: false, error: "Transport fixture received no request" })}\n`,
      );
    });
  }, 10_000);
});
