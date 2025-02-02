export class LanguageModelError extends Error {
  constructor(message: string) {
    super(message);
    this.name = "LanguageModelError";
  }
}

/**
 * The input is invalid or malformed
 */
export class InvalidInputError extends LanguageModelError {
  constructor(message: string) {
    super(`Invalid input: ${message}`);
    this.name = "InvalidInput";
  }
}

/**
 * The reqest returns a non-OK status code
 */
export class StatusCodeError extends LanguageModelError {
  statusCode: number;
  constructor(statusCode: number, message: string) {
    super(`Status error: ${message} (Status ${String(statusCode)})`);
    this.statusCode = statusCode;
    this.name = "StatusCode";
  }
}

/**
 * The input is not supported by or is incompatible with the model provider
 * (e.g. using non text for assistant message parts)
 */
export class UnsupportedError extends LanguageModelError {
  provider: string;
  constructor(provider: string, message: string) {
    super(`Unsupported by ${provider}: ${message}`);
    this.name = "Unsupported";
    this.provider = provider;
  }
}

/**
 * An output from the model is not recognized by the library.
 * Please report this issue to the library maintainers.
 */
export class NotImplementedError extends LanguageModelError {
  provider: string;
  constructor(provider: string, message: string) {
    super(`Not implemented for ${provider}: ${message}.`);
    this.name = "NotImplemented";
    this.provider = provider;
  }
}

/**
 * The response from the provider was unexpected. (e.g. no choices returned
 * in an `OpenAI` completion)
 */
export class InvariantError extends LanguageModelError {
  provider: string;
  constructor(provider: string, message: string) {
    super(`Invariant from ${provider}: ${message}`);
    this.name = "Invariant";
    this.provider = provider;
  }
}

/**
 * The model refused to process the input. (e.g. `OpenAI` refusal)
 */
export class RefusalError extends LanguageModelError {
  constructor(message: string) {
    super(`Refusal: ${message}`);
    this.name = "Refusal";
  }
}
