import type {
  AgentItem,
  AgentTool,
  InstructionParam,
  Toolkit,
  ToolkitSession,
} from "@hoangvvo/llm-agent";
import { Agent, getResponseText, tool } from "@hoangvvo/llm-agent";
import { getModel } from "./get-model.ts";

type VisitorId = "aurora-shift" | "ember-paradox";

interface RiftContext {
  visitorId: VisitorId;
}

interface RiftManifest {
  visitorName: string;
  originReality: string;
  arrivalSignature: string;
  contrabandRisk: "low" | "elevated" | "critical";
  sentimentalInventory: string[];
  outstandingAnomalies: string[];
  turbulenceLevel: "calm" | "moderate" | "volatile";
  courtesyNote: string;
}

// Mock datastore that stands in for an external manifest source resolved during createSession.
const RIFT_MANIFESTS: Record<VisitorId, RiftManifest> = {
  "aurora-shift": {
    visitorName: "Captain Lyra Moreno",
    originReality: "Aurora-9 Spiral",
    arrivalSignature: "slipped in trailing aurora dust and a three-second echo",
    contrabandRisk: "elevated",
    sentimentalInventory: [
      "Chrono Locket (Timeline 12)",
      "Folded star chart annotated in ultraviolet",
    ],
    outstandingAnomalies: [
      "Glitter fog refuses to obey gravity",
      "Field report cites duplicate footfalls arriving 4s late",
    ],
    turbulenceLevel: "moderate",
    courtesyNote: "Prefers dry humor, allergic to paradox puns.",
  },
  "ember-paradox": {
    visitorName: "Archivist Rune Tal",
    originReality: "Ember Paradox Belt",
    arrivalSignature: "emerged in a plume of cooled obsidian and smoke",
    contrabandRisk: "critical",
    sentimentalInventory: [
      "Glass bead containing their brother's timeline",
      "A singed manifesto titled 'Do Not Fold'",
    ],
    outstandingAnomalies: [
      "Customs still waiting on clearance form 88-A",
      "Phoenix feather repeats ignition loop every two minutes",
    ],
    turbulenceLevel: "volatile",
    courtesyNote: "Responds well to calm checklists and precise handoffs.",
  },
};

const delay = (ms: number) => new Promise((resolve) => setTimeout(resolve, ms));

// Simulated async lookup used inside Toolkit.createSession to hydrate session state up front.
async function fetchRiftManifest(visitorId: VisitorId): Promise<RiftManifest> {
  const manifest = RIFT_MANIFESTS[visitorId];
  if (!manifest) {
    throw new Error(`Unknown visitor ${visitorId}`);
  }
  await delay(60);
  return JSON.parse(JSON.stringify(manifest)) as RiftManifest;
}

type IntakePhase = "intake" | "recovery" | "handoff" | "closed";

// Toolkit session retains manifest snapshot and live state so we can rewrite prompts/tools each turn.
// A RunSession will hold onto this object and consult it before every call to the language model.
class LostAndFoundToolkitSession implements ToolkitSession<RiftContext> {
  readonly #manifest: RiftManifest;
  #phase: IntakePhase;
  #passVerified: boolean;
  #taggedItems: string[];
  #prophecyCount: number;
  #droneDeployed: boolean;

  constructor(manifest: RiftManifest) {
    this.#manifest = manifest;
    this.#phase = "intake";
    this.#passVerified = false;
    this.#taggedItems = [];
    this.#prophecyCount = 0;
    this.#droneDeployed = false;
  }

  getSystemPrompt(): string {
    // RunSession polls this every turn; reflect latest state in the instructions we provide.
    return this.#buildPrompt();
  }

  getTools(): AgentTool<RiftContext>[] {
    // Also polled each turn so we can expose a different toolset as the workflow advances.
    const tools = this.#buildTools();
    console.log(
      `[Toolkit] Tools for phase ${this.#phase.toUpperCase()}: ${
        tools.map((tool) => tool.name).join(", ") || "<none>"
      }`,
    );
    return tools;
  }

  async close() {
    /** noop */
  }

  #buildPrompt(): string {
    const lines: string[] = [];
    lines.push(
      "You are the Archivist manning Interdimensional Waypoint Seven's Lost & Found counter.",
    );
    lines.push(
      `Visitor: ${this.#manifest.visitorName} from ${this.#manifest.originReality} ` +
        `(${this.#manifest.arrivalSignature}).`,
    );
    lines.push(
      `Contraband risk: ${this.#manifest.contrabandRisk}. Turbulence: ${this.#manifest.turbulenceLevel}.`,
    );
    lines.push(
      `Sentimental inventory on file: ${
        this.#manifest.sentimentalInventory.length
          ? this.#manifest.sentimentalInventory.join("; ")
          : "none"
      }`,
    );
    lines.push(
      `Outstanding anomalies: ${
        this.#manifest.outstandingAnomalies.length
          ? this.#manifest.outstandingAnomalies.join("; ")
          : "none"
      }`,
    );
    if (this.#taggedItems.length > 0) {
      lines.push(`Traveler has logged: ${this.#taggedItems.join("; ")}.`);
    } else {
      lines.push(
        "No traveler-reported items logged yet; invite concise descriptions.",
      );
    }
    if (this.#droneDeployed) {
      lines.push(
        "Retrieval drone currently deployed; note its status when replying.",
      );
    }
    lines.push(`Current phase: ${this.#phase.toUpperCase()}.`);

    switch (this.#phase) {
      case "intake":
        if (!this.#passVerified) {
          lines.push(
            "Stabilise their arrival and prioritise verify_pass before promising retrieval.",
          );
        }
        break;
      case "recovery":
        lines.push(
          "Phase focus: coordinate retrieval. Summon a retrieval option or consult the prophet. Issue a quantum receipt when ready to hand off.",
        );
        break;
      case "handoff":
        lines.push(
          "Phase focus: wrap neatly. If receipt already issued, close_manifest and summarise remaining anomalies.",
        );
        break;
      case "closed":
        lines.push(
          "Manifest is archived. No tools remain; deliver a final tidy summary and dismiss traveler politely.",
        );
        break;
    }

    lines.push(
      "Tone: dry, organised, lightly amused. Reference protocol instead of improvising lore.",
    );
    lines.push(this.#manifest.courtesyNote);
    lines.push(
      "When tools are available, invoke exactly one relevant tool before finalising your answer. If no tools remain, simply summarise the closure.",
    );

    return lines.join("\n");
  }

  #buildTools(): AgentTool<RiftContext>[] {
    if (this.#phase === "closed") {
      return [];
    }

    const tools: AgentTool<RiftContext>[] = [];

    // Baseline tools remain available across phases; closures mutate session state where needed.
    tools.push(
      tool<RiftContext, { technique: string }>({
        name: "stabilize_rift",
        description:
          "Describe how you calm the rift turbulence and reassure the traveler.",
        parameters: {
          type: "object",
          properties: {
            technique: {
              type: "string",
              description:
                "Optional note about the stabilisation technique used.",
            },
          },
          required: ["technique"],
          additionalProperties: false,
        },
        execute: (args) => {
          const technique = args.technique?.trim() ?? "";
          console.log(
            `[tool] stabilize_rift invoked with technique=${technique}`,
          );
          const text =
            `I cycle the containment field to damp ${this.#manifest.turbulenceLevel} turbulence` +
            (technique.length ? ` using ${technique}` : "") +
            ".";
          return {
            content: [
              {
                type: "text",
                text,
              },
            ],
            is_error: false,
          };
        },
      }),
    );

    tools.push(
      tool<RiftContext, { item: string; timeline: string }>({
        name: "log_item",
        description:
          "Record a traveler-reported possession so recovery tools know what to fetch.",
        parameters: {
          type: "object",
          properties: {
            item: {
              type: "string",
              description: "Name of the missing item.",
            },
            timeline: {
              type: "string",
              description: "Optional timeline or reality tag for the item.",
            },
          },
          required: ["item", "timeline"],
          additionalProperties: false,
        },
        execute: (args) => {
          const timeline = args.timeline?.trim();
          const label = timeline ? `${args.item} (${timeline})` : args.item;
          this.#taggedItems.push(label);
          console.log(`[tool] log_item recorded ${label}`);
          return {
            content: [
              {
                type: "text",
                text: `Logged ${label} for retrieval queue. Current ledger: ${this.#taggedItems.join(
                  "; ",
                )}.`,
              },
            ],
            is_error: false,
          };
        },
      }),
    );

    if (!this.#passVerified) {
      // Toolkit keeps certain tools hidden until prerequisite state (verified pass) flips.
      tools.push(
        tool<RiftContext, { clearance_code: string }>({
          name: "verify_pass",
          description:
            "Validate the traveler's interdimensional pass to unlock recovery tools.",
          parameters: {
            type: "object",
            properties: {
              clearance_code: {
                type: "string",
                description: "Code supplied by the traveler for verification.",
              },
            },
            required: ["clearance_code"],
            additionalProperties: false,
          },
          execute: (args) => {
            this.#passVerified = true;
            this.#phase = "recovery";
            console.log(
              `[tool] verify_pass authenticated clearance_code=${args.clearance_code}`,
            );
            return {
              content: [
                {
                  type: "text",
                  text: `Pass authenticated with code ${args.clearance_code}. Recovery protocols online.`,
                },
              ],
              is_error: false,
            };
          },
        }),
      );
    }

    if (this.#phase === "recovery" && this.#passVerified) {
      tools.push(
        tool<RiftContext, { designation: string; target: string }>({
          name: "summon_retrieval_drone",
          description:
            "Dispatch a retrieval drone to recover a logged item from the rift queue.",
          parameters: {
            type: "object",
            properties: {
              designation: {
                type: "string",
                description:
                  "Optional drone designation to flavour the dispatch.",
              },
              target: {
                type: "string",
                description:
                  "Specific item to prioritise; defaults to the first logged item.",
              },
            },
            required: ["designation", "target"],
            additionalProperties: false,
          },
          execute: (args) => {
            this.#droneDeployed = true;
            const target = args.target?.trim().length
              ? args.target
              : (this.#taggedItems[0] ?? "the most recently logged item");
            const designation = args.designation?.trim().length
              ? args.designation
              : "Drone Theta";
            console.log(
              `[tool] summon_retrieval_drone dispatched designation=${designation} target=${target}`,
            );
            return {
              content: [
                {
                  type: "text",
                  text: `Dispatched ${designation} to retrieve ${target}.`,
                },
              ],
              is_error: false,
            };
          },
        }),
      );

      if (this.#prophecyCount === 0) {
        // Example of a single-use tool disappearing once invoked.
        tools.push(
          tool<RiftContext, { topic: string }>({
            name: "consult_prophet_agent",
            description:
              "Ping Prophet Sigma for probability guidance when the queue misbehaves.",
            parameters: {
              type: "object",
              properties: {
                topic: {
                  type: "string",
                  description: "Optional focus question for the prophet agent.",
                },
              },
              required: ["topic"],
              additionalProperties: false,
            },
            execute: (args) => {
              this.#prophecyCount += 1;
              const topic = args.topic?.trim();
              console.log(
                `[tool] consult_prophet_agent requested topic=${topic ?? "<none>"}`,
              );
              return {
                content: [
                  {
                    type: "text",
                    text: `Prophet Sigma notes anomaly priority: ${
                      this.#manifest.outstandingAnomalies[0] ??
                      "no immediate hazards"
                    }${topic ? ` while considering ${topic}.` : "."}`,
                  },
                ],
                is_error: false,
              };
            },
          }),
        );
      }

      if (this.#taggedItems.length > 0) {
        tools.push(
          tool<RiftContext, { recipient: string }>({
            name: "issue_quantum_receipt",
            description:
              "Generate a quantum receipt confirming which items are cleared for handoff.",
            parameters: {
              type: "object",
              properties: {
                recipient: {
                  type: "string",
                  description:
                    "Optional recipient line for the receipt header.",
                },
              },
              required: ["recipient"],
              additionalProperties: false,
            },
            execute: (args) => {
              this.#phase = "handoff";
              const recipient = args.recipient?.trim().length
                ? args.recipient
                : this.#manifest.visitorName;
              console.log(
                `[tool] issue_quantum_receipt issued to ${recipient} for items=${this.#taggedItems.join(", ")}`,
              );
              return {
                content: [
                  {
                    type: "text",
                    text: `Issued quantum receipt to ${recipient} for ${this.#taggedItems.join(
                      "; ",
                    )}. Handoff phase engaged.`,
                  },
                ],
                is_error: false,
              };
            },
          }),
        );
      }
    }

    if (this.#phase === "handoff") {
      // Once handoff begins, offer a closure tool that transitions to the final state.
      tools.push(
        tool<RiftContext, Record<string, never>>({
          name: "close_manifest",
          description:
            "Archive the case once items are delivered and note any lingering anomalies.",
          parameters: {
            type: "object",
            properties: {},
            required: [],
            additionalProperties: false,
          },
          execute: () => {
            this.#phase = "closed";
            console.log(
              `[tool] close_manifest archived manifest with anomalies=${this.#manifest.outstandingAnomalies.length}`,
            );
            return {
              content: [
                {
                  type: "text",
                  text: `Archived manifest with ${this.#manifest.outstandingAnomalies.length} anomaly reminder(s) for facilities.`,
                },
              ],
              is_error: false,
            };
          },
        }),
      );
    }

    return tools;
  }
}

// Toolkit wires the async manifest fetch into createSession and returns the stateful session.
class LostAndFoundToolkit implements Toolkit<RiftContext> {
  async createSession(
    context: RiftContext,
  ): Promise<ToolkitSession<RiftContext>> {
    const manifest = await fetchRiftManifest(context.visitorId);
    return new LostAndFoundToolkitSession(manifest);
  }
}

// Static tool supplied directly on the agent to illustrate coexistence with toolkit-provided tools.
const pageSecurityTool = tool<RiftContext, { reason: string }>({
  name: "page_security",
  description: "Escalate to security if contraband risk becomes unmanageable.",
  parameters: {
    type: "object",
    properties: {
      reason: {
        type: "string",
        description: "Why security needs to step in.",
      },
    },
    required: ["reason"],
    additionalProperties: false,
  },
  execute: (args, ctx) => ({
    content: [
      {
        type: "text",
        text: `Security paged for ${ctx.visitorId}: ${args.reason}.`,
      },
    ],
    is_error: false,
  }),
});

// Base agent instructions still resolve separately; toolkit prompt stacks on top each turn.
const instructions: InstructionParam<RiftContext>[] = [
  "You are the archivist at Waypoint Seven's Interdimensional Lost & Found desk.",
  "Keep responses under 120 words when possible and stay bone-dry with humour.",
  ({ visitorId }) =>
    `Reference the visitor's manifest details supplied by the toolkit for ${visitorId}. Do not invent new lore.`,
  "When tools remain, call exactly one per turn before concluding. If tools run out, summarise the closure instead.",
];

// Traditional Agent setup still works: we wire static tools, instructions, and our custom toolkit together.
const archivist = new Agent<RiftContext>({
  name: "WaypointArchivist",
  instructions,
  model: getModel("openai", "gpt-4o-mini"),
  tools: [pageSecurityTool],
  toolkits: [new LostAndFoundToolkit()],
});

async function runDemo() {
  // Reuse a RunSession so ToolkitSession state persists across multiple turns.
  const session = await archivist.createSession({ visitorId: "aurora-shift" });
  const transcript: AgentItem[] = [];

  const prompts = [
    "I just slipped through the rift and my belongings are glittering in the wrong timeline. What now?",
    "The Chrono Locket from Timeline 12 is missing, and the echo lag is getting worse.",
    "The locket links to my sister's echo; anything else before I depart?",
  ];

  for (const [index, prompt] of prompts.entries()) {
    // Accumulate the conversation transcript so each turn knows the prior history.
    transcript.push({
      type: "message",
      role: "user",
      content: [{ type: "text", text: prompt }],
    });

    // Each invocation reuses the toolkit session, so newly unlocked tools remain available.
    const response = await session.run({ input: transcript });

    console.log(`\n=== TURN ${index + 1} ===`);
    console.log(getResponseText(response));

    // Feed model/tool outputs back into the transcript for the next turn.
    transcript.push(...response.output);
  }

  await session.close();
}

await runDemo();
