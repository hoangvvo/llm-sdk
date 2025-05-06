import { Agent, getResponseText, tool } from "@hoangvvo/llm-agent";
import { zodTool } from "@hoangvvo/llm-agent/zod";
import z from "zod";
import { getModel } from "./get-model.ts";

interface DungeonRunContext {
  dungeonMaster: string;
  partyName: string;
  encounter: {
    scene: string;
    enemies: Record<string, { hp: number }>;
    downedAllies: Set<string>;
  };
  actionBudget: Record<string, number>;
}

function createDungeonContext(): DungeonRunContext {
  return {
    dungeonMaster: "Rowan",
    partyName: "Lanternbearers",
    encounter: {
      scene: "The Echo Bridge over the Sunken Keep",
      enemies: {
        ghoul: { hp: 12 },
        marauder: { hp: 9 },
      },
      downedAllies: new Set(["finley"]),
    },
    actionBudget: {
      thorne: 1,
      mira: 2,
    },
  };
}

const attackEnemyTool = tool({
  name: "attack_enemy",
  description:
    "Resolve a martial attack from a party member against an active enemy and update its hit points.",
  parameters: {
    type: "object",
    properties: {
      attacker: {
        type: "string",
        description: "Name of the party member making the attack.",
      },
      target: {
        type: "string",
        description: "Enemy to strike.",
      },
      weapon: {
        type: "string",
        description: "Weapon or maneuver used for flavour and damage bias.",
      },
    },
    required: ["attacker", "target", "weapon"],
    additionalProperties: false,
  },
  execute(
    args: { attacker: string; target: string; weapon: string },
    context: DungeonRunContext,
  ) {
    const attackerKey = args.attacker.trim().toLowerCase();
    const targetKey = args.target.trim().toLowerCase();

    const remainingActions = context.actionBudget[attackerKey] ?? 0;

    if (remainingActions <= 0) {
      return {
        content: [
          {
            type: "text",
            text: `${args.attacker} is out of actions this round. Ask another hero to step in or advance the scene.`,
          },
        ],
        is_error: true,
      };
    }

    const enemy = context.encounter.enemies[targetKey];

    if (!enemy) {
      return {
        content: [
          {
            type: "text",
            text: `No enemy named ${args.target} remains at ${context.encounter.scene}. Double-check the initiative order.`,
          },
        ],
        is_error: true,
      };
    }

    const baseDamage = args.weapon.toLowerCase().includes("axe") ? 7 : 5;
    const finesseBonus = args.weapon.toLowerCase().includes("dagger") ? 1 : 0;
    const computedDamage =
      baseDamage + (args.attacker.length % 3) + finesseBonus;

    enemy.hp = Math.max(0, enemy.hp - computedDamage);
    context.actionBudget[attackerKey] = remainingActions - 1;

    const defeatedText = enemy.hp === 0 ? ` ${args.target} is defeated!` : ``;

    return {
      content: [
        {
          type: "text",
          text: `${args.attacker} hits ${args.target} for ${String(computedDamage)} damage with the ${args.weapon}. ${args.target} now has ${String(enemy.hp)} HP.${defeatedText}`,
        },
      ],
      is_error: false,
    };
  },
});

const stabilizeAllyTool = zodTool({
  name: "stabilize_ally",
  description:
    "Spend the round stabilising a downed ally. Removes them from the downed list if available.",
  parameters: z.object({
    hero: z.string().describe("Name of the ally to stabilise."),
  }),
  execute(args, context: DungeonRunContext) {
    const heroKey = args.hero.trim().toLowerCase();
    const wasDowned = context.encounter.downedAllies.has(heroKey);

    if (!wasDowned) {
      return {
        content: [
          {
            type: "text",
            text: `${args.hero} is already on their feet. Consider taking another tactical action instead of stabilising.`,
          },
        ],
        is_error: true,
      };
    }

    context.encounter.downedAllies.delete(heroKey);

    return {
      content: [
        {
          type: "text",
          text: `${args.hero} is stabilised and ready to rejoin when the next round begins.`,
        },
      ],
      is_error: false,
    };
  },
});

const model = getModel("openai", "gpt-4o");

const dungeonCoach = new Agent<DungeonRunContext>({
  name: "Torch",
  instructions: [
    "You are Torch, a steady co-Dungeon Master. Keep answers short and, when combat actions come up, lean on the provided tools to resolve them.",
    "If a requested action involves striking an enemy, call attack_enemy. If the party wants to help someone back up, call stabilize_ally before answering.",
  ],
  model,
  tools: [attackEnemyTool, stabilizeAllyTool],
});

const successContext = createDungeonContext();
const successResponse = await dungeonCoach.run({
  context: successContext,
  input: [
    {
      type: "message",
      role: "user",
      content: [
        {
          type: "text",
          text: "Thorne will strike the ghoul with a battleaxe while Mira uses her turn to stabilise Finley. Help me resolve it.",
        },
      ],
    },
  ],
});

console.dir(successResponse, { depth: null });
console.log(getResponseText(successResponse));
console.log("Remaining enemy HP:", successContext.encounter.enemies);
console.log(
  "Downed allies after success run:",
  Array.from(successContext.encounter.downedAllies),
);

const failureContext = createDungeonContext();
failureContext.actionBudget["thorne"] = 0;
failureContext.encounter.downedAllies.clear();

const failureResponse = await dungeonCoach.run({
  context: failureContext,
  input: [
    {
      type: "message",
      role: "user",
      content: [
        {
          type: "text",
          text: "Thorne wants to swing again at the marauder, and Mira tries to stabilise Finley anyway.",
        },
      ],
    },
  ],
});

console.dir(failureResponse, { depth: null });
console.log(getResponseText(failureResponse));
