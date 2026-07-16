export const SecurityBanner = () => {
  return (
    <div className="mx-auto max-w-lg text-[10px] leading-relaxed text-slate-600">
      All API requests are made on the client side. No API keys are transmitted
      to the backend.{" "}
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
