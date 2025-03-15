#[derive(Debug, Clone, Default)]
pub struct OpenAIModelOptions {
    pub base_url: Option<String>,
    pub api_key: String,
    pub model_id: String,
}
