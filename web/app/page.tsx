"use client";

import Link from "next/link";
import { useState } from "react";
import { useSwitchboard } from "@/lib/useSwitchboard";

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
    startCall,
    endCall,
  } = useSwitchboard();

  const [lang, setLang] = useState("");

  return (
    <main className="mx-auto max-w-2xl px-6 py-16 font-mono leading-relaxed">
      <h1 className="text-2xl font-semibold">Switchboard · Phase 1</h1>
      <p className="mt-1 text-sm text-neutral-500">
        Home-services reception (plumbing/HVAC) — just talk; the agent detects
        your language and answers.
      </p>
      <Link href="/supervisor" className="text-sm text-blue-600 hover:underline">
        supervisor view →
      </Link>

      <div className="mt-8 flex items-center gap-3">
        <select
          value={lang}
          onChange={(e) => setLang(e.target.value)}
          disabled={inCall}
          className="rounded-md border border-neutral-300 px-2 py-2 text-sm disabled:opacity-50"
        >
          <option value="">Auto-detect</option>
          <option value="en">English</option>
          <option value="hi">हिन्दी</option>
        </select>

        <button
          onClick={() => (inCall ? endCall() : startCall(lang))}
          className="rounded-md bg-neutral-900 px-4 py-2 text-sm font-medium text-white hover:bg-neutral-700"
        >
          {inCall ? "End call" : "Start call"}
        </button>

        {inCall && (
          <span
            className={`inline-flex items-center gap-2 text-sm ${
              status.startsWith("agent") ? "text-amber-600" : "text-emerald-600"
            }`}
          >
            <span className="h-2 w-2 animate-pulse rounded-full bg-current" />
            {status}
          </span>
        )}
      </div>

      <p className="mt-6 text-sm">
        RTT:{" "}
        <span className="tabular-nums">
          {rttMs === null ? "—" : `${rttMs.toFixed(1)} ms`}
        </span>
        {agentLang && (
          <span className="ml-3 rounded bg-blue-100 px-2 py-0.5 text-xs text-blue-800">
            lang: {agentLang}
          </span>
        )}
        {agentState && (
          <span className="ml-2 rounded bg-neutral-200 px-2 py-0.5 text-xs">
            {agentState}
          </span>
        )}
      </p>

      {latency && (
        <p className="mt-1 text-xs text-neutral-400">
          stt {latency.stt}ms · brain {latency.brain}ms · tts {latency.tts}ms ·
          turn {latency.turn}ms
        </p>
      )}

      <div className="mt-6 space-y-3">
        <p className="text-sm">
          <span className="text-neutral-500">you said: </span>
          {transcript || "…"}
        </p>
        <p className="text-sm">
          <span className="text-neutral-500">agent: </span>
          {agentText || "…"}
        </p>
        {tool && <p className="text-xs text-neutral-400">tool: {tool}</p>}
        {citations.length > 0 && (
          <p className="text-xs text-neutral-400">
            sources: {citations.join(", ")}
          </p>
        )}
      </div>
    </main>
  );
}
