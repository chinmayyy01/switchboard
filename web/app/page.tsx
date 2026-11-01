"use client";

import { useEffect, useState, type ReactNode } from "react";
import { Badge, Card, Empty, SectionHeading, Stat } from "@/components/ui";
import { useBridge } from "@/lib/useBridge";
import { useSupervisor, type Call } from "@/lib/useSupervisor";
import { useSwitchboard } from "@/lib/useSwitchboard";

const API = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8090";
const AI = process.env.NEXT_PUBLIC_AI_URL ?? "http://localhost:8100";
const TOKEN = process.env.NEXT_PUBLIC_SUPERVISOR_TOKEN ?? "";
const api = (p: string) =>
  TOKEN
    ? `${API}${p}${p.includes("?") ? "&" : "?"}token=${encodeURIComponent(TOKEN)}`
    : `${API}${p}`;

type Tab = "overview" | "test" | "live" | "calls" | "evals";

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
    latency,
    level,
    sampleRate,
    devices,
    refreshDevices,
    startCall,
    endCall,
  } = useSwitchboard();
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
        const [r, rep] = await Promise.all([
          fetch(api("/api/calls")).then((x) => x.json()),
          fetch(api("/api/report")).then((x) => x.json()),
        ]);
        setRecords(r ?? []);
        setReport(rep ?? null);
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

  const label = async (id: string, should: boolean) => {
    try {
      await fetch(api(`/api/calls/${id}/label`), {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ should_escalate: should }),
      });
    } catch {}
  };

  const startTest = () => {
    setSelectedId("");
    startCall(lang, tenant, micId);
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
    <div className="min-h-screen bg-slate-50 lg:flex">
      <aside className="hidden w-64 shrink-0 flex-col border-r border-slate-200 bg-white lg:flex">
        <div className="flex items-center gap-2.5 px-5 py-5">
          <Logo />
          <div>
            <div className="text-sm font-semibold tracking-tight text-slate-900">
              Switchboard
            </div>
            <div className="text-[11px] text-slate-400">voice agent console</div>
          </div>
        </div>

        <nav className="flex-1 space-y-1 px-3">
          {NAV.map((item) => (
            <button
              key={item.id}
              onClick={() => setTab(item.id)}
              className={`flex w-full items-center justify-between rounded-lg px-3 py-2 text-sm font-medium transition-colors ${
                tab === item.id
                  ? "bg-slate-900 text-white"
                  : "text-slate-600 hover:bg-slate-100 hover:text-slate-900"
              }`}
            >
              <span>{item.label}</span>
              {item.id === "live" && calls.length > 0 && (
                <span
                  className={`rounded-full px-1.5 text-[10px] font-semibold ${
                    tab === item.id
                      ? "bg-white/20 text-white"
                      : "bg-slate-900 text-white"
                  }`}
                >
                  {calls.length}
                </span>
              )}
            </button>
          ))}
        </nav>

        <div className="space-y-1 border-t border-slate-100 p-4">
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
        </div>
      </aside>

      <div className="min-w-0 flex-1">
        <div className="border-b border-slate-200 bg-white lg:hidden">
          <div className="flex items-center justify-between px-4 py-3">
            <div className="flex items-center gap-2.5">
              <Logo />
              <span className="text-sm font-semibold tracking-tight text-slate-900">
                Switchboard
              </span>
            </div>
            <Badge tone={connected ? "emerald" : "slate"}>
              {connected ? "supervisor live" : "supervisor offline"}
            </Badge>
          </div>
          <div className="flex gap-1 overflow-x-auto px-3 pb-2">
            {NAV.map((item) => (
              <button
                key={item.id}
                onClick={() => setTab(item.id)}
                className={`shrink-0 rounded-lg px-3 py-1.5 text-xs font-medium transition-colors ${
                  tab === item.id
                    ? "bg-slate-900 text-white"
                    : "text-slate-600 hover:bg-slate-100"
                }`}
              >
                {item.label}
              </button>
            ))}
          </div>
        </div>

        <main className="mx-auto max-w-6xl px-4 py-6 sm:px-6 lg:py-8">
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
                      <h2 className="text-base font-semibold tracking-tight text-slate-900">
                        Start a call
                      </h2>
                      <p className="text-sm text-slate-500">
                        Choose a business, then talk.
                      </p>
                    </div>
                    <span
                      className={`inline-flex items-center gap-2 rounded-full px-3 py-1 text-xs font-medium ${
                        inCall
                          ? speaking
                            ? "bg-amber-50 text-amber-700"
                            : "bg-emerald-50 text-emerald-700"
                          : "bg-slate-100 text-slate-500"
                      }`}
                    >
                      <span
                        className={`h-2 w-2 rounded-full ${
                          inCall ? "animate-pulse bg-current" : "bg-slate-400"
                        }`}
                      />
                      {inCall ? status : "idle"}
                    </span>
                  </div>

                  <div className="mt-5 grid gap-3 sm:grid-cols-2">
                    <label className="block">
                      <span className="mb-1 block text-xs font-medium text-slate-500">
                        Business
                      </span>
                      <select
                        value={tenant}
                        onChange={(e) => setTenant(e.target.value)}
                        disabled={inCall}
                        className="w-full rounded-xl border border-slate-200 bg-white px-3 py-2.5 text-sm text-slate-900 outline-none transition focus:border-indigo-400 focus:ring-2 focus:ring-indigo-100 disabled:opacity-50"
                      >
                        <option value="rivera">Rivera Plumbing</option>
                        <option value="acme">Acme Home</option>
                      </select>
                    </label>

                    <label className="block">
                      <span className="mb-1 block text-xs font-medium text-slate-500">
                        Language
                      </span>
                      <select
                        value={lang}
                        onChange={(e) => setLang(e.target.value)}
                        disabled={inCall}
                        className="w-full rounded-xl border border-slate-200 bg-white px-3 py-2.5 text-sm text-slate-900 outline-none transition focus:border-indigo-400 focus:ring-2 focus:ring-indigo-100 disabled:opacity-50"
                      >
                        <option value="">Auto-detect</option>
                        <option value="en">English</option>
                        <option value="hi">हिन्दी</option>
                      </select>
                    </label>
                  </div>

                  <div className="mt-3">
                    <div className="mb-1 flex items-center justify-between">
                      <span className="text-xs font-medium text-slate-500">
                        Microphone
                      </span>
                      <button
                        type="button"
                        onClick={() => refreshDevices()}
                        className="text-xs font-medium text-indigo-600 hover:underline"
                      >
                        Refresh
                      </button>
                    </div>
                    <select
                      value={micId}
                      onChange={(e) => setMicId(e.target.value)}
                      disabled={inCall}
                      className="w-full rounded-xl border border-slate-200 bg-white px-3 py-2.5 text-sm text-slate-900 outline-none transition focus:border-indigo-400 focus:ring-2 focus:ring-indigo-100 disabled:opacity-50"
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
                    className={`mt-5 w-full rounded-xl px-4 py-3 text-sm font-semibold text-white transition-colors focus:outline-none focus:ring-2 focus:ring-offset-2 ${
                      inCall
                        ? "bg-rose-600 hover:bg-rose-500 focus:ring-rose-300"
                        : "bg-indigo-600 hover:bg-indigo-500 focus:ring-indigo-300"
                    }`}
                  >
                    {inCall ? "End call" : "Start call"}
                  </button>

                  <div className="mt-4 grid grid-cols-3 gap-2 text-center">
                    <MiniStat
                      label="RTT"
                      value={rttMs === null ? "—" : `${rttMs.toFixed(0)} ms`}
                    />
                    <MiniStat
                      label="STT"
                      value={latency ? `${latency.stt} ms` : "—"}
                    />
                    <MiniStat
                      label="Turn"
                      value={latency ? `${latency.turn} ms` : "—"}
                    />
                  </div>

                  <div className="mt-3">
                    <div className="flex items-center justify-between text-[10px] font-medium uppercase tracking-wide text-slate-400">
                      <span>mic level</span>
                      <span>
                        {sampleRate
                          ? `${(sampleRate / 1000).toFixed(0)} kHz`
                          : "—"}
                      </span>
                    </div>
                    <div className="mt-1 h-1.5 w-full overflow-hidden rounded-full bg-slate-100">
                      <div
                        className="h-full rounded-full bg-emerald-500 transition-[width] duration-100"
                        style={{
                          width: `${Math.min(100, (level / 6000) * 100)}%`,
                        }}
                      />
                    </div>
                  </div>

                  <div className="mt-5 border-t border-slate-100 pt-4">
                    <div className="text-[10px] font-medium uppercase tracking-wide text-slate-400">
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
                          className="rounded-full bg-slate-50 px-2.5 py-1 text-xs text-slate-600 ring-1 ring-slate-100"
                        >
                          {line}
                        </span>
                      ))}
                    </div>
                  </div>
                </Card>

                <div className="space-y-6">
                  <Card className="p-6">
                    <div className="flex items-center justify-between gap-3">
                      <h2 className="text-base font-semibold tracking-tight text-slate-900">
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
                      <div className="mt-4 flex flex-wrap gap-2 border-t border-slate-100 pt-4">
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
                    <h2 className="text-base font-semibold tracking-tight text-slate-900">
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
                    <p className="mt-5 rounded-xl bg-slate-50 p-4 text-xs leading-relaxed text-slate-500">
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
                <Card className="mb-6 border-emerald-200 p-5">
                  <div className="flex flex-wrap items-center justify-between gap-3">
                    <div>
                      <div className="flex flex-wrap items-center gap-2">
                        <span className="text-sm font-semibold text-slate-900">
                          You are on call
                        </span>
                        <span className="font-mono text-xs text-slate-500">
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
                      <p className="mt-1 text-sm text-slate-500">
                        The agent is paused. The caller hears you directly — use
                        headphones.
                      </p>
                    </div>
                    <button
                      onClick={() => setSelectedId("")}
                      className="rounded-xl bg-slate-900 px-4 py-2 text-xs font-semibold text-white transition-colors hover:bg-slate-700"
                    >
                      Release call
                    </button>
                  </div>

                  {selected?.reason && (
                    <div className="mt-4 rounded-xl border border-rose-100 bg-rose-50 px-4 py-3 text-sm text-rose-700">
                      <span className="font-medium">Why escalated: </span>
                      {selected.reason}
                    </div>
                  )}

                  <div className="mt-4 space-y-1.5">
                    {(selected?.transcript ?? []).map((turn, i) => (
                      <p key={i} className="text-sm leading-relaxed">
                        <span className="mr-1 text-xs font-medium uppercase tracking-wide text-slate-400">
                          {turn.role}
                        </span>
                        <span
                          className={
                            turn.role === "caller"
                              ? "text-slate-700"
                              : "text-indigo-900"
                          }
                        >
                          {turn.text}
                        </span>
                      </p>
                    ))}
                    {(selected?.transcript ?? []).length === 0 && (
                      <p className="text-sm text-slate-400">
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
                <Card className="divide-y divide-slate-100">
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
                    <h3 className="text-sm font-semibold text-slate-900">
                      Scoring calls
                    </h3>
                    <p className="mt-1 text-sm leading-relaxed text-slate-500">
                      Mark whether a human was actually needed. Those labels are
                      the ground truth for precision and recall — not the
                      agent&apos;s own opinion.
                    </p>
                  </Card>

                  {hasReport && (
                    <Card className="p-5">
                      <h3 className="text-sm font-semibold text-slate-900">
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
                  <h3 className="text-sm font-semibold text-slate-900">
                    Escalation confusion matrix
                  </h3>
                  <p className="mt-1 text-xs text-slate-400">
                    Based on your labels in Calls.
                  </p>
                  <div className="mt-4 grid grid-cols-[auto_1fr_1fr] gap-1 text-center text-xs">
                    <div />
                    <div className="pb-1 font-medium text-slate-400">
                      needed human
                    </div>
                    <div className="pb-1 font-medium text-slate-400">
                      should contain
                    </div>
                    <div className="pr-2 text-right font-medium text-slate-400">
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
                    <div className="pr-2 text-right font-medium text-slate-400">
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
                    <p className="mt-4 text-xs text-slate-400">
                      Label a few calls to populate this.
                    </p>
                  )}
                </Card>

                <Card className="p-5">
                  <h3 className="text-sm font-semibold text-slate-900">
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
                  <p className="mt-4 rounded-xl bg-slate-50 p-3 text-xs text-slate-500">
                    Curated suites: run <code className="font-mono">make eval</code>{" "}
                    for booking, escalation, and RAG scores.
                  </p>
                </Card>
              </div>
            </div>
          )}
        </main>
      </div>
    </div>
  );
}

function Logo() {
  return (
    <span className="grid h-9 w-9 place-items-center rounded-xl bg-indigo-600 text-white shadow-sm">
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
      ? "bg-emerald-500"
      : tone === "amber"
        ? "bg-amber-500"
        : tone === "rose"
          ? "bg-rose-500"
          : "bg-slate-300";
  return (
    <div className="flex items-center justify-between px-1 py-1 text-xs">
      <span className="text-slate-400">{label}</span>
      <span className="inline-flex items-center gap-1.5 font-medium text-slate-700">
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
        <h1 className="mt-3 text-2xl font-semibold tracking-tight text-slate-900">
          {title}
        </h1>
        <p className="mt-1 text-sm text-slate-500">{subtitle}</p>
      </div>
      <div className="flex items-center gap-3">
        {onGoTest && (
          <button
            onClick={onGoTest}
            className="rounded-xl border border-slate-200 bg-white px-3 py-2 text-xs font-semibold text-slate-700 transition-colors hover:bg-slate-50"
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
          <h1 className="mt-3 text-2xl font-semibold tracking-tight text-slate-900">
            Every call answered. Every job booked.
          </h1>
          <p className="mt-1 max-w-2xl text-sm text-slate-500">
            A working voice receptionist: it talks, books with real tools,
            answers from policy, and hands off to a human when it matters.
          </p>
        </div>
        <button
          onClick={onStartTest}
          className="rounded-xl bg-indigo-600 px-4 py-2.5 text-sm font-semibold text-white transition-colors hover:bg-indigo-500"
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
                <span className="grid h-7 w-7 shrink-0 place-items-center rounded-lg bg-indigo-50 text-[11px] font-semibold text-indigo-700">
                  {String(i + 1).padStart(2, "0")}
                </span>
                <div>
                  <div className="text-sm font-medium text-slate-900">
                    {title}
                  </div>
                  <p className="mt-0.5 text-xs leading-relaxed text-slate-500">
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
                className="text-xs font-medium text-indigo-600 hover:underline"
              >
                View all
              </button>
            }
          />
          <Card className="divide-y divide-slate-100">
            {recent.map((record) => (
              <div
                key={record.id}
                className="flex flex-wrap items-center justify-between gap-2 px-4 py-3"
              >
                <div className="flex flex-wrap items-center gap-2">
                  <span className="font-mono text-xs text-slate-500">
                    {record.id}
                  </span>
                  <Badge>{record.tenant}</Badge>
                  {record.escalated ? (
                    <Badge tone="rose">escalated</Badge>
                  ) : (
                    <Badge tone="emerald">contained</Badge>
                  )}
                </div>
                <span className="text-xs text-slate-400">
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
                className="text-xs font-medium text-indigo-600 hover:underline"
              >
                Open floor
              </button>
            }
          />
          <Card className="divide-y divide-slate-100">
            {calls.slice(0, 4).map((call) => (
              <div key={call.id} className="px-4 py-3">
                <div className="flex flex-wrap items-center gap-2">
                  <span className="font-mono text-xs text-slate-500">
                    {call.id}
                  </span>
                  <Badge>{call.tenant}</Badge>
                  {call.escalated && <Badge tone="rose">escalated</Badge>}
                  {call.state && <Badge tone="indigo">{call.state}</Badge>}
                </div>
                <p className="mt-1 truncate text-xs text-slate-500">
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
    <Card className={`p-5 ${active ? "ring-2 ring-emerald-200" : ""}`}>
      <div className="flex flex-wrap items-center gap-2">
        <span className="font-mono text-xs text-slate-500">{call.id}</span>
        {call.tenant && <Badge>{call.tenant}</Badge>}
        {call.lang && <Badge tone="indigo">{call.lang}</Badge>}
        {call.state && <Badge>{call.state}</Badge>}
        {call.escalated && <Badge tone="rose">escalated</Badge>}
        {call.taken_over ? (
          <Badge tone="emerald">you own this</Badge>
        ) : (
          <button
            onClick={onTakeOver}
            className="ml-auto rounded-lg bg-emerald-600 px-3 py-1.5 text-xs font-semibold text-white transition-colors hover:bg-emerald-500 focus:outline-none focus:ring-2 focus:ring-emerald-300"
          >
            {active ? "Rejoin" : "Take over"}
          </button>
        )}
      </div>

      {call.reason && (
        <p className="mt-2 text-xs text-rose-600">why: {call.reason}</p>
      )}

      <div className="mt-3 space-y-1.5">
        {(call.transcript ?? []).map((turn, i) => (
          <p key={i} className="text-sm leading-relaxed">
            <span className="mr-1 text-xs font-medium uppercase tracking-wide text-slate-400">
              {turn.role === "caller" ? "Caller" : "Agent"}
            </span>
            <span
              className={
                turn.role === "caller" ? "text-slate-700" : "text-indigo-900"
              }
            >
              {turn.text}
            </span>
          </p>
        ))}
        {(call.transcript ?? []).length === 0 && (
          <p className="text-sm text-slate-400">{call.last_text || "…"}</p>
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
            <span className="font-mono text-xs text-slate-500">{record.id}</span>
            <Badge>{record.tenant}</Badge>
            {record.escalated ? (
              <Badge tone="rose">escalated</Badge>
            ) : (
              <Badge tone="emerald">contained</Badge>
            )}
          </div>
          <div className="mt-1 text-xs text-slate-500">
            {record.turns} turns · {record.reason || "no escalation"}
          </div>
        </div>
        <div className="flex items-center gap-1.5">
          <button
            onClick={() => onLabel(true)}
            className={`rounded-lg px-2.5 py-1 text-xs font-medium transition-colors ${
              record.label === true
                ? "bg-rose-600 text-white"
                : "bg-slate-100 text-slate-600 hover:bg-slate-200"
            }`}
          >
            needed human
          </button>
          <button
            onClick={() => onLabel(false)}
            className={`rounded-lg px-2.5 py-1 text-xs font-medium transition-colors ${
              record.label === false
                ? "bg-emerald-600 text-white"
                : "bg-slate-100 text-slate-600 hover:bg-slate-200"
            }`}
          >
            should contain
          </button>
          <button
            onClick={onToggle}
            aria-label="Toggle transcript"
            className="grid h-7 w-7 place-items-center rounded-lg text-slate-400 transition-colors hover:bg-slate-100 hover:text-slate-700"
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
        <div className="border-t border-slate-100 bg-slate-50/70 px-4 py-4">
          {record.summary && (
            <p className="text-sm font-medium text-slate-800">
              {record.summary}
            </p>
          )}
          <div className="mt-3 space-y-2">
            {(record.transcript ?? []).map((turn, i) => (
              <p key={i} className="text-sm leading-relaxed">
                <span className="mr-1 text-xs font-medium uppercase tracking-wide text-slate-400">
                  {turn.role}
                </span>
                <span
                  className={
                    turn.role === "caller" ? "text-slate-700" : "text-indigo-900"
                  }
                >
                  {turn.text}
                </span>
              </p>
            ))}
            {(record.transcript ?? []).length === 0 && (
              <p className="text-xs text-slate-400">
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
      ? "bg-emerald-50 text-emerald-800 ring-emerald-100"
      : tone === "rose"
        ? "bg-rose-50 text-rose-800 ring-rose-100"
        : "bg-amber-50 text-amber-800 ring-amber-100";
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
      <div className="font-medium text-slate-900">{title}</div>
      <p className="text-xs leading-relaxed text-slate-500">{text}</p>
    </li>
  );
}

function Row({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex items-center justify-between">
      <dt className="text-slate-500">{label}</dt>
      <dd className="font-semibold tabular-nums text-slate-900">{value}</dd>
    </div>
  );
}

function MiniStat({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-lg bg-slate-50 px-2 py-2">
      <div className="text-[10px] font-medium uppercase tracking-wide text-slate-400">
        {label}
      </div>
      <div className="text-sm font-semibold tabular-nums text-slate-800">
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
      ? "bg-indigo-50 text-indigo-950 ring-indigo-100"
      : "bg-slate-50 text-slate-800 ring-slate-100";
  return (
    <div>
      <div className="mb-1 text-xs font-medium uppercase tracking-wide text-slate-400">
        {who}
      </div>
      <div
        className={`rounded-2xl px-4 py-3 text-sm leading-relaxed ring-1 ${styles}`}
      >
        {text || <span className="text-slate-400">…</span>}
      </div>
    </div>
  );
}

function Step({ n, title, text }: { n: string; title: string; text: string }) {
  return (
    <li className="flex gap-3">
      <span className="grid h-7 w-7 shrink-0 place-items-center rounded-full bg-indigo-50 text-xs font-semibold text-indigo-700">
        {n}
      </span>
      <div>
        <div className="text-sm font-medium text-slate-900">{title}</div>
        <p className="text-sm text-slate-500">{text}</p>
      </div>
    </li>
  );
}
