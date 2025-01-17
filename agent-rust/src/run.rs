use crate::{
    instruction, types::AgentStreamResult, AgentError, AgentRequest, AgentResponse, AgentTool,
    InstructionParam,
};
use futures::stream::StreamExt;
use futures_core::stream::BoxStream;
use llm_sdk::{
    AssistantMessage, LanguageModel, LanguageModelInput, Message, ModelResponse, Part,
    ResponseFormatOption, StreamAccumulator, ToolCallPart, ToolMessage, ToolResultPart,
};
use std::sync::Arc;

/// Manages the run session for an agent run.
/// It initializes all necessary components for the agent to run
/// and handles the execution of the agent's tasks.
/// Once finish, the session cleans up any resources used during the run.
/// The session can be reused in multiple runs.
pub struct RunSession<TCtx> {
    pub tools: Arc<Vec<AgentTool<TCtx>>>,
    pub model: Arc<dyn LanguageModel + Send + Sync>,
    pub response_format: ResponseFormatOption,
    pub instructions: Arc<Vec<InstructionParam<TCtx>>>,
}

impl<TCtx> RunSession<TCtx>
where
    TCtx: Send + Sync + 'static,
{
    /// Creates a new run session and initializes dependencies
    #[allow(clippy::unused_async)]
    pub async fn new(
        model: Arc<dyn LanguageModel + Send + Sync>,
        instructions: Arc<Vec<InstructionParam<TCtx>>>,
        tools: Arc<Vec<AgentTool<TCtx>>>,
        response_format: ResponseFormatOption,
    ) -> Self {
        Self {
            tools,
            model,
            response_format,
            instructions,
        }
    }

    /// Process the model response and decide whether to continue the loop or
    /// return the response
    async fn process(
        &self,
        input: LanguageModelInput,
        context: Arc<TCtx>,
        model_response: ModelResponse,
    ) -> Result<ProcessResult, AgentError> {
        let mut messages = input.messages.clone();

        messages.push(Message::Assistant(AssistantMessage {
            content: model_response.content.clone(),
        }));

        let tool_call_parts: Vec<&ToolCallPart> = model_response
            .content
            .iter()
            .filter_map(|part| {
                if let Part::ToolCall(tool_call) = part {
                    Some(tool_call)
                } else {
                    None
                }
            })
            .collect();

        // If no tool calls were found, return the model response as is
        if tool_call_parts.is_empty() {
            return Ok(ProcessResult::Response(AgentResponse {
                messages,
                content: model_response.content,
            }));
        }

        // Process all tool calls
        let mut tool_message = ToolMessage { content: vec![] };

        for tool_call_part in tool_call_parts {
            let agent_tool = self
                .tools
                .iter()
                .find(|tool| tool.name == tool_call_part.tool_name)
                .ok_or_else(|| {
                    AgentError::Invariant(format!(
                        "Tool {} not found for tool call",
                        tool_call_part.tool_name
                    ))
                })?;

            let tool_res = (agent_tool.execute)(tool_call_part.args.clone(), context.clone())
                .await
                .map_err(|e| AgentError::ToolExecution(e.into()))?;

            tool_message.content.push(Part::ToolResult(ToolResultPart {
                tool_call_id: tool_call_part.tool_call_id.clone(),
                tool_name: tool_call_part.tool_name.clone(),
                content: tool_res.content,
                is_error: Some(tool_res.is_error),
            }));
        }

        messages.push(Message::Tool(tool_message));

        Ok(ProcessResult::Next(
            LanguageModelInput { messages, ..input }.into(),
        ))
    }

    /// Run a non-streaming execution of the agent.
    pub async fn run(&self, request: AgentRequest<TCtx>) -> Result<AgentResponse, AgentError> {
        let mut input = self.get_llm_input(&request);
        let context = Arc::new(request.context);

        loop {
            let model_response = self.model.generate(input.clone()).await?;
            match self.process(input, context.clone(), model_response).await? {
                ProcessResult::Response(response) => {
                    return Ok(response);
                }
                ProcessResult::Next(next_input) => {
                    input = *next_input;
                }
            }
        }
    }

    /// Run a streaming execution of the agent.
    pub fn run_stream(
        &self,
        request: AgentRequest<TCtx>,
    ) -> Result<BoxStream<'static, Result<AgentStreamResult, AgentError>>, AgentError> {
        let mut input = self.get_llm_input(&request);
        let context = Arc::new(request.context);

        let session = Arc::new(Self {
            tools: self.tools.clone(),
            model: self.model.clone(),
            response_format: self.response_format.clone(),
            instructions: self.instructions.clone(),
        });

        let stream = async_stream::try_stream! {
            loop {
                let model_stream = session.model.stream(input.clone()).await?;

                tokio::pin!(model_stream);

                let mut accumulator = StreamAccumulator::new();

                while let Some(partial) = model_stream.next().await {
                    let partial = partial?;

                    accumulator.add_partial(&partial).map_err(|e| {
                        AgentError::Invariant(format!("Failed to accumulate stream: {e}"))
                    })?;

                    yield AgentStreamResult::PartialModelResponse(partial);
                }

                let model_response = accumulator.compute_response()?;

                yield AgentStreamResult::ModelResponse(model_response.clone());

                match session.process(input.clone(), context.clone(), model_response).await? {
                    ProcessResult::Response(response) => {
                        yield AgentStreamResult::Response(response);
                    }
                    ProcessResult::Next(next_input) => {
                        input = *next_input;
                    }
                }
            }
        };

        Ok(stream.boxed())
    }

    pub fn finish(self) {
        // Cleanup dependencies if needed
    }

    fn get_llm_input(&self, request: &AgentRequest<TCtx>) -> LanguageModelInput {
        LanguageModelInput {
            messages: request.messages.clone(),
            system_prompt: Some(instruction::get_prompt(
                &self.instructions,
                &request.context,
            )),
            tools: Some(self.tools.iter().map(Into::into).collect()),
            response_format: Some(self.response_format.clone()),
            ..Default::default()
        }
    }
}

enum ProcessResult {
    Response(AgentResponse),
    Next(Box<LanguageModelInput>),
}
