use chrono::Utc;
use llm_agent::{Agent, AgentTool};
use llm_sdk::{AudioOptions, LanguageModel, Modality, ReasoningOptions};
use std::sync::Arc;

use crate::{
    artifacts_tools::{
        ArtifactCreateTool, ArtifactDeleteTool, ArtifactGetTool, ArtifactListTool,
        ArtifactUpdateTool,
    },
    context::MyContext,
    finance_tools::{GetCryptoPriceTool, GetStockPriceTool},
    get_model::ModelInfo,
    information_tools::{GetNewsTool, SearchWikipediaTool},
    weather_tools::{GetCoordinatesTool, GetWeatherTool},
};

#[derive(Clone)]
pub struct AgentOptions {
    pub enabled_tools: Option<Vec<String>>,
    pub disabled_instructions: bool,
    pub temperature: Option<f64>,
    pub top_p: Option<f64>,
    pub top_k: Option<i32>,
    pub frequency_penalty: Option<f64>,
    pub presence_penalty: Option<f64>,
    pub audio: Option<AudioOptions>,
    pub reasoning: Option<ReasoningOptions>,
    pub modalities: Option<Vec<Modality>>,
}

pub fn get_available_tools() -> Vec<Box<dyn AgentTool<MyContext> + Send + Sync>> {
    vec![
        Box::new(ArtifactCreateTool),
        Box::new(ArtifactUpdateTool),
        Box::new(ArtifactGetTool),
        Box::new(ArtifactListTool),
        Box::new(ArtifactDeleteTool),
        Box::new(GetStockPriceTool),
        Box::new(GetCryptoPriceTool),
        Box::new(SearchWikipediaTool),
        Box::new(GetNewsTool),
        Box::new(GetCoordinatesTool),
        Box::new(GetWeatherTool),
    ]
}

pub fn create_agent(
    model: Arc<dyn LanguageModel + Send + Sync>,
    _model_info: &ModelInfo,
    options: &AgentOptions,
) -> Agent<MyContext> {
    let mut builder = llm_agent::AgentParams::new("MyAgent", model);

    if !options.disabled_instructions {
        builder = builder
            .add_instruction(
                "Answer in markdown format.\\nTo access certain tools, the user may have to \
                 provide corresponding API keys in the context fields on the UI.",
            )
            .add_instruction(|context: &MyContext| {
                let name = context.name.as_deref().unwrap_or("<not provided>");
                let location = context.location.as_deref().unwrap_or("<not provided>");
                let language = context.language.as_deref().unwrap_or("<not provided>");
                Ok(format!(
                    "The user name is {name}.\\nThe user location is {location}.\\nThe user \
                     speaks {language} language."
                ))
            })
            .add_instruction(|_context: &MyContext| {
                Ok(format!(
                    "The current date is {}.",
                    Utc::now().format("%a %b %d %Y")
                ))
            })
            .add_instruction(
                "For substantive deliverables (documents/specs/code), use the artifact tools (artifact_create, artifact_update, artifact_get, artifact_list, artifact_delete).\\nKeep chat replies brief and put the full document content into artifacts via these tools, rather than pasting large content into chat. Reference documents by their id.",
            );
    }

    // Add tools based on enabled_tools filter
    let enabled_tools = options.enabled_tools.as_ref();

    if enabled_tools.is_none()
        || enabled_tools
            .unwrap()
            .contains(&"get_stock_price".to_string())
    {
        builder = builder.add_tool(GetStockPriceTool);
    }
    if enabled_tools.is_none()
        || enabled_tools
            .unwrap()
            .contains(&"get_crypto_price".to_string())
    {
        builder = builder.add_tool(GetCryptoPriceTool);
    }
    if enabled_tools.is_none()
        || enabled_tools
            .unwrap()
            .contains(&"search_wikipedia".to_string())
    {
        builder = builder.add_tool(SearchWikipediaTool);
    }
    if enabled_tools.is_none() || enabled_tools.unwrap().contains(&"get_news".to_string()) {
        builder = builder.add_tool(GetNewsTool);
    }
    if enabled_tools.is_none()
        || enabled_tools
            .unwrap()
            .contains(&"get_coordinates".to_string())
    {
        builder = builder.add_tool(GetCoordinatesTool);
    }
    if enabled_tools.is_none() || enabled_tools.unwrap().contains(&"get_weather".to_string()) {
        builder = builder.add_tool(GetWeatherTool);
    }
    if enabled_tools.is_none()
        || enabled_tools
            .unwrap()
            .contains(&"artifact_create".to_string())
    {
        builder = builder.add_tool(ArtifactCreateTool);
    }
    if enabled_tools.is_none()
        || enabled_tools
            .unwrap()
            .contains(&"artifact_update".to_string())
    {
        builder = builder.add_tool(ArtifactUpdateTool);
    }
    if enabled_tools.is_none() || enabled_tools.unwrap().contains(&"artifact_get".to_string()) {
        builder = builder.add_tool(ArtifactGetTool);
    }
    if enabled_tools.is_none()
        || enabled_tools
            .unwrap()
            .contains(&"artifact_list".to_string())
    {
        builder = builder.add_tool(ArtifactListTool);
    }
    if enabled_tools.is_none()
        || enabled_tools
            .unwrap()
            .contains(&"artifact_delete".to_string())
    {
        builder = builder.add_tool(ArtifactDeleteTool);
    }

    builder = builder.max_turns(5);

    if let Some(temperature) = options.temperature {
        builder = builder.temperature(temperature);
    }
    if let Some(top_p) = options.top_p {
        builder = builder.top_p(top_p);
    }
    if let Some(top_k) = options.top_k {
        builder = builder.top_k(top_k);
    }
    if let Some(frequency_penalty) = options.frequency_penalty {
        builder = builder.frequency_penalty(frequency_penalty);
    }
    if let Some(presence_penalty) = options.presence_penalty {
        builder = builder.presence_penalty(presence_penalty);
    }

    if let Some(audio) = &options.audio {
        builder = builder.audio(audio.clone());
    }

    if let Some(reasoning) = &options.reasoning {
        builder = builder.reasoning(reasoning.clone());
    }

    if let Some(modalities) = &options.modalities {
        builder = builder.modalities(modalities.clone());
    }

    builder.build()
}
