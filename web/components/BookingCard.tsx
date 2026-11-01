"use client";

import { useEffect, useState } from "react";
import { Badge, Card } from "@/components/ui";
import type { Booking } from "@/lib/useSwitchboard";

const API = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8090";

type Geo = { lat: number; lon: number; display: string };

export function BookingCard({ booking }: { booking: Booking }) {
  const [geo, setGeo] = useState<Geo | null>(null);
  const [locating, setLocating] = useState(false);

  useEffect(() => {
    const address = booking.address.trim();
    if (!address) {
      setGeo(null);
      setLocating(false);
      return;
    }
    const ctrl = new AbortController();
    setLocating(true);
    fetch(`${API}/api/geocode?q=${encodeURIComponent(address)}`, {
      signal: ctrl.signal,
    })
      .then((r) => (r.ok ? r.json() : Promise.reject(new Error("no geo"))))
      .then((g: Geo) => setGeo(g))
      .catch(() => {
        if (!ctrl.signal.aborted) setGeo(null);
      })
      .finally(() => {
        if (!ctrl.signal.aborted) setLocating(false);
      });
    return () => ctrl.abort();
  }, [booking.address]);

  const when = formatWhen(booking.date, booking.window);
  const phone = formatPhone(booking.phone);
  const complete = Boolean(
    booking.service && booking.address && booking.date && phone,
  );
  const status = booking.booked
    ? "Booked"
    : complete
      ? "Ready to book"
      : "Collecting details";
  const tone = booking.booked ? "emerald" : complete ? "indigo" : "slate";

  const mapSrc = geo
    ? `https://www.openstreetmap.org/export/embed.html?bbox=${geo.lon - 0.008},${geo.lat - 0.004},${geo.lon + 0.008},${geo.lat + 0.004}&layer=mapnik&marker=${geo.lat},${geo.lon}`
    : booking.address
      ? `https://maps.google.com/maps?q=${encodeURIComponent(booking.address)}&z=15&output=embed`
      : "";

  return (
    <Card className="p-5">
      <div className="flex items-start justify-between gap-3">
        <div>
          <h2 className="text-base font-semibold tracking-tight text-slate-900">
            Booking
          </h2>
          <p className="text-sm text-slate-500">Live from the agent&apos;s tools</p>
        </div>
        <Badge tone={tone}>{status}</Badge>
      </div>

      <dl className="mt-4 space-y-2.5">
        <Row label="Service" value={booking.service} capitalize />
        <Row label="When" value={when} />
        <Row label="Address" value={booking.address} />
        <Row label="Callback" value={phone} />
      </dl>

      <div className="mt-4 overflow-hidden rounded-xl border border-slate-200 bg-slate-50">
        {mapSrc ? (
          <iframe
            title="Service address"
            src={mapSrc}
            className="h-44 w-full"
            loading="lazy"
            referrerPolicy="no-referrer-when-downgrade"
          />
        ) : (
          <div className="grid h-44 place-items-center px-4 text-center text-xs text-slate-400">
            The map appears once the agent has the service address.
          </div>
        )}
      </div>

      <div className="mt-2 flex items-center justify-between text-[11px] text-slate-400">
        <span>{locating ? "Locating address…" : geo ? geo.display : ""}</span>
        {booking.address && (
          <a
            href={`https://www.openstreetmap.org/search?query=${encodeURIComponent(booking.address)}`}
            target="_blank"
            rel="noreferrer"
            className="font-medium text-indigo-600 hover:underline"
          >
            Open in maps
          </a>
        )}
      </div>
    </Card>
  );
}

function Row({
  label,
  value,
  capitalize,
}: {
  label: string;
  value: string;
  capitalize?: boolean;
}) {
  return (
    <div className="flex items-baseline justify-between gap-4">
      <dt className="text-xs font-medium uppercase tracking-wide text-slate-400">
        {label}
      </dt>
      <dd
        className={`min-w-0 truncate text-right text-sm ${
          value ? "text-slate-800" : "text-slate-300"
        } ${capitalize ? "capitalize" : ""}`}
      >
        {value || "—"}
      </dd>
    </div>
  );
}

function formatWhen(date: string, window: string) {
  if (!date && !window) return "";
  let day = date;
  if (date) {
    const parsed = new Date(`${date}T12:00:00`);
    if (!Number.isNaN(parsed.getTime())) {
      day = parsed.toLocaleDateString(undefined, {
        month: "long",
        day: "numeric",
      });
    }
  }
  const part = window
    ? window.charAt(0).toUpperCase() + window.slice(1)
    : "";
  return [day, part].filter(Boolean).join(" · ");
}

function formatPhone(phone: string) {
  const digits = phone.replace(/\D/g, "");
  if (digits.length === 10) {
    return `(${digits.slice(0, 3)}) ${digits.slice(3, 6)}-${digits.slice(6)}`;
  }
  return phone;
}
