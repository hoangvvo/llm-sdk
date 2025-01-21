export class AgentError extends Error {
  constructor(message: string) {
    super(message);
    this.name = "AgentError";
  }
}

export class AgentInvariantError extends AgentError {
  constructor(message: string) {
    super(`Invariant: ${message}`);
    this.name = "AgentInvariantError";
  }
}

export class AgentTurnsExceededError extends AgentError {
  constructor(maxTurns: number) {
    super(
      `The maximum number of turns (${String(maxTurns)}) has been exceeded.`,
    );
    this.name = "AgentTurnsExceededError";
  }
}
