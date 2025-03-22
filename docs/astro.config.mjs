// @ts-check
import starlight from "@astrojs/starlight";
import { defineConfig } from "astro/config";

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
            { slug: "agent/tools" },
            { slug: "agent/execution" },
            { slug: "agent/structured-output" },
            { slug: "agent/agents-delegation" },
          ],
        },
      ],
    }),
  ],
});
