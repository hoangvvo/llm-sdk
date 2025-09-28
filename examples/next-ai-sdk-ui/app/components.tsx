import { GeistMono } from "geist/font/mono";
import Link from "next/link";
import { ReactNode } from "react";

const Code = ({ children }: { children: ReactNode }) => {
  return (
    <code
      className={`${GeistMono.className} rounded-md border bg-zinc-100 p-1 text-xs`}
    >
      {children}
    </code>
  );
};

export const Card = () => {
  return (
    <div className="w-full self-center px-8 py-6">
      <div className="flex w-full flex-col gap-2 rounded-lg border p-4">
        <div className="text font-semibold text-zinc-800">
          Stream Chat Completions
        </div>
        <div className="flex flex-col gap-4 text-sm leading-6 text-zinc-500">
          <p>
            The <Code>useChat</Code> hook can be integrated with a Node.js, Go,
            or Rust backend running llm-sdk to stream chat completions in
            real-time.
          </p>

          <p>
            The example servers implement adapters to convert from the llm-sdk
            format to the format conforming to the{" "}
            <Link
              target="_blank"
              className="text-blue-500 hover:underline"
              href="https://ai-sdk.dev/docs/ai-sdk-ui/stream-protocol#data-stream-protocol"
            >
              data stream protocol
            </Link>{" "}
            and vice versa.
          </p>
        </div>
      </div>
    </div>
  );
};
