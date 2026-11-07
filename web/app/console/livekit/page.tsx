"use client";

import Link from "next/link";
import { useState } from "react";
import { Badge, Card } from "@/components/ui";
import { useLiveKitCall } from "@/lib/useLiveKitCall";

export default function LiveKitTestPage() {
  const {
    status,
    message,
    roomName,
    transcript,
    booking,
    events,
    agentSpeaking,
    audioHostRef,
    start,
    end,
    connected,
  } = useLiveKitCall();
  const [tenant, setTenant] = useState("rivera");

  return (
    <div className="mx-auto max-w-5xl px-4 py-6 sm:px-6 lg:py-8">
      <div className="mb-6 flex flex-wrap items-end justify-between gap-4">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight text-[#EAE5DC]">
            LiveKit test call
          </h1>
          <p className="mt-1 text-sm text-[#A8A092]">
            Browser WebRTC call into the same agent that answers phone calls.
          </p>
        </div>
        <Link
          href="/console"
          className="min-h-10 rounded-lg border border-[#3A3A3A] px-4 py-2 text-sm font-medium text-[#D8D2C7] hover:bg-[#2A2A2A]"
        >
          Back to console
        </Link>
      </div>

      <Card className="p-5">
        <div className="flex flex-wrap items-center justify-between gap-4">
          <div className="flex items-center gap-3">
            <Badge
              tone={
                status === "connected"
                  ? "emerald"
                  : status === "error"
                    ? "rose"
                    : status === "connecting"
                      ? "amber"
                      : "slate"
              }
            >
              {status}
            </Badge>
            {agentSpeaking && status === "connected" && <Badge tone="indigo">agent speaking</Badge>}
            {roomName && <span className="text-xs text-[#A8A092]">room {roomName}</span>}
          </div>
          <div className="flex items-center gap-2">
            <select
              value={tenant}
              onChange={(event) => setTenant(event.target.value)}
              disabled={connected}
              aria-label="Business"
              className="min-h-10 rounded-lg border border-[#3A3A3A] bg-[#242424] px-3 py-2 text-sm text-[#D8D2C7] disabled:opacity-50"
            >
              <option value="rivera">Rivera Plumbing &amp; HVAC</option>
              <option value="acme">Acme Home Services</option>
            </select>
            <button
              type="button"
              onClick={() => (connected ? void end() : void start(tenant))}
              className={`min-h-11 rounded-lg px-5 py-2.5 text-sm font-semibold ${
                connected
                  ? "bg-rose-600 text-white hover:bg-rose-500"
                  : "bg-[#C87941] text-[#111111] hover:bg-[#D98E5A]"
              }`}
            >
              {connected ? "End call" : "Start call"}
            </button>
          </div>
        </div>
        {message && <p className="mt-3 text-sm text-rose-300">{message}</p>}
        <p className="mt-3 text-xs leading-relaxed text-[#A8A092]">
          Allow microphone access. Use headphones. The agent asks for recording consent before it
          does anything else.
        </p>
      </Card>

      <div className="mt-5 grid gap-5 lg:grid-cols-[1.4fr_0.8fr]">
        <Card className="p-5">
          <h2 className="text-base font-semibold text-[#EAE5DC]">Conversation</h2>
          <div className="mt-3 space-y-3">
            {transcript.length === 0 && (
              <p className="text-sm text-[#A8A092]">Live transcript appears here.</p>
            )}
            {transcript.map((line) => (
              <div key={`${line.id}:${line.text}`}>
                <div className="text-xs font-medium text-[#8A8379]">
                  {line.role === "caller" ? "Caller" : "Agent"}
                  {!line.final && " · live"}
                </div>
                <p
                  className={`text-sm leading-relaxed ${
                    line.final ? "text-[#D8D2C7]" : "text-[#A8A092]"
                  }`}
                >
                  {line.text}
                </p>
              </div>
            ))}
          </div>
        </Card>

        <div className="space-y-5">
          <Card className="p-5">
            <h2 className="text-base font-semibold text-[#EAE5DC]">Visit details</h2>
            {booking ? (
              <dl className="mt-3 space-y-2 text-sm">
                <Row label="Service" value={booking.service} />
                <Row label="When" value={[booking.date, booking.window].filter(Boolean).join(" · ")} />
                <Row label="Address" value={booking.address} />
                <Row label="Technician" value={booking.employee || "—"} />
                <Row label="Reference" value={booking.jobId} />
              </dl>
            ) : (
              <p className="mt-3 text-sm text-[#A8A092]">
                Booking details appear after the agent books a visit.
              </p>
            )}
          </Card>

          <Card className="p-5">
            <h2 className="text-base font-semibold text-[#EAE5DC]">Events</h2>
            {events.length === 0 ? (
              <p className="mt-3 text-sm text-[#A8A092]">No agent events yet.</p>
            ) : (
              <ul className="mt-3 space-y-2 text-xs text-[#B5AEA3]">
                {events.map((event, index) => (
                  <li key={`${event.type}-${index}`} className="truncate">
                    <span className="text-[#EAE5DC]">{event.type}</span>
                    {" · "}
                    {JSON.stringify(event).slice(0, 120)}
                  </li>
                ))}
              </ul>
            )}
          </Card>
        </div>
      </div>

      <div ref={audioHostRef} className="hidden" aria-hidden="true" />
    </div>
  );
}

function Row({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex items-baseline justify-between gap-4">
      <dt className="text-[#A8A092]">{label}</dt>
      <dd className="min-w-0 truncate text-right text-[#EAE5DC]">{value || "—"}</dd>
    </div>
  );
}
