use crate::{
    instruction,
    opentelemetry::{start_tool_span, trace_agent_run, trace_agent_stream, AgentSpanMethod},
    toolkit::ToolkitSession,
    types::{AgentItemTool, AgentStream, AgentStreamEvent},
    AgentError, AgentItem, AgentParams, AgentResponse, AgentStreamItemEvent, AgentTool,
};
use async_stream::try_stream;
use futures::{
    future::{join_all, try_join_all},
    lock::Mutex,
    stream::StreamExt,
};
use llm_sdk::{
    boxed_stream::BoxedStream, LanguageModelInput, Message, ModelResponse, Part, StreamAccumulator,
    ToolCallPart, ToolMessage, ToolResultPart,
};
use std::{collections::HashSet, sync::Arc};

/// Manages the run session for an agent.
/// It initializes all necessary components for the agent to run
/// and handles the execution of the agent's tasks.
/// Once finished, the session cleans up any resources used during the run.
/// The session can be reused in multiple runs. `RunSession` binds to a specific
///
/// context value that is used to resolve instructions and invoke tools, while
/// input items remain per run and are supplied to each invocation.
pub struct RunSession<TCtx> {
    /// Agent configuration used during the run session.
    params: Arc<AgentParams<TCtx>>,
    /// The bound context value passed to instruction resolvers and tools.
    context: Arc<TCtx>,
    /// System prompt generated from the static instruction params.
    system_prompt: Option<String>,
    /// Toolkit sessions created for this run session.
    toolkit_sessions: Arc<Vec<Box<dyn ToolkitSession<TCtx> + Send + Sync>>>,
}

impl<TCtx> RunSession<TCtx>
where
    TCtx: Send + Sync + 'static,
{
    /// Creates a new run session and initializes dependencies
    #[allow(clippy::unused_async)]
    #[allow(clippy::too_many_arguments)]
    pub async fn new(params: Arc<AgentParams<TCtx>>, context: TCtx) -> Result<Self, AgentError> {
        let system_prompt = if params.instructions.is_empty() {
            None
        } else {
            Some(
                instruction::get_prompt(&params.instructions, &context)
                    .await
                    .map_err(AgentError::Init)?,
            )
        };

        let toolkit_sessions = Self::initialize(&params, &context).await?;

        Ok(Self {
            params,
            context: Arc::new(context),
            system_prompt,
            toolkit_sessions: Arc::new(toolkit_sessions),
        })
    }

    /// `process()` flow:
    /// 1. Peek latest run item to locate assistant content.
    ///
    ///    1a. Tail is user message -> emit `Next`. Go to 3.
    ///
    ///    1b. Tail is tool/tool message -> gather processed ids, backtrack to
    ///    assistant/model content. Go to 2.
    ///
    ///    1c. Tail is assistant/model -> use its content. Go to 2.
    ///
    /// 2. Scan assistant content for tool calls.
    ///
    ///    2a. Tool calls remaining -> execute unprocessed tools, emit each
    ///    `Item`, then emit `Next`. Go to 3.
    ///
    ///    2b. No tool calls -> emit `Response`. Go to 4.
    ///
    /// 3. Outer loop: bump turn, refresh params, request model response, append
    ///    it, then re-enter step 1.
    ///
    /// 4. Return final response to caller.
    #[allow(clippy::too_many_lines)]
    fn process<'a>(
        &'a self,
        run_state: &'a RunState,
        tools: Vec<Arc<dyn AgentTool<TCtx>>>,
    ) -> BoxedStream<'a, Result<ProcessEvents, AgentError>> {
        let context_val = self.context.clone();
        let stream = try_stream! {
            let items = run_state.items().await;
            // Examining the last items in the state determines the next step.
            let last_item = items.last().cloned().ok_or_else(|| {
                AgentError::Invariant("No items in the run state.".to_string())
            })?;

            let mut content: Option<Vec<Part>> = None;
            let mut processed_tool_call_ids: HashSet<String> = HashSet::new();

            match last_item {
                AgentItem::Model(model_response) => {
                    // ========== Case: Assistant Message [from AgentItemModelResponse] ==========
                    // Last item is a model response, process it
                    content = Some(model_response.content);
                }
                AgentItem::Message(message) => match message {
                    Message::Assistant(assistant_message) => {
                        // ========== Case: Assistant Message [from AgentItemMessage] ==========
                        // Last item is an assistant message, process it
                        content = Some(assistant_message.content);
                    }
                    Message::User(_) => {
                        // ========== Case: User Message ==========
                        // last item is a user message, so we need to generate a model response
                        yield ProcessEvents::Next;
                        return;
                    }
                    Message::Tool(tool_message) => {
                        // ========== Case: Tool Results (from AgentItemMessage) ==========
                        // Track the tool call ids that have been processed to avoid duplicate execution
                        for part in tool_message.content {
                            if let Part::ToolResult(result) = part {
                                processed_tool_call_ids.insert(result.tool_call_id);
                            }
                        }

                        // We are in the middle of processing tool results, the 2nd last item should be a model response
                        let previous_item = items
                            .len()
                            .checked_sub(2)
                            .and_then(|idx| items.get(idx))
                            .cloned()
                            .ok_or_else(|| {
                                AgentError::Invariant(
                                    "No preceding assistant content found before tool results.".to_string(),
                                )
                            })?;

                        let resolved = match previous_item {
                            AgentItem::Model(model_response) => model_response.content,
                            AgentItem::Message(prev_message) => match prev_message {
                                Message::Assistant(assistant_message) => assistant_message.content,
                                _ => {
                                    Err(AgentError::Invariant(
                                        "Expected a model item or assistant message before tool results.".to_string(),
                                    ))?
                                }
                            },
                            AgentItem::Tool(_) => {
                                Err(AgentError::Invariant(
                                    "Expected a model item or assistant message before tool results.".to_string(),
                                ))?
                            }
                        };
                        content = Some(resolved);
                    }
                },
                AgentItem::Tool(_) => {
                    // ========== Case: Tool Results (from AgentItemTool) ==========
                    // Each tool result is an individual item in this representation, so there could be other
                    // AgentItemTool before this one. We loop backwards to find the first non-tool item while also
                    // tracking the called tool ids to avoid duplicate execution
                    for item in items.into_iter().rev() {
                        match item {
                            AgentItem::Tool(tool_item) => {
                                processed_tool_call_ids.insert(tool_item.tool_call_id);
                                // Continue searching for the originating model/assistant item
                            }
                            AgentItem::Model(model_response) => {
                                // Found the originating model response
                                content = Some(model_response.content);
                                break;
                            }
                            AgentItem::Message(message) => match message {
                                Message::Tool(tool_message) => {
                                    // Collect all tool call ids in the tool message
                                    for part in tool_message.content {
                                        if let Part::ToolResult(result) = part {
                                            processed_tool_call_ids.insert(result.tool_call_id);
                                        }
                                    }
                                    // Continue searching for the originating model/assistant item
                                }
                                Message::Assistant(assistant_message) => {
                                    // Found the originating model response
                                    content = Some(assistant_message.content);
                                    break;
                                }
                                Message::User(_) => {
                                    Err(AgentError::Invariant(
                                        "Expected a model item or assistant message before tool results.".to_string(),
                                    ))?;
                                }
                            },
                        }
                    }
                }
            }

            let content = content
                .filter(|v| !v.is_empty())
                .ok_or_else(|| AgentError::Invariant(
                    "No assistant content found to process.".to_string(),
                ))?;

            let tool_call_parts: Vec<ToolCallPart> = content
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
                yield ProcessEvents::Response(content);
                return;
            }

            for tool_call_part in tool_call_parts {
                if processed_tool_call_ids.contains(&tool_call_part.tool_call_id)
                {
                    // Tool call has already been processed
                    continue;
                }

                let ToolCallPart {
                    tool_call_id,
                    tool_name,
                    args,
                    ..
                } = tool_call_part;

                let agent_tool = tools
                    .iter()
                    .find(|tool| tool.name() == tool_name)
                    .ok_or_else(|| {
                        AgentError::Invariant(format!("Tool {tool_name} not found for tool call"))
                    })?;

                let tool_name_value = agent_tool.name();
                let tool_description = agent_tool.description();
                let tool_res = start_tool_span(
                    &tool_call_id,
                    &tool_name_value,
                    &tool_description,
                    agent_tool.execute(args.clone(), &context_val, run_state),
                )
                .await
                .map_err(AgentError::ToolExecution)?;

                let item = AgentItemTool {
                    tool_call_id,
                    tool_name,
                    input: args,
                    output: tool_res.content,
                    is_error: tool_res.is_error,
                };

                yield ProcessEvents::Item(AgentItem::Tool(item));
            }

            yield ProcessEvents::Next;
        };

        BoxedStream::from_stream(stream)
    }

    /// Run a non-streaming execution of the agent.
    pub async fn run(&self, request: RunSessionRequest) -> Result<AgentResponse, AgentError> {
        let RunSessionRequest { input } = request;

        trace_agent_run(&self.params.name, AgentSpanMethod::Run, async move {
            let state = RunState::new(input, self.params.max_turns);
            let mut tools = self.get_tools();

            loop {
                let mut process_stream = self.process(&state, tools);

                while let Some(event) = process_stream.next().await {
                    let event = event?;
                    match event {
                        ProcessEvents::Item(item) => {
                            state.append_item(item).await;
                        }
                        ProcessEvents::Response(final_content) => {
                            return Ok(state.create_response(final_content).await);
                        }
                        ProcessEvents::Next => {
                            state.turn().await?;
                            break;
                        }
                    }
                }

                let (input, next_tools) = self.get_turn_params(&state).await?;
                tools = next_tools;

                let model_response = self.params.model.generate(input).await?;
                state.append_model_response(model_response).await;
            }
        })
        .await
    }

    /// Run a streaming execution of the agent.
    pub fn run_stream(&self, request: RunSessionRequest) -> Result<AgentStream, AgentError> {
        let RunSessionRequest { input } = request;
        let state = Arc::new(RunState::new(input, self.params.max_turns));

        let session = Arc::new(Self {
            params: self.params.clone(),
            context: self.context.clone(),
            system_prompt: self.system_prompt.clone(),
            toolkit_sessions: self.toolkit_sessions.clone(),
        });

        let stream = async_stream::try_stream! {
            let mut tools = session.get_tools();

            loop {
                let mut process_stream = session.process(&state, tools);

                while let Some(event) = process_stream.next().await {
                    let event = event?;

                    match event {
                        ProcessEvents::Item(item) => {
                            let index = state.append_item(item.clone()).await;
                            yield AgentStreamEvent::Item(AgentStreamItemEvent { index, item });
                        }
                        ProcessEvents::Response(final_content) => {
                            let response = state.create_response(final_content).await;
                            yield AgentStreamEvent::Response(response);
                            return;
                        }
                        ProcessEvents::Next => {
                            state.turn().await?;
                            break;
                        }
                    }
                }

                let (input, next_tools) = session.get_turn_params(&state).await?;
                tools = next_tools;

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

                let (item, index) = state.append_model_response(model_response).await;
                yield AgentStreamEvent::Item(AgentStreamItemEvent { index, item });
            }
        };

        Ok(trace_agent_stream(&self.params.name, stream))
    }

    pub async fn close(self) -> Result<(), AgentError> {
        if let Ok(toolkit_sessions) = Arc::try_unwrap(self.toolkit_sessions) {
            let _ = join_all(
                toolkit_sessions
                    .into_iter()
                    .map(super::toolkit::ToolkitSession::close),
            )
            .await;
        }

        Ok(())
    }

    async fn initialize(
        params: &AgentParams<TCtx>,
        context: &TCtx,
    ) -> Result<Vec<Box<dyn ToolkitSession<TCtx> + Send + Sync>>, AgentError> {
        let toolkit_sessions = if params.toolkits.is_empty() {
            Vec::new()
        } else {
            let futures = params.toolkits.iter().map(|toolkit| async move {
                toolkit
                    .create_session(context)
                    .await
                    .map_err(AgentError::Init)
            });

            try_join_all(futures).await?
        };
        Ok(toolkit_sessions)
    }

    async fn get_turn_params(
        &self,
        state: &RunState,
    ) -> Result<(LanguageModelInput, Vec<Arc<dyn AgentTool<TCtx>>>), AgentError> {
        let mut system_prompts = Vec::new();
        if let Some(prompt) = &self.system_prompt {
            if !prompt.is_empty() {
                system_prompts.push(prompt.clone());
            }
        }

        for session in self.toolkit_sessions.iter() {
            if let Some(prompt) = session.system_prompt() {
                if !prompt.is_empty() {
                    system_prompts.push(prompt);
                }
            }
        }

        let tools = self.get_tools();

        let mut input = LanguageModelInput {
            messages: state.get_turn_messages().await,
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
        };

        if !system_prompts.is_empty() {
            input.system_prompt = Some(system_prompts.join("\n"));
        }

        if !tools.is_empty() {
            let sdk_tools = tools.iter().map(|tool| tool.as_ref().into()).collect();
            input.tools = Some(sdk_tools);
        }

        Ok((input, tools))
    }

    fn get_tools(&self) -> Vec<Arc<dyn AgentTool<TCtx>>> {
        let mut tools: Vec<Arc<dyn AgentTool<TCtx>>> = self.params.tools.clone();
        for session in self.toolkit_sessions.iter() {
            let toolkit_tools = session.tools();
            tools.extend(toolkit_tools);
        }
        tools
    }
}
/// `RunSessionRequest` contains the input items used for a run.
pub struct RunSessionRequest {
    /// Input holds the items for this run, such as LLM messages.
    pub input: Vec<AgentItem>,
}

enum ProcessEvents {
    // Emit when a new item is generated
    Item(AgentItem),
    //  Emit when the final response is ready
    Response(Vec<Part>),
    // Emit when the loop should continue to the next iteration
    Next,
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
    fn new(input: Vec<AgentItem>, max_turns: usize) -> Self {
        Self {
            max_turns,
            input,
            current_turn: Arc::new(Mutex::new(0)),
            output: Arc::new(Mutex::new(vec![])),
        }
    }

    /// Mark a new turn in the conversation and throw an error if max turns
    /// exceeded.
    async fn turn(&self) -> Result<(), AgentError> {
        let mut current_turn = self.current_turn.lock().await;
        *current_turn += 1;
        if *current_turn > self.max_turns {
            return Err(AgentError::MaxTurnsExceeded(self.max_turns));
        }
        Ok(())
    }

    /// Add `AgentItems` to the run state and return the index of the added
    /// item.
    async fn append_item(&self, item: AgentItem) -> usize {
        let mut output: futures::lock::MutexGuard<'_, Vec<AgentItem>> = self.output.lock().await;
        output.push(item);
        output.len() - 1
    }

    /// Return all items in the run, both input and output.
    pub async fn items(&self) -> Vec<AgentItem> {
        let output = self.output.lock().await;
        self.input
            .iter()
            .cloned()
            .chain(output.iter().cloned())
            .collect()
    }

    /// Append a model response to the run state and return the created item and
    /// its index.
    async fn append_model_response(&self, response: ModelResponse) -> (AgentItem, usize) {
        let mut output = self.output.lock().await;
        let item = AgentItem::Model(response);
        output.push(item.clone());
        (item, output.len() - 1)
    }

    /// Get LLM messages to use in the `LanguageModelInput` for the turn
    #[must_use]
    async fn get_turn_messages(&self) -> Vec<Message> {
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
    async fn create_response(&self, final_content: Vec<Part>) -> AgentResponse {
        let output = self.output.lock().await;
        AgentResponse {
            content: final_content,
            output: output.clone(),
        }
    }
}
