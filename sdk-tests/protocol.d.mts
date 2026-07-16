export interface StageContext {
  stages: Array<{
    assistant: unknown[];
    tool_calls: unknown[];
  }>;
}

export interface PreparedStage {
  method: "generate" | "stream";
  input: unknown;
  stage_count: number;
}

export const TEST_CASE_NAMES: Readonly<{
  GENERATE_TEXT: "generate_text";
  STREAM_TEXT: "stream_text";
  GENERATE_WITH_SYSTEM_PROMPT: "generate_with_system_prompt";
  GENERATE_TOOL_CALL: "generate_tool_call";
  STREAM_TOOL_CALL: "stream_tool_call";
  GENERATE_TEXT_FROM_TOOL_RESULT: "generate_text_from_tool_result";
  STREAM_TEXT_FROM_TOOL_RESULT: "stream_text_from_tool_result";
  GENERATE_TEXT_FROM_IMAGE_TOOL_RESULT: "generate_text_from_image_tool_result";
  GENERATE_PARALLEL_TOOL_CALLS: "generate_parallel_tool_calls";
  STREAM_PARALLEL_TOOL_CALLS: "stream_parallel_tool_calls";
  STREAM_PARALLEL_TOOL_CALLS_OF_SAME_NAME: "stream_parallel_tool_calls_of_same_name";
  STRUCTURED_RESPONSE_FORMAT: "structured_response_format";
  SOURCE_PART_INPUT: "source_part_input";
  GENERATE_AUDIO: "generate_audio";
  STREAM_AUDIO: "stream_audio";
  GENERATE_IMAGE: "generate_image";
  STREAM_IMAGE: "stream_image";
  GENERATE_IMAGE_INPUT: "generate_image_input";
  STREAM_IMAGE_INPUT: "stream_image_input";
  GENERATE_WEB_SEARCH: "generate_web_search";
  STREAM_WEB_SEARCH: "stream_web_search";
  GENERATE_REASONING: "generate_reasoning";
  STREAM_REASONING: "stream_reasoning";
  MIXED_CONTENT_CONVERSATION: "mixed_content_conversation";
  MULTI_STEP_TOOL_WORKFLOW: "multi_step_tool_workflow";
  STREAM_NESTED_TOOL_ARGUMENTS: "stream_nested_tool_arguments";
  STREAM_NESTED_STRUCTURED_RESPONSE: "stream_nested_structured_response";
  GENERATE_EXPLICIT_TEXT_OPTIONS: "generate_explicit_text_options";
  CONSECUTIVE_USER_MESSAGES: "consecutive_user_messages";
  ASSISTANT_MESSAGE_HISTORY: "assistant_message_history";
  AUTO_TOOL_CHOICE_TEXT_RESPONSE: "auto_tool_choice_text_response";
  PARALLEL_TOOL_RESULTS: "parallel_tool_results";
  SEQUENTIAL_TOOL_CHAIN: "sequential_tool_chain";
  STRUCTURED_DATA_EXTRACTION: "structured_data_extraction";
  MULTIPLE_SOURCE_DOCUMENTS: "multiple_source_documents";
  STREAM_UNICODE_TEXT: "stream_unicode_text";
  MULTI_PART_TOOL_RESULT: "multi_part_tool_result";
  REASONING_TOOL_CONTINUATION: "reasoning_tool_continuation";
  ANTHROPIC_GENERATE_REFUSAL: "anthropic_generate_refusal";
  ANTHROPIC_STREAM_REFUSAL: "anthropic_stream_refusal";
}>;

export function getTestCaseInfo(testCaseName: string): {
  name: string;
  stage_count: number;
};

export function getTestCasesByGroup(group: string): string[];

export function prepareStage(options: {
  test_case: string;
  stage: number;
  context?: StageContext;
  profile?: string;
}): PreparedStage;

export function validateOutput(options: {
  test_case: string;
  stage: number;
  content: unknown[];
  response?: unknown;
  stream?: unknown;
  profile?: string;
}): { ok: true };

export function validateError(options: {
  test_case: string;
  stage: number;
  error: { kind: string; message: string };
  profile?: string;
}): { ok: true };
