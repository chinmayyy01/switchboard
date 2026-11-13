"use client";

import { useCallback, useEffect, useState, type FormEvent } from "react";
import { Badge, Card, Empty } from "@/components/ui";

const API = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8090";

type TeamAccount = {
  id: string;
  email: string;
  name?: string;
  role: "owner" | "manager" | "viewer";
  is_owner: boolean;
  email_verified: boolean;
  created_at: number;
};

type Invite = {
  email: string;
  tenant_id: string;
  role: string;
  expires_at: number;
  created_at: number;
  accepted: boolean;
};

const ROLE_LABEL: Record<string, string> = {
  owner: "Owner",
  manager: "Manager",
  viewer: "Viewer",
};

export function AccessTab() {
  const [accounts, setAccounts] = useState<TeamAccount[]>([]);
  const [invites, setInvites] = useState<Invite[]>([]);
  const [email, setEmail] = useState("");
  const [role, setRole] = useState("viewer");
  const [inviteURL, setInviteURL] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const [loading, setLoading] = useState(true);

  const load = useCallback(async () => {
    try {
      const [accountRes, inviteRes] = await Promise.all([
        fetch(`${API}/api/team/accounts`, { credentials: "include", cache: "no-store" }),
        fetch(`${API}/api/team/invites`, { credentials: "include", cache: "no-store" }),
      ]);
      if (!accountRes.ok) throw new Error("Could not load team access.");
      setAccounts((await accountRes.json()) as TeamAccount[]);
      setInvites(inviteRes.ok ? ((await inviteRes.json()) as Invite[]) : []);
      setError("");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Could not load team access.");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  const invite = async (event: FormEvent) => {
    event.preventDefault();
    setBusy(true);
    setError("");
    setInviteURL("");
    try {
      const res = await fetch(`${API}/api/team/invites`, {
        method: "POST",
        credentials: "include",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ email, role }),
      });
      const data = (await res.json().catch(() => ({}))) as { error?: string; invite_url?: string };
      if (!res.ok) throw new Error(data.error ?? "Could not create the invite.");
      setInviteURL(data.invite_url ?? "");
      setEmail("");
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Could not create the invite.");
    } finally {
      setBusy(false);
    }
  };

  const changeRole = async (id: string, value: string) => {
    setError("");
    try {
      const res = await fetch(`${API}/api/team/accounts/${encodeURIComponent(id)}`, {
        method: "PATCH",
        credentials: "include",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ role: value }),
      });
      if (!res.ok) throw new Error("Could not change the role.");
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Could not change the role.");
    }
  };

  const remove = async (id: string) => {
    setError("");
    try {
      const res = await fetch(`${API}/api/team/accounts/${encodeURIComponent(id)}`, {
        method: "DELETE",
        credentials: "include",
      });
      if (!res.ok) throw new Error("Could not remove the account.");
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Could not remove the account.");
    }
  };

  return (
    <div className="space-y-6">
      <header>
        <h1 className="text-2xl font-semibold tracking-tight text-[#EAE5DC]">Access</h1>
        <p className="mt-1 text-sm text-[#A8A092]">
          Invite teammates, choose their role, and remove access. Signup is invitation-only.
        </p>
      </header>

      {error && (
        <div role="alert" className="rounded-xl border border-rose-400/30 bg-rose-500/10 px-4 py-3 text-sm text-rose-300">
          {error}
        </div>
      )}

      <Card className="p-5">
        <h2 className="text-base font-semibold text-[#EAE5DC]">Invite a teammate</h2>
        <form onSubmit={invite} className="mt-3 flex flex-col gap-3 sm:flex-row">
          <input
            type="email"
            required
            value={email}
            onChange={(event) => setEmail(event.target.value)}
            placeholder="teammate@example.com"
            className="min-h-10 flex-1 rounded-lg border border-[#3A3A3A] bg-[#242424] px-3 py-2 text-sm text-[#EAE5DC] outline-none focus:border-[#C87941]"
          />
          <select
            value={role}
            onChange={(event) => setRole(event.target.value)}
            className="min-h-10 rounded-lg border border-[#3A3A3A] bg-[#242424] px-3 py-2 text-sm text-[#EAE5DC]"
          >
            <option value="viewer">Viewer</option>
            <option value="manager">Manager</option>
          </select>
          <button
            type="submit"
            disabled={busy}
            className="min-h-10 rounded-lg bg-[#C87941] px-4 py-2 text-sm font-semibold text-[#111111] hover:bg-[#D98E5A] disabled:opacity-60"
          >
            {busy ? "Inviting…" : "Send invite"}
          </button>
        </form>
        {inviteURL && (
          <p className="mt-3 break-all rounded-xl bg-[#242424] p-3 text-xs text-[#B5AEA3]">
            Invite link:{" "}
            <a href={inviteURL} className="text-[#D98E5A] hover:underline">
              {inviteURL}
            </a>
          </p>
        )}
      </Card>

      <Card className="p-5">
        <h2 className="text-base font-semibold text-[#EAE5DC]">People with access</h2>
        {loading ? (
          <p className="mt-3 text-sm text-[#A8A092]">Loading…</p>
        ) : accounts.length === 0 ? (
          <Empty>No accounts yet.</Empty>
        ) : (
          <div className="mt-3 divide-y divide-[#2E2E2E]">
            {accounts.map((account) => (
              <div key={account.id} className="flex flex-wrap items-center justify-between gap-3 py-3">
                <div>
                  <div className="flex flex-wrap items-center gap-2">
                    <span className="text-sm font-medium text-[#EAE5DC]">
                      {account.name || account.email}
                    </span>
                    {account.is_owner && <Badge tone="indigo">Owner</Badge>}
                    {!account.email_verified && <Badge tone="amber">Unverified</Badge>}
                  </div>
                  <p className="text-xs text-[#8A8379]">{account.email}</p>
                </div>
                <div className="flex items-center gap-2">
                  {account.is_owner ? (
                    <span className="text-xs text-[#A8A092]">{ROLE_LABEL[account.role] ?? account.role}</span>
                  ) : (
                    <>
                      <select
                        value={account.role}
                        onChange={(event) => void changeRole(account.id, event.target.value)}
                        className="min-h-9 rounded-lg border border-[#3A3A3A] bg-[#242424] px-2 py-1 text-xs text-[#D8D2C7]"
                      >
                        <option value="viewer">Viewer</option>
                        <option value="manager">Manager</option>
                      </select>
                      <button
                        type="button"
                        onClick={() => void remove(account.id)}
                        className="min-h-9 rounded-lg border border-rose-400/30 px-3 py-1 text-xs text-rose-300 hover:bg-rose-500/10"
                      >
                        Remove
                      </button>
                    </>
                  )}
                </div>
              </div>
            ))}
          </div>
        )}
      </Card>

      <Card className="p-5">
        <h2 className="text-base font-semibold text-[#EAE5DC]">Invites</h2>
        {invites.length === 0 ? (
          <Empty>No invites yet.</Empty>
        ) : (
          <div className="mt-3 space-y-2">
            {invites.slice(0, 10).map((invite) => (
              <div key={`${invite.email}-${invite.created_at}`} className="flex flex-wrap items-center justify-between gap-2 rounded-lg bg-[#242424] px-3 py-2">
                <span className="text-sm text-[#D8D2C7]">{invite.email}</span>
                <span className="text-xs text-[#8A8379]">
                  {ROLE_LABEL[invite.role] ?? invite.role} ·{" "}
                  {invite.accepted ? "accepted" : new Date(invite.expires_at * 1000).toLocaleDateString()}
                </span>
              </div>
            ))}
          </div>
        )}
      </Card>
    </div>
  );
}
