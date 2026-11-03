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
const AI = process.env.NEXT_PUBLIC_AI_URL ?? "http://127.0.0.1:8100";
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
  { id: "test", label: "Test agent" },
  { id: "live", label: "Live calls" },
  { id: "calls", label: "Calls" },
  { id: "evals", label: "Evaluations" },
  { id: "team", label: "Team" },
];

export default function Page() {
  const {
    inCall,
    status,
    rttMs,
    transcript,
    agentText,
    agentState,
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
  const wasInCall = useRef(false);
  const [engine, setEngine] = useState<{
    brain?: string;
    stt?: string;
    tts?: string;
    deepgram?: boolean;
    sarvam?: boolean;
  }>({});

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
    Promise.all([
      fetch(`${API}/healthz`)
        .then((r) => r.json())
        .catch(() => ({})),
      fetch(`${AI}/health`)
        .then((r) => r.json())
        .catch(() => ({})),
    ]).then(([g, p]) =>
      setEngine({
        brain: g.brain,
        stt: p.stt_provider,
        tts: p.tts_provider,
        deepgram: p.deepgram,
        sarvam: p.sarvam,
      }),
    );
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
      await fetch(api(`/api/calls/${id}/label`), {
        method: "POST",
        credentials: "include",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ should_escalate: should }),
      });
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
  const selected = calls.find((c) => c.id === selectedId);
  const escalatedLive = calls.filter((c) => c.escalated).length;
  const contained = records.filter((r) => !r.escalated).length;
  const containment = records.length ? contained / records.length : 0;
  const hasReport = report !== null && report.total > 0;
  const pct = (x: number) => `${(x * 100).toFixed(0)}%`;

  const engineLabel =
    engine.brain === "llm/sarvam"
      ? "Sarvam 105B"
      : engine.brain === "llm/openai"
        ? "OpenAI"
        : engine.brain === "hybrid/ollama"
          ? "Hybrid (Ollama)"
          : engine.brain === "llm/ollama"
            ? "Ollama"
            : engine.brain === "rules"
              ? "Rules"
              : "—";
  const sttLabel =
    engine.stt === "sarvam" && engine.sarvam
      ? "Sarvam Saaras"
      : engine.stt === "deepgram" && engine.deepgram
        ? "Deepgram"
        : engine.stt === "local"
          ? "Local Whisper"
          : "—";
  const ttsLabel =
    engine.tts === "sarvam" && engine.sarvam
      ? "Sarvam Bulbul"
      : engine.tts === "piper"
        ? "Piper"
        : "—";

  return (
    <div className="min-h-screen bg-[#111111] lg:flex">
      <aside className="hidden w-64 shrink-0 flex-col border-r border-[#3A3A3A] bg-[#242424] lg:sticky lg:top-0 lg:flex lg:h-screen lg:overflow-y-auto">
        <Link href="/" className="flex items-center gap-2.5 px-5 py-5">
          <Logo />
          <div>
            <div className="text-sm font-semibold tracking-tight text-[#EAE5DC]">
              Switchboard
            </div>
            <div className="text-[11px] text-[#8A8379]">voice agent console</div>
          </div>
        </Link>

        <nav className="flex-1 space-y-1 px-3">
          {NAV.map((item) => (
            <button
              key={item.id}
              onClick={() => setTab(item.id)}
              className={`flex w-full items-center justify-between rounded-lg px-3 py-2 text-sm font-medium transition-colors ${
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

        <div className="space-y-1 border-t border-[#2E2E2E] p-4">
          <StatusRow
            label="Engine"
            value={engineLabel}
            tone={
              engine.brain === "llm/sarvam" ||
              engine.brain === "llm/openai" ||
              engine.brain === "hybrid/ollama"
                ? "emerald"
                : engine.brain === "rules"
                  ? "amber"
                  : "slate"
            }
          />
          <StatusRow
            label="Speech in"
            value={sttLabel}
            tone={sttLabel === "Deepgram" || sttLabel === "Sarvam Saaras" ? "emerald" : "amber"}
          />
          <StatusRow
            label="Voice"
            value={ttsLabel}
            tone={ttsLabel === "Sarvam Bulbul" ? "emerald" : "slate"}
          />
          <StatusRow
            label="Supervisor"
            value={connected ? "live" : "offline"}
            tone={connected ? "emerald" : "rose"}
          />
          <div className="mt-3 border-t border-[#2E2E2E] pt-3">
            <div className="truncate px-1 text-[11px] text-[#B5AEA3]">
              {authDisabled ? "Local preview" : account?.email}
            </div>
            <div className="px-1 text-[10px] text-[#8A8379]">
              {authDisabled
                ? "Auth disabled"
                : account?.is_owner
                  ? "Owner · unlimited calls"
                  : `Free calls: ${remaining}/${account?.free_call_limit ?? 0}`}
            </div>
            {!authDisabled && (
              <button
                onClick={() => logout()}
                className="mt-2 w-full rounded-lg px-1 py-1.5 text-left text-xs font-medium text-[#8A8379] transition-colors hover:text-[#D8D2C7]"
              >
                Sign out
              </button>
            )}
          </div>
          <Link
            href="/"
            className="mt-1 block rounded-lg px-1 py-2 text-xs font-medium text-[#8A8379] transition-colors hover:text-[#D8D2C7]"
          >
            Back to site
          </Link>
        </div>
      </aside>

      <div className="min-w-0 flex-1">
        <div className="border-b border-[#3A3A3A] bg-[#242424] lg:hidden">
          <div className="flex items-center justify-between px-4 py-3">
            <Link href="/" className="flex items-center gap-2.5">
              <Logo />
              <span className="text-sm font-semibold tracking-tight text-[#EAE5DC]">
                Switchboard
              </span>
            </Link>
            <div className="flex items-center gap-3">
              <Badge tone={connected ? "emerald" : "slate"}>
                {connected ? "supervisor live" : "supervisor offline"}
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
          <div className="flex gap-1 overflow-x-auto px-3 pb-2">
            {NAV.map((item) => (
              <button
                key={item.id}
                onClick={() => setTab(item.id)}
                className={`shrink-0 rounded-lg px-3 py-1.5 text-xs font-medium transition-colors ${
                  tab === item.id
                    ? "bg-[#C87941] text-[#111111]"
                    : "text-[#B5AEA3] hover:bg-[#2A2A2A]"
                }`}
              >
                {item.label}
              </button>
            ))}
          </div>
        </div>

        <main className="mx-auto max-w-6xl px-4 py-6 sm:px-6 lg:py-8">
          <div
            className={`mb-4 rounded-xl border px-4 py-3 text-sm ${
              unlimited
                ? "border-emerald-400/20 bg-emerald-500/10 text-emerald-200"
                : outOfCalls
                  ? "border-rose-400/20 bg-rose-500/10 text-rose-300"
                  : "border-[#C87941]/25 bg-[#C87941]/10 text-[#E8C3A4]"
            }`}
          >
            {authDisabled
              ? "Local preview · auth is disabled · unlimited calls."
              : unlimited
                ? "Owner account · unlimited calls."
                : outOfCalls
                  ? "You have used both free calls. Ask the owner for more access."
                  : `${remaining} of ${account?.free_call_limit ?? 0} free calls left.`}
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
                title="Test the agent"
                subtitle="Talk to it like a customer. Interrupt it. Then check how it scored."
              />

              <div className="grid gap-6 lg:grid-cols-2">
                <Card className="p-6">
                  <div className="flex items-start justify-between gap-4">
                    <div>
                      <h2 className="text-base font-semibold tracking-tight text-[#EAE5DC]">
                        Start a call
                      </h2>
                      <p className="text-sm text-[#A8A092]">
                        Choose a business, then talk.
                      </p>
                    </div>
                    <span
                      className={`inline-flex items-center gap-2 rounded-full px-3 py-1 text-xs font-medium ${
                        inCall
                          ? speaking
                            ? "bg-amber-500/10 text-amber-300"
                            : "bg-emerald-500/10 text-emerald-300"
                          : "bg-[#2E2E2E] text-[#A8A092]"
                      }`}
                    >
                      <span
                        className={`h-2 w-2 rounded-full ${
                          inCall ? "animate-pulse bg-current" : "bg-[#6E675D]"
                        }`}
                      />
                      {inCall ? status : "idle"}
                    </span>
                  </div>

                  <div className="mt-5 grid gap-3 sm:grid-cols-2">
                    <label className="block">
                      <span className="mb-1 block text-xs font-medium text-[#A8A092]">
                        Business
                      </span>
                      <select
                        value={activeTenant}
                        onChange={(e) => setTenant(e.target.value)}
                        disabled={inCall || !account?.is_owner}
                        className="w-full rounded-xl border border-[#3A3A3A] bg-[#242424] px-3 py-2.5 text-sm text-[#EAE5DC] outline-none transition focus:border-[#C87941] focus:ring-2 focus:ring-[#C87941]/30 disabled:opacity-50"
                      >
                        <option value="rivera">Rivera Plumbing</option>
                        <option value="acme">Acme Home</option>
                      </select>
                    </label>

                    <label className="block">
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
                        ? "No free calls left"
                        : "Start call"}
                  </button>
                  <p
                    className={`mt-2 text-center text-xs ${
                      outOfCalls ? "text-rose-300" : "text-[#8A8379]"
                    }`}
                  >
                    {unlimited
                      ? "Owner account · unlimited calls."
                      : outOfCalls
                        ? "You have used both free calls."
                        : `${remaining} of ${account?.free_call_limit ?? 0} free calls left.`}
                  </p>

                  <div className="mt-4 grid grid-cols-2 gap-2 text-center sm:grid-cols-4">
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
                    <div className="flex items-center justify-between text-[10px] font-medium uppercase tracking-wide text-[#8A8379]">
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

                  <div className="mt-5 border-t border-[#2E2E2E] pt-4">
                    <div className="text-[10px] font-medium uppercase tracking-wide text-[#8A8379]">
                      Try saying
                    </div>
                    <div className="mt-2 flex flex-wrap gap-1.5">
                      {[
                        "My kitchen sink is leaking",
                        "What is your call-out fee?",
                        "I need to reschedule Friday",
                      ].map((line) => (
                        <span
                          key={line}
                          className="rounded-full bg-[#2E2E2E] px-2.5 py-1 text-xs text-[#B5AEA3] ring-1 ring-[#EAE5DC]/5"
                        >
                          {line}
                        </span>
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
                        {agentLang && <Badge tone="indigo">{agentLang}</Badge>}
                        {agentState && <Badge>{agentState}</Badge>}
                      </div>
                    </div>

                    <div className="mt-5 space-y-4">
                      <Bubble who="Caller" text={transcript} tone="slate" />
                      <Bubble who="Agent" text={agentText} tone="indigo" />
                    </div>

                    {(tool || citations.length > 0) && (
                      <div className="mt-4 flex flex-wrap gap-2 border-t border-[#2E2E2E] pt-4">
                        {tool && <Badge>tool · {tool}</Badge>}
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
                      What happens under the hood
                    </h2>
                    <ol className="mt-4 space-y-4">
                      <Step
                        n="1"
                        title="It listens"
                        text="Your voice streams as 16 kHz PCM; server-side VAD finds when you finish."
                      />
                      <Step
                        n="2"
                        title="It decides"
                        text="An LLM with tools books, reschedules, or looks up policy — code validates every action."
                      />
                      <Step
                        n="3"
                        title="It speaks"
                        text="Piper streams the reply back; you can talk over it to interrupt."
                      />
                    </ol>
                    <p className="mt-5 rounded-xl bg-[#2A2A2A] p-4 text-xs leading-relaxed text-[#A8A092]">
                      Every test call lands in Calls with a transcript. Label it
                      there and it feeds the escalation metrics.
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
                subtitle="Watch the floor, then step in with the full transcript."
                action={
                  <Badge tone={connected ? "emerald" : "rose"}>
                    {connected ? "connected" : "disconnected"}
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
                        <span className="font-mono text-xs text-[#A8A092]">
                          {selectedId}
                        </span>
                        <Badge
                          tone={
                            bridgeStatus === "connected" ? "emerald" : "amber"
                          }
                        >
                          bridge · {bridgeStatus}
                        </Badge>
                      </div>
                      <p className="mt-1 text-sm text-[#A8A092]">
                        The agent is paused. The caller hears you directly — use
                        headphones.
                      </p>
                    </div>
                    <button
                      onClick={() => setSelectedId("")}
                      className="rounded-xl bg-[#2E2E2E] px-4 py-2 text-xs font-semibold text-[#EAE5DC] transition-colors hover:bg-[#3A3A3A]"
                    >
                      Release call
                    </button>
                  </div>

                  {selected?.reason && (
                    <div className="mt-4 rounded-xl border border-rose-400/20 bg-rose-500/10 px-4 py-3 text-sm text-rose-300">
                      <span className="font-medium">Why escalated: </span>
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
                    No active calls. Start one from Test agent and it appears
                    here.
                  </Empty>
                )}
              </div>
            </div>
          )}

          {tab === "calls" && (
            <div>
              <PageHeading
                badge={`${records.length} recorded`}
                title="Calls"
                subtitle="Every call keeps its transcript, outcome, and your label."
                onGoTest={() => setTab("test")}
              />

              <div className="grid gap-6 lg:grid-cols-[1.3fr_0.7fr]">
                <Card className="divide-y divide-[#2E2E2E]">
                  {records.map((record) => (
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
                  {records.length === 0 && (
                    <div className="p-4">
                      <Empty>No calls yet. Start one from Test agent.</Empty>
                    </div>
                  )}
                </Card>

                <div className="space-y-4">
                  <Card className="p-5">
                    <h3 className="text-sm font-semibold text-[#EAE5DC]">
                      Scoring calls
                    </h3>
                    <p className="mt-1 text-sm leading-relaxed text-[#A8A092]">
                      Mark whether a human was actually needed. Those labels are
                      the ground truth for precision and recall — not the
                      agent&apos;s own opinion.
                    </p>
                  </Card>

                  {hasReport && (
                    <Card className="p-5">
                      <h3 className="text-sm font-semibold text-[#EAE5DC]">
                        Current report
                      </h3>
                      <dl className="mt-3 space-y-2 text-sm">
                        <Row label="Labelled calls" value={String(report!.total)} />
                        <Row label="Precision" value={pct(report!.precision)} />
                        <Row label="Recall" value={pct(report!.recall)} />
                        <Row label="F1" value={pct(report!.f1)} />
                        <Row
                          label="Containment"
                          value={pct(report!.containment)}
                        />
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
                badge="Measured, not vibes"
                title="Evaluations"
                subtitle="Escalation quality, containment, booking completion, and grounding."
              />

              <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
                <Stat
                  label="Precision"
                  value={hasReport ? pct(report!.precision) : "—"}
                  hint={
                    hasReport
                      ? `${report!.tp} of ${report!.tp + report!.fp} raised`
                      : "label calls"
                  }
                />
                <Stat
                  label="Recall"
                  value={hasReport ? pct(report!.recall) : "—"}
                  hint={
                    hasReport ? `missed ${report!.fn}` : "label calls"
                  }
                />
                <Stat
                  label="Containment"
                  value={records.length ? pct(containment) : "—"}
                  hint={`${contained} of ${records.length} calls`}
                />
                <Stat
                  label="Recorded"
                  value={String(records.length)}
                  hint="with transcripts"
                />
              </div>

              <div className="mt-6 grid gap-6 lg:grid-cols-[0.9fr_1.1fr]">
                <Card className="p-5">
                  <h3 className="text-sm font-semibold text-[#EAE5DC]">
                    Escalation confusion matrix
                  </h3>
                  <p className="mt-1 text-xs text-[#8A8379]">
                    Based on your labels in Calls.
                  </p>
                  <div className="mt-4 grid grid-cols-[auto_1fr_1fr] gap-1 text-center text-xs">
                    <div />
                    <div className="pb-1 font-medium text-[#8A8379]">
                      needed human
                    </div>
                    <div className="pb-1 font-medium text-[#8A8379]">
                      should contain
                    </div>
                    <div className="pr-2 text-right font-medium text-[#8A8379]">
                      raised
                    </div>
                    <MatrixCell
                      value={report?.tp ?? 0}
                      tone="emerald"
                      hint="correct"
                    />
                    <MatrixCell
                      value={report?.fp ?? 0}
                      tone="rose"
                      hint="false alarm"
                    />
                    <div className="pr-2 text-right font-medium text-[#8A8379]">
                      held
                    </div>
                    <MatrixCell
                      value={report?.fn ?? 0}
                      tone="amber"
                      hint="missed"
                    />
                    <MatrixCell
                      value={report?.tn ?? 0}
                      tone="emerald"
                      hint="correct"
                    />
                  </div>
                  {!hasReport && (
                    <p className="mt-4 text-xs text-[#8A8379]">
                      Label a few calls to populate this.
                    </p>
                  )}
                </Card>

                <Card className="p-5">
                  <h3 className="text-sm font-semibold text-[#EAE5DC]">
                    What gets measured
                  </h3>
                  <ul className="mt-3 space-y-3 text-sm">
                    <Measure
                      title="Escalation precision / recall"
                      text="From your labels. Catches both false alarms and missed handoffs."
                    />
                    <Measure
                      title="Containment"
                      text="Calls closed without a human, the number the business cares about."
                    />
                    <Measure
                      title="Booking completion"
                      text="Scripted book / cancel / reschedule / emergency scenarios."
                    />
                    <Measure
                      title="Grounding and refusal"
                      text="Recall, MRR, grounded answers, and abstention on unanswerable policy questions."
                    />
                    <Measure
                      title="Latency per stage"
                      text="STT, brain, and TTS timings on every turn, plus p95."
                    />
                  </ul>
                  <p className="mt-4 rounded-xl bg-[#2A2A2A] p-3 text-xs text-[#A8A092]">
                    Curated suites: run <code className="font-mono">make eval</code>{" "}
                    for booking, escalation, and RAG scores.
                  </p>
                </Card>
              </div>
            </div>
          )}
          {tab === "team" && (
            <TeamTab
              tenant={activeTenant}
              isOwner={!!account?.is_owner}
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
      <span className="text-[#8A8379]">{label}</span>
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
        <Badge tone="indigo">{badge}</Badge>
        <h1 className="mt-3 text-2xl font-semibold tracking-tight text-[#EAE5DC]">
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
            Test agent
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
  const hasReport = report !== null && report.total > 0;
  const pct = (x: number) => `${(x * 100).toFixed(0)}%`;
  const recent = records.slice(0, 5);

  return (
    <div>
      <div className="flex flex-wrap items-end justify-between gap-4">
        <div>
          <Badge tone="indigo">Voice agent console</Badge>
          <h1 className="mt-3 text-2xl font-semibold tracking-tight text-[#EAE5DC]">
            Every call answered. Every job booked.
          </h1>
          <p className="mt-1 max-w-2xl text-sm text-[#A8A092]">
            A working voice receptionist: it talks, books with real tools,
            answers from policy, and hands off to a human when it matters.
          </p>
        </div>
        <button
          onClick={onStartTest}
          className="rounded-xl bg-[#C87941] px-4 py-2.5 text-sm font-semibold text-[#111111] transition-colors hover:bg-[#D98E5A]"
        >
          Start a test call
        </button>
      </div>

      <div className="mt-6 grid gap-3 sm:grid-cols-2 lg:grid-cols-5">
        <Stat
          label="Live calls"
          value={String(calls.length)}
          hint={calls.length ? `${escalatedLive} escalated` : "idle"}
        />
        <Stat
          label="Escalated"
          value={String(escalatedLive)}
          hint="needs a human"
        />
        <Stat
          label="Containment"
          value={records.length ? pct(containment) : "—"}
          hint={`${records.length} calls`}
        />
        <Stat
          label="Precision"
          value={hasReport ? pct(report!.precision) : "—"}
          hint={hasReport ? `${report!.total} labelled` : "label calls"}
        />
        <Stat
          label="Recall"
          value={hasReport ? pct(report!.recall) : "—"}
          hint={hasReport ? `missed ${report!.fn}` : "label calls"}
        />
      </div>

      <section className="mt-8">
        <SectionHeading
          title="Capabilities"
          subtitle="What this agent can actually do today"
        />
        <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
          {[
            [
              "Natural conversation",
              "LLM + tool calling, one question at a time, no scripted tree.",
            ],
            [
              "Real bookings",
              "Checks availability, books, reschedules, and cancels against a job store.",
            ],
            [
              "Grounded policy",
              "Answers from tenant docs with citations, and refuses when unsure.",
            ],
            [
              "Warm transfer",
              "Supervisor hears the transcript and joins the live audio bridge.",
            ],
            [
              "Multilingual",
              "Auto-detects the caller and replies in the same language.",
            ],
            [
              "Guardrails",
              "Injection blocking, PII redaction, price checks, tenant isolation.",
            ],
          ].map(([title, text], i) => (
            <Card key={title} className="p-4">
              <div className="flex items-start gap-3">
                <span className="grid h-7 w-7 shrink-0 place-items-center rounded-lg bg-[#C87941]/10 text-[11px] font-semibold text-[#D98E5A]">
                  {String(i + 1).padStart(2, "0")}
                </span>
                <div>
                  <div className="text-sm font-medium text-[#EAE5DC]">
                    {title}
                  </div>
                  <p className="mt-0.5 text-xs leading-relaxed text-[#A8A092]">
                    {text}
                  </p>
                </div>
              </div>
            </Card>
          ))}
        </div>
      </section>

      <div className="mt-8 grid gap-6 lg:grid-cols-[1.1fr_0.9fr]">
        <section>
          <SectionHeading
            title="Recent calls"
            subtitle="Transcript, outcome, and label"
            action={
              <button
                onClick={onGoCalls}
                className="text-xs font-medium text-[#D98E5A] hover:underline"
              >
                View all
              </button>
            }
          />
          <Card className="divide-y divide-[#2E2E2E]">
            {recent.map((record) => (
              <div
                key={record.id}
                className="flex flex-wrap items-center justify-between gap-2 px-4 py-3"
              >
                <div className="flex flex-wrap items-center gap-2">
                  <span className="font-mono text-xs text-[#A8A092]">
                    {record.id}
                  </span>
                  <Badge>{record.tenant}</Badge>
                  {record.escalated ? (
                    <Badge tone="rose">escalated</Badge>
                  ) : (
                    <Badge tone="emerald">contained</Badge>
                  )}
                </div>
                <span className="text-xs text-[#8A8379]">
                  {record.turns} turns
                </span>
              </div>
            ))}
            {recent.length === 0 && (
              <div className="p-4">
                <Empty>No calls yet. Start a test call.</Empty>
              </div>
            )}
          </Card>
        </section>

        <section>
          <SectionHeading
            title="Live now"
            subtitle="Active conversations on the floor"
            action={
              <button
                onClick={onGoLive}
                className="text-xs font-medium text-[#D98E5A] hover:underline"
              >
                Open floor
              </button>
            }
          />
          <Card className="divide-y divide-[#2E2E2E]">
            {calls.slice(0, 4).map((call) => (
              <div key={call.id} className="px-4 py-3">
                <div className="flex flex-wrap items-center gap-2">
                  <span className="font-mono text-xs text-[#A8A092]">
                    {call.id}
                  </span>
                  <Badge>{call.tenant}</Badge>
                  {call.escalated && <Badge tone="rose">escalated</Badge>}
                  {call.state && <Badge tone="indigo">{call.state}</Badge>}
                </div>
                <p className="mt-1 truncate text-xs text-[#A8A092]">
                  {call.last_text || "…"}
                </p>
              </div>
            ))}
            {calls.length === 0 && (
              <div className="p-4">
                <Empty>Quiet right now.</Empty>
              </div>
            )}
          </Card>
        </section>
      </div>
    </div>
  );
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
      <div className="flex flex-wrap items-center gap-2">
        <span className="font-mono text-xs text-[#A8A092]">{call.id}</span>
        {call.tenant && <Badge>{call.tenant}</Badge>}
        {call.lang && <Badge tone="indigo">{call.lang}</Badge>}
        {call.state && <Badge>{call.state}</Badge>}
        {call.escalated && <Badge tone="rose">escalated</Badge>}
        {call.taken_over ? (
          <Badge tone="emerald">you own this</Badge>
        ) : (
          <button
            onClick={onTakeOver}
            className="ml-auto rounded-lg bg-emerald-600 px-3 py-1.5 text-xs font-semibold text-[#EAE5DC] transition-colors hover:bg-emerald-500/100 focus:outline-none focus:ring-2 focus:ring-emerald-300"
          >
            {active ? "Rejoin" : "Take over"}
          </button>
        )}
      </div>

      {call.reason && (
        <p className="mt-2 text-xs text-rose-300">why: {call.reason}</p>
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
      <div className="flex flex-wrap items-center justify-between gap-3 p-4">
        <div className="min-w-0">
          <div className="flex flex-wrap items-center gap-2">
            <span className="font-mono text-xs text-[#A8A092]">{record.id}</span>
            <Badge>{record.tenant}</Badge>
            {record.escalated ? (
              <Badge tone="rose">escalated</Badge>
            ) : (
              <Badge tone="emerald">contained</Badge>
            )}
          </div>
          <div className="mt-1 text-xs text-[#A8A092]">
            {record.turns} turns · {record.reason || "no escalation"}
          </div>
        </div>
        <div className="flex items-center gap-1.5">
          <button
            onClick={() => onLabel(true)}
            className={`rounded-lg px-2.5 py-1 text-xs font-medium transition-colors ${
              record.label === true
                ? "bg-rose-600 text-[#EAE5DC]"
                : "bg-[#2E2E2E] text-[#B5AEA3] hover:bg-[#2E2E2E]"
            }`}
          >
            needed human
          </button>
          <button
            onClick={() => onLabel(false)}
            className={`rounded-lg px-2.5 py-1 text-xs font-medium transition-colors ${
              record.label === false
                ? "bg-emerald-600 text-[#EAE5DC]"
                : "bg-[#2E2E2E] text-[#B5AEA3] hover:bg-[#2E2E2E]"
            }`}
          >
            should contain
          </button>
          <button
            onClick={onToggle}
            aria-label="Toggle transcript"
            className="grid h-7 w-7 place-items-center rounded-lg text-[#8A8379] transition-colors hover:bg-[#2A2A2A] hover:text-[#D8D2C7]"
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
      <div className="text-[10px] font-medium uppercase tracking-wide text-[#8A8379]">
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
      <div className="mb-1 text-xs font-medium uppercase tracking-wide text-[#8A8379]">
        {who}
      </div>
      <div
        className={`rounded-2xl px-4 py-3 text-sm leading-relaxed ring-1 ${styles}`}
      >
        {text || <span className="text-[#8A8379]">…</span>}
      </div>
    </div>
  );
}

function Step({ n, title, text }: { n: string; title: string; text: string }) {
  return (
    <li className="flex gap-3">
      <span className="grid h-7 w-7 shrink-0 place-items-center rounded-full bg-[#C87941]/10 text-xs font-semibold text-[#D98E5A]">
        {n}
      </span>
      <div>
        <div className="text-sm font-medium text-[#EAE5DC]">{title}</div>
        <p className="text-sm text-[#A8A092]">{text}</p>
      </div>
    </li>
  );
}
