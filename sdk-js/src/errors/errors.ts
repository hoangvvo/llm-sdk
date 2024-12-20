import type { Message, Part } from "../types.js";

export class ModelUnsupportedMessagePart extends Error {
  constructor(
    provider: string,
    public llmMessage: Message,
    public part: Part,
  ) {
    super(
      `${provider} does not support message part type = ${part.type} for role = ${llmMessage.role}`,
    );
  }
}

export class InvalidValueError extends Error {
  constructor(
    public parameterName: string,
    public parameterValue: unknown,
    extraMessage?: string,
  ) {
    super(
      `${parameterName} is invalid: ${String(parameterValue)}` +
        (extraMessage ? `. ${extraMessage}` : ""),
    );
  }
}

export class NotImplementedError extends Error {
  constructor(
    public parameterName: string,
    public prameterValue: unknown,
    extraMessage?: string,
  ) {
    super(
      `${parameterName} is not implemented: ${String(prameterValue)}` +
        (extraMessage ? `. ${extraMessage}` : ""),
    );
  }
}
