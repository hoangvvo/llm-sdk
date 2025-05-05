use async_trait::async_trait;
use dotenvy::dotenv;
use llm_agent::{Agent, AgentRequest, AgentTool, AgentToolResult};
use llm_sdk::{
    openai::{OpenAIModel, OpenAIModelOptions},
    Message, Part,
};
use serde::Deserialize;
use serde_json::Value;
use std::{
    collections::{HashMap, HashSet},
    env,
    error::Error,
    sync::{Arc, Mutex},
};

#[derive(Clone)]
struct EnemyState {
    hp: i32,
}

#[derive(Clone)]
struct EncounterState {
    scene: String,
    enemies: Arc<Mutex<HashMap<String, EnemyState>>>,
    downed_allies: Arc<Mutex<HashSet<String>>>,
}

#[derive(Clone)]
struct DungeonRunContext {
    dungeon_master: String,
    party_name: String,
    encounter: EncounterState,
    action_budget: Arc<Mutex<HashMap<String, i32>>>,
}

fn create_dungeon_context() -> DungeonRunContext {
    let enemies = HashMap::from([
        ("ghoul".to_string(), EnemyState { hp: 12 }),
        ("marauder".to_string(), EnemyState { hp: 9 }),
    ]);
    let downed = HashSet::from(["finley".to_string()]);
    let action_budget = HashMap::from([("thorne".to_string(), 1), ("mira".to_string(), 2)]);

    DungeonRunContext {
        dungeon_master: "Rowan".into(),
        party_name: "Lanternbearers".into(),
        encounter: EncounterState {
            scene: "The Echo Bridge over the Sunken Keep".into(),
            enemies: Arc::new(Mutex::new(enemies)),
            downed_allies: Arc::new(Mutex::new(downed)),
        },
        action_budget: Arc::new(Mutex::new(action_budget)),
    }
}

#[derive(Deserialize)]
struct AttackEnemyParams {
    attacker: String,
    target: String,
    weapon: String,
}

struct AttackEnemyTool;

#[async_trait]
impl AgentTool<DungeonRunContext> for AttackEnemyTool {
    fn name(&self) -> String {
        "attack_enemy".to_string()
    }
    fn description(&self) -> String {
        "Resolve a martial attack from a party member against an active enemy and update its hit \
         points."
            .to_string()
    }
    fn parameters(&self) -> llm_sdk::JSONSchema {
        serde_json::json!({
            "type": "object",
            "properties": {
                "attacker": {
                    "type": "string",
                    "description": "Name of the party member making the attack."
                },
                "target": {
                    "type": "string",
                    "description": "Enemy to strike."
                },
                "weapon": {
                    "type": "string",
                    "description": "Weapon or maneuver used for flavour and damage bias."
                }
            },
            "required": ["attacker", "target", "weapon"],
            "additionalProperties": false
        })
    }
    async fn execute(
        &self,
        args: Value,
        context: &DungeonRunContext,
        _state: &llm_agent::RunState,
    ) -> Result<AgentToolResult, Box<dyn Error + Send + Sync>> {
        let params: AttackEnemyParams = serde_json::from_value(args)?;

        let attacker_key = params.attacker.trim().to_lowercase();
        let target_key = params.target.trim().to_lowercase();

        let mut action_budget = context
            .action_budget
            .lock()
            .expect("action budget mutex poisoned");
        let remaining_actions = *action_budget.get(&attacker_key).unwrap_or(&0);

        if remaining_actions <= 0 {
            return Ok(AgentToolResult {
                content: vec![Part::text(format!(
                    "{} is out of actions this round. Ask another hero to step in or advance the \
                     scene.",
                    params.attacker
                ))],
                is_error: true,
            });
        }

        let mut encounter_enemies = context
            .encounter
            .enemies
            .lock()
            .expect("encounter enemies mutex poisoned");

        let enemy = encounter_enemies.get_mut(&target_key);
        let Some(enemy_state) = enemy else {
            return Ok(AgentToolResult {
                content: vec![Part::text(format!(
                    "No enemy named {} remains at {}. Double-check the initiative order.",
                    params.target, context.encounter.scene
                ))],
                is_error: true,
            });
        };

        let weapon_lower = params.weapon.to_lowercase();
        let base_damage = if weapon_lower.contains("axe") { 7 } else { 5 };
        let finesse_bonus = i32::from(weapon_lower.contains("dagger"));
        let computed_damage = base_damage + (params.attacker.len() % 3) as i32 + finesse_bonus;

        enemy_state.hp = (enemy_state.hp - computed_damage).max(0);
        action_budget.insert(attacker_key, remaining_actions - 1);

        let defeated = if enemy_state.hp == 0 {
            format!(" {} is defeated!", params.target)
        } else {
            String::new()
        };

        Ok(AgentToolResult {
            content: vec![Part::text(format!(
                "{} hits {} for {} damage with the {}. {} now has {} HP.{}",
                params.attacker,
                params.target,
                computed_damage,
                params.weapon,
                params.target,
                enemy_state.hp,
                defeated
            ))],
            is_error: false,
        })
    }
}

#[derive(Deserialize)]
struct StabilizeAllyParams {
    hero: String,
}

struct StabilizeAllyTool;

#[async_trait]
impl AgentTool<DungeonRunContext> for StabilizeAllyTool {
    fn name(&self) -> String {
        "stabilize_ally".to_string()
    }
    fn description(&self) -> String {
        "Spend the round stabilising a downed ally. Removes them from the downed list if available."
            .to_string()
    }
    fn parameters(&self) -> llm_sdk::JSONSchema {
        serde_json::json!({
            "type": "object",
            "properties": {
                "hero": {
                    "type": "string",
                    "description": "Name of the ally to stabilise."
                }
            },
            "required": ["hero"],
            "additionalProperties": false
        })
    }
    async fn execute(
        &self,
        args: Value,
        context: &DungeonRunContext,
        _state: &llm_agent::RunState,
    ) -> Result<AgentToolResult, Box<dyn Error + Send + Sync>> {
        let params: StabilizeAllyParams = serde_json::from_value(args)?;

        let hero_key = params.hero.trim().to_lowercase();
        let mut downed = context
            .encounter
            .downed_allies
            .lock()
            .expect("downed allies mutex poisoned");

        if !downed.remove(&hero_key) {
            return Ok(AgentToolResult {
                content: vec![Part::text(format!(
                    "{} is already on their feet. Consider taking another tactical action instead \
                     of stabilising.",
                    params.hero
                ))],
                is_error: true,
            });
        }

        Ok(AgentToolResult {
            content: vec![Part::text(format!(
                "{} is stabilised and ready to rejoin when the next round begins.",
                params.hero
            ))],
            is_error: false,
        })
    }
}

fn describe_enemies(context: &DungeonRunContext) -> HashMap<String, i32> {
    context
        .encounter
        .enemies
        .lock()
        .expect("encounter enemies mutex poisoned")
        .iter()
        .map(|(name, state)| (name.clone(), state.hp))
        .collect()
}

fn describe_downed_allies(context: &DungeonRunContext) -> Vec<String> {
    context
        .encounter
        .downed_allies
        .lock()
        .expect("downed allies mutex poisoned")
        .iter()
        .cloned()
        .collect()
}

#[tokio::main]
async fn main() -> Result<(), Box<dyn Error>> {
    dotenv().ok();

    let model = Arc::new(OpenAIModel::new(
        "gpt-4o",
        OpenAIModelOptions {
            api_key: env::var("OPENAI_API_KEY")
                .expect("OPENAI_API_KEY environment variable must be set"),
            ..Default::default()
        },
    ));

    let dungeon_coach = Agent::<DungeonRunContext>::builder("Torch", model)
        .add_instruction(
            "You are Torch, a steady co-Dungeon Master. Keep answers short and, when combat \
             actions come up, lean on the provided tools to resolve them.",
        )
        .add_instruction(
            "If a requested action involves striking an enemy, call attack_enemy. If the party \
             wants to help someone back up, call stabilize_ally before answering.",
        )
        .add_tool(AttackEnemyTool)
        .add_tool(StabilizeAllyTool)
        .build();

    let success_context = create_dungeon_context();
    let success_response = dungeon_coach
        .run(AgentRequest {
            context: success_context.clone(),
            input: vec![llm_agent::AgentItem::Message(Message::user(vec![
                Part::text(
                    "Thorne will strike the ghoul with a battleaxe while Mira uses her turn to \
                     stabilise Finley. Help me resolve it.",
                ),
            ]))],
        })
        .await?;

    println!("Success response:\n{success_response:#?}");
    println!("{}", success_response.text());
    println!(
        "Remaining enemy HP: {:?}",
        describe_enemies(&success_context)
    );
    println!(
        "Downed allies after success run: {:?}",
        describe_downed_allies(&success_context)
    );

    let failure_context = create_dungeon_context();
    {
        let mut budget = failure_context
            .action_budget
            .lock()
            .expect("action budget mutex poisoned");
        budget.insert("thorne".into(), 0);
    }
    {
        let mut downed = failure_context
            .encounter
            .downed_allies
            .lock()
            .expect("downed allies mutex poisoned");
        downed.clear();
    }

    let failure_response = dungeon_coach
        .run(AgentRequest {
            context: failure_context,
            input: vec![llm_agent::AgentItem::Message(Message::user(vec![
                Part::text(
                    "Thorne wants to swing again at the marauder, and Mira tries to stabilise \
                     Finley anyway.",
                ),
            ]))],
        })
        .await?;

    println!("Failure response:\n{failure_response:#?}");

    Ok(())
}
