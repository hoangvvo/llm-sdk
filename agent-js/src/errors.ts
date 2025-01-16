export class InvariantError extends Error {
  constructor(message: string) {
    super(`Invariant: ${message}`);
    this.name = "Invariant";
  }
}
