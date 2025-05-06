import {
  arrayBufferToBase64,
  base64ToArrayBuffer,
  mergeInt16Arrays,
} from "./audio.utils.ts";
import { InvariantError } from "./errors.ts";
import type {
  AudioFormat,
  AudioPart,
  ContentDelta,
  ImagePart,
  ImagePartDelta,
  ModelResponse,
  ModelUsage,
  Part,
  PartialModelResponse,
  ReasoningPart,
  ReasoningPartDelta,
  TextPartDelta,
  ToolCallPart,
  ToolCallPartDelta,
} from "./types.ts";
import { sumModelUsage } from "./usage.utils.ts";

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

    case "tool-call": {
      const toolCallPartDelta: ToolCallPartDelta = {
        type: "tool-call",
        tool_name: delta.part.tool_name ?? "",
        args: delta.part.args ?? "",
      };
      if (delta.part.tool_call_id) {
        toolCallPartDelta.tool_call_id = delta.part.tool_call_id;
      }
      if (delta.part.id) {
        toolCallPartDelta.id = delta.part.id;
      }
      return toolCallPartDelta;
    }

    case "image": {
      const imagePartDelta: ImagePartDelta = {
        type: "image",
        image_data: delta.part.image_data ?? "",
      };
      if (delta.part.mime_type) {
        imagePartDelta.mime_type = delta.part.mime_type;
      }
      if (typeof delta.part.width === "number") {
        imagePartDelta.width = delta.part.width;
      }
      if (typeof delta.part.height === "number") {
        imagePartDelta.height = delta.part.height;
      }
      if (delta.part.id) {
        imagePartDelta.id = delta.part.id;
      }
      return imagePartDelta;
    }

    case "audio": {
      const audioData: AccumulatedAudioData = {
        type: "audio",
        audioDataChunks: delta.part.audio_data ? [delta.part.audio_data] : [],
      };
      if (delta.part.format) {
        audioData.format = delta.part.format;
      }
      if (typeof delta.part.sample_rate === "number") {
        audioData.sampleRate = delta.part.sample_rate;
      }
      if (typeof delta.part.channels === "number") {
        audioData.channels = delta.part.channels;
      }
      if (delta.part.transcript) {
        audioData.transcript = delta.part.transcript;
      }
      if (delta.part.id) {
        audioData.id = delta.part.id;
      }
      return audioData;
    }

    case "reasoning":
      return {
        ...delta.part,
      };
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
    case "text": {
      const existingPart = existing as TextPartDelta;
      existingPart.text += delta.part.text;
      break;
    }
    case "tool-call": {
      const existingPart = existing as ToolCallPartDelta;
      if (delta.part.tool_name) {
        existingPart.tool_name =
          (existingPart.tool_name ?? "") + delta.part.tool_name;
      }
      if (delta.part.tool_call_id) {
        existingPart.tool_call_id = delta.part.tool_call_id;
      }
      if (delta.part.args) {
        existingPart.args = (existingPart.args ?? "") + delta.part.args;
      }
      if (delta.part.id) {
        existingPart.id = delta.part.id;
      }
      break;
    }

    case "image": {
      const existingPart = existing as ImagePartDelta;
      if (delta.part.image_data) {
        existingPart.image_data =
          (existingPart.image_data ?? "") + delta.part.image_data;
      }
      if (delta.part.mime_type) {
        existingPart.mime_type = delta.part.mime_type;
      }
      if (typeof delta.part.width === "number") {
        existingPart.width = delta.part.width;
      }
      if (typeof delta.part.height === "number") {
        existingPart.height = delta.part.height;
      }
      if (delta.part.id) {
        existingPart.id = delta.part.id;
      }
      break;
    }

    case "audio": {
      const existingPart = existing as AccumulatedAudioData;
      if (delta.part.audio_data) {
        existingPart.audioDataChunks.push(delta.part.audio_data);
      }
      if (delta.part.format) {
        existingPart.format = delta.part.format;
      }
      if (delta.part.sample_rate) {
        existingPart.sampleRate = delta.part.sample_rate;
      }
      if (delta.part.channels) {
        existingPart.channels = delta.part.channels;
      }
      if (delta.part.transcript) {
        existingPart.transcript = existingPart.transcript ?? "";
        existingPart.transcript += delta.part.transcript;
      }
      if (delta.part.id) {
        existingPart.id = delta.part.id;
      }
      break;
    }
    case "reasoning": {
      const existingPart = existing as ReasoningPartDelta;
      if (delta.part.text) {
        existingPart.text = existingPart.text + delta.part.text;
      }
      if (delta.part.signature) {
        existingPart.signature = delta.part.signature;
      }
      if (delta.part.id) {
        existingPart.id = delta.part.id;
      }
      break;
    }
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
    const toolCalPart: ToolCallPart = {
      type: "tool-call",
      tool_call_id: data.tool_call_id,
      tool_name: data.tool_name,
      args: JSON.parse(data.args ?? "{}") as Record<string, unknown>,
    };
    if (data.id) {
      toolCalPart.id = data.id;
    }
    return toolCalPart;
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

  const imagePart: ImagePart = {
    type: "image",
    image_data: data.image_data,
    mime_type: data.mime_type,
  };
  if (typeof data.width === "number") {
    imagePart.width = data.width;
  }
  if (typeof data.height === "number") {
    imagePart.height = data.height;
  }
  if (data.id) {
    imagePart.id = data.id;
  }
  return imagePart;
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

  const audioPart: AudioPart = {
    type: "audio",
    audio_data: audioData,
    format: data.format,
  };
  if (typeof data.sampleRate === "number") {
    audioPart.sample_rate = data.sampleRate;
  }
  if (typeof data.channels === "number") {
    audioPart.channels = data.channels;
  }
  if (data.transcript) {
    audioPart.transcript = data.transcript;
  }
  if (data.id) {
    audioPart.id = data.id;
  }
  return audioPart;
}

/**
 * Creates a reasoning part from accumulated reasoning data
 */
function createReasoningPart(data: ReasoningPartDelta): Part {
  const reasoningPart: ReasoningPart = {
    type: "reasoning",
    text: data.text,
  };
  if (data.signature) {
    reasoningPart.signature = data.signature;
  }
  if (data.id) {
    reasoningPart.id = data.id;
  }
  return reasoningPart;
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
      return createReasoningPart(data);
  }
}

/**
 * Manages the accumulation and merging of content deltas for streaming responses
 */
export class StreamAccumulator {
  readonly #accumulatedParts = new Map<number, AccumulatedData>();
  #accumulatedUsage?: ModelUsage;
  #accumulatedCost?: number;

  /**
   * Adds a chunk of content deltas to the accumulator
   */
  addPartial(partial: PartialModelResponse): void {
    if (partial.delta) {
      this.#processDelta(partial.delta);
    }
    if (partial.usage) {
      this.#processUsage(partial.usage, partial.cost);
    }
  }

  /**
   * Computes the final response from accumulated deltas
   */
  computeResponse(): ModelResponse {
    const content = Array.from(this.#accumulatedParts.entries())
      .sort(([a], [b]) => a - b)
      .map(([index, data]) => createPart(data, index));
    const response: ModelResponse = { content };
    if (this.#accumulatedUsage) {
      response.usage = this.#accumulatedUsage;
    }
    if (this.#accumulatedCost !== undefined) {
      response.cost = this.#accumulatedCost;
    }
    return response;
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

  #processUsage(usage: ModelUsage, cost?: number): void {
    this.#accumulatedUsage = this.#accumulatedUsage ?? {
      input_tokens: 0,
      output_tokens: 0,
    };
    this.#accumulatedUsage = sumModelUsage([this.#accumulatedUsage, usage]);

    if (cost) {
      this.#accumulatedCost = this.#accumulatedCost ?? 0;
      this.#accumulatedCost += cost;
    }
  }
}
