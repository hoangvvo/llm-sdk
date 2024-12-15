export class ModelUnsupportedMessagePart extends Error {
  constructor(provider: string, part: string) {
    super(`${provider} does not support message part: ${part}`);
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
