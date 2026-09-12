# llm-sdk for Rust

A Rust library that enables the development of applications that can interact with different language models through a unified interface.

## Installation

```bash
cargo add llm-sdk-rs --features openai
cargo add rustls --no-default-features --features ring,std,tls12
```

Applications using the default HTTP client select the process-wide Rustls provider once at
startup. A custom `reqwest::Client` supplied through model options owns its own TLS setup.

```rust
rustls::crypto::ring::default_provider()
    .install_default()
    .expect("Rustls provider already selected");
```

## Usage

All models implement the `LanguageModel` trait.

```rust
use llm_sdk::{
    openai::{OpenAIModel, OpenAIModelOptions},
    LanguageModel,
};

pub fn get_model(model_id: &str) -> Box<dyn LanguageModel> {
    Box::new(OpenAIModel::new(
        model_id.to_string(),
        OpenAIModelOptions {
            api_key: std::env::var("OPENAI_API_KEY")
                .expect("OPENAI_API_KEY environment variable must be set"),
            ..Default::default()
        },
    ))
}
```

Below is an example to generate text:

```rust
use dotenvy::dotenv;
use llm_sdk::{LanguageModelInput, Message, Part};

mod common;

#[tokio::main]
async fn main() {
    dotenv().ok();

    let model = common::get_model("openai", "gpt-5.6-terra");

    let response = model
        .generate(LanguageModelInput {
            messages: vec![
                Message::user(vec![Part::text("Tell me a story.")]),
                Message::assistant(vec![Part::text(
                    "Sure! What kind of story would you like to hear?",
                )]),
                Message::user(vec![Part::text("a fairy tale")]),
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
- [`describe-image-url`: Describe image from URL](./examples/describe-image-url.rs)
- [`summarize-audio`: Summarize audio](./examples/summarize-audio.rs)
- [`summarize-file`: Summarize file](./examples/summarize-file.rs)
- [`summarize-file-url`: Summarize file from URL](./examples/summarize-file-url.rs)
- [`tool-use`: Function calling](./examples/tool-use.rs)
- [`tool-search`: Tool search](./examples/tool-search.rs)
- [`stream-tool-search`: Stream tool search](./examples/stream-tool-search.rs)
- [`web-search`: Web search](./examples/web-search.rs)
- [`stream-web-search`: Stream web search](./examples/stream-web-search.rs)
- [`structured-output`: Structured output](./examples/structured-output.rs)
- [`generate-reasoning`: Reasoning](./examples/generate-reasoning.rs)
- [`stream-reasoning`: Stream reasoning](./examples/stream-reasoning.rs)
- [`generate-citations`: Generate citations](./examples/generate-citations.rs)
- [`stream-citations`: Stream citations](./examples/stream-citations.rs)

```bash
cargo run --example generate-text --features examples,openai
```

## Migration

### To 0.4.0

- Use `from_data` or `from_url` instead of `new` for `ImagePart`, `AudioPart`, and `FilePart`.
- Wrap inline `ImagePart.data` and `AudioPart.data` in `Some(...)`; handle `None` for URL-only media. Set `url: None` in inline media literals.
- Update exhaustive matches: `Part` adds `File`; `Tool`, `ToolCall`, `ToolResult`, and `ToolCallDelta` add `ToolSearch`.
- `ReasoningPartDelta.text` is now `String`. Use an empty string for deltas without text.
- Add the new optional fields to struct literals, or use `..Default::default()` where available.
- Replace `Tool { name, description, parameters }` with `FunctionTool::new(...)`.
- `ToolResultPart::is_error` has been replaced with the required `status` field (`Completed`, `Failed`, or `Cancelled`).
- Match `ToolCall::Function(call)` to read `call.name` and `call.args` in place of `tool_name` and `args`.
- Match `ToolResult::Function(result)` to read `result.name` and `result.content` in place of `tool_name` and `content`.
- Move `ToolCallPartDelta.tool_name` and `args` into `ToolCallDelta::Function` as `name` and `args`.

### To 0.2.0

- `image_data` and `audio_data` have been renamed to just `data` in `ImagePart` and `AudioPart`.

# Testing

```bash
cargo test --package llm-sdk-rs --test core_tests
```

## License

[MIT](https://github.com/hoangvvo/llm-sdk/blob/main/LICENSE)
