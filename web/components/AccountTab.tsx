"use client";

import { useState, type FormEvent } from "react";
import { PasswordInput } from "@/components/PasswordInput";
import { Alert, Badge, Card } from "@/components/ui";

const API = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8090";

const roleLabel: Record<string, string> = {
  owner: "Owner",
  manager: "Manager",
  viewer: "Viewer",
};

export function AccountTab({ email, role }: { email?: string; role?: string }) {
  const [current, setCurrent] = useState("");
  const [next, setNext] = useState("");
  const [confirm, setConfirm] = useState("");
  const [status, setStatus] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  const submit = async (event: FormEvent) => {
    event.preventDefault();
    setError("");
    setStatus("");
    if (next !== confirm) {
      setError("New passwords do not match.");
      return;
    }
    if (next.length < 8) {
      setError("Use at least 8 characters.");
      return;
    }
    setBusy(true);
    try {
      const res = await fetch(`${API}/api/auth/change-password`, {
        method: "POST",
        credentials: "include",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ current_password: current, new_password: next }),
      });
      if (!res.ok) {
        const data = (await res.json().catch(() => ({}))) as { error?: string };
        throw new Error(data.error ?? "Could not update the password.");
      }
      setCurrent("");
      setNext("");
      setConfirm("");
      setStatus("Password updated. Other sessions were signed out.");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Could not update the password.");
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="space-y-6">
      <header>
        <h1 className="text-2xl font-semibold tracking-tight text-[#EAE5DC]">Account</h1>
        <p className="mt-1 text-sm text-[#A8A092]">Your sign-in details and password.</p>
      </header>

      <Card className="p-5">
        <h2 className="text-base font-semibold text-[#EAE5DC]">Sign-in</h2>
        <dl className="mt-3 grid gap-3 sm:grid-cols-2">
          <div>
            <dt className="text-xs font-medium text-[#8A8379]">Email</dt>
            <dd className="mt-0.5 truncate text-sm text-[#D8D2C7]">{email ?? "—"}</dd>
          </div>
          <div>
            <dt className="text-xs font-medium text-[#8A8379]">Role</dt>
            <dd className="mt-0.5">
              <Badge tone="slate">{role ? roleLabel[role] ?? role : "—"}</Badge>
            </dd>
          </div>
        </dl>
      </Card>

      <Card className="p-5">
        <h2 className="text-base font-semibold text-[#EAE5DC]">Change password</h2>
        <p className="mt-1 text-xs text-[#8A8379]">
          Updating your password signs out your other devices. This session stays active.
        </p>

        {error && (
          <Alert tone="error" className="mt-4">
            {error}
          </Alert>
        )}
        {status && (
          <Alert tone="success" className="mt-4">
            {status}
          </Alert>
        )}

        <form onSubmit={submit} className="mt-4 space-y-4 sm:max-w-md">
          <label className="block">
            <span className="mb-1 block text-xs font-medium text-[#A8A092]">Current password</span>
            <PasswordInput
              autoComplete="current-password"
              required
              value={current}
              onChange={setCurrent}
              className="w-full rounded-xl border border-[#3A3A3A] bg-[#242424] px-3 py-2.5 text-sm text-[#EAE5DC] outline-none focus:border-[#C87941]"
            />
          </label>
          <label className="block">
            <span className="mb-1 block text-xs font-medium text-[#A8A092]">New password</span>
            <PasswordInput
              autoComplete="new-password"
              required
              minLength={8}
              value={next}
              onChange={setNext}
              className="w-full rounded-xl border border-[#3A3A3A] bg-[#242424] px-3 py-2.5 text-sm text-[#EAE5DC] outline-none focus:border-[#C87941]"
            />
            <span className="mt-1 block text-[11px] text-[#8A8379]">At least 8 characters.</span>
          </label>
          <label className="block">
            <span className="mb-1 block text-xs font-medium text-[#A8A092]">Confirm new password</span>
            <PasswordInput
              autoComplete="new-password"
              required
              minLength={8}
              value={confirm}
              onChange={setConfirm}
              className="w-full rounded-xl border border-[#3A3A3A] bg-[#242424] px-3 py-2.5 text-sm text-[#EAE5DC] outline-none focus:border-[#C87941]"
            />
          </label>
          <button
            type="submit"
            disabled={busy}
            className="rounded-xl bg-[#C87941] px-5 py-2.5 text-sm font-semibold text-[#111111] hover:bg-[#D98E5A] disabled:opacity-60"
          >
            {busy ? "Updating…" : "Update password"}
          </button>
        </form>
      </Card>
    </div>
  );
}
