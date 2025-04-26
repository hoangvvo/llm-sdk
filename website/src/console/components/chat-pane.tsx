import type { AgentItem } from "@hoangvvo/llm-agent";
import type { AudioPart, Part } from "@hoangvvo/llm-sdk";
import { useEffect, useState } from "react";
import { base64ToArrayBuffer } from "../lib/utils.ts";
import { WavPacker } from "../lib/wavtools/wav_packer.ts";
import { Markdown } from "./markdown.tsx";
import { SecurityBanner } from "./security-banner.tsx";

interface ChatPaneProps {
  items: AgentItem[];
  streamingParts: Part[];
}

export function ChatPane({ items, streamingParts }: ChatPaneProps) {
  return (
    <div className="flex-1 space-y-6 overflow-x-hidden overflow-y-auto px-8 py-6">
      {items.length === 0 && streamingParts.length === 0 ? (
        <div className="console-placeholder p-10">
          <SecurityBanner />
        </div>
      ) : null}
      {items.map((item, index) => (
        <ConversationItem key={`${String(index)}-${item.type}`} item={item} />
      ))}
      {streamingParts.length > 0 ? (
        <div className="console-card-stream">
          <div className="console-section-title mb-3 text-sky-600">
            Assistant · streaming
          </div>
          <PartsList parts={streamingParts} />
        </div>
      ) : null}
    </div>
  );
}

function ConversationItem({ item }: { item: AgentItem }) {
  if (item.type === "message" && item.role === "user") {
    return (
      <div className="console-card-user">
        <div className="console-section-title mb-3 text-slate-600">You</div>
        <PartsList parts={item.content} />
      </div>
    );
  }

  if (
    item.type === "model" ||
    (item.type === "message" && item.role === "assistant")
  ) {
    return (
      <div className="console-card-assistant">
        <div className="console-section-title mb-3 text-sky-600">Assistant</div>
        <PartsList parts={item.content} />
        {item.type === "model" && (
          <div className="console-subheading mt-4 tracking-normal!">
            {item.usage ? (
              <span>
                usage: in {item.usage.input_tokens} · out{" "}
                {item.usage.output_tokens}
              </span>
            ) : null}
            {"  |  "}
            {item.cost ? <span>cost: ${item.cost.toFixed(6)}</span> : null}
          </div>
        )}
      </div>
    );
  }

  const tools: {
    tool_name: string;
    tool_call_id: string;
    input: Record<string, unknown>;
    output: Part[];
    is_error: boolean;
  }[] = [];

  if (item.type === "tool") {
    tools.push({
      tool_name: item.tool_name,
      tool_call_id: item.tool_call_id,
      input: item.input,
      output: item.output,
      is_error: item.is_error,
    });
  } else {
    item.content.forEach((part) => {
      if (part.type === "tool-result") {
        tools.push({
          tool_name: part.tool_name,
          tool_call_id: part.tool_call_id,
          input: {}, // input is not available in tool-result part
          output: part.content,
          is_error: !!part.is_error,
        });
      }
    });
  }

  return (
    <>
      {tools.map((tool) => (
        <div className="console-card-tool" key={tool.tool_call_id}>
          <div className="console-section-title mb-3 text-indigo-500">
            Tool · {tool.tool_name}
          </div>
          <div className="mb-3 text-[13px] text-slate-600">
            <span className="font-semibold text-slate-900">Call ID:</span>{" "}
            {tool.tool_call_id}
          </div>
          <div className="mb-3">
            <div className="console-subheading">Input</div>
            <pre className="mt-1 rounded-md bg-slate-100 p-3 text-xs break-words whitespace-pre-wrap text-slate-800">
              {JSON.stringify(tool.input, null, 2)}
            </pre>
          </div>
          <div>
            <div className="console-subheading">Output</div>
            <PartsList parts={tool.output} />
          </div>
          {tool.is_error ? (
            <div className="console-subheading mt-3 text-rose-500!">
              Tool reported an error.
            </div>
          ) : null}
        </div>
      ))}
    </>
  );
}

function PartsList({ parts }: { parts: Part[] }) {
  return (
    <div className="flex flex-col items-start gap-4 text-sm leading-relaxed text-slate-700">
      {parts.map((part, index) => (
        <PartView key={index} part={part} />
      ))}
    </div>
  );
}

function PartView({ part }: { part: Part }) {
  switch (part.type) {
    case "text":
      return <Markdown>{part.text}</Markdown>;
    case "image":
      return (
        <img
          src={`data:${part.mime_type};base64,${part.image_data}`}
          alt="Shared"
          className="max-h-64 w-auto rounded-md border border-slate-200"
        />
      );
    case "audio":
      return <AudioPartView part={part} />;
    case "source":
      return (
        <div className="console-card-source text-xs text-slate-600">
          <div className="console-subheading">Source</div>
          <div className="mt-2 font-semibold text-slate-800">{part.title}</div>
          <div className="mt-2">
            <PartsList parts={part.content} />
          </div>
        </div>
      );
    case "tool-call":
      return (
        <div className="console-card-tool-call">
          <div className="console-subheading text-amber-600">
            Requested tool
          </div>
          <div className="mt-2 text-amber-700">{part.tool_name}</div>
          <div className="mt-3 text-[11px] break-words whitespace-pre-wrap text-amber-800">
            {JSON.stringify(part.args, null, 2)}
          </div>
        </div>
      );
    case "tool-result":
      return (
        <div className="console-card-tool-result">
          <div className="console-subheading text-emerald-600">Tool result</div>
          <div className="mt-2">
            <PartsList parts={part.content} />
          </div>
          {part.is_error ? (
            <div className="mt-2 text-rose-500">Marked as error.</div>
          ) : null}
        </div>
      );
    case "reasoning":
      return (
        <div className="console-card-reasoning">
          <div className="console-subheading mb-2">Reasoning</div>
          <Markdown>{part.text}</Markdown>
        </div>
      );
    default:
      return (
        <pre className="console-surface text-xs break-words whitespace-pre-wrap text-slate-800">
          {JSON.stringify(part, null, 2)}
        </pre>
      );
  }
}

function AudioPartView({ part }: { part: AudioPart }) {
  const src = useAudioSource(part);
  const transcript = part.transcript?.trim();

  return (
    <div className="w-full space-y-2">
      {src ? (
        <audio src={src} controls className="w-full" />
      ) : (
        <div className="console-surface p-2 text-xs tracking-[0.2em] text-slate-500 uppercase">
          Audio loading…
        </div>
      )}
      {transcript ? (
        <div className="text-xs text-slate-500">
          <Markdown>{transcript}</Markdown>
        </div>
      ) : null}
    </div>
  );
}

function useAudioSource(part: AudioPart): string | null {
  const [source, setSource] = useState<string | null>(null);

  useEffect(() => {
    if (!part.audio_data) {
      setSource(null);
      return () => {
        /* noop */
      };
    }

    let objectUrl: string | null = null;
    let cancelled = false;

    const assignSource = () => {
      try {
        if (part.format === "linear16") {
          const buffer = base64ToArrayBuffer(part.audio_data);
          const int16 = new Int16Array(buffer);
          const floatChannel = new Float32Array(int16.length);
          for (let i = 0; i < int16.length; i += 1) {
            floatChannel[i] = int16[i] / 0x8000;
          }
          const packer = new WavPacker();
          const wav = packer.pack(part.sample_rate ?? 16000, {
            bitsPerSample: 16,
            channels: [floatChannel],
            data: int16,
          });
          objectUrl = wav.url;
          if (!cancelled) {
            setSource(wav.url);
          }
        } else {
          const dataUrl = `data:${audioFormatToMime(part.format)};base64,${part.audio_data}`;
          if (!cancelled) {
            setSource(dataUrl);
          }
        }
      } catch (err) {
        console.error("Failed to prepare audio source", err);
        if (!cancelled) {
          setSource(null);
        }
      }
    };

    assignSource();

    return () => {
      cancelled = true;
      if (objectUrl) {
        URL.revokeObjectURL(objectUrl);
      }
    };
  }, [part.audio_data, part.format, part.sample_rate]);

  return source;
}

function audioFormatToMime(format: AudioPart["format"]): string {
  switch (format) {
    case "mp3":
      return "audio/mpeg";
    case "wav":
      return "audio/wav";
    case "aac":
      return "audio/aac";
    case "opus":
      return "audio/opus";
    case "flac":
      return "audio/flac";
    case "mulaw":
    case "alaw":
      return "audio/basic";
    case "linear16":
      return "audio/l16";
  }
}
