import type { LanguageModelError } from "@hoangvvo/llm-sdk";

export class AgentError extends Error {
  constructor(message: string) {
    super(message);
    this.name = "AgentError";
  }
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
