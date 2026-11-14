"use client";

import { useCallback, useEffect, useState, type FormEvent } from "react";
import { Alert, Card } from "@/components/ui";

const API = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8090";

type UsageLine = {
  kind: string;
  model: string;
  units: number;
  cost_minor: number;
};

type Summary = {
  period: string;
  currency: string;
  action: string;
  cap_minor: number;
  spent_minor: number;
  remaining_minor: number;
  default_cap_minor: number;
  items: UsageLine[];
};

const KIND_LABELS: Record<string, string> = {
  llm_input: "Reply model input",
  llm_output: "Reply model output",
  tts_characters: "Voice characters",
  stt_seconds: "Speech seconds",
  call_seconds: "Call time",
};

const ACTIONS: Record<string, string> = {
  transfer: "Transfer the caller to a person",
  refuse: "Stop booking and end politely",
  degrade: "Keep answering with the cheaper stack",
};

function money(minor: number, currency: string) {
  return new Intl.NumberFormat("en-IN", {
    style: "currency",
    currency: currency || "INR",
    maximumFractionDigits: 2,
  }).format(minor / 100);
}

function units(kind: string, value: number) {
  if (kind === "stt_seconds" || kind === "call_seconds") {
    return `${value.toLocaleString("en-IN")} s`;
  }
  if (kind === "tts_characters") {
    return `${value.toLocaleString("en-IN")} chars`;
  }
  return value.toLocaleString("en-IN");
}

export function BudgetCard({ tenant }: { tenant: string }) {
  const [summary, setSummary] = useState<Summary | null>(null);
  const [cap, setCap] = useState("");
  const [action, setAction] = useState("transfer");
  const [error, setError] = useState("");
  const [status, setStatus] = useState("");
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);

  const apply = useCallback((data: Summary) => {
    setSummary(data);
    setAction(data.action || "transfer");
    setCap(data.cap_minor > 0 ? String(data.cap_minor / 100) : "");
  }, []);

  useEffect(() => {
    let active = true;
    setLoading(true);
    setError("");
    setStatus("");
    fetch(`${API}/api/budget?tenant=${encodeURIComponent(tenant)}`, {
      credentials: "include",
      cache: "no-store",
    })
      .then(async (res) => {
        if (!res.ok) throw new Error("Could not load usage.");
        const data = (await res.json()) as Summary;
        if (active) apply(data);
      })
      .catch((err: Error) => {
        if (active) setError(err.message);
      })
      .finally(() => {
        if (active) setLoading(false);
      });
    return () => {
      active = false;
    };
  }, [apply, tenant]);

  const save = async (event: FormEvent) => {
    event.preventDefault();
    setSaving(true);
    setError("");
    setStatus("");
    const value = cap.trim() === "" ? 0 : Number(cap);
    if (!Number.isFinite(value) || value < 0) {
      setError("Enter a cap of zero or more, or leave it empty for no cap.");
      setSaving(false);
      return;
    }
    try {
      const res = await fetch(`${API}/api/budget?tenant=${encodeURIComponent(tenant)}`, {
        method: "PATCH",
        credentials: "include",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ cap_minor: Math.round(value * 100), action }),
      });
      if (!res.ok) throw new Error((await res.text()) || "Could not save the budget.");
      apply((await res.json()) as Summary);
      setStatus("Budget saved. New calls use this cap.");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Could not save the budget.");
    } finally {
      setSaving(false);
    }
  };

  const currency = summary?.currency || "INR";
  const spent = summary?.spent_minor ?? 0;
  const capMinor = summary?.cap_minor ?? 0;
  const percent = capMinor > 0 ? Math.min(100, Math.round((spent / capMinor) * 100)) : 0;
  const atLimit = capMinor > 0 && spent >= capMinor;
  const nearLimit = capMinor > 0 && !atLimit && spent >= capMinor * 0.8;

  return (
    <Card className="p-5">
      <h2 className="text-base font-semibold text-[#EAE5DC]">Usage and budget</h2>
      <p className="mt-1 max-w-xl text-xs leading-relaxed text-[#8A8379]">
        What this workspace has spent this month and what happens when the cap is reached.
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

      {loading || !summary ? (
        <p className="mt-4 text-sm text-[#A8A092]">Loading usage…</p>
      ) : (
        <>
          <div className="mt-4 grid gap-3 sm:grid-cols-3">
            <div>
              <div className="text-xs font-medium text-[#8A8379]">Spent this month</div>
              <div className="mt-0.5 text-2xl font-semibold tracking-tight text-[#EAE5DC]">
                {money(spent, currency)}
              </div>
            </div>
            <div>
              <div className="text-xs font-medium text-[#8A8379]">Monthly cap</div>
              <div className="mt-0.5 text-2xl font-semibold tracking-tight text-[#EAE5DC]">
                {capMinor > 0 ? money(capMinor, currency) : "No cap"}
              </div>
            </div>
            <div>
              <div className="text-xs font-medium text-[#8A8379]">
                {capMinor > 0 ? "Remaining" : "At the cap"}
              </div>
              <div className="mt-0.5 text-2xl font-semibold tracking-tight text-[#EAE5DC]">
                {capMinor > 0 ? money(summary.remaining_minor, currency) : "—"}
              </div>
            </div>
          </div>

          {capMinor > 0 && (
            <div className="mt-4">
              <div className="h-1.5 overflow-hidden rounded-full bg-[#EAE5DC]/10">
                <div
                  className={`h-full rounded-full ${
                    atLimit ? "bg-[#E08A80]" : nearLimit ? "bg-[#D9A45B]" : "bg-[#C87941]"
                  }`}
                  style={{ width: `${percent}%` }}
                />
              </div>
              <p className="mt-1.5 text-xs text-[#8A8379]">
                {percent}% used · {ACTIONS[summary.action] ?? summary.action}
              </p>
            </div>
          )}

          {atLimit && (
            <Alert tone="error" className="mt-4">
              The cap is reached. {ACTIONS[summary.action] ?? summary.action} on the next call.
            </Alert>
          )}
          {nearLimit && (
            <Alert tone="warning" className="mt-4">
              You have used {percent}% of this month&apos;s budget.
            </Alert>
          )}

          <form onSubmit={save} className="mt-5 flex flex-col gap-3 border-t border-[#2E2E2E] pt-4 sm:flex-row sm:items-end">
            <label className="block sm:w-40">
              <span className="mb-1 block text-xs font-medium text-[#A8A092]">
                Cap ({currency})
              </span>
              <input
                inputMode="decimal"
                value={cap}
                onChange={(event) => setCap(event.target.value)}
                placeholder="No cap"
                className="w-full rounded-xl border border-[#3A3A3A] bg-[#242424] px-3 py-2.5 text-sm text-[#EAE5DC] outline-none focus:border-[#C87941]"
              />
            </label>
            <label className="block sm:flex-1">
              <span className="mb-1 block text-xs font-medium text-[#A8A092]">At the cap</span>
              <select
                value={action}
                onChange={(event) => setAction(event.target.value)}
                className="w-full rounded-xl border border-[#3A3A3A] bg-[#242424] px-3 py-2.5 text-sm text-[#EAE5DC] outline-none focus:border-[#C87941]"
              >
                <option value="transfer">Transfer to a person</option>
                <option value="refuse">Stop booking and end</option>
                <option value="degrade">Keep answering, cheaper stack</option>
              </select>
            </label>
            <button
              type="submit"
              disabled={saving}
              className="rounded-xl bg-[#C87941] px-5 py-2.5 text-sm font-semibold text-[#111111] hover:bg-[#D98E5A] disabled:opacity-60"
            >
              {saving ? "Saving…" : "Save budget"}
            </button>
          </form>

          <div className="mt-5 border-t border-[#2E2E2E] pt-4">
            <h3 className="text-xs font-medium uppercase tracking-wide text-[#8A8379]">
              Breakdown · {summary.period}
            </h3>
            {summary.items.length === 0 ? (
              <p className="mt-2 text-sm text-[#8A8379]">No usage recorded this month.</p>
            ) : (
              <ul className="mt-2 divide-y divide-[#2E2E2E] text-sm">
                {summary.items.map((item) => (
                  <li key={`${item.kind}:${item.model}`} className="flex items-center justify-between gap-3 py-2">
                    <span className="min-w-0 truncate text-[#D8D2C7]">
                      {KIND_LABELS[item.kind] ?? item.kind}
                      {item.model && item.model !== "default" && (
                        <span className="ml-2 text-xs text-[#8A8379]">{item.model}</span>
                      )}
                    </span>
                    <span className="shrink-0 tabular-nums text-[#A8A092]">
                      {units(item.kind, item.units)} · {money(item.cost_minor, currency)}
                    </span>
                  </li>
                ))}
              </ul>
            )}
          </div>
        </>
      )}
    </Card>
  );
}
