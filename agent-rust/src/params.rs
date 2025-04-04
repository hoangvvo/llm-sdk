use crate::{Agent, AgentTool, InstructionParam};
use llm_sdk::{AudioOptions, LanguageModel, Modality, ReasoningOptions, ResponseFormatOption};
use std::sync::Arc;

/// Parameters required to create a new agent.
/// # Default Values
/// - `instructions`: `vec![]`
/// - `tools`: `vec![]`
/// - `response_format`: `ResponseFormatOption::Text`
/// - `max_turns`: 10
/// - `temperature`: `None`
/// - `top_p`: `None`
/// - `top_k`: `None`
/// - `presence_penalty`: `None`
/// - `frequency_penalty`: `None`
/// - `modalities`: `None`
/// - `audio`: `None`
/// - `reasoning`: `None`
pub struct AgentParams<TCtx> {
    pub name: String,
    /// The default language model to use for the agent.
    pub model: Arc<dyn LanguageModel + Send + Sync>,
    /// Instructions to be added to system messages when executing the agent.
    /// This can include formatting instructions or other guidance for the
    /// agent.
    pub instructions: Vec<InstructionParam<TCtx>>,
    /// The tools that the agent can use to perform tasks.
    pub tools: Vec<Box<dyn AgentTool<TCtx>>>,
    /// The expected format of the response. Either text or structured output.
    pub response_format: ResponseFormatOption,
    /// Max number of turns for agent to run to protect against infinite loops.
    pub max_turns: usize,
    /// Amount of randomness injected into the response. Ranges from 0.0 to 1.0
    pub temperature: Option<f64>,
    /// An alternative to sampling with temperature, called nucleus sampling,
    /// where the model considers the results of the tokens with `top_p`
    /// probability mass. Ranges from 0.0 to 1.0
    pub top_p: Option<f64>,
    /// Only sample from the top K options for each subsequent token.
    /// Used to remove 'long tail' low probability responses.
    /// Must be a non-negative integer.
    pub top_k: Option<i32>,
    /// Positive values penalize new tokens based on whether they appear in the
    /// text so far, increasing the model's likelihood to talk about new
    /// topics.
    pub presence_penalty: Option<f64>,
    /// Positive values penalize new tokens based on their existing frequency in
    /// the text so far, decreasing the model's likelihood to repeat the
    /// same line verbatim.
    pub frequency_penalty: Option<f64>,
    /// The modalities that the model should support.
    pub modalities: Option<Vec<Modality>>,
    /// Options for audio generation.
    pub audio: Option<AudioOptions>,
    /// Options for reasoning generation.
    pub reasoning: Option<ReasoningOptions>,
}

impl<TCtx> AgentParams<TCtx>
where
    TCtx: Send + Sync + 'static,
{
    pub fn new(name: &str, model: Arc<dyn LanguageModel + Send + Sync>) -> Self {
        Self {
            name: name.to_string(),
            model,
            instructions: Vec::new(),
            tools: Vec::new(),
            response_format: ResponseFormatOption::Text,
            max_turns: 10,
            temperature: None,
            top_p: None,
            top_k: None,
            presence_penalty: None,
            frequency_penalty: None,
            audio: None,
            reasoning: None,
            modalities: None,
        }
    }

    /// Add an instruction
    #[must_use]
    pub fn add_instruction(mut self, instruction: impl Into<InstructionParam<TCtx>>) -> Self {
        self.instructions.push(instruction.into());
        self
    }

    /// Set the instructions
    #[must_use]
    pub fn instructions(mut self, instructions: Vec<InstructionParam<TCtx>>) -> Self {
        self.instructions = instructions;
        self
    }

    /// Add a tool
    #[must_use]
    pub fn add_tool(mut self, tool: impl AgentTool<TCtx> + 'static) -> Self {
        self.tools.push(Box::new(tool));
        self
    }

    /// Set the response format
    #[must_use]
    pub fn response_format(mut self, response_format: ResponseFormatOption) -> Self {
        self.response_format = response_format;
        self
    }

    /// Set the max turns
    #[must_use]
    pub fn max_turns(mut self, max_turns: usize) -> Self {
        self.max_turns = max_turns;
        self
    }

    /// Set the temperature for sampling
    /// Amount of randomness injected into the response. Ranges from 0.0 to 1.0
    #[must_use]
    pub fn temperature(mut self, temperature: f64) -> Self {
        self.temperature = Some(temperature);
        self
    }

    /// Set the `top_p` for nucleus sampling
    /// An alternative to sampling with temperature, called nucleus sampling,
    /// where the model considers the results of the tokens with `top_p`
    /// probability mass. Ranges from 0.0 to 1.0
    #[must_use]
    pub fn top_p(mut self, top_p: f64) -> Self {
        self.top_p = Some(top_p);
        self
    }

    /// Set the `top_k` for sampling
    /// Only sample from the top K options for each subsequent token.
    /// Used to remove 'long tail' low probability responses.
    /// Must be a non-negative integer.
    #[must_use]
    pub fn top_k(mut self, top_k: i32) -> Self {
        self.top_k = Some(top_k);
        self
    }

    /// Set the presence penalty
    /// Positive values penalize new tokens based on whether they appear in the
    /// text so far, increasing the model's likelihood to talk about new
    /// topics.
    #[must_use]
    pub fn presence_penalty(mut self, presence_penalty: f64) -> Self {
        self.presence_penalty = Some(presence_penalty);
        self
    }

    /// Set the frequency penalty
    /// Positive values penalize new tokens based on their existing frequency in
    /// the text so far, decreasing the model's likelihood to repeat the
    /// same line verbatim.
    #[must_use]
    pub fn frequency_penalty(mut self, frequency_penalty: f64) -> Self {
        self.frequency_penalty = Some(frequency_penalty);
        self
    }

    /// Set the modalities that the model should support.
    #[must_use]
    pub fn modalities(mut self, modalities: Vec<Modality>) -> Self {
        self.modalities = Some(modalities);
        self
    }

    /// Set the audio options for generation.
    #[must_use]
    pub fn audio(mut self, audio: AudioOptions) -> Self {
        self.audio = Some(audio);
        self
    }

    /// Set the reasoning options for generation.
    #[must_use]
    pub fn reasoning(mut self, reasoning: ReasoningOptions) -> Self {
        self.reasoning = Some(reasoning);
        self
    }

    #[must_use]
    pub fn build(self) -> Agent<TCtx> {
        Agent::new(self)
    }
}
