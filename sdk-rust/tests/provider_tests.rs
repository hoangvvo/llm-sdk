#[cfg(any(feature = "anthropic", feature = "google", feature = "openai"))]
mod common;

#[cfg(feature = "anthropic")]
#[path = "anthropic_test.rs"]
mod anthropic;

#[cfg(feature = "google")]
#[path = "google_test.rs"]
mod google;

#[cfg(feature = "openai")]
#[path = "openai_test.rs"]
mod openai;

#[cfg(feature = "openai")]
#[path = "openai_chat_test.rs"]
mod openai_chat;
