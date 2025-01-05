import type { AudioFormat } from "./types.js";

export function base64ToArrayBuffer(base64: string): ArrayBuffer {
  const binaryString = atob(base64);
  const len = binaryString.length;
  const bytes = new Uint8Array(len);
  for (let i = 0; i < len; i++) {
    bytes[i] = binaryString.charCodeAt(i);
  }
  return bytes.buffer;
}

export function arrayBufferToBase64(
  arrayBuffer: ArrayBuffer | Float32Array | Int16Array,
): string {
  if (arrayBuffer instanceof Float32Array) {
    arrayBuffer = floatTo16BitPCM(arrayBuffer);
  } else if (arrayBuffer instanceof Int16Array) {
    arrayBuffer = arrayBuffer.buffer as ArrayBuffer;
  }
  let binary = "";
  const bytes = new Uint8Array(arrayBuffer);
  const chunkSize = 0x8000; // 32KB chunk size
  for (let i = 0; i < bytes.length; i += chunkSize) {
    const chunk = bytes.subarray(i, i + chunkSize);
    binary += String.fromCharCode(...chunk);
  }
  return btoa(binary);
}

export function floatTo16BitPCM(float32Array: Float32Array): ArrayBuffer {
  const buffer = new ArrayBuffer(float32Array.length * 2);
  const view = new DataView(buffer);
  let offset = 0;
  for (let i = 0; i < float32Array.length; i++, offset += 2) {
    // eslint-disable-next-line @typescript-eslint/no-non-null-assertion
    const s = Math.max(-1, Math.min(1, float32Array[i]!));
    view.setInt16(offset, s < 0 ? s * 0x8000 : s * 0x7fff, true);
  }
  return buffer;
}

export function mergeInt16Arrays(arrayBuffers: unknown[]) {
  const int16Arrays = arrayBuffers.map((arrayBuffer, index) => {
    if (arrayBuffer instanceof ArrayBuffer) {
      return new Int16Array(arrayBuffer);
    }
    if (arrayBuffer instanceof Int16Array) {
      return arrayBuffer;
    }
    throw new Error(
      `Item at index ${String(index)} must be ArrayBuffer or Int16Array`,
    );
  });
  const totalLength = int16Arrays.reduce((acc, cur) => acc + cur.length, 0);
  const newValues = new Int16Array(totalLength);
  let offset = 0;
  int16Arrays.forEach((int16Array) => {
    newValues.set(int16Array, offset);
    offset += int16Array.length;
  });
  return newValues;
}

const audioFormatToMimeTypeMap: Record<AudioFormat, string> = {
  wav: "audio/wav",
  linear16: "audio/L16",
  flac: "audio/flac",
  mulaw: "audio/basic",
  alaw: "audio/basic",
  mp3: "audio/mpeg",
  opus: 'audio/ogg; codecs="opus"',
  aac: "audio/aac",
};

export function mapAudioFormatToMimeType(format: AudioFormat): string {
  return audioFormatToMimeTypeMap[format] || "application/octet-stream";
}

export function mapMimeTypeToAudioFormat(mimeType: string): AudioFormat {
  const format = Object.keys(audioFormatToMimeTypeMap).find((key) =>
    mimeType.includes(audioFormatToMimeTypeMap[key as AudioFormat]),
  );
  if (!format) {
    throw new Error(`Unsupported audio format for mime type: ${mimeType}`);
  }
  return format as AudioFormat;
}
