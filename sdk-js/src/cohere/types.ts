export type PatchedAssistantMessageV2ContentItem =
  | {
      type: "text";
      text: string;
    }
  | {
      type: "thinking";
      thinking: string;
    };
