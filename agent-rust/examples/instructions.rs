use dotenvy::dotenv;
use llm_agent::{Agent, AgentRequest, InstructionParam};
use llm_sdk::{
    openai::{OpenAIModel, OpenAIModelOptions},
    Message, Part,
};
use std::{env, error::Error, sync::Arc};
use tokio::time::{sleep, Duration};

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
            "You are Torch, a supportive guide who keeps tabletop role-playing sessions moving. Offer concrete options instead of long monologues.",
        )
        .add_instruction(|ctx: &DungeonRunContext| {
            Ok(format!(
                "You are helping {}, the Dungeon Master for the {}. They are running the quest \"{}\" and need a quick nudge that favors the party's {}.",
                ctx.dungeon_master, ctx.party_name, ctx.current_quest, ctx.highlight_player_class
            ))
        })
        .add_instruction(InstructionParam::AsyncFunc(Box::new(|ctx: &DungeonRunContext| {
            let hint = ctx.oracle_hint.clone();
            Box::pin(async move {
                sleep(Duration::from_millis(25)).await;
                Ok(format!("Weave in the oracle whisper: \"{}\" so it feels like an in-world hint.", hint))
            })
        })))
        .build();

    let context = DungeonRunContext {
        dungeon_master: "Rowan".into(),
        party_name: "Lanternbearers".into(),
        current_quest: "Echoes of the Sunken Keep".into(),
        highlight_player_class: "ranger".into(),
        oracle_hint: "the moss remembers every secret step".into(),
    };

    let response = dungeon_coach
        .run(AgentRequest {
            context,
            input: vec![llm_agent::AgentItem::Message(Message::user(vec![
                Part::text("The party is stuck at a collapsed bridge. What should happen next?"),
            ]))],
        })
        .await?;

    println!("{}", response.text());

    Ok(())
}
