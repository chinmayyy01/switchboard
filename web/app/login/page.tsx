"use client";

import { useEffect, useState, type FormEvent } from "react";
import Link from "next/link";
import { PasswordInput } from "@/components/PasswordInput";
import { Alert } from "@/components/ui";

const API = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8090";
const authDisabled = process.env.NEXT_PUBLIC_AUTH_DISABLED === "true";

function safeNext(raw: string | null): string {
  if (!raw) return "/console";
  if (raw === "/console" || raw.startsWith("/console?") || raw.startsWith("/console/")) {
    return raw;
  }
  return "/console";
}

export default function LoginPage() {
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [next, setNext] = useState("/console");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const [verifyRequired, setVerifyRequired] = useState(false);
  const [resent, setResent] = useState(false);

  useEffect(() => {
    if (authDisabled) {
      window.location.replace("/console");
      return;
    }
    const params = new URLSearchParams(window.location.search);
    setNext(safeNext(params.get("next")));
  }, []);

  const submit = async (event: FormEvent) => {
    event.preventDefault();
    setBusy(true);
    setError("");
    try {
      const res = await fetch(`${API}/api/auth/login`, {
        method: "POST",
        credentials: "include",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ email, password }),
      });
      if (!res.ok) {
        const data = (await res.json().catch(() => ({}))) as { error?: string; code?: string };
        setVerifyRequired(data.code === "verify_required");
        setError(data.error ?? "Could not sign in.");
        return;
      }
      window.location.assign(next);
    } catch {
      setError("Cannot reach the console API. Is the Go service running?");
    } finally {
      setBusy(false);
    }
  };

  if (authDisabled) {
    return (
      <div className="grid min-h-screen place-items-center bg-[#111111]">
        <div className="text-sm text-[#A8A092]">Auth is disabled. Opening the console…</div>
      </div>
    );
  }

  return (
    <main className="relative grid min-h-screen place-items-center overflow-hidden bg-[#111111] px-5 py-12">
      <div className="pointer-events-none absolute inset-0 bg-[radial-gradient(circle_at_50%_0%,rgba(200,121,65,0.16),transparent_55%)]" />
      <div className="relative w-full max-w-sm">
        <Link href="/" className="mb-8 flex items-center justify-center gap-2.5">
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
          <span className="text-sm font-semibold tracking-tight text-[#EAE5DC]">
            Switchboard
          </span>
        </Link>

        <div className="rounded-2xl border border-[#3A3A3A] bg-[#242424] p-6 shadow-[0_24px_80px_rgba(0,0,0,0.35)]">
          <h1 className="text-xl font-semibold tracking-tight text-[#EAE5DC]">
            Sign in to the console
          </h1>
          <p className="mt-1 text-sm text-[#A8A092]">
            Bookings, live calls, and the voice agent. Workspace owners and
            managers are unlimited; viewers include 2 free calls.
          </p>

          <form onSubmit={submit} className="mt-6 space-y-3">
            <label className="block">
              <span className="mb-1 block text-xs font-medium text-[#A8A092]">
                Email
              </span>
              <input
                type="email"
                autoComplete="email"
                required
                value={email}
                onChange={(event) => setEmail(event.target.value)}
                className="w-full rounded-xl border border-[#3A3A3A] bg-[#1A1A1A] px-3 py-2.5 text-sm text-[#EAE5DC] outline-none transition focus:border-[#C87941] focus:ring-2 focus:ring-[#C87941]/30"
              />
            </label>
            <label className="block">
              <span className="mb-1 block text-xs font-medium text-[#A8A092]">
                Password
              </span>
              <PasswordInput
                autoComplete="current-password"
                required
                value={password}
                onChange={setPassword}
                className="w-full rounded-xl border border-[#3A3A3A] bg-[#1A1A1A] px-3 py-2.5 text-sm text-[#EAE5DC] outline-none transition focus:border-[#C87941] focus:ring-2 focus:ring-[#C87941]/30"
              />
            </label>

            {error && (
              <Alert tone="error">{error}</Alert>
            )}

            <button
              type="submit"
              disabled={busy}
              className="w-full rounded-xl bg-[#C87941] px-4 py-2.5 text-sm font-semibold text-[#111111] transition-colors hover:bg-[#D98E5A] disabled:cursor-not-allowed disabled:opacity-60"
            >
              {busy ? "Signing in…" : "Sign in"}
            </button>

            {verifyRequired && (
              <button
                type="button"
                onClick={async () => {
                  setResent(false);
                  await fetch(`${API}/api/auth/resend-verification`, {
                    method: "POST",
                    headers: { "Content-Type": "application/json" },
                    body: JSON.stringify({ email }),
                  });
                  setResent(true);
                }}
                className="w-full rounded-xl border border-[#3A3A3A] px-4 py-2 text-xs font-medium text-[#D8D2C7] hover:bg-[#242424]"
              >
                {resent ? "Verification email sent" : "Resend verification email"}
              </button>
            )}

            <div className="flex items-center justify-between text-xs">
              <Link href="/forgot-password" className="text-[#D98E5A] hover:underline">
                Forgot password?
              </Link>
              <Link href="/signup" className="text-[#D98E5A] hover:underline">
                Create account
              </Link>
            </div>
          </form>

          <p className="mt-4 text-center text-[11px] leading-relaxed text-[#8A8379]">
            New business? Create a workspace. Teammates can also join with an
            invite from their owner.
          </p>
        </div>

        <Link
          href="/"
          className="mt-6 block text-center text-xs font-medium text-[#8A8379] transition-colors hover:text-[#D8D2C7]"
        >
          Back to site
        </Link>
      </div>
    </main>
  );
}
