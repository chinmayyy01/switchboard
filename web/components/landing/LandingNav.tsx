"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { type } from "@/components/landing/type";

const links = [
  { label: "Product", href: "#product" },
  { label: "How it works", href: "#how" },
  { label: "Proof", href: "#proof" },
  { label: "FAQ", href: "#faq" },
];

const curve = "ease-[cubic-bezier(0.22,1,0.36,1)]";

function Mark() {
  return (
    <span className="grid h-9 w-9 place-items-center rounded-xl bg-[#C87941] text-[#111111]">
      <svg
        viewBox="0 0 24 24"
        className="h-5 w-5"
        fill="none"
        stroke="currentColor"
        strokeWidth="2"
        strokeLinecap="round"
        aria-hidden="true"
      >
        <path d="M4 12v1M8 8v8M12 5v14M16 8v8M20 11v2" />
      </svg>
    </span>
  );
}

export function LandingNav() {
  const [open, setOpen] = useState(false);

  useEffect(() => {
    document.body.style.overflow = open ? "hidden" : "";
    return () => {
      document.body.style.overflow = "";
    };
  }, [open]);

  return (
    <>
      <div className="fixed inset-x-3 top-3 z-50 flex justify-center">
        <nav className="flex w-full max-w-4xl items-center justify-between rounded-2xl border border-[#3A3A3A] bg-[#2A2A2A]/80 px-3 py-2 shadow-[0_12px_40px_rgba(20,17,15,0.08)] backdrop-blur-xl">
          <Link
            href="/"
            aria-current="page"
            aria-label="Switchboard home"
            className="flex min-w-0 items-center gap-2 rounded-lg px-1 py-1 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[#D98E5A] focus-visible:ring-offset-2 focus-visible:ring-offset-[#111111]"
          >
            <Mark />
            <span className="truncate text-sm font-semibold tracking-tight text-[#EAE5DC]">
              Switchboard
            </span>
          </Link>

          <div className="hidden items-center gap-1 md:flex">
            {links.map((link) => (
              <a
                key={link.href}
                href={link.href}
                className={`rounded-lg px-3 py-2 text-sm font-medium text-[#A8A092] transition-colors duration-300 ${curve} hover:text-[#EAE5DC] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[#D98E5A] focus-visible:ring-offset-2 focus-visible:ring-offset-[#111111]`}
              >
                {link.label}
              </a>
            ))}
          </div>

          <div className="flex shrink-0 items-center gap-2">
            <Link
              href="/console?tab=test"
              className={`hidden rounded-xl bg-[#C87941] px-3 py-2 ${type.buttonSm} text-[#111111] transition-[background-color,transform] duration-300 ${curve} hover:bg-[#D98E5A] active:scale-[0.98] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[#D98E5A] focus-visible:ring-offset-2 focus-visible:ring-offset-[#111111] sm:inline-flex`}
            >
              Talk to the agent
            </Link>
            <button
              type="button"
              onClick={() => setOpen((value) => !value)}
              aria-expanded={open}
              aria-controls="mobile-menu"
              aria-label={open ? "Close menu" : "Open menu"}
              className="grid h-9 w-9 shrink-0 place-items-center rounded-xl text-[#EAE5DC] transition-colors duration-300 hover:bg-[#EAE5DC]/5 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[#D98E5A] focus-visible:ring-offset-2 focus-visible:ring-offset-[#111111] md:hidden"
            >
              <span className="flex flex-col gap-2" aria-hidden="true">
                <span
                  className={`block h-0.5 w-5 bg-current transition-transform duration-500 ${curve} ${
                    open ? "translate-y-2 rotate-45" : ""
                  }`}
                />
                <span
                  className={`block h-0.5 w-5 bg-current transition-transform duration-500 ${curve} ${
                    open ? "-translate-y-2 -rotate-45" : ""
                  }`}
                />
              </span>
            </button>
          </div>
        </nav>
      </div>

      <div
        id="mobile-menu"
        className={`fixed inset-0 z-40 md:hidden ${
          open ? "pointer-events-auto" : "pointer-events-none"
        }`}
      >
        <div
          className={`absolute inset-0 bg-[#111111]/95 backdrop-blur-2xl transition-opacity duration-500 ${curve} ${
            open ? "opacity-100" : "opacity-0"
          }`}
        />
        <div className="relative flex min-h-screen flex-col justify-center px-6 pt-24">
          {links.map((link, i) => (
            <a
              key={link.href}
              href={link.href}
              onClick={() => setOpen(false)}
              style={{ transitionDelay: open ? `${i * 60}ms` : "0ms" }}
              className={`rounded-lg py-3 text-3xl font-semibold tracking-tight text-[#EAE5DC] transition-[transform,opacity] duration-500 ${curve} ${
                open ? "translate-y-0 opacity-100" : "translate-y-6 opacity-0"
              }`}
            >
              {link.label}
            </a>
          ))}
          <Link
            href="/console?tab=test"
            onClick={() => setOpen(false)}
            style={{ transitionDelay: open ? "300ms" : "0ms" }}
            className={`mt-8 rounded-xl bg-[#C87941] px-3 py-2 text-center ${type.buttonMd} text-[#111111] transition-[transform,opacity] duration-500 ${curve} ${
              open ? "translate-y-0 opacity-100" : "translate-y-6 opacity-0"
            }`}
          >
            Talk to the agent
          </Link>
        </div>
      </div>
    </>
  );
}
