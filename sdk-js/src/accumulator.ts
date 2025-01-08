import {
  arrayBufferToBase64,
  base64ToArrayBuffer,
  mergeInt16Arrays,
} from "./audio.utils.ts";
import { InvariantError } from "./errors.ts";
import type {
  AudioFormat,
  AudioPartDelta,
  ContentDelta,
  ModelResponse,
  ModelUsage,
  Part,
  PartialModelResponse,
  TextPartDelta,
  ToolCallPartDelta,
} from "./types.ts";

/**
 * Internal representation of accumulated audio data with chunks stored separately
 */
interface AccumulatedAudioData {
  type: "audio";
  audioDataChunks: string[];
  format?: AudioFormat;
  sampleRate?: number;
  channels?: number;
  transcript?: string;
  id?: string;
}

/**
 * Internal representation of accumulated text data
 */
interface AccumulatedTextData {
  type: "text";
  text: string;
  id?: string;
}

/**
 * Internal representation of accumulated tool call data
 */
interface AccumulatedToolCallData {
  type: "tool-call";
  toolName: string;
  toolCallId?: string;
  args: string;
  id?: string;
}

type AccumulatedData =
  | AccumulatedTextData
  | AccumulatedToolCallData
  | AccumulatedAudioData;

/**
 * Initializes accumulated data from a delta
 */
function initializeAccumulatedData(delta: ContentDelta): AccumulatedData {
  switch (delta.part.type) {
    case "text":
      return {
        type: "text",
        text: delta.part.text,
        ...(delta.part.id && { id: delta.part.id }),
      };

    case "tool-call":
      return {
        type: "tool-call",
        toolName: delta.part.tool_name ?? "",
        ...(delta.part.tool_call_id && {
          toolCallId: delta.part.tool_call_id,
        }),
        args: delta.part.args ?? "",
        ...(delta.part.id && { id: delta.part.id }),
      };

    case "audio":
      return {
        type: "audio",
        audioDataChunks: delta.part.audio_data ? [delta.part.audio_data] : [],
        ...(delta.part.format && { format: delta.part.format }),
        ...(typeof delta.part.sample_rate === "number" && {
          sampleRate: delta.part.sample_rate,
        }),
        ...(typeof delta.part.channels === "number" && {
          channels: delta.part.channels,
        }),
        ...(delta.part.transcript && {
          transcript: delta.part.transcript,
        }),
        ...(delta.part.id && { id: delta.part.id }),
      };
  }
}

/**
 * Merges text delta with existing text data
 */
function mergeTextDelta(
  existing: AccumulatedTextData,
  delta: TextPartDelta,
): void {
  existing.text += delta.text;
  if (delta.id) {
    existing.id = delta.id;
  }
}

/**
 * Merges tool call delta with existing tool call data
 */
function mergeToolCallDelta(
  existing: AccumulatedToolCallData,
  delta: ToolCallPartDelta,
): void {
  if (delta.tool_name) {
    existing.toolName += delta.tool_name;
  }
  if (delta.tool_call_id) {
    existing.toolCallId = delta.tool_call_id;
  }
  if (delta.args) {
    existing.args += delta.args;
  }
  if (delta.id) {
    existing.id = delta.id;
  }
}

/**
 * Merges audio delta with existing audio data
 */
function mergeAudioDelta(
  existing: AccumulatedAudioData,
  delta: AudioPartDelta,
): void {
  if (delta.audio_data) {
    existing.audioDataChunks.push(delta.audio_data);
  }
  if (delta.format) {
    existing.format = delta.format;
  }
  if (delta.sample_rate) {
    existing.sampleRate = delta.sample_rate;
  }
  if (delta.channels) {
    existing.channels = delta.channels;
  }
  if (delta.transcript) {
    existing.transcript = existing.transcript ?? "";
    existing.transcript += delta.transcript;
  }
  if (delta.id) {
    existing.id = delta.id;
  }
}

/**
 * Merges an incoming delta with existing accumulated data
 */
function mergeDelta(existing: AccumulatedData, delta: ContentDelta): void {
  // Type guard to ensure matching types
  if (existing.type !== delta.part.type) {
    throw new Error(
      `Type mismatch at index ${String(delta.index)}: ` +
        `existing type is ${existing.type}, incoming type is ${delta.part.type}`,
    );
  }

  switch (delta.part.type) {
    case "text":
      mergeTextDelta(existing as AccumulatedTextData, delta.part);
      break;

    case "tool-call":
      mergeToolCallDelta(existing as AccumulatedToolCallData, delta.part);
      break;

    case "audio":
      mergeAudioDelta(existing as AccumulatedAudioData, delta.part);
      break;
  }
}

/**
 * Creates a text part from accumulated text data
 */
function createTextPart(data: AccumulatedTextData): Part {
  return {
    type: "text",
    text: data.text,
    ...(data.id && { id: data.id }),
  };
}

/**
 * Creates a tool call part from accumulated tool call data
 */
function createToolCallPart(
  data: AccumulatedToolCallData,
  index: number,
): Part {
  if (!data.toolCallId || !data.toolName) {
    throw new Error(
      `Missing required fields at index ${String(index)}: ` +
        `tool_call_id=${String(data.toolCallId)}, tool_name=${data.toolName}`,
    );
  }

  try {
    return {
      type: "tool-call",
      tool_call_id: data.toolCallId,
      tool_name: data.toolName,
      args: JSON.parse(data.args) as Record<string, unknown>,
      ...(data.id && { id: data.id }),
    };
  } catch (e) {
    throw new InvariantError(
      "",
      `Invalid tool call arguments: ${data.args}: ${(e as Error).message}`,
    );
  }
}

/**
 * Creates an audio part from accumulated audio data
 */
function createAudioPart(data: AccumulatedAudioData): Part {
  if (data.format !== "linear16") {
    throw new Error(
      `Only linear16 format is supported for audio concatenation. ` +
        `Received: ${data.format ?? "undefined"}`,
    );
  }

  const audioArrays = data.audioDataChunks.map(base64ToArrayBuffer);
  const concatenated = mergeInt16Arrays(audioArrays);
  const audioData = arrayBufferToBase64(concatenated);

  return {
    type: "audio",
    audio_data: audioData,
    format: data.format,
    ...(data.sampleRate && { sample_rate: data.sampleRate }),
    ...(data.channels && { channels: data.channels }),
    ...(data.transcript && { transcript: data.transcript }),
    ...(data.id && { id: data.id }),
  };
}

/**
 * Creates a final Part from accumulated data
 */
function createPart(data: AccumulatedData, index: number): Part {
  switch (data.type) {
    case "text":
      return createTextPart(data);

    case "tool-call":
      return createToolCallPart(data, index);

    case "audio":
      return createAudioPart(data);
  }
}

/**
 * Manages the accumulation and merging of content deltas for streaming responses
 */
export class StreamAccumulator {
  private readonly accumulatedParts = new Map<number, AccumulatedData>();
  private accumulatedUsage?: ModelUsage;

  /**
   * Adds a chunk of content deltas to the accumulator
   */
  addPartial(partial: PartialModelResponse): void {
    if (partial.delta) {
      this.processDelta(partial.delta);
    }
    if (partial.usage) {
      this.processUsage(partial.usage);
    }
  }

  /**
   * Computes the final response from accumulated deltas
   */
  computeResponse(): ModelResponse {
    const content = Array.from(this.accumulatedParts.entries())
      .sort(([a], [b]) => a - b)
      .map(([index, data]) => createPart(data, index));

    return {
      content,
      ...(this.accumulatedUsage && { usage: this.accumulatedUsage }),
    };
  }

  /**
   * Gets the number of accumulated parts
   */
  get size(): number {
    return this.accumulatedParts.size;
  }

  /**
   * Checks if the accumulator has any data
   */
  get isEmpty(): boolean {
    return this.accumulatedParts.size === 0;
  }

  /**
   * Processes a single delta, either merging with existing or creating new
   */
  private processDelta(delta: ContentDelta): void {
    const existing = this.accumulatedParts.get(delta.index);

    if (existing) {
      mergeDelta(existing, delta);
    } else {
      this.accumulatedParts.set(delta.index, initializeAccumulatedData(delta));
    }
  }

  private processUsage(usage: ModelUsage): void {
    this.accumulatedUsage = this.accumulatedUsage ?? { ...usage };

    this.accumulatedUsage.input_tokens += usage.input_tokens;
    this.accumulatedUsage.output_tokens += usage.output_tokens;
  }
}
