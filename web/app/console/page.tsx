"use client";

import { useEffect, useRef, useState, type ReactNode } from "react";
import Link from "next/link";
import { BookingCard } from "@/components/BookingCard";
import { PhotoCard } from "@/components/PhotoCard";
import { TeamTab } from "@/components/TeamTab";
import { Badge, Card, Empty, SectionHeading, Stat } from "@/components/ui";
import { useBridge } from "@/lib/useBridge";
import { useSession } from "@/lib/session";
import { useSupervisor, type Call } from "@/lib/useSupervisor";
import { useSwitchboard } from "@/lib/useSwitchboard";

const API = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8090";
const authDisabled = process.env.NEXT_PUBLIC_AUTH_DISABLED === "true";
const api = (p: string) => `${API}${p}`;

type Tab = "overview" | "test" | "live" | "calls" | "evals" | "team";

type CallRecord = {
  id: string;
  tenant: string;
  lang: string;
  turns: number;
  escalated: boolean;
  reason: string;
  ended_at: string;
  label?: boolean;
  transcript?: { role: string; text: string }[];
  summary?: string;
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

const NAV: { id: Tab; label: string }[] = [
  { id: "overview", label: "Overview" },
  { id: "test", label: "Test a call" },
  { id: "live", label: "Live calls" },
  { id: "calls", label: "Call history" },
  { id: "evals", label: "Agent quality" },
  { id: "team", label: "Team" },
];

export default function Page() {
  const {
    inCall,
    status,
    rttMs,
    transcript,
    agentText,
    agentLang,
    tool,
    citations,
    booking,
    latency,
    level,
    sampleRate,
    devices,
    refreshDevices,
    startCall,
    endCall,
    sendText,
  } = useSwitchboard();
  const { account, logout, refresh } = useSession();
  const { calls, connected } = useSupervisor();
  const [tab, setTab] = useState<Tab>("overview");
  const [selectedId, setSelectedId] = useState("");
  const { status: bridgeStatus } = useBridge(selectedId);
  const [lang, setLang] = useState("en");
  const [tenant, setTenant] = useState("rivera");
  const [micId, setMicId] = useState("");
  const [records, setRecords] = useState<CallRecord[]>([]);
  const [report, setReport] = useState<Report | null>(null);
  const [expandedId, setExpandedId] = useState("");
  const [callQuery, setCallQuery] = useState("");
  const [callFilter, setCallFilter] = useState<"all" | "handled" | "help" | "empty">("all");
  const [callLimit, setCallLimit] = useState(10);
  const wasInCall = useRef(false);
  const [assistantStatus, setAssistantStatus] = useState<
    "checking" | "ready" | "offline"
  >("checking");

  useEffect(() => {
    const load = async () => {
      try {
        const [callsRes, reportRes] = await Promise.all([
          fetch(api("/api/calls"), { credentials: "include" }),
          fetch(api("/api/report"), { credentials: "include" }),
        ]);
        if (callsRes.status === 401) {
          window.location.assign("/login");
          return;
        }
        const calls = callsRes.ok ? await callsRes.json() : [];
        const report = reportRes.ok ? await reportRes.json() : null;
        setRecords(Array.isArray(calls) ? calls : []);
        setReport(report);
      } catch {}
    };
    load();
    const t = setInterval(load, 2000);
    return () => clearInterval(t);
  }, []);

  useEffect(() => {
    fetch(`${API}/healthz`)
      .then((response) => response.json())
      .then((health) =>
        setAssistantStatus(health.brain ? "ready" : "offline"),
      )
      .catch(() => setAssistantStatus("offline"));
  }, []);

  useEffect(() => {
    const t = new URLSearchParams(window.location.search).get("tab");
    const allowed = ["overview", "test", "live", "calls", "evals", "team"];
    if (t && allowed.includes(t)) setTab(t as Tab);
  }, []);

  useEffect(() => {
    if (account && !account.is_owner && account.tenant_id) {
      setTenant(account.tenant_id);
    }
  }, [account]);

  useEffect(() => {
    if (inCall) {
      refresh();
      wasInCall.current = true;
    } else if (wasInCall.current) {
      refresh();
      wasInCall.current = false;
    }
  }, [inCall, refresh]);

  const remaining = account
    ? Math.max(0, account.free_call_limit - account.calls_used)
    : 0;
  const unlimited = !!account?.is_owner;
  const outOfCalls = !!account && !unlimited && remaining <= 0;
  const activeTenant = account?.is_owner ? tenant : account?.tenant_id || tenant;

  const label = async (id: string, should: boolean) => {
    try {
      const response = await fetch(api(`/api/calls/${id}/label`), {
        method: "POST",
        credentials: "include",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ should_escalate: should }),
      });
      if (response.ok) {
        setRecords((current) =>
          current.map((record) =>
            record.id === id ? { ...record, label: should } : record,
          ),
        );
      }
    } catch {}
  };

  const startTest = () => {
    if (outOfCalls) return;
    setSelectedId("");
    startCall(lang, activeTenant, micId);
    setTab("test");
  };

  const takeOver = async (id: string) => {
    if (inCall) await endCall();
    setSelectedId(id);
    setTab("live");
  };

  const speaking = status.startsWith("agent");
  const callStatusLabel = !inCall
    ? "Ready to test"
    : speaking
      ? "Receptionist speaking"
      : status.startsWith("listening") || status === "connected"
        ? "Listening — speak when ready"
        : status === "mic error"
          ? "Microphone unavailable"
          : status === "call failed"
            ? "Could not connect"
            : status === "closed"
              ? "Call ended"
              : "Connecting";
  const selected = calls.find((c) => c.id === selectedId);
  const escalatedLive = calls.filter((c) => c.escalated).length;
  const completedRecords = records.filter((record) => record.turns > 0);
  const contained = completedRecords.filter((record) => !record.escalated).length;
  const containment = completedRecords.length
    ? contained / completedRecords.length
    : 0;
  const hasReport = report !== null && report.total > 0;
  const pct = (x: number) => `${(x * 100).toFixed(0)}%`;
  const filteredRecords = records.filter((record) => {
    const outcomeMatches =
      callFilter === "all" ||
      (callFilter === "help" && record.escalated) ||
      (callFilter === "handled" && !record.escalated && record.turns > 0) ||
      (callFilter === "empty" && !record.escalated && record.turns === 0);
    const searchText = [
      callSummary(record),
      record.reason,
      ...(record.transcript ?? []).map((turn) => turn.text),
    ]
      .join(" ")
      .toLowerCase();
    return outcomeMatches && searchText.includes(callQuery.trim().toLowerCase());
  });
  const visibleRecords = filteredRecords.slice(0, callLimit);

  return (
    <div className="console-app min-h-screen bg-[#111111] lg:flex">
      <aside className="console-sidebar hidden w-64 shrink-0 flex-col border-r border-[#3A3A3A] bg-[#242424] lg:sticky lg:top-0 lg:flex lg:h-screen lg:overflow-y-auto">
        <Link href="/" className="flex items-center gap-2.5 px-5 py-5">
          <Logo />
          <div>
            <div className="text-sm font-semibold tracking-tight text-[#EAE5DC]">
              Switchboard
            </div>
            <div className="text-xs text-[#A8A092]">AI receptionist</div>
          </div>
        </Link>

        <nav aria-label="Main navigation" className="flex-1 space-y-1 px-3">
          {NAV.map((item) => (
            <button
              key={item.id}
              type="button"
              aria-current={tab === item.id ? "page" : undefined}
              onClick={() => setTab(item.id)}
              className={`flex min-h-10 w-full items-center justify-between rounded-lg px-3 py-2 text-sm font-medium transition-colors ${
                tab === item.id
                  ? "bg-[#C87941] text-[#111111]"
                  : "text-[#B5AEA3] hover:bg-[#2A2A2A] hover:text-[#EAE5DC]"
              }`}
            >
              <span>{item.label}</span>
              {item.id === "live" && calls.length > 0 && (
                <span
                  className={`rounded-full px-1.5 text-[10px] font-semibold ${
                    tab === item.id
                      ? "bg-[#111111]/15 text-[#111111]"
                      : "bg-[#2E2E2E] text-[#EAE5DC]"
                  }`}
                >
                  {calls.length}
                </span>
              )}
            </button>
          ))}
        </nav>

        <div className="space-y-2 border-t border-[#2E2E2E] p-4">
          <div className="px-1 text-xs font-medium text-[#A8A092]">System status</div>
          <StatusRow
            label="Assistant"
            value={
              assistantStatus === "ready"
                ? "Ready"
                : assistantStatus === "checking"
                  ? "Checking…"
                  : "Unavailable"
            }
            tone={assistantStatus === "ready" ? "emerald" : assistantStatus === "offline" ? "rose" : "slate"}
          />
          <StatusRow
            label="Phone line"
            value="Not connected"
            tone="amber"
          />
          <div className="mt-3 border-t border-[#2E2E2E] pt-3">
            <div className="truncate px-1 text-[11px] text-[#B5AEA3]">
              {authDisabled ? "Preview workspace" : account?.email}
            </div>
            <div className="px-1 text-[10px] text-[#A8A092]">
              {authDisabled
                ? "Unlimited test calls"
                : account?.is_owner
                  ? "Owner · unlimited calls"
                  : `Free calls: ${remaining}/${account?.free_call_limit ?? 0}`}
            </div>
            {!authDisabled && (
              <button
                onClick={() => logout()}
                className="mt-2 w-full rounded-lg px-1 py-1.5 text-left text-xs font-medium text-[#A8A092] transition-colors hover:text-[#D8D2C7]"
              >
                Sign out
              </button>
            )}
          </div>
          <Link
            href="/"
            className="mt-1 block rounded-lg px-1 py-2 text-xs font-medium text-[#A8A092] transition-colors hover:text-[#D8D2C7]"
          >
            Back to site
          </Link>
        </div>
      </aside>

      <div className="console-sheet min-w-0 flex-1">
        <div className="border-b border-[#3A3A3A] bg-[#242424] lg:hidden">
          <div className="flex items-center justify-between px-4 py-3">
            <Link href="/" className="flex items-center gap-2.5">
              <Logo />
                <span className="text-sm font-semibold tracking-tight text-[#EAE5DC]">
                  Switchboard
                </span>
              </Link>
            <div className="flex items-center gap-3">
              <Badge tone={assistantStatus === "ready" ? "emerald" : assistantStatus === "offline" ? "rose" : "amber"}>
                {assistantStatus === "ready"
                  ? "Assistant ready"
                  : assistantStatus === "checking"
                    ? "Checking assistant"
                    : "Assistant unavailable"}
              </Badge>
              {!authDisabled && (
                <button
                  onClick={() => logout()}
                  className="text-xs font-medium text-[#8A8379] transition-colors hover:text-[#D8D2C7]"
                >
                  Sign out
                </button>
              )}
            </div>
          </div>
          <nav aria-label="Main navigation" className="grid grid-cols-3 gap-1 px-3 pb-3">
            {NAV.map((item) => (
              <button
                key={item.id}
                type="button"
                aria-current={tab === item.id ? "page" : undefined}
                onClick={() => setTab(item.id)}
                className={`min-h-10 rounded-lg px-2 py-2 text-xs font-medium transition-colors ${
                  tab === item.id
                    ? "bg-[#C87941] text-[#111111]"
                    : "text-[#B5AEA3] hover:bg-[#2A2A2A] hover:text-[#EAE5DC]"
                }`}
              >
                {item.label}
              </button>
            ))}
          </nav>
        </div>

        <main className="mx-auto max-w-[1440px] px-4 py-6 sm:px-6 lg:px-8 lg:py-8">
          <div
            className={`mb-5 rounded-lg border px-4 py-3 text-sm leading-relaxed ${
              authDisabled
                ? "border-[#C87941]/25 bg-[#C87941]/10 text-[#E8C3A4]"
                : unlimited
                  ? "border-emerald-400/20 bg-emerald-500/10 text-emerald-200"
                  : outOfCalls
                    ? "border-rose-400/20 bg-rose-500/10 text-rose-300"
                    : "border-[#C87941]/25 bg-[#C87941]/10 text-[#E8C3A4]"
            }`}
          >
            {authDisabled
              ? "Preview workspace · You can test calls in your browser."
              : unlimited
                ? "Owner workspace · No call limit."
                : outOfCalls
                  ? "You have used both trial calls. Contact the workspace owner to continue."
                  : `${remaining} trial calls remaining.`}
          </div>
          {tab === "overview" && (
            <Overview
              calls={calls}
              records={records}
              report={report}
              containment={containment}
              escalatedLive={escalatedLive}
              onGoLive={() => setTab("live")}
              onGoCalls={() => setTab("calls")}
              onStartTest={() => {
                if (inCall) {
                  setTab("test");
                } else {
                  startTest();
                }
              }}
            />
          )}

          {tab === "test" && (
            <div>
              <PageHeading
                badge="Browser test call"
                title="Try your receptionist"
                subtitle="Speak naturally, ask a question, or try booking a visit. You can interrupt at any time."
              />

              <div className="grid items-start gap-6 lg:grid-cols-2">
                <Card className="p-6">
                  <div className="flex items-start justify-between gap-4">
                    <div>
                      <h2 className="text-base font-semibold tracking-tight text-[#EAE5DC]">
                        Start a call
                      </h2>
                      <p className="text-sm text-[#A8A092]">
                        {account?.is_owner && !authDisabled
                          ? "Choose a business and language, then talk."
                          : "Choose a language, then try a few common requests."}
                      </p>
                    </div>
                    <span
                      className={`inline-flex items-center gap-2 rounded-full px-3 py-1 text-xs font-medium ${
                        inCall
                          ? speaking
                          ? "bg-amber-500/10 text-amber-300"
                            : status === "mic error" || status === "call failed"
                              ? "bg-rose-500/10 text-rose-300"
                              : "bg-emerald-500/10 text-emerald-300"
                          : "bg-[#2E2E2E] text-[#A8A092]"
                      }`}
                    >
                      <span
                        className={`h-2 w-2 rounded-full ${
                          inCall ? "animate-pulse bg-current" : "bg-[#6E675D]"
                        }`}
                      />
                      {callStatusLabel}
                    </span>
                  </div>

                  <div className="mt-5 grid gap-3 sm:grid-cols-2">
                    {account?.is_owner && !authDisabled && (
                      <label className="block">
                        <span className="mb-1 block text-xs font-medium text-[#A8A092]">
                          Business
                        </span>
                        <select
                          value={activeTenant}
                          onChange={(e) => setTenant(e.target.value)}
                          disabled={inCall}
                          className="w-full rounded-xl border border-[#3A3A3A] bg-[#242424] px-3 py-2.5 text-sm text-[#EAE5DC] outline-none transition focus:border-[#C87941] focus:ring-2 focus:ring-[#C87941]/30 disabled:opacity-50"
                        >
                          <option value="rivera">Rivera Plumbing &amp; HVAC</option>
                          <option value="acme">Acme Home Services</option>
                        </select>
                      </label>
                    )}

                    <label className={`block ${account?.is_owner && !authDisabled ? "" : "sm:col-span-2"}`}>
                      <span className="mb-1 block text-xs font-medium text-[#A8A092]">
                        Language
                      </span>
                      <select
                        value={lang}
                        onChange={(e) => setLang(e.target.value)}
                        disabled={inCall}
                        className="w-full rounded-xl border border-[#3A3A3A] bg-[#242424] px-3 py-2.5 text-sm text-[#EAE5DC] outline-none transition focus:border-[#C87941] focus:ring-2 focus:ring-[#C87941]/30 disabled:opacity-50"
                      >
                        <option value="">Auto-detect</option>
                        <option value="en">English</option>
                        <option value="hi">हिन्दी</option>
                      </select>
                    </label>
                  </div>

                  <div className="mt-3">
                    <div className="mb-1 flex items-center justify-between">
                      <span className="text-xs font-medium text-[#A8A092]">
                        Microphone
                      </span>
                      <button
                        type="button"
                        onClick={() => refreshDevices()}
                        className="text-xs font-medium text-[#D98E5A] hover:underline"
                      >
                        Refresh
                      </button>
                    </div>
                    <select
                      value={micId}
                      onChange={(e) => setMicId(e.target.value)}
                      disabled={inCall}
                      className="w-full rounded-xl border border-[#3A3A3A] bg-[#242424] px-3 py-2.5 text-sm text-[#EAE5DC] outline-none transition focus:border-[#C87941] focus:ring-2 focus:ring-[#C87941]/30 disabled:opacity-50"
                    >
                      <option value="">System default</option>
                      {devices.map((device) => (
                        <option key={device.deviceId} value={device.deviceId}>
                          {device.label}
                        </option>
                      ))}
                    </select>
                    <p className="mt-1 text-xs text-[#A8A092]">
                      Your browser may ask for microphone access when the call starts.
                    </p>
                  </div>

                  <button
                    onClick={() => (inCall ? endCall() : startTest())}
                    disabled={!inCall && outOfCalls}
                    className={`mt-5 w-full rounded-xl px-4 py-3 text-sm font-semibold transition-colors focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-offset-[#111111] disabled:cursor-not-allowed disabled:opacity-50 ${
                      inCall
                        ? "bg-rose-600 text-[#EAE5DC] hover:bg-rose-500 focus:ring-rose-300"
                        : "bg-[#C87941] text-[#111111] hover:bg-[#D98E5A] focus:ring-[#C87941]/40"
                    }`}
                  >
                    {inCall
                      ? "End call"
                      : outOfCalls
                        ? "No trial calls left"
                        : "Start browser call"}
                  </button>
                  <p
                    className={`mt-2 text-center text-xs ${
                      outOfCalls ? "text-rose-300" : "text-[#A8A092]"
                    }`}
                  >
                    {authDisabled
                      ? "Unlimited browser test calls in this preview."
                      : unlimited
                        ? "Owner workspace · no call limit."
                      : outOfCalls
                        ? "You have used both free calls."
                        : `${remaining} of ${account?.free_call_limit ?? 0} free calls left.`}
                  </p>

                  <details className="mt-4 rounded-xl border border-[#3A3A3A] bg-[#2A2A2A] px-4 py-3">
                    <summary className="cursor-pointer text-sm font-medium text-[#D8D2C7]">
                      Connection details (optional)
                    </summary>
                    <div className="mt-3 grid grid-cols-2 gap-2 text-center sm:grid-cols-4">
                    <MiniStat
                      label="RTT"
                      value={rttMs === null ? "—" : `${rttMs.toFixed(0)} ms`}
                    />
                    <MiniStat
                      label="STT"
                      value={latency ? `${latency.stt} ms` : "—"}
                    />
                    <MiniStat
                      label="1st audio"
                      value={
                        latency && latency.firstAudio >= 0
                          ? `${latency.firstAudio} ms`
                          : "—"
                      }
                    />
                    <MiniStat
                      label="Turn"
                      value={latency ? `${latency.turn} ms` : "—"}
                    />
                    </div>

                    <div className="mt-3">
                    <div className="flex items-center justify-between text-[10px] font-medium uppercase tracking-wide text-[#A8A092]">
                      <span>mic level</span>
                      <span>
                        {sampleRate
                          ? `${(sampleRate / 1000).toFixed(0)} kHz`
                          : "—"}
                      </span>
                    </div>
                    <div className="mt-1 h-1.5 w-full overflow-hidden rounded-full bg-[#2E2E2E]">
                      <div
                        className="h-full rounded-full bg-emerald-500/100 transition-[width] duration-100"
                        style={{
                          width: `${Math.min(100, (level / 6000) * 100)}%`,
                        }}
                      />
                    </div>
                    </div>
                  </details>

                  <div className="mt-5 border-t border-[#2E2E2E] pt-4">
                    <div className="text-sm font-medium text-[#D8D2C7]">
                      Try asking
                    </div>
                    <p className="mt-1 text-xs text-[#A8A092]">Choose an example or say it out loud.</p>
                    <div className="mt-2 flex flex-wrap gap-1.5">
                      {[
                        "My kitchen sink is leaking",
                        "What is your call-out fee?",
                        "I need to reschedule Friday",
                      ].map((line) => (
                        <button
                          key={line}
                          type="button"
                          disabled={!inCall}
                          onClick={() => sendText(line)}
                          className="min-h-9 rounded-lg bg-[#2E2E2E] px-3 py-2 text-xs text-[#B5AEA3] transition-colors hover:bg-[#3A3A3A] disabled:cursor-not-allowed disabled:opacity-60"
                        >
                          {line}
                        </button>
                      ))}
                    </div>
                  </div>
                </Card>

                <div className="space-y-6">
                  <BookingCard booking={booking} />
                  <PhotoCard inCall={inCall} onSendText={sendText} />

                  <Card className="p-6">
                    <div className="flex items-center justify-between gap-3">
                      <h2 className="text-base font-semibold tracking-tight text-[#EAE5DC]">
                        Conversation
                      </h2>
                      <div className="flex flex-wrap gap-2">
                        {agentLang && <Badge tone="indigo">{languageName(agentLang)}</Badge>}
                      </div>
                    </div>

                    <div className="mt-5 space-y-4">
                      <Bubble who="Caller" text={transcript} tone="slate" />
                      <Bubble who="Agent" text={agentText} tone="indigo" />
                    </div>

                    {(tool || citations.length > 0) && (
                      <div className="mt-4 flex flex-wrap gap-2 border-t border-[#2E2E2E] pt-4">
                        {tool && <Badge>{toolLabel(tool)}</Badge>}
                        {citations.map((c) => (
                          <Badge key={c} tone="emerald">
                            source · {c}
                          </Badge>
                        ))}
                      </div>
                    )}
                  </Card>

                  <Card className="p-6">
                    <h2 className="text-base font-semibold tracking-tight text-[#EAE5DC]">
                      Before you start
                    </h2>
                    <ul className="mt-3 list-disc space-y-2 pl-5 text-sm leading-relaxed text-[#A8A092]">
                      <li>Allow microphone access when your browser asks.</li>
                      <li>Use headphones to avoid echo. You can interrupt the receptionist at any time.</li>
                      <li>Call transcripts are saved in Call history. Audio is not recorded in this preview.</li>
                    </ul>
                    <p className="mt-4 rounded-xl bg-[#2A2A2A] p-4 text-xs leading-relaxed text-[#A8A092]">
                      For a quick start, choose an example prompt above after the call connects.
                    </p>
                  </Card>
                </div>
              </div>
            </div>
          )}

          {tab === "live" && (
            <div>
              <PageHeading
                badge="Warm transfer"
                title="Live calls"
                subtitle="Join a live conversation when the receptionist asks your team for help."
                action={
                  <Badge tone={connected ? "emerald" : "rose"}>
                    {connected ? "Ready to join" : "Reconnecting…"}
                  </Badge>
                }
              />

              {selectedId && (
                <Card className="mb-6 border-emerald-400/30 p-5">
                  <div className="flex flex-wrap items-center justify-between gap-3">
                    <div>
                      <div className="flex flex-wrap items-center gap-2">
                        <span className="text-sm font-semibold text-[#EAE5DC]">
                          You are on call
                        </span>
                        <Badge
                          tone={
                            bridgeStatus === "connected" ? "emerald" : "amber"
                          }
                        >
                          {bridgeStatus === "connected" ? "Audio connected" : "Connecting audio"}
                        </Badge>
                      </div>
                      <p className="mt-1 text-sm text-[#A8A092]">
                      The receptionist is paused. The caller can hear you now. Use headphones to avoid echo.
                      </p>
                    </div>
                    <button
                      onClick={() => setSelectedId("")}
                      className="rounded-xl bg-[#2E2E2E] px-4 py-2 text-xs font-semibold text-[#EAE5DC] transition-colors hover:bg-[#3A3A3A]"
                    >
                      Leave call
                    </button>
                  </div>

                  {selected?.reason && (
                    <div className="mt-4 rounded-xl border border-rose-400/20 bg-rose-500/10 px-4 py-3 text-sm text-rose-300">
                      <span className="font-medium">Reason for handoff: </span>
                      {selected.reason}
                    </div>
                  )}

                  <div className="mt-4 space-y-1.5">
                    {(selected?.transcript ?? []).map((turn, i) => (
                      <p key={i} className="text-sm leading-relaxed">
                        <span className="mr-1 text-xs font-medium uppercase tracking-wide text-[#8A8379]">
                          {turn.role}
                        </span>
                        <span
                          className={
                            turn.role === "caller"
                              ? "text-[#D8D2C7]"
                              : "text-[#EAE5DC]"
                          }
                        >
                          {turn.text}
                        </span>
                      </p>
                    ))}
                    {(selected?.transcript ?? []).length === 0 && (
                      <p className="text-sm text-[#8A8379]">
                        Waiting for the transcript…
                      </p>
                    )}
                  </div>
                </Card>
              )}

              <div className="space-y-3">
                {calls.map((call) => (
                  <LiveCallCard
                    key={call.id}
                    call={call}
                    active={call.id === selectedId}
                    onTakeOver={() => takeOver(call.id)}
                  />
                ))}
                {calls.length === 0 && (
                  <Empty>
                    No callers are waiting. Calls that need your help will appear here.
                  </Empty>
                )}
              </div>
            </div>
          )}

          {tab === "calls" && (
            <div>
              <PageHeading
                badge={`${records.length} recorded`}
                title="Call history"
                subtitle="Review previous conversations and see how each one ended."
                onGoTest={() => setTab("test")}
              />

              <div className="grid min-w-0 grid-cols-1 gap-6 lg:grid-cols-[minmax(0,2fr)_minmax(280px,0.8fr)]">
                <div className="min-w-0 space-y-3">
                  <div className="flex min-w-0 flex-col gap-3 sm:flex-row">
                    <label className="min-w-0 flex-1">
                      <span className="sr-only">Search calls</span>
                      <input
                        type="search"
                        value={callQuery}
                        onChange={(event) => {
                          setCallQuery(event.target.value);
                          setCallLimit(10);
                        }}
                        placeholder="Search calls or transcripts"
                        className="min-h-11 w-full rounded-lg border border-[#3A3A3A] bg-[#242424] px-3 py-2 text-sm text-[#EAE5DC] placeholder:text-[#A8A092] focus:border-[#C87941] focus:outline-none"
                      />
                    </label>
                    <label className="sm:w-56">
                      <span className="sr-only">Filter calls by outcome</span>
                      <select
                        value={callFilter}
                        onChange={(event) => {
                          setCallFilter(event.target.value as typeof callFilter);
                          setCallLimit(10);
                        }}
                        className="min-h-11 w-full rounded-lg border border-[#3A3A3A] bg-[#242424] px-3 py-2 text-sm text-[#EAE5DC] focus:border-[#C87941] focus:outline-none"
                      >
                        <option value="all">All calls</option>
                        <option value="handled">Handled by receptionist</option>
                        <option value="help">Team help requested</option>
                        <option value="empty">No details captured</option>
                      </select>
                    </label>
                  </div>
                  <p className="text-xs text-[#A8A092]">
                    {filteredRecords.length} {filteredRecords.length === 1 ? "call" : "calls"}
                  </p>
                  <Card className="min-w-0 divide-y divide-[#2E2E2E]">
                    {visibleRecords.map((record) => (
                      <RecordRow
                        key={record.id}
                        record={record}
                        expanded={expandedId === record.id}
                        onToggle={() =>
                          setExpandedId(expandedId === record.id ? "" : record.id)
                        }
                        onLabel={(value) => label(record.id, value)}
                      />
                    ))}
                    {filteredRecords.length === 0 && (
                      <div className="p-4">
                        <Empty>
                          {records.length === 0
                            ? "Your conversations will appear here after they end."
                            : "No calls match those search and filter choices."}
                        </Empty>
                      </div>
                    )}
                  </Card>
                  {visibleRecords.length < filteredRecords.length && (
                    <button
                      type="button"
                      onClick={() => setCallLimit((limit) => limit + 10)}
                      className="min-h-10 w-full rounded-lg border border-[#3A3A3A] bg-[#242424] px-4 py-2 text-sm font-medium text-[#D8D2C7] hover:bg-[#2A2A2A]"
                    >
                      Show {Math.min(10, filteredRecords.length - visibleRecords.length)} older calls
                    </button>
                  )}
                </div>

                <div className="space-y-4">
                  <Card className="p-5">
                    <h3 className="text-sm font-semibold text-[#EAE5DC]">
                      Review call handling
                    </h3>
                    <p className="mt-1 text-sm leading-relaxed text-[#A8A092]">
                      Tell us whether a teammate should have joined the call. Your feedback helps improve when the receptionist asks for help.
                    </p>
                  </Card>

                  {hasReport && (
                    <Card className="p-5">
                      <h3 className="text-sm font-semibold text-[#EAE5DC]">
                        Call handling summary
                      </h3>
                        <dl className="mt-3 space-y-2 text-sm">
                        <Row label="Calls reviewed" value={String(report!.total)} />
                        <Row label="Correct team requests" value={pct(report!.precision)} />
                        <Row label="Missed requests for help" value={String(report!.fn)} />
                        <Row label="Handled without a teammate" value={pct(report!.containment)} />
                      </dl>
                    </Card>
                  )}
                </div>
              </div>
            </div>
          )}

          {tab === "evals" && (
            <div>
              <PageHeading
                badge="Agent quality"
                title="Agent quality"
                subtitle="See how often the receptionist handles calls and asks your team for help at the right time."
              />

              <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
                <Stat
                  label="Correct team requests"
                  value={hasReport ? pct(report!.precision) : "—"}
                  hint={
                    hasReport
                      ? `${report!.tp} correct out of ${report!.tp + report!.fp} requests`
                      : "Review calls to measure"
                  }
                />
                <Stat
                  label="Missed requests for help"
                  value={hasReport ? pct(report!.recall) : "—"}
                  hint={
                    hasReport ? `${report!.fn} calls` : "Review calls to measure"
                  }
                />
                <Stat
                  label="Handled without a teammate"
                  value={completedRecords.length ? pct(containment) : "—"}
                  hint={`${contained} of ${completedRecords.length} completed calls`}
                />
                <Stat
                  label="Calls reviewed"
                  value={String(report?.total ?? 0)}
                  hint="Add feedback in Call history"
                />
              </div>

              <div className="mt-6 grid gap-6 lg:grid-cols-[0.9fr_1.1fr]">
                <Card className="p-5">
                  <h3 className="text-sm font-semibold text-[#EAE5DC]">
                    Team handoff decisions
                  </h3>
                  <p className="mt-1 text-xs text-[#8A8379]">
                    The receptionist&apos;s choice compared with your review.
                  </p>
                  <div className="mt-4 grid grid-cols-[auto_1fr_1fr] gap-1 text-center text-xs">
                    <div />
                    <div className="pb-1 font-medium text-[#8A8379]">
                      Teammate needed
                    </div>
                    <div className="pb-1 font-medium text-[#8A8379]">
                      Receptionist could handle
                    </div>
                    <div className="pr-2 text-right font-medium text-[#8A8379]">
                      Asked for help
                    </div>
                    <MatrixCell
                      value={report?.tp ?? 0}
                      tone="emerald"
                      hint="right call"
                    />
                    <MatrixCell
                      value={report?.fp ?? 0}
                      tone="rose"
                      hint="asked unnecessarily"
                    />
                    <div className="pr-2 text-right font-medium text-[#8A8379]">
                      Kept the call
                    </div>
                    <MatrixCell
                      value={report?.fn ?? 0}
                      tone="amber"
                      hint="missed help"
                    />
                    <MatrixCell
                      value={report?.tn ?? 0}
                      tone="emerald"
                      hint="right call"
                    />
                  </div>
                  {!hasReport && (
                    <p className="mt-4 text-xs text-[#8A8379]">
                      Review a few calls in Call history to see these results.
                    </p>
                  )}
                </Card>

                <Card className="p-5">
                  <h3 className="text-sm font-semibold text-[#EAE5DC]">
                    What we track
                  </h3>
                  <ul className="mt-3 space-y-3 text-sm">
                    <Measure
                      title="The right time to ask your team for help"
                      text="Your feedback shows if the receptionist asked unnecessarily or missed a call that needed a person."
                    />
                    <Measure
                      title="Calls handled without a teammate"
                      text="The share of completed conversations the receptionist resolved on its own."
                    />
                    <Measure
                      title="Successful visit changes"
                      text="Checks that booking, rescheduling, and cancellation work as expected."
                    />
                    <Measure
                      title="Accurate business answers"
                      text="Checks that policy answers come from your business information and that the receptionist says when it does not know."
                    />
                    <Measure
                      title="Response time"
                      text="Measures how quickly the receptionist listens, responds, and speaks."
                    />
                  </ul>
                  <details className="mt-4 rounded-xl bg-[#2A2A2A] p-3 text-xs text-[#A8A092]">
                    <summary className="cursor-pointer font-medium text-[#D8D2C7]">
                      Technical evaluation details
                    </summary>
                    <p className="mt-2 leading-relaxed">
                      The development team can run the booking, handoff, and answer-quality test suites with <code className="font-mono">make eval</code>.
                    </p>
                  </details>
                </Card>
              </div>
            </div>
          )}
          {tab === "team" && (
            <TeamTab
              tenant={activeTenant}
              canSwitchTenant={!!account?.is_owner && !authDisabled}
              onTenantChange={setTenant}
            />
          )}
        </main>
      </div>
    </div>
  );
}

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

function StatusRow({
  label,
  value,
  tone,
}: {
  label: string;
  value: string;
  tone: "slate" | "emerald" | "amber" | "rose";
}) {
  const dot =
    tone === "emerald"
      ? "bg-emerald-500/100"
      : tone === "amber"
        ? "bg-amber-500/100"
        : tone === "rose"
          ? "bg-rose-500/100"
          : "bg-[#6E675D]";
  return (
    <div className="flex items-center justify-between px-1 py-1 text-xs">
      <span className="text-[#A8A092]">{label}</span>
      <span className="inline-flex items-center gap-1.5 font-medium text-[#D8D2C7]">
        <span className={`h-1.5 w-1.5 rounded-full ${dot}`} />
        {value}
      </span>
    </div>
  );
}

function PageHeading({
  badge,
  title,
  subtitle,
  action,
  onGoTest,
}: {
  badge: string;
  title: string;
  subtitle: string;
  action?: ReactNode;
  onGoTest?: () => void;
}) {
  return (
    <div className="mb-6 flex flex-wrap items-end justify-between gap-4">
      <div>
        <h1 className="text-2xl font-semibold tracking-tight text-[#EAE5DC]">
          {title}
        </h1>
        <p className="mt-1 text-sm text-[#A8A092]">{subtitle}</p>
      </div>
      <div className="flex items-center gap-3">
        {onGoTest && (
          <button
            onClick={onGoTest}
            className="rounded-xl border border-[#3A3A3A] bg-[#242424] px-3 py-2 text-xs font-semibold text-[#D8D2C7] transition-colors hover:bg-[#2A2A2A]"
          >
            Try a call
          </button>
        )}
        {action}
      </div>
    </div>
  );
}

function Overview({
  calls,
  records,
  report,
  containment,
  escalatedLive,
  onGoLive,
  onGoCalls,
  onStartTest,
}: {
  calls: Call[];
  records: CallRecord[];
  report: Report | null;
  containment: number;
  escalatedLive: number;
  onGoLive: () => void;
  onGoCalls: () => void;
  onStartTest: () => void;
}) {
  const pct = (x: number) => `${(x * 100).toFixed(0)}%`;
  const recent = records.slice(0, 5);
  const completed = records.filter((record) => record.turns > 0);
  const needsHelp = calls.filter((call) => call.escalated).slice(0, 4);

  return (
    <div className="space-y-7">
      <div className="flex flex-wrap items-end justify-between gap-4">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight text-[#EAE5DC]">
            Overview
          </h1>
          <p className="mt-1 max-w-2xl text-sm text-[#A8A092]">
            See what your receptionist handled and where your team may need to step in.
          </p>
        </div>
        <button
          type="button"
          onClick={onStartTest}
          className="min-h-11 rounded-lg bg-[#C87941] px-4 py-2.5 text-sm font-semibold text-[#111111] transition-colors hover:bg-[#D98E5A] focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-[#C87941]"
        >
          Try a test call
        </button>
      </div>

      <section aria-label="Call summary" className="grid gap-3 sm:grid-cols-3">
        <Stat
          label="Calls in history"
          value={String(records.length)}
          hint="All saved calls"
        />
        <Stat
          label="Handled by receptionist"
          value={completed.length ? pct(containment) : "—"}
          hint={
            completed.length
              ? `Across ${completed.length} completed calls`
              : "No completed calls yet"
          }
        />
        <Stat
          label="Callers waiting now"
          value={String(calls.length)}
          hint={escalatedLive ? `${escalatedLive} need your help` : "No one waiting"}
        />
      </section>

      <section aria-labelledby="phone-line-heading" className="rounded-xl border border-[#C87941]/25 bg-[#C87941]/10 p-4 sm:flex sm:items-center sm:justify-between sm:gap-6">
        <div className="flex items-start gap-3">
          <span aria-hidden="true" className="mt-1 h-2.5 w-2.5 shrink-0 rounded-full bg-amber-500" />
          <div>
            <h2 id="phone-line-heading" className="text-sm font-semibold text-[#EAE5DC]">
              Your business phone is not connected
            </h2>
            <p className="mt-1 text-sm leading-relaxed text-[#A8A092]">
              This preview only supports browser calls, so customers cannot call your receptionist yet.
            </p>
          </div>
        </div>
        <button
          type="button"
          onClick={onStartTest}
          className="mt-3 min-h-10 shrink-0 rounded-lg border border-[#3A3A3A] bg-[#2E2E2E] px-3 py-2 text-sm font-medium text-[#EAE5DC] transition-colors hover:bg-[#3A3A3A] focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-[#C87941] sm:mt-0"
        >
          Test the receptionist
        </button>
      </section>

      <div className="grid items-start gap-7 xl:grid-cols-[minmax(0,1.4fr)_minmax(320px,0.8fr)]">
        <section>
          <SectionHeading
            title="Recent calls"
            subtitle="What your receptionist heard and how each call ended"
            action={
              <button
                type="button"
                onClick={onGoCalls}
                className="mr-2 min-h-9 whitespace-nowrap rounded-md px-2 text-sm font-medium text-[#D98E5A] hover:bg-[#2A2A2A] focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-[#C87941]"
              >
                See all calls
              </button>
            }
          />
          <Card className="divide-y divide-[#2E2E2E]">
            {recent.map((record) => (
              <div
                key={record.id}
                className="flex flex-wrap items-center justify-between gap-3 px-4 py-4"
              >
                <div className="min-w-0 flex-1">
                  <div className="truncate text-sm font-medium text-[#EAE5DC]">
                    {callSummary(record)}
                  </div>
                  <div className="mt-1 flex flex-wrap items-center gap-x-2 gap-y-1 text-xs text-[#A8A092]">
                    <span>{formatCallTime(record.ended_at)}</span>
                    <span aria-hidden="true">·</span>
                    <span>{record.turns} {record.turns === 1 ? "message" : "messages"}</span>
                  </div>
                </div>
                <Badge tone={record.escalated ? "rose" : record.turns > 0 ? "emerald" : "slate"}>
                  {recordOutcome(record)}
                </Badge>
              </div>
            ))}
            {recent.length === 0 && (
              <div className="p-4">
                <Empty>
                  <div className="font-medium text-[#D8D2C7]">Your call history will appear here.</div>
                  <div className="mt-1">Try a browser call to see how your receptionist works.</div>
                  <button
                    type="button"
                    onClick={onStartTest}
                    className="mt-4 min-h-10 rounded-lg bg-[#C87941] px-4 py-2 text-sm font-semibold text-[#111111] hover:bg-[#D98E5A] focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-[#C87941]"
                  >
                    Try a test call
                  </button>
                </Empty>
              </div>
            )}
          </Card>
        </section>

        <section>
          <SectionHeading
            title="Callers waiting now"
            subtitle="A teammate can join calls that need personal help"
            action={
              <button
                type="button"
                onClick={onGoLive}
                className="mr-2 min-h-9 whitespace-nowrap rounded-md px-2 text-sm font-medium text-[#D98E5A] hover:bg-[#2A2A2A] focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-[#C87941]"
              >
                View live calls
              </button>
            }
          />
          <Card className="divide-y divide-[#2E2E2E]">
            {needsHelp.map((call) => (
              <div key={call.id} className="px-4 py-3">
                <div className="flex flex-wrap items-center gap-2">
                  <Badge tone="rose">Needs your help</Badge>
                  {call.tenant && <span className="text-xs text-[#A8A092]">{call.tenant}</span>}
                </div>
                <p className="mt-2 truncate text-sm text-[#D8D2C7]">
                  {call.last_text || "Waiting for more details"}
                </p>
              </div>
            ))}
            {needsHelp.length === 0 && (
              <div className="p-4">
                <Empty>No callers need a teammate right now.</Empty>
              </div>
            )}
          </Card>
        </section>
      </div>

      <section aria-label="Receptionist capabilities" className="border-t border-[#3A3A3A] pt-5">
        <h2 className="text-sm font-semibold text-[#EAE5DC]">What your receptionist can do</h2>
        <p className="mt-1 max-w-3xl text-sm leading-relaxed text-[#A8A092]">
          Answer common questions from your business information, check staff availability, book or change a visit, and bring your team into a call when needed.
        </p>
        <p className="mt-2 text-xs text-[#8A8379]">
          {report && report.total > 0
            ? `${report.total} calls have been reviewed to improve handoffs.`
            : "Review calls in Agent quality to help improve when the receptionist asks your team for help."}
        </p>
      </section>
    </div>
  );
}

function formatCallTime(value: string) {
  if (!value) return "Time unavailable";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "Time unavailable";
  return date.toLocaleString(undefined, {
    month: "short",
    day: "numeric",
    hour: "numeric",
    minute: "2-digit",
  });
}

function callSummary(record: CallRecord) {
  const callerText = record.transcript?.find((turn) => turn.role === "caller")?.text;
  if (callerText) return callerText;
  const summary = record.summary
    ?.replace(/^handled automatically\.?\s*/i, "")
    .replace(/^caller asked:\s*/i, "")
    .trim();
  return summary || "No caller details captured";
}

function recordOutcome(record: CallRecord) {
  if (record.escalated) return "Team help";
  return record.turns > 0 ? "Handled" : "No details";
}

function toolLabel(tool: string) {
  const labels: Record<string, string> = {
    set_service: "Confirming the service",
    save_address: "Saving the service address",
    set_time: "Checking the requested time",
    set_phone: "Saving the callback number",
    check_availability: "Checking team availability",
    book: "Booking the visit",
    lookup_job: "Looking up the visit",
    reschedule: "Changing the visit time",
    cancel: "Cancelling the visit",
    lookup_policy: "Checking business information",
    transfer_to_human: "Asking a teammate to join",
  };
  return labels[tool] ?? tool.replaceAll("_", " ");
}

function languageName(code: string) {
  if (code.toLowerCase().startsWith("hi")) return "Hindi";
  if (code.toLowerCase().startsWith("en")) return "English";
  return code;
}

function liveCallStatus(call: Call) {
  const state = call.state?.toLowerCase() ?? "";
  if (call.taken_over) return "Your teammate is speaking with the caller";
  if (call.escalated) return "Waiting for a teammate to join";
  if (state.includes("speak")) return "The receptionist is speaking";
  if (state.includes("listen")) return "Listening to the caller";
  if (state.includes("tool") || state.includes("think")) {
    return "Working on the caller’s request";
  }
  return "Call in progress";
}

function LiveCallCard({
  call,
  active,
  onTakeOver,
}: {
  call: Call;
  active: boolean;
  onTakeOver: () => void;
}) {
  return (
    <Card className={`p-5 ${active ? "ring-2 ring-emerald-400/30" : ""}`}>
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <div className="flex flex-wrap items-center gap-2">
            <span className="text-sm font-semibold text-[#EAE5DC]">
              {call.escalated ? "Caller needs help" : "Call in progress"}
            </span>
            {call.lang && <Badge tone="indigo">{languageName(call.lang)}</Badge>}
          </div>
          <p className="mt-1 text-xs text-[#A8A092]">{liveCallStatus(call)}</p>
        </div>
        {call.taken_over ? (
          <Badge tone="emerald">A teammate has joined</Badge>
        ) : (
          <button
            type="button"
            onClick={onTakeOver}
            className="min-h-10 rounded-lg bg-emerald-600 px-4 py-2 text-sm font-semibold text-[#EAE5DC] transition-colors hover:bg-emerald-500/100 focus:outline-none focus:ring-2 focus:ring-emerald-300"
          >
            {active ? "Rejoin call" : "Join call"}
          </button>
        )}
      </div>

      {call.reason && (
        <p className="mt-2 text-xs text-rose-300">Reason for handoff: {call.reason}</p>
      )}

      <div className="mt-3 space-y-1.5">
        {(call.transcript ?? []).map((turn, i) => (
          <p key={i} className="text-sm leading-relaxed">
            <span className="mr-1 text-xs font-medium uppercase tracking-wide text-[#8A8379]">
              {turn.role === "caller" ? "Caller" : "Agent"}
            </span>
            <span
              className={
                turn.role === "caller" ? "text-[#D8D2C7]" : "text-[#EAE5DC]"
              }
            >
              {turn.text}
            </span>
          </p>
        ))}
        {(call.transcript ?? []).length === 0 && (
          <p className="text-sm text-[#8A8379]">{call.last_text || "…"}</p>
        )}
      </div>
    </Card>
  );
}

function RecordRow({
  record,
  expanded,
  onToggle,
  onLabel,
}: {
  record: CallRecord;
  expanded: boolean;
  onToggle: () => void;
  onLabel: (value: boolean) => void;
}) {
  return (
    <div>
      <div className="flex flex-col gap-3 p-4 xl:flex-row xl:items-center xl:justify-between">
        <div className="min-w-0 flex-1">
          <div className="truncate text-sm font-medium text-[#EAE5DC]">
            {callSummary(record)}
          </div>
          <div className="mt-1 flex flex-wrap items-center gap-x-2 text-xs text-[#A8A092]">
            <span>{formatCallTime(record.ended_at)}</span>
            <span aria-hidden="true">·</span>
            <span>{record.turns} {record.turns === 1 ? "message" : "messages"}</span>
            <Badge tone={record.escalated ? "rose" : record.turns > 0 ? "emerald" : "slate"}>
              {recordOutcome(record)}
            </Badge>
          </div>
        </div>
        <div className="flex w-full min-w-0 flex-col gap-2 xl:max-w-[440px] xl:shrink-0">
          <span className="text-xs text-[#A8A092]">Should a teammate have joined?</span>
          <div className="grid w-full min-w-0 grid-cols-1 gap-2 sm:grid-cols-2 xl:grid-cols-[minmax(0,1fr)_minmax(0,1fr)_36px]">
          <button
            type="button"
            aria-pressed={record.label === true}
            onClick={() => onLabel(true)}
            className={`min-h-9 w-full min-w-0 rounded-md px-3 py-2 text-left text-xs font-medium transition-colors ${
              record.label === true
                ? "bg-rose-600 text-[#EAE5DC]"
                : "bg-[#2E2E2E] text-[#B5AEA3] hover:bg-[#3A3A3A]"
            }`}
          >
            Yes, needed
          </button>
          <button
            type="button"
            aria-pressed={record.label === false}
            onClick={() => onLabel(false)}
            className={`min-h-9 w-full min-w-0 rounded-md px-3 py-2 text-left text-xs font-medium transition-colors ${
              record.label === false
                ? "bg-emerald-600 text-[#EAE5DC]"
                : "bg-[#2E2E2E] text-[#B5AEA3] hover:bg-[#3A3A3A]"
            }`}
          >
            No, handled well
          </button>
          <button
            onClick={onToggle}
            type="button"
            aria-label={expanded ? "Hide transcript" : "Show transcript"}
            aria-expanded={expanded}
            className="grid h-9 w-9 place-items-center rounded-lg text-[#A8A092] transition-colors hover:bg-[#2A2A2A] hover:text-[#D8D2C7]"
          >
            <svg
              viewBox="0 0 24 24"
              className={`h-4 w-4 transition-transform ${expanded ? "rotate-180" : ""}`}
              fill="none"
              stroke="currentColor"
              strokeWidth="2"
              strokeLinecap="round"
            >
              <path d="M6 9l6 6 6-6" />
            </svg>
          </button>
          </div>
        </div>
      </div>

      {expanded && (
        <div className="border-t border-[#2E2E2E] bg-[#2A2A2A] px-4 py-4">
          {record.summary && (
            <p className="text-sm font-medium text-[#EAE5DC]">
              {record.summary}
            </p>
          )}
          <div className="mt-3 space-y-2">
            {(record.transcript ?? []).map((turn, i) => (
              <p key={i} className="text-sm leading-relaxed">
                <span className="mr-1 text-xs font-medium uppercase tracking-wide text-[#8A8379]">
                  {turn.role}
                </span>
                <span
                  className={
                    turn.role === "caller" ? "text-[#D8D2C7]" : "text-[#EAE5DC]"
                  }
                >
                  {turn.text}
                </span>
              </p>
            ))}
            {(record.transcript ?? []).length === 0 && (
              <p className="text-xs text-[#8A8379]">
                No transcript stored for this call.
              </p>
            )}
          </div>
        </div>
      )}
    </div>
  );
}

function MatrixCell({
  value,
  tone,
  hint,
}: {
  value: number;
  tone: "emerald" | "rose" | "amber";
  hint: string;
}) {
  const styles =
    tone === "emerald"
      ? "bg-emerald-500/10 text-emerald-300 ring-emerald-400/20"
      : tone === "rose"
        ? "bg-rose-500/10 text-rose-300 ring-rose-400/20"
        : "bg-amber-500/10 text-amber-300 ring-amber-400/20";
  return (
    <div className={`rounded-lg px-2 py-3 ring-1 ${styles}`}>
      <div className="text-lg font-semibold tabular-nums">{value}</div>
      <div className="text-[10px] opacity-70">{hint}</div>
    </div>
  );
}

function Measure({ title, text }: { title: string; text: string }) {
  return (
    <li>
      <div className="font-medium text-[#EAE5DC]">{title}</div>
      <p className="text-xs leading-relaxed text-[#A8A092]">{text}</p>
    </li>
  );
}

function Row({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex items-center justify-between">
      <dt className="text-[#A8A092]">{label}</dt>
      <dd className="font-semibold tabular-nums text-[#EAE5DC]">{value}</dd>
    </div>
  );
}

function MiniStat({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-lg bg-[#2E2E2E] px-2 py-2">
      <div className="text-[10px] font-medium uppercase tracking-wide text-[#A8A092]">
        {label}
      </div>
      <div className="text-sm font-semibold tabular-nums text-[#EAE5DC]">
        {value}
      </div>
    </div>
  );
}

function Bubble({
  who,
  text,
  tone,
}: {
  who: string;
  text: string;
  tone: "slate" | "indigo";
}) {
  const styles =
    tone === "indigo"
      ? "bg-[#C87941]/10 text-[#EAE5DC] ring-[#C87941]/30"
      : "bg-[#2A2A2A] text-[#EAE5DC] ring-[#EAE5DC]/5";
  return (
    <div>
      <div className="mb-1 text-xs font-medium uppercase tracking-wide text-[#A8A092]">
        {who}
      </div>
      <div
        className={`rounded-2xl px-4 py-3 text-sm leading-relaxed ring-1 ${styles}`}
      >
        {text || <span className="text-[#A8A092]">…</span>}
      </div>
    </div>
  );
}
