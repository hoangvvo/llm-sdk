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
  ImagePartDelta,
  ModelResponse,
  ModelUsage,
  Part,
  PartialModelResponse,
  ReasoningPartDelta,
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
  audio_id?: string;
}

type AccumulatedData =
  | TextPartDelta
  | ToolCallPartDelta
  | ImagePartDelta
  | ReasoningPartDelta
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
      };

    case "tool-call":
      return {
        type: "tool-call",
        tool_name: delta.part.tool_name ?? "",
        ...(delta.part.tool_call_id && {
          tool_call_id: delta.part.tool_call_id,
        }),
        args: delta.part.args ?? "",
      };

    case "image":
      return {
        type: "image",
        image_data: delta.part.image_data ?? "",
        ...(delta.part.mime_type && { mime_type: delta.part.mime_type }),
        ...(typeof delta.part.width === "number" && {
          width: delta.part.width,
        }),
        ...(typeof delta.part.height === "number" && {
          height: delta.part.height,
        }),
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
        ...(delta.part.audio_id && { audio_id: delta.part.audio_id }),
      };

    case "reasoning":
      return {
        type: "reasoning",
        ...(delta.part.text && { text: delta.part.text }),
        ...(delta.part.summary && { summary: delta.part.summary }),
        ...(delta.part.signature && { signature: delta.part.signature }),
      };
  }
}

/**
 * Merges text delta with existing text data
 */
function mergeTextDelta(existing: TextPartDelta, delta: TextPartDelta): void {
  existing.text += delta.text;
}

/**
 * Merges tool call delta with existing tool call data
 */
function mergeToolCallDelta(
  existing: ToolCallPartDelta,
  delta: ToolCallPartDelta,
): void {
  if (delta.tool_name) {
    existing.tool_name = (existing.tool_name ?? "") + delta.tool_name;
  }
  if (delta.tool_call_id) {
    existing.tool_call_id = delta.tool_call_id;
  }
  if (delta.args) {
    existing.args = (existing.args ?? "") + delta.args;
  }
}

/**
 * Merges image delta with existing image data
 */
function mergeImageDelta(
  existing: ImagePartDelta,
  delta: ImagePartDelta,
): void {
  if (delta.image_data) {
    existing.image_data = (existing.image_data ?? "") + delta.image_data;
  }
  if (delta.mime_type) {
    existing.mime_type = delta.mime_type;
  }
  if (typeof delta.width === "number") {
    existing.width = delta.width;
  }
  if (typeof delta.height === "number") {
    existing.height = delta.height;
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
  if (delta.audio_id) {
    existing.audio_id = delta.audio_id;
  }
}

/**
 * Merges reasoning delta with existing reasoning data
 */
function mergeReasoningDelta(
  existing: ReasoningPartDelta,
  delta: ReasoningPartDelta,
): void {
  if (delta.text) {
    existing.text = (existing.text ?? "") + delta.text;
  }
  if (delta.summary) {
    existing.summary = (existing.summary ?? "") + delta.summary;
  }
  if (delta.signature) {
    existing.signature = delta.signature;
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
      mergeTextDelta(existing as TextPartDelta, delta.part);
      break;

    case "tool-call":
      mergeToolCallDelta(existing as ToolCallPartDelta, delta.part);
      break;

    case "image":
      mergeImageDelta(existing as ImagePartDelta, delta.part);
      break;

    case "audio":
      mergeAudioDelta(existing as AccumulatedAudioData, delta.part);
      break;

    case "reasoning":
      mergeReasoningDelta(existing as ReasoningPartDelta, delta.part);
  }
}

/**
 * Creates a text part from accumulated text data
 */
function createTextPart(data: TextPartDelta): Part {
  return {
    type: "text",
    text: data.text,
  };
}

/**
 * Creates a tool call part from accumulated tool call data
 */
function createToolCallPart(data: ToolCallPartDelta, index: number): Part {
  if (!data.tool_call_id || !data.tool_name) {
    throw new Error(
      `Missing required fields at index ${String(index)}: ` +
        `tool_call_id=${String(data.tool_call_id)}, tool_name=${String(data.tool_name)}`,
    );
  }

  try {
    return {
      type: "tool-call",
      tool_call_id: data.tool_call_id,
      tool_name: data.tool_name,
      args: JSON.parse(data.args ?? "{}") as Record<string, unknown>,
    };
  } catch (e) {
    throw new InvariantError(
      "",
      `Invalid tool call arguments: ${String(data.args)}: ${(e as Error).message}`,
    );
  }
}

function createImagePart(data: ImagePartDelta, index: number): Part {
  if (!data.image_data || !data.mime_type) {
    throw new Error(
      `Missing required fields at index ${String(index)}: ` +
        `image_data=${String(data.image_data)}, mime_type=${String(data.mime_type)}`,
    );
  }

  return {
    type: "image",
    image_data: data.image_data,
    mime_type: data.mime_type,
    ...(data.width && { width: data.width }),
    ...(data.height && { height: data.height }),
  };
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
    ...(data.audio_id && { audio_id: data.audio_id }),
  };
}

/**
 * Creates a reasoning part from accumulated reasoning data
 */
function createReasoningPart(data: ReasoningPartDelta, index: number): Part {
  if (data.text === undefined) {
    if (data.summary) {
      // Fall back to summary if text is not available
      data.text = data.summary;
    } else {
      throw new Error(
        `Missing required fields at index ${String(index)}: ` +
          `text=${String(data.text)}, summary=${String(data.summary)}, signature=${String(data.signature)}`,
      );
    }
  }

  return {
    type: "reasoning",
    text: data.text,
    ...(data.summary && { summary: data.summary }),
    ...(data.signature && { signature: data.signature }),
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

    case "image":
      return createImagePart(data, index);

    case "audio":
      return createAudioPart(data);

    case "reasoning":
      return createReasoningPart(data, index);
  }
}

/**
 * Manages the accumulation and merging of content deltas for streaming responses
 */
export class StreamAccumulator {
  readonly #accumulatedParts = new Map<number, AccumulatedData>();
  #accumulatedUsage?: ModelUsage;

  /**
   * Adds a chunk of content deltas to the accumulator
   */
  addPartial(partial: PartialModelResponse): void {
    if (partial.delta) {
      this.#processDelta(partial.delta);
    }
    if (partial.usage) {
      this.#processUsage(partial.usage);
    }
  }

  /**
   * Computes the final response from accumulated deltas
   */
  computeResponse(): ModelResponse {
    const content = Array.from(this.#accumulatedParts.entries())
      .sort(([a], [b]) => a - b)
      .map(([index, data]) => createPart(data, index));

    return {
      content,
      ...(this.#accumulatedUsage && { usage: this.#accumulatedUsage }),
    };
  }

  /**
   * Gets the number of accumulated parts
   */
  get size(): number {
    return this.#accumulatedParts.size;
  }

  /**
   * Checks if the accumulator has any data
   */
  get isEmpty(): boolean {
    return this.#accumulatedParts.size === 0;
  }

  /**
   * Processes a single delta, either merging with existing or creating new
   */
  #processDelta(delta: ContentDelta): void {
    const existing = this.#accumulatedParts.get(delta.index);

    if (existing) {
      mergeDelta(existing, delta);
    } else {
      this.#accumulatedParts.set(delta.index, initializeAccumulatedData(delta));
    }
  }

  #processUsage(usage: ModelUsage): void {
    this.#accumulatedUsage = this.#accumulatedUsage ?? { ...usage };

    this.#accumulatedUsage.input_tokens += usage.input_tokens;
    this.#accumulatedUsage.output_tokens += usage.output_tokens;
  }
}
