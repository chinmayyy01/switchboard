"use client";

import { useState } from "react";
import type { Booking } from "@/lib/booking";
import type { useLiveKitCall } from "@/lib/useLiveKitCall";
import { BookingCard } from "@/components/BookingCard";
import { PhotoCard } from "@/components/PhotoCard";
import { Badge, Card } from "@/components/ui";

type LiveKitCall = ReturnType<typeof useLiveKitCall>;

const SUGGESTIONS = [
  "My kitchen sink is leaking",
  "What is your call-out fee?",
  "I need to reschedule Friday",
];

export function LiveKitTestCall({
  call,
  tenant,
  canSwitchTenant,
  tenants,
  onTenantChange,
}: {
  call: LiveKitCall;
  tenant: string;
  canSwitchTenant: boolean;
  tenants: { id: string; name: string }[];
  onTenantChange: (tenant: string) => void;
}) {
  const [provider, setProvider] = useState<"default" | "sarvam">("default");
  const connected = call.connected;
  const statusLabel =
    call.status === "idle"
      ? "Ready to test"
      : call.status === "connecting"
        ? "Connecting…"
        : call.status === "error"
          ? "Could not connect"
          : call.agentSpeaking
            ? "Receptionist speaking"
            : "Listening — speak when ready";

  const booking: Booking = {
    service: call.booking?.service ?? "",
    address: call.booking?.address ?? "",
    date: call.booking?.date ?? "",
    window: call.booking?.window ?? "",
    phone: call.booking?.phone ?? "",
    booked: Boolean(call.booking),
  };

  return (
    <div className="grid items-start gap-6 lg:grid-cols-2">
      <Card className="p-6">
        <div className="flex items-start justify-between gap-4">
          <div>
            <h2 className="text-base font-semibold tracking-tight text-[#EAE5DC]">
              Start a call
            </h2>
            <p className="text-sm text-[#A8A092]">
              WebRTC through LiveKit, using the same agent that answers phone calls.
            </p>
          </div>
          <span
            className={`inline-flex items-center gap-2 rounded-full px-3 py-1 text-xs font-medium ${
              call.status === "connected"
                ? call.agentSpeaking
                  ? "bg-[#EAE5DC]/[0.06] text-[#D9A45B]"
                  : "bg-[#EAE5DC]/[0.06] text-[#7FBF95]"
                : call.status === "error"
                  ? "bg-[#EAE5DC]/[0.06] text-[#E08A80]"
                  : "bg-[#2E2E2E] text-[#A8A092]"
            }`}
          >
            <span
              className={`h-2 w-2 rounded-full ${
                connected ? "animate-pulse bg-current" : "bg-[#6E675D]"
              }`}
            />
            {statusLabel}
          </span>
        </div>

        {canSwitchTenant && (
          <label className="mt-5 block">
            <span className="mb-1 block text-xs font-medium text-[#A8A092]">Business</span>
            <select
              value={tenant}
              onChange={(event) => onTenantChange(event.target.value)}
              disabled={connected}
              className="w-full rounded-xl border border-[#3A3A3A] bg-[#242424] px-3 py-2.5 text-sm text-[#EAE5DC] outline-none transition focus:border-[#C87941] focus:ring-2 focus:ring-[#C87941]/30 disabled:opacity-50"
            >
              {tenants.map((item) => (
                <option key={item.id} value={item.id}>
                  {item.name}
                </option>
              ))}
            </select>
          </label>
        )}

        <label className="mt-3 block">
          <span className="mb-1 block text-xs font-medium text-[#A8A092]">Voice stack</span>
          <select
            value={provider}
            onChange={(event) => setProvider(event.target.value as "default" | "sarvam")}
            disabled={connected}
            className="w-full rounded-xl border border-[#3A3A3A] bg-[#242424] px-3 py-2.5 text-sm text-[#EAE5DC] outline-none transition focus:border-[#C87941] focus:ring-2 focus:ring-[#C87941]/30 disabled:opacity-50"
          >
            <option value="default">Default · Deepgram + Gemini + Cartesia</option>
            <option value="sarvam">Sarvam · Saaras + 105B + Bulbul</option>
          </select>
        </label>

        <button
          type="button"
          onClick={() => (connected ? void call.end() : void call.start(tenant, provider))}
          className={`mt-5 w-full rounded-xl px-4 py-3 text-sm font-semibold transition-colors focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-offset-[#111111] ${
            connected
              ? "bg-[#8F3B36] text-[#EAE5DC] hover:bg-[#A0453F] focus:ring-[#8F3B36]/40"
              : "bg-[#C87941] text-[#111111] hover:bg-[#D98E5A] focus:ring-[#C87941]/40"
          }`}
        >
          {connected ? "End call" : "Start browser call"}
        </button>

        {call.audioBlocked && (
          <button
            type="button"
            onClick={() => void call.enableAudio()}
            className="mt-2 w-full rounded-xl border border-[#3A3A3A] bg-[#2E2E2E] px-4 py-2.5 text-sm font-semibold text-[#EAE5DC] hover:bg-[#3A3A3A]"
          >
            Enable audio
          </button>
        )}
        {call.message && (
          <p className="mt-2 text-center text-xs text-[#E08A80]">{call.message}</p>
        )}
        <p className="mt-2 text-center text-xs text-[#8A8379]">
          Your browser will ask for microphone access.
        </p>

        <details className="mt-4 rounded-xl border border-[#3A3A3A] bg-[#2A2A2A] px-4 py-3">
          <summary className="cursor-pointer text-sm font-medium text-[#D8D2C7]">
            Connection details (optional)
          </summary>
          <dl className="mt-3 space-y-1.5 text-xs">
            <div className="flex justify-between gap-3">
              <dt className="text-[#A8A092]">Room</dt>
              <dd className="truncate font-mono text-[#D8D2C7]">{call.roomName || "—"}</dd>
            </div>
            <div className="flex justify-between gap-3">
              <dt className="text-[#A8A092]">Transport</dt>
              <dd className="text-[#D8D2C7]">WebRTC · LiveKit</dd>
            </div>
            <div className="flex justify-between gap-3">
              <dt className="text-[#A8A092]">Stack</dt>
              <dd className="text-[#D8D2C7]">{provider === "sarvam" ? "Sarvam" : "Default"}</dd>
            </div>
            <div className="flex justify-between gap-3">
              <dt className="text-[#A8A092]">Agent</dt>
              <dd className="text-[#D8D2C7]">
                {call.agentSpeaking ? "speaking" : connected ? "listening" : "idle"}
              </dd>
            </div>
          </dl>
        </details>

        <div className="mt-5 border-t border-[#2E2E2E] pt-4">
          <div className="text-sm font-medium text-[#D8D2C7]">Try asking</div>
          <p className="mt-1 text-xs text-[#A8A092]">
            Choose an example or say it out loud.
          </p>
          <div className="mt-2 flex flex-wrap gap-1.5">
            {SUGGESTIONS.map((line) => (
              <button
                key={line}
                type="button"
                disabled={!connected}
                onClick={() => void call.sendText(line)}
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
        <PhotoCard inCall={connected} onSendText={call.sendText} />

        <Card className="p-6">
          <div className="flex items-center justify-between gap-3">
            <h2 className="text-base font-semibold tracking-tight text-[#EAE5DC]">
              Conversation
            </h2>
            <Badge tone={connected ? "emerald" : "slate"}>
              {connected ? "live" : "idle"}
            </Badge>
          </div>

          <div className="mt-5 max-h-[420px] space-y-3 overflow-y-auto">
            {call.transcript.length === 0 && (
              <p className="text-sm text-[#A8A092]">Live transcript appears here.</p>
            )}
            {call.transcript.map((line) => (
              <div key={`${line.id}:${line.text}`}>
                <div className="mb-1 text-xs font-medium uppercase tracking-wide text-[#8A8379]">
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

          {call.events.length > 0 && (
            <div className="mt-4 flex flex-wrap gap-2 border-t border-[#2E2E2E] pt-4">
              {call.events.slice(-4).map((event, index) => (
                <Badge key={`${event.type}-${index}`} tone="indigo">
                  {event.type}
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
            <li>
              The agent says it is an AI assistant and asks before keeping a text transcript. Calls are never recorded.
            </li>
          </ul>
        </Card>
      </div>
    </div>
  );
}
