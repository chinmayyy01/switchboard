"use client";

import { useState, type FormEvent } from "react";
import Link from "next/link";

const API = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8090";

export default function ForgotPasswordPage() {
  const [email, setEmail] = useState("");
  const [sent, setSent] = useState(false);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  const submit = async (event: FormEvent) => {
    event.preventDefault();
    setBusy(true);
    setError("");
    try {
      const res = await fetch(`${API}/api/auth/forgot-password`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ email }),
      });
      if (!res.ok && res.status !== 204) {
        const data = (await res.json().catch(() => ({}))) as { error?: string };
        throw new Error(data.error ?? "Could not send the reset link.");
      }
      setSent(true);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Could not send the reset link.");
    } finally {
      setBusy(false);
    }
  };

  return (
    <main className="grid min-h-screen place-items-center bg-[#111111] px-6 text-[#EAE5DC]">
      <div className="w-full max-w-md rounded-2xl border border-[#2E2E2E] bg-[#1A1A1A] p-6">
        <h1 className="text-xl font-semibold tracking-tight">Reset your password</h1>
        <p className="mt-1 text-sm text-[#A8A092]">
          If an account exists, we will email a one-hour reset link.
        </p>
        {sent ? (
          <div className="mt-5 space-y-3">
            <p className="rounded-xl border border-emerald-400/20 bg-emerald-500/10 px-3 py-2 text-sm text-emerald-300">
              Check your inbox for the reset link.
            </p>
            <Link href="/login" className="block text-center text-sm text-[#D98E5A] hover:underline">
              Back to sign in
            </Link>
          </div>
        ) : (
          <form onSubmit={submit} className="mt-5 space-y-4">
            <label className="block">
              <span className="mb-1 block text-xs font-medium text-[#A8A092]">Email</span>
              <input
                type="email"
                autoComplete="email"
                required
                value={email}
                onChange={(event) => setEmail(event.target.value)}
                className="w-full rounded-xl border border-[#3A3A3A] bg-[#111111] px-3 py-2.5 text-sm outline-none focus:border-[#C87941]"
              />
            </label>
            {error && (
              <p className="rounded-xl border border-rose-400/20 bg-rose-500/10 px-3 py-2 text-xs text-rose-300">
                {error}
              </p>
            )}
            <button
              type="submit"
              disabled={busy}
              className="w-full rounded-xl bg-[#C87941] px-4 py-2.5 text-sm font-semibold text-[#111111] hover:bg-[#D98E5A] disabled:opacity-60"
            >
              {busy ? "Sending…" : "Send reset link"}
            </button>
          </form>
        )}
      </div>
    </main>
  );
}
