import type { FC } from "react";

const links = [
  { href: "/agent/", label: "Docs" },
  { href: "/console/chat/", label: "Chat" },
  { href: "/console/realtime/", label: "Realtime" },
];

export const Nav: FC = () => {
  return (
    <aside className="h-12 border-gray-200 px-4 lg:h-full lg:w-24 lg:border-r lg:p-2">
      <nav className="flex items-center lg:flex-col">
        <a
          href="https://github.com/hoangvvo/llm-sdk"
          className="mr-auto block flex-none lg:mr-0"
        >
          <img
            src="/logo-light.svg"
            alt="Logo"
            className="mx-auto h-12 w-12 opacity-70 transition-opacity hover:opacity-80"
          />
        </a>
        {links.map((link) => (
          <a
            key={link.href}
            href={link.href}
            className="flex flex-col items-center gap-0.5 rounded p-2 text-center text-gray-600 hover:bg-gray-100 hover:text-gray-900"
          >
            <span className="font-mono text-sm">{link.label}</span>
          </a>
        ))}
      </nav>
    </aside>
  );
};
