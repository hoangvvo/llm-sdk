use crate::{
    instruction,
    types::{AgentStream, AgentStreamEvent},
    AgentError, AgentItem, AgentRequest, AgentResponse, AgentTool, InstructionParam,
};
use futures::{lock::Mutex, stream::StreamExt};
use llm_sdk::{
    AssistantMessage, LanguageModel, LanguageModelInput, Message, ModelResponse, Part,
    ResponseFormatOption, StreamAccumulator, ToolCallPart, ToolMessage, ToolResultPart,
};
use std::sync::Arc;

/// Manages the run session for an agent.
/// It initializes all necessary components for the agent to run
/// and handles the execution of the agent's tasks.
/// Once finish, the session cleans up any resources used during the run.
/// The session can be reused in multiple runs.
pub struct RunSession<TCtx> {
    tools: Arc<Vec<Box<dyn AgentTool<TCtx>>>>,
    model: Arc<dyn LanguageModel + Send + Sync>,
    response_format: ResponseFormatOption,
    instructions: Arc<Vec<InstructionParam<TCtx>>>,
    max_turns: usize,
    temperature: Option<f64>,
    top_p: Option<f64>,
    top_k: Option<f64>,
    presence_penalty: Option<f64>,
    frequency_penalty: Option<f64>,
}

impl<TCtx> RunSession<TCtx>
where
    TCtx: Send + Sync + 'static,
{
    /// Creates a new run session and initializes dependencies
    #[allow(clippy::unused_async)]
    #[allow(clippy::too_many_arguments)]
    pub async fn new(
        model: Arc<dyn LanguageModel + Send + Sync>,
        instructions: Arc<Vec<InstructionParam<TCtx>>>,
        tools: Arc<Vec<Box<dyn AgentTool<TCtx>>>>,
        response_format: ResponseFormatOption,
        max_turns: usize,
        temperature: Option<f64>,
        top_p: Option<f64>,
        top_k: Option<f64>,
        presence_penalty: Option<f64>,
        frequency_penalty: Option<f64>,
    ) -> Self {
        Self {
            tools,
            model,
            response_format,
            instructions,
            max_turns,
            temperature,
            top_p,
            top_k,
            presence_penalty,
            frequency_penalty,
        }
    }

    /// Process the model response and decide whether to continue the loop or
    /// return the response
    async fn process(
        &self,
        context: Arc<TCtx>,
        run_state: Arc<Mutex<RunState>>,
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

        for tool_call_part in tool_call_parts {
            let ToolCallPart {
                tool_call_id,
                tool_name,
                args,
                ..
            } = tool_call_part;

            let agent_tool = self
                .tools
                .iter()
                .find(|tool| tool.name() == tool_name)
                .ok_or_else(|| {
                    AgentError::Invariant(format!("Tool {tool_name} not found for tool call"))
                })?;

            let tool_res = agent_tool
                .execute(args, &context, run_state.clone())
                .await
                .map_err(AgentError::ToolExecution)?;

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
        let state = Arc::new(Mutex::new(RunState::new(
            request.input.clone(),
            self.max_turns,
        )));

        let (input, context) = self.get_llm_input(request);

        loop {
            let mut input = input.clone();
            input.messages = state.lock().await.get_turn_messages();
            let model_response = self.model.generate(input).await?;

            state
                .lock()
                .await
                .append_message(Message::Assistant(AssistantMessage {
                    content: model_response.content.clone(),
                }));

            match self
                .process(context.clone(), state.clone(), model_response)
                .await?
            {
                ProcessResult::Response(final_content) => {
                    return Ok(state.lock().await.create_response(final_content));
                }
                ProcessResult::Next(next_messages) => {
                    for message in next_messages {
                        state.lock().await.append_message(message);
                    }
                }
            }

            state.lock().await.turn()?;
        }
    }

    /// Run a streaming execution of the agent.
    pub fn run_stream(&self, request: AgentRequest<TCtx>) -> Result<AgentStream, AgentError> {
        let state = Arc::new(Mutex::new(RunState::new(
            request.input.clone(),
            self.max_turns,
        )));

        let (input, context) = self.get_llm_input(request);

        let session = Arc::new(Self {
            tools: self.tools.clone(),
            model: self.model.clone(),
            response_format: self.response_format.clone(),
            instructions: self.instructions.clone(),
            max_turns: self.max_turns,
            temperature: self.temperature,
            top_p: self.top_p,
            top_k: self.top_k,
            presence_penalty: self.presence_penalty,
            frequency_penalty: self.frequency_penalty,
        });

        let stream = async_stream::try_stream! {
            loop {
                let mut input = input.clone();
                input.messages = state.lock().await.get_turn_messages();

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

                state.lock().await
                    .append_message(assistant_message.clone());
                yield AgentStreamEvent::Message(assistant_message);

                match session.process(context.clone(), state.clone(), model_response).await? {
                    ProcessResult::Response(final_content) => {
                        let response = state.lock().await.create_response(final_content);
                        yield AgentStreamEvent::Response(response);
                        break;
                    }
                    ProcessResult::Next(next_messages) => {
                        for message in next_messages {
                            state.lock().await.append_message(message.clone());
                            yield AgentStreamEvent::Message(message);
                        }
                    }
                }

                state.lock().await.turn()?;
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
                // messages will be computed from getTurnMessages
                messages: vec![],
                system_prompt: Some(instruction::get_prompt(
                    &self.instructions,
                    &request.context,
                )),
                tools: Some(self.tools.iter().map(|tool| tool.as_ref().into()).collect()),
                response_format: Some(self.response_format.clone()),
                temperature: self.temperature,
                top_p: self.top_p,
                top_k: self.top_k,
                presence_penalty: self.presence_penalty,
                frequency_penalty: self.frequency_penalty,
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

pub struct RunState {
    max_turns: usize,
    input: Vec<AgentItem>,

    /// The current turn number in the run.
    pub current_turn: usize,
    /// All items generated during the run, such as new `ToolMessage` and
    /// `AssistantMessage`
    output: Vec<AgentItem>,
}

impl RunState {
    #[must_use]
    pub fn new(input: Vec<AgentItem>, max_turns: usize) -> Self {
        Self {
            max_turns,
            input,
            current_turn: 0,
            output: vec![],
        }
    }

    /// Mark a new turn in the conversation and throw an error if max turns
    /// exceeded.
    pub fn turn(&mut self) -> Result<(), AgentError> {
        self.current_turn += 1;
        if self.current_turn > self.max_turns {
            return Err(AgentError::MaxTurnsExceeded(self.max_turns));
        }
        Ok(())
    }

    /// Add a message to the run state.
    pub fn append_message(&mut self, message: Message) {
        self.output.push(AgentItem::Message(message));
    }

    /// Get LLM messages to use in the `LanguageModelInput` for the turn
    #[must_use]
    pub fn get_turn_messages(&self) -> Vec<Message> {
        [
            self.input
                .iter()
                .map(|item| match item {
                    AgentItem::Message(msg) => msg.clone(),
                })
                .collect::<Vec<_>>(),
            self.output
                .iter()
                .map(|item| match item {
                    AgentItem::Message(msg) => msg.clone(),
                })
                .collect(),
        ]
        .concat()
    }

    #[must_use]
    pub fn create_response(&self, final_content: Vec<Part>) -> AgentResponse {
        AgentResponse {
            content: final_content,
            output: self.output.clone(),
        }
    }
}
