"use client";

import { useEffect, useState } from "react";
import { Badge, Card, Empty } from "@/components/ui";

const API = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8090";
const api = (p: string) => `${API}${p}`;

type Member = {
  id: string;
  name: string;
  skills: string[];
  days: string[];
  shifts: string[];
  busy: string[];
};

export function TeamTab({
  tenant,
  canSwitchTenant,
  onTenantChange,
}: {
  tenant: string;
  canSwitchTenant: boolean;
  onTenantChange: (tenant: string) => void;
}) {
  const [members, setMembers] = useState<Member[]>([]);

  useEffect(() => {
    let active = true;
    const load = async () => {
      try {
        const res = await fetch(api(`/api/staff?tenant=${tenant}`), {
          credentials: "include",
        });
        if (!res.ok) return;
        const data = (await res.json()) as Member[];
        if (active) setMembers(data ?? []);
      } catch {}
    };
    load();
    const t = setInterval(load, 2000);
    return () => {
      active = false;
      clearInterval(t);
    };
  }, [tenant]);

  return (
    <div>
      <header className="mb-6 flex flex-wrap items-end justify-between gap-4">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight text-[#EAE5DC]">
            Your team
          </h1>
          <p className="mt-1 text-sm text-[#A8A092]">
            See who can take each type of job and when they are available.
          </p>
        </div>
        {canSwitchTenant && (
          <select
            value={tenant}
            onChange={(e) => onTenantChange(e.target.value)}
            aria-label="Business"
            className="min-h-10 rounded-lg border border-[#3A3A3A] bg-[#242424] px-3 py-2 text-sm font-medium text-[#D8D2C7]"
          >
            <option value="rivera">Rivera Plumbing &amp; HVAC</option>
            <option value="acme">Acme Home Services</option>
          </select>
        )}
      </header>

      <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
        {members.map((m) => (
          <Card key={m.id} className="p-5">
            <div className="flex items-start justify-between gap-3">
              <div>
                <div className="text-sm font-semibold text-[#EAE5DC]">{m.name}</div>
              </div>
              <Badge tone={(m.busy?.length ?? 0) ? "amber" : "emerald"}>
                {(m.busy?.length ?? 0)
                  ? `${m.busy.length} ${m.busy.length === 1 ? "job" : "jobs"} booked`
                  : "Available"}
              </Badge>
            </div>

            <div className="mt-3">
              <div className="mb-2 text-xs font-medium text-[#A8A092]">Services</div>
              <div className="flex flex-wrap gap-1.5">
              {m.skills.map((s) => (
                <Badge key={s} tone="indigo">
                  {s}
                </Badge>
              ))}
              </div>
            </div>

            <dl className="mt-4 space-y-1.5 text-xs">
              <div className="flex justify-between gap-3">
                <dt className="text-[#A8A092]">Working days</dt>
                <dd className="text-right text-[#D8D2C7]">
                  {formatDays(m.days ?? [])}
                </dd>
              </div>
              <div className="flex justify-between gap-3">
                <dt className="text-[#A8A092]">Hours</dt>
                <dd className="text-right text-[#D8D2C7]">
                  {m.shifts?.map((shift) => shift.charAt(0).toUpperCase() + shift.slice(1)).join(", ") || "Any time"}
                </dd>
              </div>
            </dl>

            <div className="mt-4 border-t border-[#2E2E2E] pt-3">
                <div className="mb-1 text-xs font-medium text-[#A8A092]">
                Upcoming jobs
              </div>
              {(m.busy?.length ?? 0) === 0 ? (
              <div className="text-xs text-[#B5AEA3]">No upcoming jobs.</div>
              ) : (
                <ul className="space-y-1">
                  {(m.busy ?? []).map((slot) => {
                    const [date, window] = slot.split("|");
                    return (
                      <li key={slot} className="text-xs text-[#B5AEA3]">
                        {formatDate(date)} · {window}
                      </li>
                    );
                  })}
                </ul>
              )}
            </div>
          </Card>
        ))}
      </div>

      {members.length === 0 && (
        <Empty>No team members have been added yet.</Empty>
      )}
    </div>
  );
}

function formatDate(iso: string) {
  const d = new Date(`${iso}T12:00:00`);
  if (Number.isNaN(d.getTime())) return iso;
  return d.toLocaleDateString(undefined, { month: "short", day: "numeric" });
}

function formatDays(days: string[]) {
  const order = ["mon", "tue", "wed", "thu", "fri", "sat", "sun"];
  const normalized = days.map((day) => day.toLowerCase());
  if (normalized.length === 0) return "Any day";
  const indices = normalized.map((day) => order.indexOf(day)).filter((n) => n >= 0).sort((a, b) => a - b);
  if (indices.length === 7) return "Every day";
  if (indices.every((day, index) => index === 0 || day === indices[index - 1] + 1)) {
    return `${formatShortDay(order[indices[0]])}–${formatShortDay(order[indices[indices.length - 1]])}`;
  }
  return normalized.map(formatShortDay).join(", ");
}

function formatShortDay(day: string) {
  const names: Record<string, string> = {
    mon: "Mon",
    tue: "Tue",
    wed: "Wed",
    thu: "Thu",
    fri: "Fri",
    sat: "Sat",
    sun: "Sun",
  };
  return names[day] ?? day;
}
