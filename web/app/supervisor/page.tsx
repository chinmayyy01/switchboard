"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import { useSupervisor } from "@/lib/useSupervisor";

const API = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8090";

type CallRecord = {
  id: string;
  lang: string;
  turns: number;
  escalated: boolean;
  reason: string;
  ended_at: string;
  label?: boolean;
};

type Report = {
  total: number;
  tp: number;
  fp: number;
  tn: number;
  fn: number;
  precision: number;
  recall: number;
  f1: number;
  containment: number;
};

export default function SupervisorPage() {
  const { calls, connected } = useSupervisor();
  const [records, setRecords] = useState<CallRecord[]>([]);
  const [report, setReport] = useState<Report | null>(null);

  useEffect(() => {
    const load = async () => {
      try {
        const [r, rep] = await Promise.all([
          fetch(`${API}/api/calls`).then((x) => x.json()),
          fetch(`${API}/api/report`).then((x) => x.json()),
        ]);
        setRecords(r ?? []);
        setReport(rep ?? null);
      } catch {}
    };
    load();
    const t = setInterval(load, 2000);
    return () => clearInterval(t);
  }, []);

  const label = async (id: string, should: boolean) => {
    try {
      await fetch(`${API}/api/calls/${id}/label`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ should_escalate: should }),
      });
    } catch {}
  };

  const takeOver = async (id: string) => {
    try {
      await fetch(`${API}/api/calls/${id}/takeover`, { method: "POST" });
    } catch {}
  };

  const pct = (x: number) => `${(x * 100).toFixed(0)}%`;

  return (
    <main className="mx-auto max-w-3xl px-6 py-16 font-mono leading-relaxed">
      <div className="flex items-baseline justify-between">
        <h1 className="text-2xl font-semibold">Switchboard · Supervisor</h1>
        <Link href="/" className="text-sm text-blue-600 hover:underline">
          caller view →
        </Link>
      </div>

      <p className="mt-1 text-sm text-neutral-500">
        live calls: {calls.length} ·{" "}
        <span className={connected ? "text-emerald-600" : "text-red-500"}>
          {connected ? "connected" : "disconnected"}
        </span>
      </p>

      <ul className="mt-4 divide-y divide-neutral-200">
        {calls.map((c) => (
          <li key={c.id} className="py-3">
            <div className="flex items-center gap-2 text-sm">
              <span className="rounded bg-neutral-200 px-2 py-0.5 text-xs">
                {c.id}
              </span>
              {c.lang && (
                <span className="rounded bg-blue-100 px-2 py-0.5 text-xs text-blue-800">
                  {c.lang}
                </span>
              )}
              {c.state && (
                <span className="text-xs text-neutral-500">{c.state}</span>
              )}
              {c.escalated && (
                <span className="rounded bg-red-100 px-2 py-0.5 text-xs font-semibold text-red-700">
                  ESCALATED
                </span>
              )}
              {c.taken_over ? (
                <span className="rounded bg-emerald-100 px-2 py-0.5 text-xs font-semibold text-emerald-700">
                  YOU OWN THIS
                </span>
              ) : (
                c.escalated && (
                  <button
                    onClick={() => takeOver(c.id)}
                    className="rounded bg-emerald-600 px-2 py-0.5 text-xs text-white hover:bg-emerald-500"
                  >
                    take over
                  </button>
                )
              )}
            </div>
            {c.reason && (
              <p className="mt-1 text-xs text-red-600">why: {c.reason}</p>
            )}
            <ul className="mt-1 space-y-0.5">
              {(c.transcript ?? []).map((t, i) => (
                <li key={i} className="text-xs">
                  <span
                    className={
                      t.role === "caller"
                        ? "text-neutral-500"
                        : "text-emerald-700"
                    }
                  >
                    {t.role}:{" "}
                  </span>
                  {t.text}
                </li>
              ))}
              {(c.transcript ?? []).length === 0 && (
                <li className="text-xs text-neutral-400">
                  {c.last_text || "…"}
                </li>
              )}
            </ul>
          </li>
        ))}
        {calls.length === 0 && (
          <li className="py-3 text-sm text-neutral-400">no active calls</li>
        )}
      </ul>

      <h2 className="mt-10 text-lg font-semibold">Escalation policy</h2>
      {report && report.total > 0 ? (
        <div className="mt-2 grid grid-cols-2 gap-x-8 gap-y-1 text-sm sm:grid-cols-4">
          <Stat label="precision" value={pct(report.precision)} />
          <Stat label="recall" value={pct(report.recall)} />
          <Stat label="f1" value={pct(report.f1)} />
          <Stat label="containment" value={pct(report.containment)} />
          <div className="col-span-full mt-1 text-xs text-neutral-500">
            TP {report.tp} · FP {report.fp} · TN {report.tn} · FN {report.fn} ·
            labeled {report.total}
          </div>
        </div>
      ) : (
        <p className="mt-2 text-sm text-neutral-400">
          label some calls below to compute precision / recall
        </p>
      )}

      <h2 className="mt-10 text-lg font-semibold">Recent calls</h2>
      <p className="mt-1 text-xs text-neutral-500">
        Mark whether each call actually needed a human (ground truth).
      </p>
      <ul className="mt-3 divide-y divide-neutral-200">
        {records.map((r) => (
          <li key={r.id} className="flex items-center justify-between py-2 text-sm">
            <div className="min-w-0">
              <span className="rounded bg-neutral-200 px-2 py-0.5 text-xs">
                {r.id}
              </span>
              <span className="ml-2 text-neutral-500">
                {r.escalated ? `escalated (${r.reason})` : "contained"} ·{" "}
                {r.turns} turns
              </span>
            </div>
            <div className="flex shrink-0 gap-1">
              <button
                onClick={() => label(r.id, true)}
                className={`rounded px-2 py-0.5 text-xs ${
                  r.label === true
                    ? "bg-red-600 text-white"
                    : "bg-neutral-100 hover:bg-neutral-200"
                }`}
              >
                needed human
              </button>
              <button
                onClick={() => label(r.id, false)}
                className={`rounded px-2 py-0.5 text-xs ${
                  r.label === false
                    ? "bg-emerald-600 text-white"
                    : "bg-neutral-100 hover:bg-neutral-200"
                }`}
              >
                should contain
              </button>
            </div>
          </li>
        ))}
        {records.length === 0 && (
          <li className="py-2 text-sm text-neutral-400">no calls yet</li>
        )}
      </ul>
    </main>
  );
}

function Stat({ label, value }: { label: string; value: string }) {
  return (
    <div>
      <div className="text-xs text-neutral-500">{label}</div>
      <div className="text-lg tabular-nums">{value}</div>
    </div>
  );
}
