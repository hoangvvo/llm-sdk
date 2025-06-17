import type { AudioFormat } from "./types.ts";

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
