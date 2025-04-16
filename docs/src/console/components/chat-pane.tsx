import type { AgentItem } from "@hoangvvo/llm-agent";
import type { AudioPart, Part } from "@hoangvvo/llm-sdk";
import { useEffect, useState } from "react";
import { base64ToArrayBuffer } from "../lib/utils.ts";
import { WavPacker } from "../lib/wavtools/wav_packer.ts";
import { Markdown } from "./markdown.tsx";

interface ChatPaneProps {
  items: AgentItem[];
  streamingParts: Part[];
}

export function ChatPane({ items, streamingParts }: ChatPaneProps) {
  return (
    <main className="flex-1 space-y-6 overflow-y-auto px-8 py-6">
      {items.length === 0 && streamingParts.length === 0 ? (
        <div className="rounded-xl border border-dashed border-slate-200/80 bg-white/70 p-10 text-slate-500">
          <p className="text-md uppercase text-centeer tracking-[0.2em]">
            Welcome to the Chat Console
          </p>
          <p className="text-xs mt-4">
            Make sure to start any of the example servers:
          </p>
          <div className="mt-4 rounded bg-slate-100 p-3 text-xs text-slate-700">
            cd ../agent-js &amp;&amp; node --env-file ../.env examples/server.ts
          </div>
          <div className="mt-2 rounded bg-slate-100 p-3 text-xs text-slate-700">
            cd ../agent-rust &amp;&amp; cargo run --example server
          </div>
          <div className="mt-2 rounded bg-slate-100 p-3 text-xs text-slate-700">
            cd ../agent-go &amp;&amp; go run ./examples/server
          </div>
        </div>
      ) : null}
      {items.map((item, index) => (
        <ConversationItem key={`${String(index)}-${item.type}`} item={item} />
      ))}
      {streamingParts.length > 0 ? (
        <div className="rounded-2xl border border-sky-100 bg-sky-50/70 p-5">
          <div className="mb-3 text-[11px] uppercase tracking-[0.35em] text-sky-600">
            Assistant · streaming
          </div>
          <PartsList parts={streamingParts} />
        </div>
      ) : null}
    </main>
  );
}

function ConversationItem({ item }: { item: AgentItem }) {
  if (item.type === "message") {
    const heading = item.role === "user" ? "You" : "Assistant";
    return (
      <div className="rounded-2xl border border-rose-100 bg-rose-50/70 p-5">
        <div className="mb-3 text-[11px] uppercase tracking-[0.3em] text-slate-500">
          {heading}
        </div>
        <PartsList parts={item.content} />
      </div>
    );
  }

  if (item.type === "model") {
    return (
      <div className="rounded-2xl border border-sky-100 bg-sky-50/70 p-5">
        <div className="mb-3 text-[11px] uppercase tracking-[0.3em] text-sky-600">
          Assistant
        </div>
        <PartsList parts={item.content} />
        {item.usage ? (
          <div className="mt-4 text-[11px] uppercase tracking-[0.25em] text-slate-500">
            usage · in {item.usage.input_tokens} · out{" "}
            {item.usage.output_tokens}
          </div>
        ) : null}
      </div>
    );
  }

  return (
    <div className="rounded-2xl border border-indigo-100 bg-indigo-50/60 p-5">
      <div className="mb-3 text-[11px] uppercase tracking-[0.3em] text-indigo-500">
        Tool · {item.tool_name}
      </div>
      <div className="mb-3 text-[13px] text-slate-600">
        <span className="font-semibold text-slate-900">Call ID:</span>{" "}
        {item.tool_call_id}
      </div>
      <div className="mb-3">
        <div className="text-[11px] uppercase tracking-[0.25em] text-slate-500">
          Input
        </div>
        <pre className="mt-1 whitespace-pre-wrap break-words rounded-md bg-slate-100 p-3 text-xs text-slate-800">
          {JSON.stringify(item.input, null, 2)}
        </pre>
      </div>
      <div>
        <div className="text-[11px] uppercase tracking-[0.25em] text-slate-500">
          Output
        </div>
        <PartsList parts={item.output} />
      </div>
      {item.is_error ? (
        <div className="mt-3 text-[11px] uppercase tracking-[0.25em] text-rose-500">
          Tool reported an error.
        </div>
      ) : null}
    </div>
  );
}

function PartsList({ parts }: { parts: Part[] }) {
  console.log("Rendering PartsList with parts:", parts);
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
        <div className="rounded-xl border border-slate-200 bg-white p-3 text-xs text-slate-600">
          <div className="text-[11px] uppercase tracking-[0.25em] text-slate-500">
            Source
          </div>
          <div className="mt-2 font-semibold text-slate-800">{part.title}</div>
          <div className="mt-2">
            <PartsList parts={part.content} />
          </div>
        </div>
      );
    case "tool-call":
      return (
        <div className="rounded-xl border border-amber-200 bg-amber-50 p-3 text-xs text-amber-700">
          <div className="text-[11px] uppercase tracking-[0.25em] text-amber-600">
            Requested tool
          </div>
          <div className="mt-2 text-amber-700">{part.tool_name}</div>
          <div className="mt-3 whitespace-pre-wrap break-words text-[11px] text-amber-800">
            {JSON.stringify(part.args, null, 2)}
          </div>
        </div>
      );
    case "tool-result":
      return (
        <div className="rounded-xl border border-emerald-200 bg-emerald-50 p-3 text-xs text-emerald-700">
          <div className="text-[11px] uppercase tracking-[0.25em] text-emerald-600">
            Tool result
          </div>
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
        <div className="rounded-xl border border-slate-200 bg-slate-100 p-3 text-sm text-slate-700">
          <div className="mb-2 text-[11px] uppercase tracking-[0.25em] text-slate-500">
            Reasoning
          </div>
          <Markdown>{part.text}</Markdown>
        </div>
      );
    default:
      return (
        <pre className="whitespace-pre-wrap break-words rounded-md bg-slate-100 p-3 text-xs text-slate-800">
          {JSON.stringify(part, null, 2)}
        </pre>
      );
  }
}

function AudioPartView({ part }: { part: AudioPart }) {
  const src = useAudioSource(part);
  const transcript = part.transcript?.trim();

  return (
    <div className="space-y-2 w-full">
      {src ? (
        <audio src={src} controls className="w-full" />
      ) : (
        <div className="rounded-lg border border-slate-200 bg-white px-3 py-2 text-xs uppercase tracking-[0.25em] text-slate-400">
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
