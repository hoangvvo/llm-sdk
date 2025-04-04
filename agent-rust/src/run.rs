use crate::{
    instruction,
    types::{AgentItemTool, AgentStream, AgentStreamEvent},
    AgentError, AgentItem, AgentParams, AgentRequest, AgentResponse,
};
use futures::{lock::Mutex, stream::StreamExt};
use llm_sdk::{
    LanguageModelInput, Message, ModelResponse, Part, StreamAccumulator, ToolCallPart, ToolMessage,
    ToolResultPart,
};
use std::sync::Arc;

/// Manages the run session for an agent.
/// It initializes all necessary components for the agent to run
/// and handles the execution of the agent's tasks.
/// Once finish, the session cleans up any resources used during the run.
/// The session can be reused in multiple runs.
pub struct RunSession<TCtx> {
    params: Arc<AgentParams<TCtx>>,
}

impl<TCtx> RunSession<TCtx>
where
    TCtx: Send + Sync + 'static,
{
    /// Creates a new run session and initializes dependencies
    #[allow(clippy::unused_async)]
    #[allow(clippy::too_many_arguments)]
    pub async fn new(params: Arc<AgentParams<TCtx>>) -> Self {
        Self { params }
    }

    /// Process the model response and decide whether to continue the loop or
    /// return the response
    async fn process(
        &self,
        context: Arc<TCtx>,
        run_state: &RunState,
        parts: Vec<Part>,
    ) -> Result<ProcessResult, AgentError> {
        let tool_call_parts: Vec<ToolCallPart> = parts
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
            return Ok(ProcessResult::Response(parts));
        }

        // Build AgentItems for all tool results
        let mut items: Vec<AgentItem> = vec![];

        for tool_call_part in tool_call_parts {
            let ToolCallPart {
                tool_call_id,
                tool_name,
                args,
                ..
            } = tool_call_part;

            let agent_tool = self
                .params
                .tools
                .iter()
                .find(|tool| tool.name() == tool_name)
                .ok_or_else(|| {
                    AgentError::Invariant(format!("Tool {tool_name} not found for tool call"))
                })?;

            let input_args = args.clone();
            let tool_res = agent_tool
                .execute(args, &context, run_state)
                .await
                .map_err(AgentError::ToolExecution)?;

            items.push(AgentItem::Tool(AgentItemTool {
                tool_call_id,
                tool_name,
                input: input_args,
                output: tool_res.content,
                is_error: tool_res.is_error,
            }));
        }

        Ok(ProcessResult::Items(items))
    }

    /// Run a non-streaming execution of the agent.
    pub async fn run(&self, request: AgentRequest<TCtx>) -> Result<AgentResponse, AgentError> {
        let state = RunState::new(request.input.clone(), self.params.max_turns);

        let (input, context) = self.get_llm_input(request).await?;

        loop {
            let mut input = input.clone();

            input.messages = state.get_turn_messages().await;
            let model_response = self.params.model.generate(input).await?;
            let content = model_response.content.clone();

            state.append_model_response(model_response).await;

            match self.process(context.clone(), &state, content).await? {
                ProcessResult::Response(final_content) => {
                    return Ok(state.create_response(final_content).await);
                }
                ProcessResult::Items(items) => {
                    state.append_items(items).await;
                }
            }

            state.turn().await?;
        }
    }

    /// Run a streaming execution of the agent.
    pub async fn run_stream(&self, request: AgentRequest<TCtx>) -> Result<AgentStream, AgentError> {
        let state = Arc::new(RunState::new(request.input.clone(), self.params.max_turns));

        let (input, context) = self.get_llm_input(request).await?;

        let session = Arc::new(Self {
            params: self.params.clone(),
        });

        let stream = async_stream::try_stream! {
            loop {
                let mut input = input.clone();

                input.messages = state.get_turn_messages().await;

                let mut model_stream = session.params.model.stream(input).await?;

                let mut accumulator = StreamAccumulator::new();

                while let Some(partial) = model_stream.next().await {
                    let partial = partial?;

                    accumulator.add_partial(partial.clone()).map_err(|e| {
                        AgentError::Invariant(format!("Failed to accumulate stream: {e}"))
                    })?;

                    yield AgentStreamEvent::Partial(partial);
                }

                let model_response = accumulator.compute_response()?;

                let content = model_response.content.clone();

                let item = state.append_model_response(model_response).await;
                yield AgentStreamEvent::Item(item);

                match session.process(context.clone(), &state, content).await? {
                    ProcessResult::Response(final_content) => {
                        let response = state.create_response(final_content).await;
                        yield AgentStreamEvent::Response(response);
                        break;
                    }
                    ProcessResult::Items(items) => {
                        state.append_items(items.clone()).await;
                        for item in items {
                            yield AgentStreamEvent::Item(item);
                        }
                    }
                }

                state.turn().await?;
            }
        };

        Ok(AgentStream::from_stream(stream))
    }

    pub fn finish(self) {
        // Cleanup dependencies if needed
    }

    async fn get_llm_input(
        &self,
        request: AgentRequest<TCtx>,
    ) -> Result<(LanguageModelInput, Arc<TCtx>), AgentError> {
        let system_prompt = instruction::get_prompt(&self.params.instructions, &request.context)
            .await
            .map_err(AgentError::Init)?;

        Ok((
            LanguageModelInput {
                // messages will be computed from getTurnMessages
                messages: vec![],
                system_prompt: Some(system_prompt),
                tools: Some(
                    self.params
                        .tools
                        .iter()
                        .map(|tool| tool.as_ref().into())
                        .collect(),
                ),
                response_format: Some(self.params.response_format.clone()),
                temperature: self.params.temperature,
                top_p: self.params.top_p,
                top_k: self.params.top_k,
                presence_penalty: self.params.presence_penalty,
                frequency_penalty: self.params.frequency_penalty,
                modalities: self.params.modalities.clone(),
                reasoning: self.params.reasoning.clone(),
                audio: self.params.audio.clone(),
                ..Default::default()
            },
            Arc::new(request.context),
        ))
    }
}

enum ProcessResult {
    Response(Vec<Part>),
    // Return when new items need to be added to the output and continue processing
    Items(Vec<AgentItem>),
}

pub struct RunState {
    max_turns: usize,
    input: Vec<AgentItem>,

    /// The current turn number in the run.
    pub current_turn: Arc<Mutex<usize>>,
    /// All items generated during the run, such as new tool and model items
    output: Arc<Mutex<Vec<AgentItem>>>,
}

impl RunState {
    #[must_use]
    pub fn new(input: Vec<AgentItem>, max_turns: usize) -> Self {
        Self {
            max_turns,
            input,
            current_turn: Arc::new(Mutex::new(0)),
            output: Arc::new(Mutex::new(vec![])),
        }
    }

    /// Mark a new turn in the conversation and throw an error if max turns
    /// exceeded.
    pub async fn turn(&self) -> Result<(), AgentError> {
        let mut current_turn = self.current_turn.lock().await;
        *current_turn += 1;
        if *current_turn > self.max_turns {
            return Err(AgentError::MaxTurnsExceeded(self.max_turns));
        }
        Ok(())
    }

    /// Add message items to the run state.
    pub async fn append_messages(&self, messages: Vec<Message>) {
        if messages.is_empty() {
            return;
        }
        let mut output = self.output.lock().await;
        output.extend(messages.into_iter().map(AgentItem::Message));
    }

    /// Add AgentItems to the run state.
    pub async fn append_items(&self, items: Vec<AgentItem>) {
        if items.is_empty() {
            return;
        }
        let mut output = self.output.lock().await;
        output.extend(items);
    }

    /// Append a model response to the run state and return the created item.
    pub async fn append_model_response(&self, response: ModelResponse) -> AgentItem {
        let mut output = self.output.lock().await;
        let item = AgentItem::Model(response);
        output.push(item.clone());
        item
    }

    /// Get LLM messages to use in the `LanguageModelInput` for the turn
    #[must_use]
    pub async fn get_turn_messages(&self) -> Vec<Message> {
        let output = self.output.lock().await;
        let mut messages: Vec<Message> = Vec::new();
        let iter = self.input.iter().cloned().chain(output.iter().cloned());

        for item in iter {
            match item {
                AgentItem::Message(msg) => messages.push(msg),
                AgentItem::Model(model_response) => {
                    messages.push(Message::assistant(model_response.content));
                }
                AgentItem::Tool(tool) => {
                    let tool_part = Part::ToolResult(ToolResultPart {
                        tool_call_id: tool.tool_call_id,
                        tool_name: tool.tool_name,
                        content: tool.output,
                        is_error: Some(tool.is_error),
                    });

                    match messages.last_mut() {
                        Some(Message::Tool(last_tool_message)) => {
                            last_tool_message.content.push(tool_part);
                        }
                        _ => {
                            messages.push(Message::Tool(ToolMessage {
                                content: vec![tool_part],
                            }));
                        }
                    }
                }
            }
        }

        messages
    }

    #[must_use]
    pub async fn create_response(&self, final_content: Vec<Part>) -> AgentResponse {
        let output = self.output.lock().await;
        AgentResponse {
            content: final_content,
            output: output.clone(),
        }
    }
}
