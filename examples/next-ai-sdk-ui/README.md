# AI SDK, Next.js, and llm-sdk Examples

These examples show you how to use the [AI SDK](https://ai-sdk.dev/docs) with [Next.js](https://nextjs.org) and [llm-sdk](https://github.com/hoangvvo/llm-sdk).

This is modified from the [next-fastapi example](https://github.com/vercel/ai/tree/main/examples/next-fastapi)

## How to use

To run the example locally you need to:

- Run the Next.js frontend:

  ```bash
  npm install
  npm run dev
  ```

Run one of the example backends:

- [Node.js backend](../../agent-js/examples/ai-sdk-ui.ts)

```bash
cd ../../agent-js
node examples/ai-sdk-ui.ts
```

- [Go backend](../../agent-go/examples/ai-sdk-ui/main.go)

```bash
cd ../../agent-go
go run ./examples/ai-sdk-ui
```

- [Rust backend](../../agent-rs/examples/ai-sdk-ui/src/main.rs)

```bash
cd ../../agent-rs
cargo run --example ai-sdk-ui
```

## Learn More

To learn more about the AI SDK, Next.js, and llm-sdk take a look at the following resources:

- [AI SDK Docs](https://ai-sdk.dev/docs) - view documentation and reference for the AI SDK.
- [Next.js Docs](https://nextjs.org/docs) - learn about Next.js features and API.
- [llm-sdk Docs](https://llm-sdk.hoangvvo.com/) - learn about llm-sdk features and API.
