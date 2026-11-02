import type { ReactNode } from "react";
import Link from "next/link";

function Logo() {
  return (
    <span className="grid h-9 w-9 place-items-center rounded-xl bg-[#C87941] text-[#111111] shadow-none">
      <svg
        viewBox="0 0 24 24"
        className="h-5 w-5"
        fill="none"
        stroke="currentColor"
        strokeWidth="2"
        strokeLinecap="round"
      >
        <path d="M4 12v1M8 8v8M12 5v14M16 8v8M20 11v2" />
      </svg>
    </span>
  );
}

function NavLink({
  href,
  active,
  children,
}: {
  href: string;
  active?: boolean;
  children: ReactNode;
}) {
  return (
    <Link
      href={href}
      className={`rounded-lg px-3 py-1.5 font-medium transition-colors ${
        active
          ? "bg-[#C87941] text-[#111111]"
          : "text-[#A8A092] hover:bg-[#2A2A2A] hover:text-[#EAE5DC]"
      }`}
    >
      {children}
    </Link>
  );
}

export function AppHeader({ current }: { current?: "caller" | "supervisor" }) {
  return (
    <header className="sticky top-0 z-10 border-b border-[#3A3A3A] bg-[#242424]/80 backdrop-blur">
      <div className="mx-auto flex max-w-6xl items-center justify-between px-6 py-3.5">
        <Link href="/" className="flex items-center gap-2.5">
          <Logo />
          <span className="text-[15px] font-semibold tracking-tight text-[#EAE5DC]">
            Switchboard
          </span>
        </Link>
        <nav className="flex items-center gap-1 text-sm">
          <NavLink href="/" active={current === "caller"}>
            Caller
          </NavLink>
          <NavLink href="/supervisor" active={current === "supervisor"}>
            Supervisor
          </NavLink>
        </nav>
      </div>
    </header>
  );
}
