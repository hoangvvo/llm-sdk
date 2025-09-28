// @ts-check
import react from "@astrojs/react";
import starlight from "@astrojs/starlight";
import tailwindcss from "@tailwindcss/vite";
import mermaid from "astro-mermaid";
import { defineConfig } from "astro/config";
import { ion } from "starlight-ion-theme";
import { viteStaticCopy } from "vite-plugin-static-copy";

// https://astro.build/config
export default defineConfig({
  site: "https://llm-sdk.hoangvvo.com",
  trailingSlash: "always",
  integrations: [
    starlight({
      title: "llm-sdk",
      logo: {
        light: "./public/logo-light.svg",
        dark: "./public/logo-dark.svg",
        alt: "llm-sdk",
        replacesTitle: true,
      },
      social: [
        {
          icon: "github",
          label: "GitHub",
          href: "https://github.com/hoangvvo/llm-sdk",
        },
      ],
      sidebar: [
        {
          label: "SDK",
          items: [
            { slug: "sdk" },
            { slug: "sdk/providers" },
            { slug: "sdk/language-model" },
            { slug: "sdk/text-generation" },
            { slug: "sdk/image-generation" },
            { slug: "sdk/audio-generation" },
            { slug: "sdk/image-understanding" },
            { slug: "sdk/audio-understanding" },
            { slug: "sdk/function-calling" },
            { slug: "sdk/structured-output" },
            { slug: "sdk/reasoning" },
            { slug: "sdk/citations" },
            { slug: "sdk/testing" },
          ],
        },
        {
          label: "Agent",
          items: [
            { slug: "agent" },
            { slug: "agent/agent" },
            { slug: "agent/instructions" },
            { slug: "agent/tools" },
            { slug: "agent/toolkits" },
            { slug: "agent/run" },
            { slug: "agent/agent-vs-run-session" },
            { slug: "agent/resumability" },
            { slug: "agent/structured-output" },
            { slug: "agent/mcp" },
            { slug: "agent/delegation" },
            { slug: "agent/memory" },
            { slug: "agent/artifacts" },
            { slug: "agent/planner-executor" },
            { slug: "agent/human-in-the-loop" },
            { slug: "agent/testing" },
          ],
        },
        {
          label: "Integrations",
          items: [{ slug: "integrations/vercel-ai-sdk" }],
        },
        {
          label: "Observability",
          items: [{ slug: "observability/tracing" }],
        },
        {
          label: "Demo",
          items: [
            {
              label: "Chat",
              link: "/console/chat/",
            },
            {
              label: "Realtime",
              link: "/console/realtime/",
            },
          ],
        },
      ],
      plugins: [ion()],
      customCss: ["./src/styles/custom.css"],
    }),
    react(),
    mermaid({
      theme: "forest",
      autoTheme: true,
    }),
  ],
  vite: {
    plugins: [
      tailwindcss(),
      viteStaticCopy({
        targets: [
          {
            src: "../node_modules/onnxruntime-web/dist/*.wasm",
            dest: "src/onnxruntime-web/",
          },
          {
            src: "../node_modules/@ricky0123/vad-web/dist/vad.worklet.bundle.min.js",
            dest: "src/vad-web/",
          },
          {
            src: "../node_modules/@ricky0123/vad-web/dist/*.onnx",
            dest: "src/vad-web/",
          },
          {
            src: "../node_modules/onnxruntime-web/dist/*.mjs",
            dest: "src/onnxruntime-web/",
          },
        ],
      }),
    ],
  },
});
