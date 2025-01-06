# llm-sdk for Rust

`llm-sdk` is Rust library that provides a unified API to access the LLM APIs of various providers.

## Usage

All models implement the `LanguageModel` trait:

```rust
use llm_sdk::{
    openai::{OpenAIModel, OpenAIModelOptions},
    LanguageModel,
};

pub fn get_model(provider: &str, model_id: &str) -> Box<dyn LanguageModel> {
    match provider {
        "openai" => Box::new(OpenAIModel::new(OpenAIModelOptions {
            model_id: model_id.to_string(),
            api_key: std::env::var("OPENAI_API_KEY")
                .expect("OPENAI_API_KEY environment variable must be set"),
            ..Default::default()
        })),
        _ => panic!("Unsupported provider: {provider}"),
    }
}

```

Below is an example to generate text:

```rust
use dotenvy::dotenv;
use llm_sdk::{AssistantMessage, LanguageModelInput, Message, Part, UserMessage};

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
                Message::Assistant(AssistantMessage {
                    content: vec![Part::Text(
                        "What kind of story would you like to hear?".into(),
                    )],
                }),
                Message::User(UserMessage {
                    content: vec![Part::Text("A fairy tale.".into())],
                }),
            ],
            ..Default::default()
        })
        .await
        .unwrap();

    println!("{response:#?}");
}

```

Find examples in the [examples](./examples/) folder to learn how to:

- [Generate text](./examples/generate-text.rs)
- [Stream text](./examples/stream-text.rs)
- [Describe image](./examples/describe-image.rs)
- [Function calling](./examples/tool-use.rs)
- [Generate audio](./examples/generate-audio.rs)
- [Stream audio](./examples/stream-audio.rs)

```bash
cargo run --example generate-text
```

## License

[MIT](https://github.com/hoangvvo/llm-sdk/blob/main/LICENSE)
