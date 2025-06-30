# llm-sdk for Rust

A Rust library that provides a unified API to access the LLM APIs of various providers.

## Usage

All models implement the `LanguageModel` trait:

```rust
use llm_sdk::{
    google::{GoogleModel, GoogleModelOptions},
    openai::{OpenAIChatModel, OpenAIChatModelOptions, OpenAIModel, OpenAIModelOptions},
    LanguageModel,
};

pub fn get_model(provider: &str, model_id: &str) -> Box<dyn LanguageModel> {
    match provider {
        "openai" => Box::new(OpenAIModel::new(
            model_id.to_string(),
            OpenAIModelOptions {
                api_key: std::env::var("OPENAI_API_KEY")
                    .expect("OPENAI_API_KEY environment variable must be set"),
                ..Default::default()
            },
        )),
        "openai-chat-completion" => Box::new(OpenAIChatModel::new(
            model_id.to_string(),
            OpenAIChatModelOptions {
                api_key: std::env::var("OPENAI_API_KEY")
                    .expect("OPENAI_API_KEY environment variable must be set"),
                ..Default::default()
            },
        )),
        "google" => Box::new(GoogleModel::new(
            model_id.to_string(),
            GoogleModelOptions {
                api_key: std::env::var("GOOGLE_API_KEY")
                    .expect("GOOGLE_API_KEY environment variable must be set"),
                ..Default::default()
            },
        )),
        _ => panic!("Unsupported provider: {provider}"),
    }
}
```

Below is an example to generate text:

```rust
use dotenvy::dotenv;
use llm_sdk::{LanguageModelInput, Message, Part, UserMessage};

mod common;

#[tokio::main]
async fn main() {
    dotenv().ok();

    let model = common::get_model("openai", "gpt-4o");

    let response = model
        .generate(LanguageModelInput {
            messages: vec![
                Message::User(UserMessage {
                    content: vec![Part::Text("Tell me a story.".into())],
                }),
                // Or use the convenient function to create a user message
                Message::assistant(vec![Part::text(
                    "Sure! What kind of story would you like to hear?",
                )]),
                Message::user(vec![Part::Text("a fairy tale".into())]),
            ],
            ..Default::default()
        })
        .await
        .unwrap();

    println!("{response:#?}");
}
```

## Examples

Find examples in the [examples](./examples/) folder to learn how to:

- [`generate-text`: Generate text](./examples/generate-text.rs)
- [`stream-text`: Stream text](./examples/stream-text.rs)
- [`generate-audio`: Generate audio](./examples/generate-audio.rs)
- [`stream-audio`: Stream audio](./examples/stream-audio.rs)
- [`generate-image`: Generate image](./examples/generate-image.rs)
- [`describe-image`: Describe image](./examples/describe-image.rs)
- [`summarize-audio`: Summarize audio](./examples/summarize-audio.rs)
- [`tool-use`: Function calling](./examples/tool-use.rs)
- [`structured-output`: Structured output](./examples/structured-output.rs)
- [`generate-reasoning`: Reasoning](./examples/generate-reasoning.rs)
- [`stream-reasoning`: Stream reasoning](./examples/stream-reasoning.rs)
- [`generate-citations`: Generate citations](./examples/generate-citations.rs)
- [`stream-citations`: Stream citations](./examples/stream-citations.rs)

```bash
cargo run --example generate-text
```

## License

[MIT](https://github.com/hoangvvo/llm-sdk/blob/main/LICENSE)
