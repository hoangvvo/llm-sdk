"use client";

import { Card } from "@/app/components";
import { useChat } from "@ai-sdk/react";
import { getToolName, isToolUIPart } from "ai";
import { GeistMono } from "geist/font/mono";
import { useRef, useState } from "react";

export default function Page() {
  const [input, setInput] = useState("");
  const [files, setFiles] = useState<FileList | undefined>(undefined);
  const fileInputRef = useRef<HTMLInputElement>(null);

  const { messages, sendMessage, status, error } = useChat();

  return (
    <div className="flex flex-col gap-2">
      <div className="flex flex-col gap-2 p-4">
        {messages.map((message) => (
          <div key={message.id} className="flex flex-row gap-2">
            <div className="w-24 flex-shrink-0 text-zinc-500">{`${message.role}: `}</div>

            <div className="flex flex-col gap-2">
              {message.parts.map((part, index) => {
                if (part.type === "text") {
                  return <div key={index}>{part.text}</div>;
                } else if (isToolUIPart(part)) {
                  return (
                    <div
                      key={index}
                      className={`${GeistMono.className} rounded-lg bg-zinc-100 p-3 text-sm text-zinc-500`}
                    >
                      {`${getToolName(part)}(${JSON.stringify(
                        part.input,
                        null,
                        2,
                      )})`}
                    </div>
                  );
                }
              })}
            </div>
          </div>
        ))}
      </div>

      {messages.length === 0 && <Card />}

      {error && (
        <div className="p-4 text-red-500">{`Error: ${error.message}`}</div>
      )}

      <form
        onSubmit={(event) => {
          event.preventDefault();

          sendMessage({ text: input, files });
          setInput("");
          setFiles(undefined);

          if (fileInputRef.current) {
            fileInputRef.current.value = "";
          }
        }}
        className="fixed bottom-0 flex h-28 w-full flex-col gap-3 border-t p-4"
      >
        <div className="fixed right-8 bottom-32 flex flex-row items-end gap-2">
          {files
            ? Array.from(files).map((attachment) => {
                const { type } = attachment;

                if (type.startsWith("image/")) {
                  return (
                    <div key={attachment.name}>
                      <img
                        className="w-24 rounded-md"
                        src={URL.createObjectURL(attachment)}
                        alt={attachment.name}
                      />
                      <span className="text-sm text-zinc-500">
                        {attachment.name}
                      </span>
                    </div>
                  );
                } else if (type.startsWith("text/")) {
                  return (
                    <div
                      key={attachment.name}
                      className="flex w-24 flex-shrink-0 flex-col gap-1 text-sm text-zinc-500"
                    >
                      <div className="h-20 w-16 rounded-md bg-zinc-100" />
                      {attachment.name}
                    </div>
                  );
                }
              })
            : ""}
        </div>
        <input
          type="file"
          onChange={(event) => {
            if (event.target.files) {
              setFiles(event.target.files);
            }
          }}
          multiple
          ref={fileInputRef}
        />
        <input
          value={input}
          placeholder="What's the weather in San Francisco?"
          onChange={(e) => setInput(e.target.value)}
          className="w-full bg-transparent outline-none"
          disabled={status !== "ready"}
        />
      </form>
    </div>
  );
}
