"use client";

import { useEffect, useState, type FormEvent } from "react";
import Link from "next/link";
import { PasswordInput } from "@/components/PasswordInput";
import { Alert } from "@/components/ui";

const API = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8090";

type Invite = {
  email: string;
  tenant_id: string;
  role: string;
  expires_at: number;
};

export default function SignupPage() {
  const [token, setToken] = useState("");
  const [invite, setInvite] = useState<Invite | null>(null);
  const [name, setName] = useState("");
  const [business, setBusiness] = useState("");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [confirm, setConfirm] = useState("");
  const [error, setError] = useState("");
  const [devLink, setDevLink] = useState("");
  const [tenantName, setTenantName] = useState("");
  const [done, setDone] = useState(false);
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    const value = new URLSearchParams(window.location.search).get("token") ?? "";
    setToken(value);
    if (!value) return;
    fetch(`${API}/api/auth/invite?token=${encodeURIComponent(value)}`, { cache: "no-store" })
      .then(async (res) => {
        if (!res.ok) throw new Error("This invite is invalid or has expired.");
        setInvite((await res.json()) as Invite);
      })
      .catch((err: Error) => setError(err.message));
  }, []);

  const isInvite = Boolean(token);

  const submit = async (event: FormEvent) => {
    event.preventDefault();
    if (password !== confirm) {
      setError("Passwords do not match.");
      return;
    }
    setBusy(true);
    setError("");
    try {
      const res = await fetch(`${API}/api/auth/signup`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          token,
          name,
          business,
          email: isInvite ? invite?.email ?? "" : email,
          password,
        }),
      });
      const data = (await res.json().catch(() => ({}))) as {
        error?: string;
        dev_link?: string;
        tenant_name?: string;
      };
      if (!res.ok) throw new Error(data.error ?? "Could not create the account.");
      setDevLink(data.dev_link ?? "");
      setTenantName(data.tenant_name ?? "");
      setDone(true);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Could not create the account.");
    } finally {
      setBusy(false);
    }
  };

  return (
    <main className="grid min-h-screen place-items-center bg-[#111111] px-6 py-12 text-[#EAE5DC]">
      <div className="w-full max-w-md">
        <Link href="/" className="mb-6 block text-center text-sm font-semibold tracking-tight">
          Switchboard
        </Link>
        <div className="rounded-2xl border border-[#2E2E2E] bg-[#1A1A1A] p-6">
          <h1 className="text-xl font-semibold tracking-tight">
            {isInvite ? "Accept your invite" : "Create your workspace"}
          </h1>
          <p className="mt-1 text-sm text-[#A8A092]">
            {isInvite
              ? `Joining ${invite?.tenant_id ?? "a workspace"} as ${invite?.role ?? "a teammate"}.`
              : "Set up your business and start taking calls."}
          </p>

          {done ? (
            <div className="mt-5 space-y-3">
              <Alert tone="success">
                {tenantName ? `${tenantName} is ready. ` : ""}
                Check your inbox for the verification link.
              </Alert>
              {devLink && (
                <a href={devLink} className="block break-all rounded-xl bg-[#242424] p-3 text-xs text-[#D98E5A] hover:underline">
                  Development: verify now
                </a>
              )}
              <Link href="/login" className="block text-center text-sm text-[#D98E5A] hover:underline">
                Back to sign in
              </Link>
            </div>
          ) : (
            <form onSubmit={submit} className="mt-5 space-y-4">
              <label className="block">
                <span className="mb-1 block text-xs font-medium text-[#A8A092]">Your name</span>
                <input
                  required
                  value={name}
                  onChange={(event) => setName(event.target.value)}
                  className="w-full rounded-xl border border-[#3A3A3A] bg-[#111111] px-3 py-2.5 text-sm outline-none focus:border-[#C87941]"
                />
              </label>

              {!isInvite && (
                <>
                  <label className="block">
                    <span className="mb-1 block text-xs font-medium text-[#A8A092]">Business name</span>
                    <input
                      required
                      value={business}
                      onChange={(event) => setBusiness(event.target.value)}
                      placeholder="Rivera Plumbing & HVAC"
                      className="w-full rounded-xl border border-[#3A3A3A] bg-[#111111] px-3 py-2.5 text-sm outline-none focus:border-[#C87941]"
                    />
                  </label>
                  <label className="block">
                    <span className="mb-1 block text-xs font-medium text-[#A8A092]">Work email</span>
                    <input
                      type="email"
                      autoComplete="email"
                      required
                      value={email}
                      onChange={(event) => setEmail(event.target.value)}
                      className="w-full rounded-xl border border-[#3A3A3A] bg-[#111111] px-3 py-2.5 text-sm outline-none focus:border-[#C87941]"
                    />
                  </label>
                </>
              )}

              {isInvite && (
                <label className="block">
                  <span className="mb-1 block text-xs font-medium text-[#A8A092]">Email</span>
                  <input
                    readOnly
                    value={invite?.email ?? ""}
                    className="w-full rounded-xl border border-[#2E2E2E] bg-[#242424] px-3 py-2.5 text-sm text-[#A8A092]"
                  />
                </label>
              )}

              <label className="block">
                <span className="mb-1 block text-xs font-medium text-[#A8A092]">Password</span>
                <PasswordInput
                  autoComplete="new-password"
                  required
                  minLength={8}
                  value={password}
                  onChange={setPassword}
                  className="w-full rounded-xl border border-[#3A3A3A] bg-[#111111] px-3 py-2.5 text-sm outline-none focus:border-[#C87941]"
                />
                <span className="mt-1 block text-[11px] text-[#8A8379]">At least 8 characters.</span>
              </label>
              <label className="block">
                <span className="mb-1 block text-xs font-medium text-[#A8A092]">Confirm password</span>
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
                disabled={busy || (isInvite && !invite)}
                className="w-full rounded-xl bg-[#C87941] px-4 py-2.5 text-sm font-semibold text-[#111111] hover:bg-[#D98E5A] disabled:opacity-60"
              >
                {busy ? "Creating account…" : isInvite ? "Create account" : "Create workspace"}
              </button>
            </form>
          )}
        </div>
        <Link href="/login" className="mt-6 block text-center text-xs text-[#8A8379] hover:text-[#D8D2C7]">
          Already have an account? Sign in
        </Link>
      </div>
    </main>
  );
}
