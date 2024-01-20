use crate::{
    instruction,
    types::{AgentStream, AgentStreamEvent},
    AgentError, AgentRequest, AgentResponse, AgentTool, InstructionParam,
};
use futures::stream::StreamExt;
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
        context: Arc<TCtx>,
        model_response: ModelResponse,
    ) -> Result<ProcessResult, AgentError> {
        let tool_call_parts: Vec<ToolCallPart> = model_response
            .content
            .iter()
            .filter_map(|part| {
                if let Part::ToolCall(tool_call) = part {
                    Some(tool_call.clone())
                } else {
                    None
                }
            })
            .collect();

        // If no tool calls were found, return the model response as is
        if tool_call_parts.is_empty() {
            return Ok(ProcessResult::Response(model_response.content));
        }

        let mut next_messages: Vec<Message> = vec![];

        // Process all tool calls
        let mut tool_message = ToolMessage { content: vec![] };

        for tool_call_part in tool_call_parts.into_iter() {
            let ToolCallPart {
                tool_call_id,
                tool_name,
                args,
                ..
            } = tool_call_part;

            let agent_tool = self
                .tools
                .iter()
                .find(|tool| tool.name == *tool_name)
                .ok_or_else(|| {
                    AgentError::Invariant(format!("Tool {} not found for tool call", tool_name))
                })?;

            let tool_res = agent_tool
                .call(args, context.clone())
                .await
                .map_err(|e| AgentError::ToolExecution(e.into()))?;

            tool_message.content.push(Part::ToolResult(ToolResultPart {
                tool_call_id,
                tool_name,
                content: tool_res.content,
                is_error: Some(tool_res.is_error),
            }));
        }

        next_messages.push(Message::Tool(tool_message));

        Ok(ProcessResult::Next(next_messages))
    }

    /// Run a non-streaming execution of the agent.
    pub async fn run(&self, request: AgentRequest<TCtx>) -> Result<AgentResponse, AgentError> {
        let (input, context) = self.get_llm_input(request);

        let mut new_messages = vec![];

        loop {
            let mut input = input.clone();
            input.messages.append(new_messages.clone().as_mut());
            let model_response = self.model.generate(input).await?;

            new_messages.push(Message::Assistant(AssistantMessage {
                content: model_response.content.clone(),
            }));

            match self.process(context.clone(), model_response).await? {
                ProcessResult::Response(content) => {
                    return Ok(AgentResponse {
                        content,
                        new_messages,
                    });
                }
                ProcessResult::Next(next_messages) => {
                    new_messages.extend(next_messages);
                    continue;
                }
            }
        }
    }

    /// Run a streaming execution of the agent.
    pub fn run_stream(&self, request: AgentRequest<TCtx>) -> Result<AgentStream, AgentError> {
        let (input, context) = self.get_llm_input(request);

        let session = Arc::new(Self {
            tools: self.tools.clone(),
            model: self.model.clone(),
            response_format: self.response_format.clone(),
            instructions: self.instructions.clone(),
        });

        let mut new_messages = vec![];

        let stream = async_stream::try_stream! {
            loop {
                let mut input = input.clone();
                input.messages.append(new_messages.clone().as_mut());

                let mut model_stream = session.model.stream(input).await?;

                let mut accumulator = StreamAccumulator::new();

                while let Some(partial) = model_stream.next().await {
                    let partial = partial?;

                    accumulator.add_partial(partial.clone()).map_err(|e| {
                        AgentError::Invariant(format!("Failed to accumulate stream: {e}"))
                    })?;

                    yield AgentStreamEvent::Partial(partial);
                }

                let model_response = accumulator.compute_response()?;

                let assistant_message = Message::Assistant(AssistantMessage {
                    content: model_response.content.clone(),
                });

                new_messages.push(assistant_message.clone());
                yield AgentStreamEvent::Message(assistant_message);

                match session.process(context.clone(), model_response).await? {
                    ProcessResult::Response(content) => {
                        yield AgentStreamEvent::Response(AgentResponse { new_messages, content });
                        break;
                    }
                    ProcessResult::Next(next_messages) => {
                        new_messages.extend(next_messages.clone());
                        for message in next_messages {
                            yield AgentStreamEvent::Message(message);
                        }
                        continue;
                    }
                }
            }
        };

        Ok(AgentStream::from_stream(stream))
    }

    pub fn finish(self) {
        // Cleanup dependencies if needed
    }

    fn get_llm_input(&self, request: AgentRequest<TCtx>) -> (LanguageModelInput, Arc<TCtx>) {
        (
            LanguageModelInput {
                messages: request.messages,
                system_prompt: Some(instruction::get_prompt(
                    &self.instructions,
                    &request.context,
                )),
                tools: Some(self.tools.iter().map(Into::into).collect()),
                response_format: Some(self.response_format.clone()),
                ..Default::default()
            },
            Arc::new(request.context),
        )
    }
}

enum ProcessResult {
    Response(Vec<Part>),
    // Return when new messages need to be added to the input and continue processing
    Next(Vec<Message>),
}
