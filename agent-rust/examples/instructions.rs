use dotenvy::dotenv;
use llm_agent::{Agent, AgentRequest, InstructionParam, RunOptions};
use llm_sdk::{Message, Part};
use std::error::Error;
use tokio::time::{sleep, Duration};

mod common;

#[derive(Clone)]
struct DungeonRunContext {
    dungeon_master: String,
    party_name: String,
    current_quest: String,
    highlight_player_class: String,
    oracle_hint: String,
}

#[tokio::main]
async fn main() -> Result<(), Box<dyn Error>> {
    dotenv().ok();

    let provider = std::env::var("PROVIDER").unwrap_or_else(|_| "openai".to_string());
    let model_id = std::env::var("MODEL").unwrap_or_else(|_| "gpt-5.6-terra".to_string());
    let model = common::get_model(
        &provider,
        &model_id,
        llm_sdk::LanguageModelMetadata::default(),
        None,
    )
    .expect("failed to create model");

    let dungeon_coach = Agent::<DungeonRunContext>::builder("Torch", model)
        .add_instruction(
            "You are Torch, a supportive guide who keeps tabletop role-playing sessions moving. \
             Offer concrete options instead of long monologues.",
        )
        .add_instruction(|ctx: &DungeonRunContext| {
            Ok(format!(
                "You are helping {}, the Dungeon Master for the {}. They are running the quest \
                 \"{}\" and need a quick nudge that favors the party's {}.",
                ctx.dungeon_master, ctx.party_name, ctx.current_quest, ctx.highlight_player_class
            ))
        })
        .add_instruction(InstructionParam::AsyncFunc(Box::new(
            |ctx: &DungeonRunContext| {
                let hint = ctx.oracle_hint.clone();
                Box::pin(async move {
                    sleep(Duration::from_millis(25)).await;
                    Ok(format!(
                        "Weave in the oracle whisper: \"{hint}\" so it feels like an in-world \
                         hint."
                    ))
                })
            },
        )))
        .build();

    let context = DungeonRunContext {
        dungeon_master: "Rowan".into(),
        party_name: "Lanternbearers".into(),
        current_quest: "Echoes of the Sunken Keep".into(),
        highlight_player_class: "ranger".into(),
        oracle_hint: "the moss remembers every secret step".into(),
    };

    let response = dungeon_coach
        .run(
            AgentRequest {
                context,
                input: vec![llm_agent::AgentItem::Message(Message::user(vec![
                    Part::text(
                        "The party is stuck at a collapsed bridge. What should happen next?",
                    ),
                ]))],
            },
            RunOptions::default(),
        )
        .await?;

    println!("{}", response.text());

    Ok(())
}
