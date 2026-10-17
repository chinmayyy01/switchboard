"use client";

import { useSwitchboard } from "@/lib/useSwitchboard";

export default function Page() {
  const { running, status, rttMs, toggle } = useSwitchboard();

  return (
    <main className="mx-auto max-w-2xl px-6 py-16 font-mono leading-relaxed">
      <h1 className="text-2xl font-semibold">Switchboard · Phase 0</h1>

      <button
        onClick={toggle}
        className="mt-6 rounded-md bg-neutral-900 px-4 py-2 text-sm font-medium text-white hover:bg-neutral-700"
      >
        {running ? "Stop" : "Start"}
      </button>

      <p className="mt-4 text-sm">
        status: <span className="text-neutral-500">{status}</span>
      </p>

      <p className="my-2 text-3xl tabular-nums">
        {rttMs === null ? "RTT — ms" : `RTT ${rttMs.toFixed(1)} ms`}
      </p>

      <p className="text-sm text-neutral-500">
        Round-trip of a 20 ms PCM frame: mic → Go → back.
      </p>
    </main>
  );
}
