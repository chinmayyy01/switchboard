import type { Metadata } from "next";
import type { ReactNode } from "react";

export const metadata: Metadata = {
  title: "Console | Switchboard",
  robots: { index: false },
};

export default function ConsoleLayout({ children }: { children: ReactNode }) {
  return children;
}
