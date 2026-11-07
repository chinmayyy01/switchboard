"use client";

import { useEffect, useState, type FormEvent } from "react";
import { Badge, Card, Empty } from "@/components/ui";

const API = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8090";
const api = (path: string) => `${API}${path}`;

const DAYS = [
  ["mon", "Monday"],
  ["tue", "Tuesday"],
  ["wed", "Wednesday"],
  ["thu", "Thursday"],
  ["fri", "Friday"],
  ["sat", "Saturday"],
  ["sun", "Sunday"],
] as const;
const SHIFTS = ["morning", "afternoon", "evening"] as const;
const ROLES = [
  ["technician", "Technician"],
  ["dispatcher", "Dispatcher"],
  ["manager", "Manager"],
  ["other", "Other"],
] as const;

type Member = {
  id: string;
  name: string;
  role: string;
  email: string;
  phone: string;
  skills: string[];
  days: string[];
  shifts: string[];
  active: boolean;
  busy: string[];
};

type EmployeeForm = {
  id?: string;
  name: string;
  role: string;
  email: string;
  phone: string;
  skills: string;
  days: string[];
  shifts: string[];
  active: boolean;
};

const emptyForm: EmployeeForm = {
  name: "",
  role: "technician",
  email: "",
  phone: "",
  skills: "",
  days: ["mon", "tue", "wed", "thu", "fri"],
  shifts: ["morning", "afternoon"],
  active: true,
};

export function TeamTab({
  tenant,
  canSwitchTenant,
  canManageStaff,
  onTenantChange,
}: {
  tenant: string;
  canSwitchTenant: boolean;
  canManageStaff: boolean;
  onTenantChange: (tenant: string) => void;
}) {
  const [members, setMembers] = useState<Member[]>([]);
  const [form, setForm] = useState<EmployeeForm | null>(null);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");

  const reload = async () => {
    try {
      const response = await fetch(
        api(`/api/staff?tenant=${encodeURIComponent(tenant)}`),
        { credentials: "include" },
      );
      if (!response.ok) throw new Error("Could not load your team.");
      const data = (await response.json()) as Member[];
      setMembers(normalizeMembers(data));
      setError("");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Could not load your team.");
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    let active = true;
    const load = async () => {
      try {
        const response = await fetch(
          api(`/api/staff?tenant=${encodeURIComponent(tenant)}`),
          { credentials: "include" },
        );
        if (!response.ok) throw new Error("Could not load your team.");
        const data = (await response.json()) as Member[];
        if (active) {
          setMembers(normalizeMembers(data));
          setError("");
        }
      } catch (err) {
        if (active) {
          setError(err instanceof Error ? err.message : "Could not load your team.");
        }
      } finally {
        if (active) setLoading(false);
      }
    };
    setLoading(true);
    load();
    const timer = setInterval(load, 5000);
    return () => {
      active = false;
      clearInterval(timer);
    };
  }, [tenant]);

  const save = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (!form) return;
    setSaving(true);
    setError("");
    const payload = {
      name: form.name,
      role: form.role,
      email: form.email,
      phone: form.phone,
      skills: form.skills.split(",").map((skill) => skill.trim()).filter(Boolean),
      days: form.days,
      shifts: form.shifts,
      active: form.active,
    };
    try {
      const response = await fetch(
        api(
          form.id
            ? `/api/staff/${encodeURIComponent(form.id)}?tenant=${encodeURIComponent(tenant)}`
            : `/api/staff?tenant=${encodeURIComponent(tenant)}`,
        ),
        {
          method: form.id ? "PUT" : "POST",
          credentials: "include",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify(payload),
        },
      );
      if (!response.ok) {
        const message = await response.text();
        throw new Error(message || "Could not save this employee.");
      }
      setForm(null);
      await reload();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Could not save this employee.");
    } finally {
      setSaving(false);
    }
  };

  const toggleActive = async (member: Member) => {
    setError("");
    try {
      const response = await fetch(
        api(`/api/staff/${encodeURIComponent(member.id)}?tenant=${encodeURIComponent(tenant)}`),
        {
          method: "PATCH",
          credentials: "include",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ active: !member.active }),
        },
      );
      if (!response.ok) throw new Error("Could not update this employee.");
      await reload();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Could not update this employee.");
    }
  };

  const edit = (member: Member) => {
    setError("");
    setForm({
      id: member.id,
      name: member.name,
      role: member.role || "technician",
      email: member.email || "",
      phone: member.phone || "",
      skills: (member.skills ?? []).join(", "),
      days: member.days ?? [],
      shifts: member.shifts ?? [],
      active: member.active,
    });
  };

  return (
    <div>
      <header className="mb-6 flex flex-wrap items-end justify-between gap-4">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight text-[#EAE5DC]">
            Your team
          </h1>
          <p className="mt-1 text-sm text-[#A8A092]">
            Add employees, set their work roles, and choose when they can take jobs.
          </p>
        </div>
        <div className="flex items-center gap-2">
          {canSwitchTenant && (
            <select
              value={tenant}
              onChange={(event) => onTenantChange(event.target.value)}
              aria-label="Business"
              className="min-h-10 rounded-lg border border-[#3A3A3A] bg-[#242424] px-3 py-2 text-sm font-medium text-[#D8D2C7]"
            >
              <option value="rivera">Rivera Plumbing &amp; HVAC</option>
              <option value="acme">Acme Home Services</option>
            </select>
          )}
          {canManageStaff && (
            <button
              type="button"
              onClick={() => {
                setError("");
                setForm({ ...emptyForm });
              }}
              className="min-h-10 rounded-lg bg-[#C87941] px-4 py-2 text-sm font-semibold text-[#111111] hover:bg-[#D98E5A]"
            >
              Add employee
            </button>
          )}
        </div>
      </header>

      {error && (
        <div role="alert" className="mb-4 rounded-xl border border-rose-400/30 bg-rose-500/10 px-4 py-3 text-sm text-rose-300">
          {error}
        </div>
      )}

      {form && (
        <Card className="mb-5 p-5">
          <form onSubmit={save} className="space-y-5">
            <div className="flex items-start justify-between gap-4">
              <div>
                <h2 className="text-lg font-semibold text-[#EAE5DC]">
                  {form.id ? "Edit employee" : "Add an employee"}
                </h2>
                <p className="mt-1 text-sm text-[#A8A092]">
                  Work roles describe each person&apos;s job; account access is managed separately. Technicians can be assigned visits.
                </p>
              </div>
              <button
                type="button"
                onClick={() => setForm(null)}
                className="min-h-9 rounded-lg px-3 text-sm text-[#B5AEA3] hover:bg-[#2A2A2A]"
              >
                Cancel
              </button>
            </div>

            <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
              <label className="block lg:col-span-2">
                <span className="mb-1 block text-sm font-medium text-[#D8D2C7]">Full name</span>
                <input
                  required
                  autoFocus
                  value={form.name}
                  onChange={(event) => setForm({ ...form, name: event.target.value })}
                  className={inputClass}
                  placeholder="e.g. Maya Patel"
                />
              </label>
              <label className="block">
                <span className="mb-1 block text-sm font-medium text-[#D8D2C7]">Work role</span>
                <select
                  value={form.role}
                  onChange={(event) => setForm({ ...form, role: event.target.value })}
                  className={inputClass}
                >
                  {ROLES.map(([value, label]) => (
                    <option key={value} value={value}>{label}</option>
                  ))}
                </select>
              </label>
              <label className="block">
                <span className="mb-1 block text-sm font-medium text-[#D8D2C7]">Email (optional)</span>
                <input
                  type="email"
                  value={form.email}
                  onChange={(event) => setForm({ ...form, email: event.target.value })}
                  className={inputClass}
                  placeholder="name@business.com"
                />
              </label>
              <label className="block">
                <span className="mb-1 block text-sm font-medium text-[#D8D2C7]">Phone (optional)</span>
                <input
                  type="tel"
                  value={form.phone}
                  onChange={(event) => setForm({ ...form, phone: event.target.value })}
                  className={inputClass}
                  placeholder="Callback number"
                />
              </label>
              <label className="block sm:col-span-2">
                <span className="mb-1 block text-sm font-medium text-[#D8D2C7]">Services they can handle</span>
                <input
                  value={form.skills}
                  onChange={(event) => setForm({ ...form, skills: event.target.value })}
                  required={form.role === "technician"}
                  className={inputClass}
                  placeholder="Leak repair, drain cleaning"
                />
                <span className="mt-1 block text-xs text-[#A8A092]">Separate services with commas. Required for technicians.</span>
              </label>
            </div>

            <fieldset>
              <legend className="mb-2 text-sm font-medium text-[#D8D2C7]">Working days</legend>
              <p className="mb-2 text-xs text-[#A8A092]">Leave every day unchecked to allow any day.</p>
              <div className="flex flex-wrap gap-x-4 gap-y-2">
                {DAYS.map(([value, label]) => (
                  <label key={value} className="inline-flex min-h-9 items-center gap-2 text-sm text-[#B5AEA3]">
                    <input
                      type="checkbox"
                      checked={form.days.includes(value)}
                      onChange={() => setForm({
                        ...form,
                        days: toggleValue(form.days, value),
                      })}
                      className="h-4 w-4 accent-[#C87941]"
                    />
                    {label}
                  </label>
                ))}
              </div>
            </fieldset>

            <fieldset>
              <legend className="mb-2 text-sm font-medium text-[#D8D2C7]">Available shifts</legend>
              <p className="mb-2 text-xs text-[#A8A092]">Leave every shift unchecked to allow any time.</p>
              <div className="flex flex-wrap gap-x-4 gap-y-2">
                {SHIFTS.map((shift) => (
                  <label key={shift} className="inline-flex min-h-9 items-center gap-2 text-sm capitalize text-[#B5AEA3]">
                    <input
                      type="checkbox"
                      checked={form.shifts.includes(shift)}
                      onChange={() => setForm({
                        ...form,
                        shifts: toggleValue(form.shifts, shift),
                      })}
                      className="h-4 w-4 accent-[#C87941]"
                    />
                    {shift}
                  </label>
                ))}
              </div>
            </fieldset>

            <label className="inline-flex min-h-9 items-center gap-2 text-sm text-[#B5AEA3]">
              <input
                type="checkbox"
                checked={form.active}
                onChange={(event) => setForm({ ...form, active: event.target.checked })}
                className="h-4 w-4 accent-[#C87941]"
              />
              Active and available for assignment
            </label>

            <button
              type="submit"
              disabled={saving}
              className="min-h-11 rounded-lg bg-[#C87941] px-5 py-2.5 text-sm font-semibold text-[#111111] hover:bg-[#D98E5A] disabled:cursor-wait disabled:opacity-60"
            >
              {saving ? "Saving…" : form.id ? "Save changes" : "Add employee"}
            </button>
          </form>
        </Card>
      )}

      {loading && members.length === 0 ? (
        <Empty>Loading your team…</Empty>
      ) : members.length === 0 ? (
        <Empty>
          <div className="font-medium text-[#D8D2C7]">No employees yet.</div>
          {canManageStaff && <div className="mt-1">Add a technician to start assigning visits.</div>}
        </Empty>
      ) : (
        <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
          {members.map((member) => (
            <Card key={member.id} className="p-5">
              <div className="flex items-start justify-between gap-3">
                <div className="min-w-0">
                  <h2 className="truncate text-base font-semibold text-[#EAE5DC]">{member.name}</h2>
                  <p className="mt-1 text-sm capitalize text-[#A8A092]">{roleLabel(member.role)}</p>
                  {(member.email || member.phone) && (
                    <p className="mt-1 truncate text-xs text-[#B5AEA3]">
                      {[member.email, member.phone].filter(Boolean).join(" · ")}
                    </p>
                  )}
                </div>
                <Badge tone={!member.active ? "slate" : member.busy.length ? "amber" : "emerald"}>
                  {!member.active ? "Inactive" : member.busy.length ? `${member.busy.length} booked` : "Active"}
                </Badge>
              </div>

              {member.role === "technician" && (
                <div className="mt-4">
                  <div className="mb-2 text-xs font-medium text-[#A8A092]">Services</div>
                  <div className="flex flex-wrap gap-1.5">
                    {member.skills.map((skill) => <Badge key={skill} tone="indigo">{skill}</Badge>)}
                    {member.skills.length === 0 && <span className="text-sm text-[#A8A092]">No services set</span>}
                  </div>
                </div>
              )}

              <dl className="mt-4 space-y-2 text-sm">
                <div className="flex justify-between gap-3">
                  <dt className="text-[#A8A092]">Working days</dt>
                  <dd className="text-right text-[#D8D2C7]">{formatDays(member.days)}</dd>
                </div>
                <div className="flex justify-between gap-3">
                  <dt className="text-[#A8A092]">Hours</dt>
                  <dd className="text-right capitalize text-[#D8D2C7]">{member.shifts.join(", ") || "Any time"}</dd>
                </div>
              </dl>

              <div className="mt-4 border-t border-[#2E2E2E] pt-3">
                <div className="mb-1 text-xs font-medium text-[#A8A092]">Upcoming jobs</div>
                {member.busy.length === 0 ? (
                  <div className="text-sm text-[#B5AEA3]">No upcoming jobs.</div>
                ) : (
                  <ul className="space-y-1 text-sm text-[#B5AEA3]">
                    {member.busy.map((slot) => {
                      const [date, shift] = slot.split("|");
                      return <li key={slot}>{formatDate(date)} · {shift}</li>;
                    })}
                  </ul>
                )}
              </div>

              {canManageStaff && (
                <div className="mt-4 flex flex-wrap gap-2 border-t border-[#2E2E2E] pt-3">
                  <button
                    type="button"
                    onClick={() => edit(member)}
                    className="min-h-9 rounded-lg bg-[#2E2E2E] px-3 py-1.5 text-sm font-medium text-[#D8D2C7] hover:bg-[#3A3A3A]"
                  >
                    Edit
                  </button>
                  <button
                    type="button"
                    onClick={() => void toggleActive(member)}
                    className="min-h-9 rounded-lg px-3 py-1.5 text-sm font-medium text-[#D98E5A] hover:bg-[#2A2A2A]"
                  >
                    {member.active ? "Deactivate" : "Reactivate"}
                  </button>
                </div>
              )}
            </Card>
          ))}
        </div>
      )}
    </div>
  );
}

const inputClass = "min-h-11 w-full rounded-lg border border-[#3A3A3A] bg-[#242424] px-3 py-2 text-sm text-[#EAE5DC] placeholder:text-[#8A8379] focus:border-[#C87941] focus:outline-none";

function toggleValue(values: string[], value: string) {
  return values.includes(value)
    ? values.filter((item) => item !== value)
    : [...values, value];
}

function normalizeMembers(value: Member[]) {
  if (!Array.isArray(value)) return [];
  return value.map((member) => ({
    ...member,
    role: member.role || "technician",
    email: member.email || "",
    phone: member.phone || "",
    skills: member.skills ?? [],
    days: member.days ?? [],
    shifts: member.shifts ?? [],
    active: member.active ?? true,
    busy: member.busy ?? [],
  }));
}

function roleLabel(role: string) {
  const labels: Record<string, string> = {
    technician: "Technician",
    dispatcher: "Dispatcher",
    manager: "Manager",
    other: "Other",
  };
  return labels[role] ?? "Employee";
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
  const indices = normalized.map((day) => order.indexOf(day)).filter((index) => index >= 0).sort((a, b) => a - b);
  if (indices.length === 7) return "Every day";
  if (indices.every((day, index) => index === 0 || day === indices[index - 1] + 1)) {
    return `${shortDay(order[indices[0]])}–${shortDay(order[indices[indices.length - 1]])}`;
  }
  return normalized.map(shortDay).join(", ");
}

function shortDay(day: string) {
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
