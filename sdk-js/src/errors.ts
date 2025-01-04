/**
 * The input is not supported by or is incompatible with the model
 * (e.g. using non text for assistant message parts)
 */
export class UnsupportedError extends Error {
  constructor(message: string) {
    super(message);
    this.name = "Unsupported";
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
    super(message);
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
