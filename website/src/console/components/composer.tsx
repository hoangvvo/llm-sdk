import type { AudioFormat, Part } from "@hoangvvo/llm-sdk";
import type { ClipboardEvent as ReactClipboardEvent } from "react";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { createId } from "../lib/utils.ts";
import { WavPacker } from "../lib/wavtools/wav_packer.ts";
import { WavRecorder } from "../lib/wavtools/wav_recorder.ts";

interface ComposerProps {
  isStreaming: boolean;
  onError: (message: string | null) => void;
  onSend: (parts: Part[]) => Promise<void> | void;
  onAbort: () => void;
  disabled?: boolean;
}

interface Attachment {
  id: string;
  file: File;
  kind: "image" | "audio";
  preview: string;
}

export function Composer({
  isStreaming,
  onError,
  onSend,
  onAbort,
  disabled = false,
}: ComposerProps) {
  const [inputText, setInputText] = useState("");
  const [attachments, setAttachments] = useState<Attachment[]>([]);
  const [isRecording, setIsRecording] = useState(false);
  const recorderRef = useRef<WavRecorder | null>(null);
  const recordingChunksRef = useRef<Int16Array[]>([]);
  const recordingSampleRateRef = useRef(16000);
  const formRef = useRef<HTMLFormElement | null>(null);
  const imageInputRef = useRef<HTMLInputElement | null>(null);
  const audioInputRef = useRef<HTMLInputElement | null>(null);

  useEffect(() => {
    return () => {
      if (recorderRef.current) {
        recorderRef.current.end().catch(() => {
          /* noop */
        });
      }
      revokePreviews(attachments);
    };
  }, [attachments]);

  const canSubmit = useMemo(() => {
    const trimmed = inputText.trim();
    return (
      !disabled &&
      !isStreaming &&
      !isRecording &&
      (trimmed.length > 0 || attachments.length > 0)
    );
  }, [attachments.length, disabled, inputText, isRecording, isStreaming]);

  const handleFileAdd = useCallback(
    (files: FileList | File[] | null, kind: Attachment["kind"]) => {
      if (!files) {
        return;
      }
      const list = Array.isArray(files) ? files : Array.from(files);
      if (list.length === 0) {
        return;
      }
      const additions: Attachment[] = list.map((file) => ({
        id: createId(),
        file,
        kind,
        preview: URL.createObjectURL(file),
      }));
      setAttachments((prev) => [...prev, ...additions]);
    },
    [],
  );

  const handleAttachmentRemove = useCallback((id: string) => {
    setAttachments((prev) => {
      const next: Attachment[] = [];
      const removed: Attachment[] = [];
      for (const item of prev) {
        if (item.id === id) {
          removed.push(item);
        } else {
          next.push(item);
        }
      }
      revokePreviews(removed);
      return next;
    });
  }, []);

  const handleStartRecording = useCallback(async () => {
    if (disabled || isRecording || isStreaming) {
      return;
    }
    try {
      const sampleRate = 16000;
      recordingChunksRef.current = [];
      recordingSampleRateRef.current = sampleRate;
      const recorder = new WavRecorder({ sampleRate });
      await recorder.requestPermission();
      await recorder.begin();
      await recorder.record((chunk) => {
        let data: Int16Array;
        if (chunk.mono instanceof Int16Array) {
          data = new Int16Array(chunk.mono);
        } else if (chunk.mono instanceof ArrayBuffer) {
          data = new Int16Array(chunk.mono.slice(0));
        } else {
          data = new Int16Array(0);
        }
        if (data.length > 0) {
          recordingChunksRef.current.push(data);
        }
      });
      recorderRef.current = recorder;
      setIsRecording(true);
    } catch (err) {
      recorderRef.current = null;
      setIsRecording(false);
      onError(err instanceof Error ? err.message : "Failed to start recording");
    }
  }, [disabled, isRecording, isStreaming, onError]);

  const handleStopRecording = useCallback(async () => {
    const recorder = recorderRef.current;
    if (!recorder) {
      return;
    }
    try {
      await recorder.pause().catch(() => {
        /* noop */
      });
      await recorder.end();
      recorderRef.current = null;

      const combined = concatenateInt16Chunks(recordingChunksRef.current);
      recordingChunksRef.current = [];
      if (combined.length === 0) {
        throw new Error("No audio detected in recording");
      }
      const floatChannel = new Float32Array(combined.length);
      for (let i = 0; i < combined.length; i += 1) {
        floatChannel[i] = combined[i] / 0x8000;
      }
      const packer = new WavPacker();
      const wav = packer.pack(recordingSampleRateRef.current, {
        bitsPerSample: 16,
        channels: [floatChannel],
        data: combined,
      });
      const file = new File([wav.blob], `recording-${String(Date.now())}.wav`, {
        type: wav.blob.type || "audio/wav",
      });
      const preview = URL.createObjectURL(file);
      setAttachments((prev) => [
        ...prev,
        {
          id: createId(),
          file,
          kind: "audio",
          preview,
        },
      ]);
      URL.revokeObjectURL(wav.url);
    } catch (err) {
      onError(err instanceof Error ? err.message : "Failed to stop recording");
    } finally {
      recorderRef.current = null;
      setIsRecording(false);
    }
  }, [onError]);

  const handlePaste = useCallback(
    (event: ReactClipboardEvent<HTMLTextAreaElement>) => {
      const clipboard = event.clipboardData;

      const imageFiles: File[] = [];
      const fromFiles = clipboard.files;
      if (fromFiles.length > 0) {
        for (const file of Array.from(fromFiles)) {
          if (file.type.startsWith("image/")) {
            imageFiles.push(file);
          }
        }
      }

      if (imageFiles.length === 0) {
        for (const item of Array.from(clipboard.items)) {
          if (item.kind === "file" && item.type.startsWith("image/")) {
            const file = item.getAsFile();
            if (file) {
              imageFiles.push(file);
            }
          }
        }
      }

      if (imageFiles.length === 0) {
        return;
      }

      event.preventDefault();

      const timestamp = Date.now();
      const target = event.currentTarget;
      const normalized: File[] = imageFiles.map((file, index) => {
        if (file.name) {
          return file;
        }
        const extension = getImageExtension(file.type);
        const name = `pasted-image-${String(timestamp)}-${String(index)}.${extension}`;
        return new File([file], name, {
          type: file.type || `image/${extension}`,
        });
      });

      handleFileAdd(normalized, "image");

      const pastedText = clipboard.getData("text/plain");
      if (pastedText) {
        const selectionStart = target.selectionStart;
        const selectionEnd = target.selectionEnd;
        setInputText(
          (prev) =>
            prev.slice(0, selectionStart) +
            pastedText +
            prev.slice(selectionEnd),
        );
        const nextCaret = selectionStart + pastedText.length;
        requestAnimationFrame(() => {
          target.setSelectionRange(nextCaret, nextCaret);
        });
      }
    },
    [handleFileAdd],
  );

  const handleSubmit = useCallback(async () => {
    if (!canSubmit) {
      return;
    }
    let parts: Part[];
    try {
      parts = await buildInputParts(inputText, attachments);
    } catch (err) {
      onError(err instanceof Error ? err.message : "Failed to prepare request");
      return;
    }

    void onSend(parts);
    setInputText("");
    revokePreviews(attachments);
    setAttachments([]);
  }, [attachments, canSubmit, inputText, onError, onSend]);

  return (
    <form
      ref={formRef}
      className="border-t border-slate-200/70 bg-white/70 px-8 py-5"
      onSubmit={(event) => {
        event.preventDefault();
        void handleSubmit();
      }}
    >
      <div className="flex flex-col gap-3">
        <textarea
          className="console-textarea"
          placeholder="Send a message..."
          value={inputText}
          onChange={(event) => {
            setInputText(event.target.value);
          }}
          onPaste={handlePaste}
          onKeyDown={(event) => {
            if (event.key === "Enter" && !event.shiftKey) {
              event.preventDefault();
              if (canSubmit) {
                formRef.current?.requestSubmit();
              }
            }
          }}
          disabled={disabled || isStreaming}
        />
        {attachments.length > 0 ? (
          <div className="flex flex-wrap gap-3">
            {attachments.map((attachment) => (
              <AttachmentPreview
                key={attachment.id}
                attachment={attachment}
                onRemove={() => {
                  handleAttachmentRemove(attachment.id);
                }}
              />
            ))}
          </div>
        ) : null}
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div className="flex items-center gap-2">
            <button
              type="button"
              className="console-button"
              onClick={() => imageInputRef.current?.click()}
              disabled={disabled || isStreaming || isRecording}
            >
              Add Image
            </button>
            <button
              type="button"
              className="console-button"
              onClick={() => audioInputRef.current?.click()}
              disabled={disabled || isStreaming || isRecording}
            >
              Add Audio
            </button>
            <button
              type="button"
              className={`console-button ${
                isRecording ? "console-button-danger" : "console-button-success"
              }`}
              onClick={() => {
                if (isRecording) {
                  void handleStopRecording();
                } else {
                  void handleStartRecording();
                }
              }}
              disabled={disabled || isStreaming}
            >
              {isRecording ? "Stop Recording" : "Record Audio"}
            </button>
            <input
              ref={imageInputRef}
              type="file"
              accept="image/*"
              className="hidden"
              multiple
              onChange={(event) => {
                handleFileAdd(event.target.files, "image");
                event.target.value = "";
              }}
            />
            <input
              ref={audioInputRef}
              type="file"
              accept="audio/*"
              className="hidden"
              multiple
              onChange={(event) => {
                handleFileAdd(event.target.files, "audio");
                event.target.value = "";
              }}
            />
          </div>
          <div className="flex items-center gap-2">
            {isStreaming ? (
              <button
                type="button"
                className="console-button console-button-danger"
                onClick={onAbort}
              >
                Stop
              </button>
            ) : null}
            <button
              type="submit"
              className="console-button console-button-primary"
              disabled={!canSubmit}
            >
              Send
            </button>
          </div>
        </div>
        {isRecording ? (
          <p className="console-subheading text-rose-500">Recording…</p>
        ) : null}
      </div>
    </form>
  );
}

function revokePreviews(list: Attachment[]) {
  for (const item of list) {
    URL.revokeObjectURL(item.preview);
  }
}

async function buildInputParts(
  text: string,
  attachments: Attachment[],
): Promise<Part[]> {
  const parts: Part[] = [];
  const trimmed = text.trim();
  if (trimmed.length > 0) {
    parts.push({ type: "text", text: trimmed });
  }

  for (const attachment of attachments) {
    const base64 = await fileToBase64(attachment.file);
    if (attachment.kind === "image") {
      parts.push({
        type: "image",
        mime_type: attachment.file.type || "image/png",
        data: base64,
      });
    } else {
      const format = getAudioFormat(attachment.file);
      parts.push({
        type: "audio",
        format,
        data: base64,
      });
    }
  }

  return parts;
}

async function fileToBase64(file: File): Promise<string> {
  const buffer = await file.arrayBuffer();
  const bytes = new Uint8Array(buffer);
  let binary = "";
  for (let i = 0; i < bytes.byteLength; i += 1) {
    binary += String.fromCharCode(bytes[i]);
  }
  return btoa(binary);
}

function getImageExtension(mimeType: string): string {
  const [, subtype = "png"] = mimeType.split("/");
  if (!subtype) {
    return "png";
  }
  if (subtype.includes("jpeg") || subtype.includes("jpg")) {
    return "jpg";
  }
  if (subtype.includes("png")) {
    return "png";
  }
  if (subtype.includes("gif")) {
    return "gif";
  }
  if (subtype.includes("webp")) {
    return "webp";
  }
  if (subtype.includes("svg")) {
    return "svg";
  }
  const cleaned = subtype.split(";")[0]?.trim();
  return cleaned && cleaned.length > 0 ? cleaned : "png";
}

function getAudioFormat(file: File): AudioFormat {
  const mime = file.type.toLowerCase();
  if (mime.includes("wav")) return "wav";
  if (mime.includes("mpeg") || mime.includes("mp3")) return "mp3";
  if (mime.includes("flac")) return "flac";
  if (mime.includes("aac") || mime.includes("m4a")) return "aac";
  if (mime.includes("opus")) return "opus";
  if (mime.includes("mulaw") || mime.includes("mu-law")) return "mulaw";
  if (mime.includes("alaw") || mime.includes("a-law")) return "alaw";
  if (mime.includes("linear16") || mime.includes("l16")) return "linear16";
  return "wav";
}

function concatenateInt16Chunks(chunks: Int16Array[]): Int16Array {
  if (chunks.length === 0) {
    return new Int16Array();
  }
  const total = chunks.reduce((sum, chunk) => sum + chunk.length, 0);
  const result = new Int16Array(total);
  let offset = 0;
  for (const chunk of chunks) {
    result.set(chunk, offset);
    offset += chunk.length;
  }
  return result;
}

function AttachmentPreview({
  attachment,
  onRemove,
}: {
  attachment: Attachment;
  onRemove: () => void;
}) {
  return (
    <div className="console-preview">
      <button
        type="button"
        className="console-button absolute top-1 right-1 z-10 px-2 py-0 text-[11px]"
        onClick={onRemove}
      >
        ✕
      </button>
      <div className="flex-1">
        {attachment.kind === "image" ? (
          <img
            src={attachment.preview}
            alt={attachment.file.name}
            className="h-32 w-full object-cover"
          />
        ) : (
          <audio controls src={attachment.preview} className="h-32 w-full" />
        )}
      </div>
      <div className="truncate px-3 py-2 text-[11px] text-slate-600">
        {attachment.file.name}
      </div>
    </div>
  );
}
