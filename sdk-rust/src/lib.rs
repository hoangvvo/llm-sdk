#[cfg(all(
    any(feature = "anthropic", feature = "google", feature = "openai"),
    not(any(feature = "tls-aws-lc", feature = "tls-ring"))
))]
compile_error!("enabling an LLM provider requires either `tls-aws-lc` or `tls-ring`");

mod accumulator;
#[cfg(feature = "anthropic")]
pub mod anthropic;
pub mod audio_part_utils;
pub mod boxed_stream;
#[allow(dead_code)]
mod client_utils;
mod errors;
#[cfg(feature = "google")]
pub mod google;
#[allow(dead_code)]
mod id_utils;
mod language_model;
pub mod llm_sdk_test;
#[cfg(feature = "openai")]
pub mod openai;
#[allow(dead_code)]
mod opentelemetry;
#[allow(dead_code)]
mod source_part_utils;
#[allow(dead_code)]
mod stream_utils;
#[allow(dead_code)]
mod tool_result_utils;
mod types;
pub mod types_ext;
pub mod usage_ext;
mod utils;

pub use accumulator::StreamAccumulator;
pub use errors::{LanguageModelError, LanguageModelResult};
pub use language_model::{LanguageModel, LanguageModelMetadata, LanguageModelStream};
pub use types::*;
