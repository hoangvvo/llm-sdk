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
use std::sync::Arc;

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

    /// Process the model response and decide whether to continue the loop or
    /// return the response
    async fn process<'a>(
        &'a self,
        run_state: &'a RunState,
        parts: Vec<Part>,
        tools: Vec<Arc<dyn AgentTool<TCtx>>>,
    ) -> BoxedStream<'a, Result<ProcessEvents, AgentError>> {
        let context = self.context.clone();
        let stream = try_stream! {
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
                yield ProcessEvents::Response(parts);
                return;
            }

            for tool_call_part in tool_call_parts {
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

                let input_args = args.clone();
                let tool_name_value = agent_tool.name();
                let tool_description = agent_tool.description();
                let tool_res = start_tool_span(
                    &tool_call_id,
                    &tool_name_value,
                    &tool_description,
                    agent_tool.execute(args, &context, run_state),
                )
                .await
                .map_err(AgentError::ToolExecution)?;

                let item = AgentItemTool {
                    tool_call_id,
                    tool_name,
                    input: input_args,
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

            loop {
                let (input, tools) = self.get_turn_params(&state).await?;
                let model_response = self.params.model.generate(input).await?;
                let content = model_response.content.clone();

                state.append_model_response(model_response).await;

                let mut process_stream = self.process(&state, content, tools).await;

                while let Some(event) = process_stream.next().await {
                    let event = event?;
                    match event {
                        ProcessEvents::Item(items) => {
                            state.append_item(items).await;
                        }
                        ProcessEvents::Response(final_content) => {
                            return Ok(state.create_response(final_content).await);
                        }
                        ProcessEvents::Next => {
                            /* continue the loop */
                            break;
                        }
                    }
                }

                state.turn().await?;
            }
        })
        .await
    }

    /// Run a streaming execution of the agent.
    pub async fn run_stream(&self, request: RunSessionRequest) -> Result<AgentStream, AgentError> {
        let RunSessionRequest { input } = request;
        let state = Arc::new(RunState::new(input, self.params.max_turns));

        let session = Arc::new(Self {
            params: self.params.clone(),
            context: self.context.clone(),
            system_prompt: self.system_prompt.clone(),
            toolkit_sessions: self.toolkit_sessions.clone(),
        });

        let stream = async_stream::try_stream! {
            loop {
                let (input, tools) = session.get_turn_params(&state).await?;

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

                let (item, index) = state.append_model_response(model_response).await;
                yield AgentStreamEvent::Item(AgentStreamItemEvent { index, item });

                let mut process_stream = session.process(&state, content, tools).await;

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
                            /* continue the loop */
                        }
                    }
                }

                state.turn().await?;
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

        let mut tools: Vec<Arc<dyn AgentTool<TCtx>>> = self.params.tools.clone();

        for session in self.toolkit_sessions.iter() {
            if let Some(prompt) = session.system_prompt() {
                if !prompt.is_empty() {
                    system_prompts.push(prompt);
                }
            }

            let toolkit_tools = session.tools();
            tools.extend(toolkit_tools);
        }

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

    /// Add `AgentItems` to the run state and return the index of the added
    /// item.
    pub async fn append_item(&self, item: AgentItem) -> usize {
        let mut output: futures::lock::MutexGuard<'_, Vec<AgentItem>> = self.output.lock().await;
        output.push(item);
        output.len() - 1
    }

    /// Append a model response to the run state and return the created item and
    /// its index.
    pub async fn append_model_response(&self, response: ModelResponse) -> (AgentItem, usize) {
        let mut output = self.output.lock().await;
        let item = AgentItem::Model(response);
        output.push(item.clone());
        (item, output.len() - 1)
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
