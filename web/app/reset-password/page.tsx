"use client";

import { useEffect, useState, type FormEvent } from "react";
import Link from "next/link";
import { PasswordInput } from "@/components/PasswordInput";
import { Alert } from "@/components/ui";

const API = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8090";

export default function ResetPasswordPage() {
  const [token, setToken] = useState("");
  const [password, setPassword] = useState("");
  const [confirm, setConfirm] = useState("");
  const [error, setError] = useState("");
  const [done, setDone] = useState(false);
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    setToken(new URLSearchParams(window.location.search).get("token") ?? "");
  }, []);

  const submit = async (event: FormEvent) => {
    event.preventDefault();
    if (password !== confirm) {
      setError("Passwords do not match.");
      return;
    }
    setBusy(true);
    setError("");
    try {
      const res = await fetch(`${API}/api/auth/reset-password`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ token, password }),
      });
      const data = (await res.json().catch(() => ({}))) as { error?: string };
      if (!res.ok) throw new Error(data.error ?? "Could not reset the password.");
      setDone(true);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Could not reset the password.");
    } finally {
      setBusy(false);
    }
  };

  return (
    <main className="grid min-h-screen place-items-center bg-[#111111] px-6 text-[#EAE5DC]">
      <div className="w-full max-w-md rounded-2xl border border-[#2E2E2E] bg-[#1A1A1A] p-6">
        <h1 className="text-xl font-semibold tracking-tight">Choose a new password</h1>
        {done ? (
          <div className="mt-5 space-y-3">
            <Alert tone="success">Password updated. Sign in with the new password.</Alert>
            <Link href="/login" className="block text-center text-sm text-[#D98E5A] hover:underline">
              Sign in
            </Link>
          </div>
        ) : (
          <form onSubmit={submit} className="mt-5 space-y-4">
            <label className="block">
              <span className="mb-1 block text-xs font-medium text-[#A8A092]">New password</span>
              <PasswordInput
                autoComplete="new-password"
                required
                minLength={8}
                value={password}
                onChange={setPassword}
                className="w-full rounded-xl border border-[#3A3A3A] bg-[#111111] px-3 py-2.5 text-sm outline-none focus:border-[#C87941]"
              />
            </label>
            <label className="block">
              <span className="mb-1 block text-xs font-medium text-[#A8A092]">Confirm</span>
              <PasswordInput
                autoComplete="new-password"
                required
                minLength={8}
                value={confirm}
                onChange={setConfirm}
                className="w-full rounded-xl border border-[#3A3A3A] bg-[#111111] px-3 py-2.5 text-sm outline-none focus:border-[#C87941]"
              />
            </label>
            {error && (
              <Alert tone="error">{error}</Alert>
            )}
            <button
              type="submit"
              disabled={busy || !token}
              className="w-full rounded-xl bg-[#C87941] px-4 py-2.5 text-sm font-semibold text-[#111111] hover:bg-[#D98E5A] disabled:opacity-60"
            >
              {busy ? "Updating…" : "Update password"}
            </button>
          </form>
        )}
      </div>
    </main>
  );
}
