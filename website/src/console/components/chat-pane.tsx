import type { AgentItem } from "@hoangvvo/llm-agent";
import type {
  AudioFormat,
  AudioPart,
  Part,
  TextPart,
  ToolResultStatus,
} from "@hoangvvo/llm-sdk";
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
    <div className="min-h-0 flex-1 space-y-6 overflow-x-hidden overflow-y-auto px-4 py-6">
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
    status: ToolResultStatus;
  }[] = [];

  if (item.type === "tool") {
    tools.push({
      tool_name: item.tool_name,
      tool_call_id: item.tool_call_id,
      input: item.input,
      output: item.output,
      status: item.status,
    });
  } else {
    item.content.forEach((part) => {
      if (part.type === "tool-result") {
        if (part.result.type !== "function") return;
        tools.push({
          tool_name: part.result.name,
          tool_call_id: part.tool_call_id,
          input: {}, // input is not available in tool-result part
          output: part.result.content,
          status: part.status,
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
            <pre className="mt-1 max-h-[200px] overflow-y-auto rounded-md bg-slate-100 p-3 text-xs break-words whitespace-pre-wrap text-slate-800">
              {JSON.stringify(tool.input, null, 2)}
            </pre>
          </div>
          <div>
            <div className="console-subheading">Output</div>
            <div className="max-h-[200px] overflow-y-auto">
              <PartsList parts={tool.output} />
            </div>
          </div>
          {tool.status !== "completed" ? (
            <div className="console-subheading mt-3 text-rose-500!">
              Tool status: {tool.status}.
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
      return <TextPartView part={part} />;
    case "image":
      return (
        <img
          src={`data:${part.mime_type};base64,${part.data}`}
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
          <div className="mt-2 text-amber-700">
            {part.call.type === "function" ? part.call.name : "Web search"}
          </div>
          <div className="mt-3 max-h-[100px] overflow-y-auto text-[11px] break-words whitespace-pre-wrap text-amber-800">
            {JSON.stringify(
              part.call.type === "function" ? part.call.args : part.call,
              null,
              2,
            )}
          </div>
        </div>
      );
    case "tool-result":
      return (
        <div className="console-card-tool-result">
          <div className="console-subheading text-emerald-600">Tool result</div>
          <div className="mt-2">
            {part.result.type === "function" ? (
              <PartsList parts={part.result.content} />
            ) : (
              <pre className="text-xs whitespace-pre-wrap">
                {JSON.stringify(part.result, null, 2)}
              </pre>
            )}
          </div>
          {part.status !== "completed" ? (
            <div className="mt-2 text-rose-500">Status: {part.status}.</div>
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

function TextPartView({ part }: { part: TextPart }) {
  return (
    <div className="w-full space-y-3">
      <Markdown>{part.text}</Markdown>
      {part.citations && part.citations.length > 0 ? (
        <ol className="space-y-1 border-t border-slate-200 pt-2 text-xs text-slate-500">
          {part.citations.map((citation, index) => {
            const href = citationHref(citation.source);
            const label = citation.title ?? citation.source;
            return (
              <li key={`${citation.source}-${String(index)}`}>
                <span className="mr-1">[{String(index + 1)}]</span>
                {href ? (
                  <a
                    href={href}
                    target="_blank"
                    rel="noreferrer"
                    className="break-all text-sky-700 underline decoration-sky-300 underline-offset-2"
                  >
                    {label}
                  </a>
                ) : (
                  <span className="break-all">{label}</span>
                )}
              </li>
            );
          })}
        </ol>
      ) : null}
    </div>
  );
}

function citationHref(source: string): string | null {
  try {
    const url = new URL(source);
    return url.protocol === "http:" || url.protocol === "https:"
      ? url.toString()
      : null;
  } catch {
    return null;
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
    if (!part.data) {
      return;
    }

    let objectUrl: string | null = null;
    let cancelled = false;

    const assignSource = () => {
      try {
        if (part.format === "linear16") {
          const buffer = base64ToArrayBuffer(part.data);
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
          const dataUrl = `data:${audioFormatToMime(part.format)};base64,${part.data}`;
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
  }, [part.data, part.format, part.sample_rate]);

  return part.data ? source : null;
}

function audioFormatToMime(format: AudioFormat): string {
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
