// @ts-check
import react from "@astrojs/react";
import starlight from "@astrojs/starlight";
import tailwindcss from "@tailwindcss/vite";
import { defineConfig } from "astro/config";
import { viteStaticCopy } from "vite-plugin-static-copy";

// https://astro.build/config
export default defineConfig({
  integrations: [
    starlight({
      title: "llm-sdk",
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
            { slug: "sdk/language-model" },
            { slug: "sdk/text-generation" },
            { slug: "sdk/image-generation" },
            { slug: "sdk/audio-generation" },
            { slug: "sdk/image-understanding" },
            { slug: "sdk/audio-understanding" },
            { slug: "sdk/function-calling" },
            { slug: "sdk/structured-output" },
            { slug: "sdk/reasoning" },
          ],
        },
        {
          label: "Agent",
          items: [
            { slug: "agent" },
            { slug: "agent/agent" },
            { slug: "agent/instructions" },
            { slug: "agent/tools" },
            { slug: "agent/execution" },
            { slug: "agent/structured-output" },
            { slug: "agent/agents-delegation" },
          ],
        },
        {
          label: "Demo",
          items: [
            {
              label: "Chat",
              link: "/console/chat",
            },
            {
              label: "Realtime",
              link: "/console/realtime",
            },
          ],
        },
      ],
    }),
    react(),
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
