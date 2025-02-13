/**
 * The agent instruction can either be a string or a function
 * to be called with the agent context and return a string.
 */
export type InstructionParam<TContext> =
  | string
  | ((ctx: TContext) => string)
  | ((ctx: TContext) => Promise<string>);

export async function getPromptForInstructionParams<TContext>(
  instructions: InstructionParam<TContext>[],
  ctx: TContext,
) {
  return Promise.all(
    instructions.map((instruction) => {
      return typeof instruction === "function" ? instruction(ctx) : instruction;
    }),
  ).then((results) => results.join("\n"));
}
