import type {
  AgentTool,
  InstructionParam,
  Toolkit,
  ToolkitSession,
} from "@hoangvvo/llm-agent";
import { Agent, getResponseText, tool } from "@hoangvvo/llm-agent";
import { getModel } from "./get-model.ts";

type MatchId = "sun-showdown" | "dream-dusk";

type Weather = "harsh_sunlight" | "none";

type MoveId =
  | "flamethrower"
  | "solar_beam"
  | "air_slash"
  | "shadow_ball"
  | "hypnosis"
  | "dream_eater"
  | "nightmare";

interface BattleContext {
  matchId: MatchId;
}

interface BattleState {
  weather: Weather;
  arena: string;
  crowdNote: string;
  pokemon: {
    name: string;
    ability: string;
    item?: string;
    moves: MoveId[];
  };
  opponent: {
    name: string;
    typing: string[];
    status: "healthy" | "asleep";
    hint: string;
  };
}

const MATCHES: Record<MatchId, BattleState> = {
  "sun-showdown": {
    weather: "harsh_sunlight",
    arena: "Pyrite Crater",
    crowdNote: "Crowd roars when dazzling sun-fuelled attacks are described.",
    pokemon: {
      name: "Charizard",
      ability: "Solar Power",
      item: "Choice Specs",
      moves: ["flamethrower", "solar_beam", "air_slash"],
    },
    opponent: {
      name: "Ferrothorn",
      typing: ["Grass", "Steel"],
      status: "healthy",
      hint: "likes to turtle behind Leech Seed and Iron Defense.",
    },
  },
  "dream-dusk": {
    weather: "none",
    arena: "Midnight Colosseum",
    crowdNote: "Spectators fall silent for sinister dream tactics.",
    pokemon: {
      name: "Haunter",
      ability: "Levitate",
      item: "Wide Lens",
      moves: ["shadow_ball", "hypnosis", "dream_eater", "nightmare"],
    },
    opponent: {
      name: "Gardevoir",
      typing: ["Psychic", "Fairy"],
      status: "asleep",
      hint: "was building Calm Mind stacks before dozing off.",
    },
  },
};

const delay = (ms: number) => new Promise((resolve) => setTimeout(resolve, ms));

async function loadBattle(matchId: MatchId): Promise<BattleState> {
  const snapshot = MATCHES[matchId];
  if (!snapshot) {
    throw new Error(`Unknown match ${matchId}`);
  }
  await delay(25); // mimic an async fetch that Toolkits must handle in createSession
  return JSON.parse(JSON.stringify(snapshot)) as BattleState;
}

type MoveAvailability = (state: BattleState) => boolean;

type MoveExecutor = (state: BattleState, target?: string) => string;

interface MoveSpec {
  name: MoveId;
  description: string;
  available?: MoveAvailability;
  execute: MoveExecutor;
}

const MOVE_LIBRARY: Record<MoveId, MoveSpec> = {
  flamethrower: {
    name: "flamethrower",
    description:
      "Flamethrower is a dependable Fire-type strike. Mention how Harsh Sunlight boosts it.",
    execute: (state, target = state.opponent.name) =>
      `I scorch ${target} with Flamethrower${
        state.weather === "harsh_sunlight"
          ? ", the sunlight turning the flames white-hot."
          : "."
      }`,
  },
  solar_beam: {
    name: "solar_beam",
    description:
      "Solar Beam normally takes a turn to charge, but fires instantly in Harsh Sunlight.",
    available: (state) => state.weather === "harsh_sunlight",
    execute: (state, target = state.opponent.name) =>
      `I gather sunlight and unleash Solar Beam on ${target} without needing to charge.`,
  },
  air_slash: {
    name: "air_slash",
    description:
      "Air Slash provides Flying coverage with a chance to flinch slower foes.",
    execute: (state, target = state.opponent.name) =>
      `I ride the thermals around ${state.arena} and carve ${target} with Air Slash.`,
  },
  shadow_ball: {
    name: "shadow_ball",
    description:
      "Shadow Ball is Haunter's safest Ghost attack versus Psychic targets.",
    execute: (state, target = state.opponent.name) =>
      `I hurl Shadow Ball at ${target}, disrupting their ${state.opponent.typing.join("/")} defenses.`,
  },
  hypnosis: {
    name: "hypnosis",
    description:
      "Hypnosis can return the opponent to sleep if they start to wake.",
    execute: (state, target = state.opponent.name) =>
      `I sway and cast Hypnosis toward ${target}, readying follow-up dream tactics.`,
  },
  dream_eater: {
    name: "dream_eater",
    description:
      "Dream Eater only works while the opponent sleeps, draining them and healing me.",
    available: (state) => state.opponent.status === "asleep",
    execute: (state, target = state.opponent.name) =>
      `I feast on ${target}'s dreams, siphoning strength back to ${state.pokemon.name}.`,
  },
  nightmare: {
    name: "nightmare",
    description:
      "Nightmare curses a sleeping foe to lose HP at the end of each turn.",
    available: (state) => state.opponent.status === "asleep",
    execute: (state, target = state.opponent.name) =>
      `I lace ${target}'s dreams with a Nightmare so they suffer each turn while asleep.`,
  },
};

function buildMoveTools(state: BattleState): AgentTool<BattleContext>[] {
  return state.pokemon.moves
    .map((moveId) => {
      const spec = MOVE_LIBRARY[moveId];
      if (!spec) return undefined;
      if (spec.available && !spec.available(state)) return undefined;
      return tool<BattleContext, { target?: string }>({
        name: spec.name,
        description: spec.description,
        parameters: {
          type: "object",
          properties: {
            target: {
              type: "string",
              description:
                "Optional override; defaults to the opposing Pokémon.",
            },
          },
          required: [],
          additionalProperties: false,
        },
        execute: (args) => ({
          content: [
            {
              type: "text",
              text: spec.execute(state, args.target),
            },
          ],
          is_error: false,
        }),
      });
    })
    .filter((entry): entry is AgentTool<BattleContext> => Boolean(entry));
}

// ToolkitSession capturing a battle snapshot so prompt/tools reflect dynamic match conditions derived in createSession.
class BattleToolkitSession implements ToolkitSession<BattleContext> {
  #prompt: string;
  #tools: AgentTool<BattleContext>[];

  constructor(private readonly state: BattleState) {
    // Toolkit sessions compute prompt and tools once so getSystemPrompt/getTools stay synchronous.
    this.#prompt = this.#buildPrompt();
    this.#tools = buildMoveTools(state);
  }

  getSystemPrompt(): string {
    return this.#prompt;
  }

  getTools(): AgentTool<BattleContext>[] {
    return this.#tools;
  }

  async close() {
    // Nothing to release in this example, but real toolkits could close DB handles here.
  }

  #buildPrompt(): string {
    const parts: string[] = [];
    parts.push(
      `You are ${this.state.pokemon.name} battling in ${this.state.arena}.`,
    );
    if (this.state.weather === "harsh_sunlight") {
      parts.push(
        "Harsh Sunlight supercharges Fire moves, lets Solar Beam skip its charge, and weakens Water coverage.",
      );
    } else {
      parts.push("There is no active weather effect.");
    }
    parts.push(
      `Ability: ${this.state.pokemon.ability}${
        this.state.pokemon.item ? `, holding ${this.state.pokemon.item}` : ""
      }.`,
    );
    parts.push(
      `Opponent: ${this.state.opponent.name} (${this.state.opponent.typing.join(
        "/",
      )}), currently ${this.state.opponent.status}. ${this.state.opponent.hint}`,
    );
    parts.push(`Crowd note: ${this.state.crowdNote}`);
    parts.push(
      "Call one available move tool (or use_item / attempt_escape) before finalising and explain why the field makes it sensible.",
    );
    return parts.join(" ");
  }
}

// Toolkit implementation that runs async lookups in createSession so dynamic guidance/tools
// are computed once and the session surface can stay synchronous.
class BattleToolkit implements Toolkit<BattleContext> {
  async createSession(
    context: BattleContext,
  ): Promise<ToolkitSession<BattleContext>> {
    const state = await loadBattle(context.matchId);
    // createSession runs async operations so we can derive dynamic tools and instructions before reuse.
    return new BattleToolkitSession(state);
  }
}

const useItemTool = tool<BattleContext, { item: string }>({
  name: "use_item",
  description: "Use a held or bag item to swing the battle.",
  parameters: {
    type: "object",
    properties: {
      item: { type: "string", description: "Item to use." },
    },
    required: ["item"],
    additionalProperties: false,
  },
  execute: (args) => ({
    content: [
      {
        type: "text",
        text: `I use the ${args.item} to shift momentum.`,
      },
    ],
    is_error: false,
  }),
});

const attemptEscapeTool = tool<BattleContext, Record<string, never>>({
  name: "attempt_escape",
  description: "Attempt to flee if the match is unwinnable.",
  parameters: {
    type: "object",
    properties: {},
    required: [],
    additionalProperties: false,
  },
  execute: () => ({
    content: [
      {
        type: "text",
        text: "I search for an opening to retreat from the field.",
      },
    ],
    is_error: false,
  }),
});

const instructions: InstructionParam<BattleContext>[] = [
  "Speak in first person as the active Pokémon.",
  "Always invoke exactly one tool before ending your answer, and mention the current field conditions while justifying it.",
];

const battleCoach = new Agent<BattleContext>({
  name: "Satoshi",
  instructions,
  model: getModel("openai", "gpt-4o"),
  tools: [useItemTool, attemptEscapeTool],
  toolkits: [new BattleToolkit()],
});

async function runExample(matchId: MatchId, prompt: string) {
  const response = await battleCoach.run({
    context: { matchId },
    input: [
      {
        type: "message",
        role: "user",
        content: [{ type: "text", text: prompt }],
      },
    ],
  });
  console.log(`\n=== ${matchId} ===`);
  console.log(getResponseText(response));
}

await runExample(
  "sun-showdown",
  "Ferrothorn is hiding behind Iron Defense again—what's our play?",
);
await runExample(
  "dream-dusk",
  "Gardevoir is still asleep—press the advantage before it wakes up!",
);
