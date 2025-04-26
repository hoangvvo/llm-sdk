export const SecurityBanner = () => {
  return (
    <div className="mx-auto max-w-lg text-[10px] leading-relaxed text-slate-600">
      Data, including the provided API keys, is not retained by the example
      servers after the request is completed. API keys and other preferences are
      stored in the browser's local storage for your convenience.{" "}
      <a
        href="https://github.com/hoangvvo/llm-sdk/tree/main/website"
        className="text-sky-600 underline"
        target="_blank"
        rel="noreferrer"
      >
        Run this locally
      </a>
    </div>
  );
};
