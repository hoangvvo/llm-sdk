/**
 * The input is not supported by or is incompatible with the model
 * (e.g. using non text for assistant message parts)
 */
export class UnsupportedError extends Error {
  constructor(message: string) {
    super(`An input is not supported by the model: ${message}`);
    this.name = "Unsupported";
  }
}

/**
 * An output from the model is not recognized by the library.
 * Please report this issue to the library maintainers.
 */
export class NotImplementedError extends Error {
  constructor(message: string) {
    super(
      `An output from the model is not recognized by the library: ${message}.`,
    );
    this.name = "NotImplemented";
  }
}

/**
 * The input is invalid or malformed
 */
export class InvalidInputError extends Error {
  constructor(message: string) {
    super(message);
    this.name = "InvalidInput";
  }
}

/**
 * The response from the provider was unexpected. (e.g. no choices returned
 * in an `OpenAI` completion)
 */
export class InvariantError extends Error {
  constructor(message: string) {
    super(`Unexpected response from the model: ${message}`);
    this.name = "Invariant";
  }
}

/**
 * The model refused to process the input. (e.g. `OpenAI` refusal)
 */
export class RefusalError extends Error {
  constructor(message: string) {
    super(message);
    this.name = "Refusal";
  }
}
