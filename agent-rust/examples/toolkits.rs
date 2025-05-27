use std::{env, sync::Arc, time::Duration};

use async_trait::async_trait;
use dotenvy::dotenv;
use llm_agent::{
    Agent, AgentParams, AgentRequest, AgentTool, AgentToolResult, Toolkit, ToolkitSession,
};
use llm_sdk::{
    openai::{OpenAIModel, OpenAIModelOptions},
    Message, Part,
};
use serde_json::Value;
use tokio::time::sleep;

type MatchId = &'static str;

#[derive(Clone, Copy, PartialEq, Eq)]
enum Weather {
    HarshSunlight,
    None,
}

#[derive(Clone, Copy)]
enum MoveKind {
    Flamethrower,
    SolarBeam,
    AirSlash,
    ShadowBall,
    Hypnosis,
    DreamEater,
    Nightmare,
}

#[derive(Clone)]
struct PokemonState {
    name: &'static str,
    ability: &'static str,
    item: Option<&'static str>,
    moves: &'static [MoveKind],
}

#[derive(Clone)]
struct OpponentState {
    name: &'static str,
    typing: &'static [&'static str],
    status: &'static str,
    hint: &'static str,
}

#[derive(Clone)]
struct BattleState {
    weather: Weather,
    arena: &'static str,
    crowd_note: &'static str,
    pokemon: PokemonState,
    opponent: OpponentState,
}

#[derive(Clone)]
struct BattleContext {
    match_id: MatchId,
}

struct MoveTool {
    kind: MoveKind,
    state: Arc<BattleState>,
}

#[async_trait]
impl AgentTool<BattleContext> for MoveTool {
    fn name(&self) -> String {
        self.kind.name().to_string()
    }

    fn description(&self) -> String {
        self.kind.description().to_string()
    }

    fn parameters(&self) -> llm_sdk::JSONSchema {
        serde_json::json!({
            "type": "object",
            "properties": {
                "target": {
                    "type": "string",
                    "description": "Optional target override; defaults to the opposing Pokémon."
                }
            },
            "required": [],
            "additionalProperties": false
        })
    }

    async fn execute(
        &self,
        args: Value,
        _context: &BattleContext,
        _run_state: &llm_agent::RunState,
    ) -> Result<AgentToolResult, Box<dyn std::error::Error + Send + Sync>> {
        let target = args
            .get("target")
            .and_then(Value::as_str)
            .unwrap_or(self.state.opponent.name);

        Ok(AgentToolResult {
            content: vec![Part::text(self.kind.execute(&self.state, target))],
            is_error: false,
        })
    }
}

struct UseItemTool;
struct AttemptEscapeTool;

#[async_trait]
impl AgentTool<BattleContext> for UseItemTool {
    fn name(&self) -> String {
        "use_item".into()
    }

    fn description(&self) -> String {
        "Use a held or bag item to swing the battle.".into()
    }

    fn parameters(&self) -> llm_sdk::JSONSchema {
        serde_json::json!({
            "type": "object",
            "properties": {
                "item": { "type": "string", "description": "Name of the item to use." }
            },
            "required": ["item"],
            "additionalProperties": false
        })
    }

    async fn execute(
        &self,
        args: Value,
        _context: &BattleContext,
        _run_state: &llm_agent::RunState,
    ) -> Result<AgentToolResult, Box<dyn std::error::Error + Send + Sync>> {
        let item = args
            .get("item")
            .and_then(Value::as_str)
            .ok_or_else(|| "item is required")?;
        Ok(AgentToolResult {
            content: vec![Part::text(format!("I use the {item} to shift momentum."))],
            is_error: false,
        })
    }
}

#[async_trait]
impl AgentTool<BattleContext> for AttemptEscapeTool {
    fn name(&self) -> String {
        "attempt_escape".into()
    }

    fn description(&self) -> String {
        "Attempt to flee if the battle is unwinnable.".into()
    }

    fn parameters(&self) -> llm_sdk::JSONSchema {
        serde_json::json!({
            "type": "object",
            "properties": {},
            "required": [],
            "additionalProperties": false
        })
    }

    async fn execute(
        &self,
        _args: Value,
        _context: &BattleContext,
        _run_state: &llm_agent::RunState,
    ) -> Result<AgentToolResult, Box<dyn std::error::Error + Send + Sync>> {
        Ok(AgentToolResult {
            content: vec![Part::text(
                "I search for an opening to retreat from the field.",
            )],
            is_error: false,
        })
    }
}

/// ToolkitSession caching the prompt and tools so the agent sees synchronous getters backed by
/// the async work performed during create_session.
struct BattleToolkitSession {
    prompt: String,
    tools: Vec<Arc<dyn AgentTool<BattleContext>>>,
}

#[async_trait]
impl ToolkitSession<BattleContext> for BattleToolkitSession {
    fn system_prompt(&self) -> Option<String> {
        Some(self.prompt.clone())
    }

    fn tools(&self) -> Vec<Arc<dyn AgentTool<BattleContext>>> {
        self.tools.clone()
    }

    async fn close(self: Box<Self>) -> Result<(), Box<dyn std::error::Error + Send + Sync>> {
        Ok(())
    }
}

/// Toolkit that performs async lookups in create_session to enable dynamic guidance/capabilities
/// per match while still returning a synchronous ToolkitSession.
struct BattleToolkit;

#[async_trait]
impl Toolkit<BattleContext> for BattleToolkit {
    async fn create_session(
        &self,
        context: &BattleContext,
    ) -> Result<
        Box<dyn ToolkitSession<BattleContext> + Send + Sync>,
        Box<dyn std::error::Error + Send + Sync>,
    > {
        let state = Arc::new(load_battle_state(context.match_id).await?);
        // create_session performs async fetches so the returned session can expose synchronous
        // prompt/tool views while still adapting to each battle.
        let session = BattleToolkitSession {
            prompt: build_prompt(&state),
            tools: build_move_tools(&state),
        };
        Ok(Box::new(session))
    }
}

async fn load_battle_state(
    match_id: MatchId,
) -> Result<BattleState, Box<dyn std::error::Error + Send + Sync>> {
    let state = match match_id {
        "sun-showdown" => BattleState {
            weather: Weather::HarshSunlight,
            arena: "Pyrite Crater",
            crowd_note: "Crowd roars when attacks lean into the blazing sun.",
            pokemon: PokemonState {
                name: "Charizard",
                ability: "Solar Power",
                item: Some("Choice Specs"),
                moves: &[
                    MoveKind::Flamethrower,
                    MoveKind::SolarBeam,
                    MoveKind::AirSlash,
                ],
            },
            opponent: OpponentState {
                name: "Ferrothorn",
                typing: &["Grass", "Steel"],
                status: "healthy",
                hint: "likes to turtle behind Leech Seed and Iron Defense.",
            },
        },
        "dream-dusk" => BattleState {
            weather: Weather::None,
            arena: "Midnight Colosseum",
            crowd_note: "Spectators fall silent for sinister dream tactics.",
            pokemon: PokemonState {
                name: "Haunter",
                ability: "Levitate",
                item: Some("Wide Lens"),
                moves: &[
                    MoveKind::ShadowBall,
                    MoveKind::Hypnosis,
                    MoveKind::DreamEater,
                    MoveKind::Nightmare,
                ],
            },
            opponent: OpponentState {
                name: "Gardevoir",
                typing: &["Psychic", "Fairy"],
                status: "asleep",
                hint: "was stacking Calm Mind boosts before dozing off.",
            },
        },
        _ => return Err(format!("unknown match {match_id}").into()),
    };
    sleep(Duration::from_millis(25)).await;
    Ok(state)
}

fn build_move_tools(state: &BattleState) -> Vec<Arc<dyn AgentTool<BattleContext>>> {
    state
        .pokemon
        .moves
        .iter()
        .filter(|kind| kind.available(state))
        .map(|kind| {
            Arc::new(MoveTool {
                kind: *kind,
                state: Arc::new(state.clone()),
            }) as Arc<dyn AgentTool<BattleContext>>
        })
        .collect()
}

fn build_prompt(state: &BattleState) -> String {
    let mut parts = vec![format!(
        "You are {} battling in {}.",
        state.pokemon.name, state.arena
    )];
    match state.weather {
        Weather::HarshSunlight => parts.push(
            "Harsh Sunlight supercharges Fire attacks, lets Solar Beam skip charging, and weakens Water coverage.".into(),
        ),
        Weather::None => parts.push("There is no active weather effect.".into()),
    }
    if let Some(item) = state.pokemon.item {
        parts.push(format!(
            "Ability: {}, holding {}.",
            state.pokemon.ability, item
        ));
    } else {
        parts.push(format!("Ability: {}.", state.pokemon.ability));
    }
    parts.push(format!(
        "Opponent: {} ({}), currently {}. {}",
        state.opponent.name,
        state.opponent.typing.join("/"),
        state.opponent.status,
        state.opponent.hint
    ));
    parts.push(format!("Crowd note: {}", state.crowd_note));
    parts.push(
        "Call one available move tool (or use_item / attempt_escape) before finalising and explain why the field makes it sensible.".into(),
    );
    parts.join(" ")
}

impl MoveKind {
    fn name(&self) -> &'static str {
        match self {
            MoveKind::Flamethrower => "flamethrower",
            MoveKind::SolarBeam => "solar_beam",
            MoveKind::AirSlash => "air_slash",
            MoveKind::ShadowBall => "shadow_ball",
            MoveKind::Hypnosis => "hypnosis",
            MoveKind::DreamEater => "dream_eater",
            MoveKind::Nightmare => "nightmare",
        }
    }

    fn description(&self) -> &'static str {
        match self {
            MoveKind::Flamethrower => {
                "Flamethrower is a dependable Fire-type strike that thrives in Harsh Sunlight."
            }
            MoveKind::SolarBeam => {
                "Solar Beam normally charges, but fires instantly in Harsh Sunlight."
            }
            MoveKind::AirSlash => {
                "Air Slash provides Flying coverage with a flinch chance against slower foes."
            }
            MoveKind::ShadowBall => {
                "Shadow Ball is Haunter's safest Ghost attack versus Psychic targets."
            }
            MoveKind::Hypnosis => "Hypnosis can return the opponent to sleep if they stir.",
            MoveKind::DreamEater => {
                "Dream Eater only works while the opponent sleeps, draining them and healing me."
            }
            MoveKind::Nightmare => {
                "Nightmare curses a sleeping foe to lose HP at the end of each turn."
            }
        }
    }

    fn available(&self, state: &BattleState) -> bool {
        match self {
            MoveKind::SolarBeam => matches!(state.weather, Weather::HarshSunlight),
            MoveKind::DreamEater | MoveKind::Nightmare => state.opponent.status == "asleep",
            _ => true,
        }
    }

    fn execute(&self, state: &BattleState, target: &str) -> String {
        match self {
            MoveKind::Flamethrower => {
                let bonus = if matches!(state.weather, Weather::HarshSunlight) {
                    ", the sunlight turning the flames white-hot"
                } else {
                    ""
                };
                format!("I scorch {target} with Flamethrower{bonus}.")
            }
            MoveKind::SolarBeam => format!(
                "I gather sunlight and unleash Solar Beam on {target} without needing to charge."
            ),
            MoveKind::AirSlash => format!(
                "I ride the thermals around {} and carve {target} with Air Slash.",
                state.arena
            ),
            MoveKind::ShadowBall => format!(
                "I hurl Shadow Ball at {target}, disrupting their {} defenses.",
                state.opponent.typing.join("/")
            ),
            MoveKind::Hypnosis => {
                format!("I sway and cast Hypnosis toward {target}, setting up dream tactics.")
            }
            MoveKind::DreamEater => format!(
                "I feast on {target}'s dreams, restoring power to {}.",
                state.pokemon.name
            ),
            MoveKind::Nightmare => format!(
                "I lace {target}'s dreams with Nightmare so they suffer each turn while asleep."
            ),
        }
    }
}

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error + Send + Sync>> {
    dotenv().ok();
    let api_key = env::var("OPENAI_API_KEY")?;
    let model = Arc::new(OpenAIModel::new(
        "gpt-4o",
        OpenAIModelOptions {
            api_key,
            ..Default::default()
        },
    ));

    let agent = Agent::new(
        AgentParams::new("Satoshi", model)
            .add_instruction("Speak in first person as the active Pokémon.".to_string())
            .add_instruction(
                "Always invoke exactly one tool before ending your answer, and mention how the field conditions justify it.".to_string(),
            )
            .add_tool(UseItemTool)
            .add_tool(AttemptEscapeTool)
            .add_toolkit(BattleToolkit),
    );

    run_example(
        &agent,
        "sun-showdown",
        "Ferrothorn is hiding behind Iron Defense again—what's our play?",
    )
    .await?;
    run_example(
        &agent,
        "dream-dusk",
        "Gardevoir is still asleep—press the advantage before it wakes up!",
    )
    .await?;

    Ok(())
}

async fn run_example(
    agent: &Agent<BattleContext>,
    match_id: MatchId,
    prompt: &str,
) -> Result<(), Box<dyn std::error::Error + Send + Sync>> {
    println!("\n=== {match_id} ===");
    let response = agent
        .run(AgentRequest {
            context: BattleContext { match_id },
            input: vec![llm_agent::AgentItem::Message(Message::user(vec![
                Part::text(prompt),
            ]))],
        })
        .await?;
    println!("{}", response.text());
    Ok(())
}
