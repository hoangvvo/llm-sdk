import type { LanguageModelError } from "@hoangvvo/llm-sdk";
import type { AgentRunSnapshot } from "./types.ts";

export class AgentError extends Error {
  /**
   * A best-effort snapshot of work completed before the run failed.
   */
  snapshot?: AgentRunSnapshot;

  constructor(message: string) {
    super(message);
    this.name = "AgentError";
  }

  /** @internal */
  withSnapshot(snapshot: AgentRunSnapshot): this {
    this.snapshot ??= snapshot;
    return this;
  }
}

export function agentErrorWithSnapshot(
  error: unknown,
  snapshot: AgentRunSnapshot,
): AgentError {
  if (error instanceof AgentError) {
    return error.withSnapshot(snapshot);
  }
  const wrapped = new AgentError(
    error instanceof Error ? error.message : String(error),
  );
  wrapped.cause = error;
  return wrapped.withSnapshot(snapshot);
}

export class AgentLanguageModelError extends AgentError {
  constructor(err: LanguageModelError) {
    super(`Language model error: ${err.message}`);
    this.name = "LanguageModelError";
    this.cause = err;
  }
}

export class AgentInvariantError extends AgentError {
  constructor(message: string) {
    super(`Invariant: ${message}`);
    this.name = "AgentInvariantError";
  }
}

export class AgentToolExecutionError extends AgentError {
  constructor(err: unknown) {
    super(
      `Tool execution failed: ${err instanceof Error ? err.message : String(err)}`,
    );
    this.name = "AgentToolExecutionError";
    this.cause = err;
  }
}

export class AgentMaxTurnsExceededError extends AgentError {
  constructor(maxTurns: number) {
    super(
      `The maximum number of turns (${String(maxTurns)}) has been exceeded.`,
    );
    this.name = "AgentTurnsExceededError";
  }
}

export class AgentInitError extends AgentError {
  constructor(err: unknown) {
    super(
      `Run initialization error: ${err instanceof Error ? err.message : String(err)}`,
    );
    this.name = "AgentInitError";
    this.cause = err;
  }
}

export class AgentCleanupError extends AgentError {
  constructor(err: unknown) {
    super(
      `Run cleanup error: ${err instanceof Error ? err.message : String(err)}`,
    );
    this.name = "AgentCleanupError";
    this.cause = err;
  }
}
