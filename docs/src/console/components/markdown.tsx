import ReactMarkdown from "react-markdown";

interface MarkdownProps {
  children: string;
}

export function Markdown({ children }: MarkdownProps) {
  return (
    <div className="space-y-3 [&_p]:text-slate-700 [&_p]:leading-relaxed [&_ul]:list-disc [&_ul]:pl-5 [&_ol]:list-decimal [&_ol]:pl-5 [&_li]:ml-1 [&_code:not(pre_*)]:rounded [&_code:not(pre_*)]:bg-slate-200/60 [&_code:not(pre_*)]:px-1.5 [&_code:not(pre_*)]:py-0.5 [&_pre]:rounded-lg [&_pre]:bg-slate-900/90 [&_pre]:p-4 [&_pre_code]:text-slate-100">
      <ReactMarkdown>{children}</ReactMarkdown>
    </div>
  );
}
