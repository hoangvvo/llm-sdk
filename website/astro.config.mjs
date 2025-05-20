// @ts-check
import react from "@astrojs/react";
import starlight from "@astrojs/starlight";
import tailwindcss from "@tailwindcss/vite";
import { defineConfig } from "astro/config";
import { ion } from "starlight-ion-theme";
import { viteStaticCopy } from "vite-plugin-static-copy";

// https://astro.build/config
export default defineConfig({
  site: "https://llm-sdk.hoangvvo.workers.dev",
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
            { slug: "sdk/language-model" },
            { slug: "sdk/text-generation" },
            { slug: "sdk/image-generation" },
            { slug: "sdk/audio-generation" },
            { slug: "sdk/image-understanding" },
            { slug: "sdk/audio-understanding" },
            { slug: "sdk/function-calling" },
            { slug: "sdk/structured-output" },
            { slug: "sdk/reasoning" },
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
            { slug: "agent/run" },
            { slug: "agent/structured-output" },
            { slug: "agent/delegation" },
            { slug: "agent/memory" },
            { slug: "agent/artifacts" },
            { slug: "agent/planner-executor" },
            { slug: "agent/testing" },
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
      plugins: [ion()],
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
