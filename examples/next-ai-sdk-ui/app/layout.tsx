import { GeistSans } from "geist/font/sans";
import Link from "next/link";
import "./globals.css";
import { LogoNext } from "./icons";

import { Metadata } from "next";

export const metadata: Metadata = {
  title: "AI SDK and llm-sdk Examples",
};

export default function RootLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <html lang="en">
      <body className={GeistSans.className}>
        <Link href="/">
          <div className="flex flex-row items-center gap-2 border-b p-4">
            <LogoNext />
            <div className="text-sm text-zinc-500">+</div>
            <img
              src="https://llm-sdk.hoangvvo.com/logo-light.svg"
              alt="LLM SDK"
              className="w-12"
            />
          </div>
        </Link>
        {children}
      </body>
    </html>
  );
}
