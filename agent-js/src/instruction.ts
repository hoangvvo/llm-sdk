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
  const results = await Promise.all(
    instructions.map((instruction) =>
      typeof instruction === "function"
        ? Promise.resolve(instruction(ctx))
        : Promise.resolve(instruction),
    ),
  );

  return results.join("\n");
}
