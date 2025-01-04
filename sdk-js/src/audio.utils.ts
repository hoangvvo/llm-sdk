import type { AudioFormat } from "./types.js";

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
