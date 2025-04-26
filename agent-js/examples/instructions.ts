import { Agent, getTextFromResponse } from "@hoangvvo/llm-agent";
import { getModel } from "./get-model.ts";

interface DungeonRunContext {
  dungeonMaster: string;
  partyName: string;
  currentQuest: string;
  highlightPlayerClass: string;
  getOracleWhisper(): Promise<string>;
}

const model = getModel("openai", "gpt-4o");

const dungeonCoach = new Agent<DungeonRunContext>({
  name: "Torch",
  instructions: [
    "You are Torch, a supportive guide who keeps tabletop role-playing sessions moving. Offer concrete options instead of long monologues.",
    (context) =>
      `You are helping ${context.dungeonMaster}, the Dungeon Master for the ${context.partyName}. They are running the quest "${context.currentQuest}" and need a quick nudge that favors the party's ${context.highlightPlayerClass}.`,
    async (context) => {
      const whisper = await context.getOracleWhisper();
      return `Weave in the oracle whisper: "${whisper}" so it feels like an in-world hint.`;
    },
  ],
  model,
});

const context: DungeonRunContext = {
  dungeonMaster: "Rowan",
  partyName: "Lanternbearers",
  currentQuest: "Echoes of the Sunken Keep",
  highlightPlayerClass: "ranger",
  async getOracleWhisper() {
    await new Promise((resolve) => setTimeout(resolve, 25));
    return "the moss remembers every secret step";
  },
};

const response = await dungeonCoach.run({
  context,
  input: [
    {
      type: "message",
      role: "user",
      content: [
        {
          type: "text",
          text: "The party is stuck at a collapsed bridge. What should happen next?",
        },
      ],
    },
  ],
});

console.log(getTextFromResponse(response));
