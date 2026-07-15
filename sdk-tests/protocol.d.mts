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
}>;

export function getTestCaseInfo(testCaseName: string): {
  name: string;
  stage_count: number;
};

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
  profile?: string;
}): { ok: true };
