mod common;
use futures::stream::StreamExt;
use llm_sdk::{openai::*, StreamAccumulator, *};
use std::{env, error::Error, sync::LazyLock};
use tokio::test;

static OPENAI_MODEL: LazyLock<OpenAIChatModel> = LazyLock::new(|| {
    dotenvy::dotenv().ok();

    OpenAIChatModel::new(OpenAIModelOptions {
        model_id: "gpt-4o".to_string(),
        api_key: env::var("OPENAI_API_KEY")
            .expect("OPENAI_API_KEY must be set")
            .to_string(),
        ..Default::default()
    })
    .with_metadata(LanguageModelMetadata {
        capabilities: Some(vec![
            LanguageModelCapability::FunctionCalling,
            LanguageModelCapability::ImageInput,
            LanguageModelCapability::StructuredOutput,
        ]),
        ..Default::default()
    })
});

static OPENAI_AUDIO_MODEL: LazyLock<OpenAIChatModel> = LazyLock::new(|| {
    dotenvy::dotenv().ok();

    OpenAIChatModel::new(OpenAIModelOptions {
        model_id: "gpt-4o-audio-preview".to_string(),
        api_key: env::var("OPENAI_API_KEY")
            .expect("OPENAI_API_KEY must be set")
            .to_string(),
        ..Default::default()
    })
    .with_metadata(LanguageModelMetadata {
        capabilities: Some(vec![
            LanguageModelCapability::AudioInput,
            LanguageModelCapability::AudioOutput,
            LanguageModelCapability::FunctionCalling,
            LanguageModelCapability::ImageInput,
            LanguageModelCapability::StructuredOutput,
        ]),
        ..Default::default()
    })
});

test_set!(OPENAI_MODEL, generate_text);

test_set!(OPENAI_MODEL, stream_text);

test_set!(OPENAI_MODEL, generate_with_system_prompt);

test_set!(OPENAI_MODEL, generate_tool_call);

test_set!(OPENAI_MODEL, stream_tool_call);

test_set!(OPENAI_MODEL, generate_text_from_tool_result);

test_set!(OPENAI_MODEL, stream_text_from_tool_result);

test_set!(OPENAI_MODEL, generate_parallel_tool_calls);

test_set!(OPENAI_MODEL, stream_parallel_tool_calls);

test_set!(OPENAI_MODEL, stream_parallel_tool_calls_same_name);

test_set!(OPENAI_MODEL, structured_response_format);

test_set!(OPENAI_MODEL, source_part_input);

#[test]
async fn test_generate_audio() -> Result<(), Box<dyn Error>> {
    let response = OPENAI_AUDIO_MODEL
        .generate(LanguageModelInput {
            modalities: Some(vec![Modality::Text, Modality::Audio]),
            extra: Some(serde_json::json!({
                "audio": {
                    "voice": "alloy",
                    "format": "pcm16"
                }
            })),
            messages: vec![Message::User(UserMessage {
                content: vec![Part::Text(TextPart {
                    text: "Hello".to_string(),
                })],
            })],
            ..Default::default()
        })
        .await?;

    let audio_part = response
        .content
        .into_iter()
        .find_map(|part| match part {
            Part::Audio(audio) => Some(audio),
            _ => None,
        })
        .ok_or_else(|| "Audio part must be present".to_string())?;

    assert!(
        !audio_part.audio_data.is_empty(),
        "Audio data must be present"
    );
    assert!(
        audio_part.transcript.is_some_and(|t| !t.is_empty()),
        "Transcript must be present"
    );
    assert!(
        audio_part.audio_id.is_some_and(|id| !id.is_empty()),
        "Audio part ID must be present"
    );

    Ok(())
}

#[test]
async fn test_stream_audio() -> Result<(), Box<dyn Error>> {
    let mut stream = OPENAI_AUDIO_MODEL
        .stream(LanguageModelInput {
            modalities: Some(vec![Modality::Text, Modality::Audio]),
            extra: Some(serde_json::json!({
                "audio": {
                    "voice": "alloy",
                    "format": "pcm16"
                }
            })),
            messages: vec![Message::User(UserMessage {
                content: vec![Part::Text(TextPart {
                    text: "Hello".to_string(),
                })],
            })],
            ..Default::default()
        })
        .await?;

    let mut accumulator = StreamAccumulator::new();

    while let Some(partial_response) = stream.next().await {
        let partial_response = partial_response.unwrap();
        accumulator.add_partial(partial_response).unwrap();
    }

    let response = accumulator.compute_response()?;

    let audio_part = response
        .content
        .into_iter()
        .find_map(|part| match part {
            Part::Audio(audio) => Some(audio),
            _ => None,
        })
        .ok_or_else(|| "Audio part must be present".to_string())?;

    assert!(
        !audio_part.audio_data.is_empty(),
        "Audio data must be present"
    );
    assert!(
        audio_part.transcript.is_some_and(|t| !t.is_empty()),
        "Transcript must be present"
    );
    assert!(
        audio_part.audio_id.is_some_and(|id| !id.is_empty()),
        "Audio part ID must be present"
    );

    Ok(())
}
