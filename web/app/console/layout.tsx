import type { Metadata } from "next";
import type { ReactNode } from "react";
import { AuthGate } from "@/components/AuthGate";
import { SessionProvider } from "@/lib/session";

export const metadata: Metadata = {
  title: "Console | Switchboard",
  robots: { index: false },
};

export default function ConsoleLayout({ children }: { children: ReactNode }) {
  return (
    <SessionProvider>
      <AuthGate>{children}</AuthGate>
    </SessionProvider>
  );
}
