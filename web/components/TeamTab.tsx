"use client";

import { useEffect, useState } from "react";
import { Badge, Card, Empty, SectionHeading } from "@/components/ui";

const API = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8090";
const TOKEN = process.env.NEXT_PUBLIC_SUPERVISOR_TOKEN ?? "";
const api = (p: string) =>
  TOKEN
    ? `${API}${p}${p.includes("?") ? "&" : "?"}token=${encodeURIComponent(TOKEN)}`
    : `${API}${p}`;

type Member = {
  id: string;
  name: string;
  skills: string[];
  days: string[];
  shifts: string[];
  busy: string[];
};

export function TeamTab() {
  const [tenant, setTenant] = useState("rivera");
  const [members, setMembers] = useState<Member[]>([]);

  useEffect(() => {
    let active = true;
    const load = async () => {
      try {
        const res = await fetch(api(`/api/staff?tenant=${tenant}`));
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
      <SectionHeading
        title="Team"
        subtitle="Fake field staff, skills, and live assignments"
        action={
          <select
            value={tenant}
            onChange={(e) => setTenant(e.target.value)}
            className="rounded-lg border border-[#3A3A3A] bg-[#242424] px-3 py-1.5 text-xs font-medium text-[#D8D2C7]"
          >
            <option value="rivera">Rivera Plumbing</option>
            <option value="acme">Acme Home</option>
          </select>
        }
      />

      <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
        {members.map((m) => (
          <Card key={m.id} className="p-5">
            <div className="flex items-start justify-between gap-3">
              <div>
                <div className="text-sm font-semibold text-[#EAE5DC]">{m.name}</div>
                <div className="text-xs text-[#8A8379]">{m.id}</div>
              </div>
              <Badge tone={(m.busy?.length ?? 0) ? "amber" : "emerald"}>
                {(m.busy?.length ?? 0) ? `${m.busy.length} booked` : "free"}
              </Badge>
            </div>

            <div className="mt-3 flex flex-wrap gap-1.5">
              {m.skills.map((s) => (
                <Badge key={s} tone="indigo">
                  {s}
                </Badge>
              ))}
            </div>

            <dl className="mt-4 space-y-1.5 text-xs">
              <div className="flex justify-between gap-3">
                <dt className="text-[#8A8379]">Days</dt>
                <dd className="text-right text-[#D8D2C7]">
                  {m.days?.join(", ") || "any"}
                </dd>
              </div>
              <div className="flex justify-between gap-3">
                <dt className="text-[#8A8379]">Shifts</dt>
                <dd className="text-right text-[#D8D2C7]">
                  {m.shifts?.join(", ") || "any"}
                </dd>
              </div>
            </dl>

            <div className="mt-4 border-t border-[#2E2E2E] pt-3">
              <div className="mb-1 text-[10px] font-medium uppercase tracking-wide text-[#8A8379]">
                Assigned
              </div>
              {(m.busy?.length ?? 0) === 0 ? (
                <div className="text-xs text-[#8A8379]">No jobs assigned.</div>
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
        <Empty>No staff configured for this tenant.</Empty>
      )}
    </div>
  );
}

function formatDate(iso: string) {
  const d = new Date(`${iso}T12:00:00`);
  if (Number.isNaN(d.getTime())) return iso;
  return d.toLocaleDateString(undefined, { month: "short", day: "numeric" });
}
