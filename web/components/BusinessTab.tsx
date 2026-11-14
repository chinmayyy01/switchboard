"use client";

import { useCallback, useEffect, useState, type FormEvent } from "react";
import { Alert, Badge, Card } from "@/components/ui";
import { BudgetCard } from "@/components/BudgetCard";
import { DocumentsCard } from "@/components/DocumentsCard";

const API = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8090";

type Persona = {
  agent_name?: string;
  tone?: string;
  languages?: string[];
};

type TenantConfig = {
  id: string;
  name: string;
  services: string[];
  windows: string[];
  persona?: Persona;
  disclosure?: string;
  manager_number?: string;
};

const WINDOWS = [
  ["morning", "Morning"],
  ["afternoon", "Afternoon"],
  ["evening", "Evening"],
] as const;

export function BusinessTab({
  tenant,
  tenants,
  canSwitchTenant,
  onTenantChange,
}: {
  tenant: string;
  tenants: { id: string; name: string }[];
  canSwitchTenant: boolean;
  onTenantChange: (tenant: string) => void;
}) {
  const [config, setConfig] = useState<TenantConfig | null>(null);
  const [name, setName] = useState("");
  const [services, setServices] = useState("");
  const [windows, setWindows] = useState<string[]>([]);
  const [agentName, setAgentName] = useState("");
  const [tone, setTone] = useState("");
  const [languages, setLanguages] = useState("");
  const [disclosure, setDisclosure] = useState("");
  const [managerNumber, setManagerNumber] = useState("");
  const [status, setStatus] = useState("");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);

  const apply = useCallback((cfg: TenantConfig) => {
    setConfig(cfg);
    setName(cfg.name ?? "");
    setServices((cfg.services ?? []).join(", "));
    setWindows(cfg.windows ?? []);
    setAgentName(cfg.persona?.agent_name ?? "");
    setTone(cfg.persona?.tone ?? "");
    setLanguages((cfg.persona?.languages ?? []).join(", "));
    setDisclosure(cfg.disclosure ?? "");
    setManagerNumber(cfg.manager_number ?? "");
  }, []);

  useEffect(() => {
    let active = true;
    setLoading(true);
    setError("");
    setStatus("");
    fetch(`${API}/api/tenant?tenant=${encodeURIComponent(tenant)}`, {
      credentials: "include",
      cache: "no-store",
    })
      .then(async (res) => {
        if (!res.ok) throw new Error("Could not load business settings.");
        const cfg = (await res.json()) as TenantConfig;
        if (active) apply(cfg);
      })
      .catch((err: Error) => {
        if (active) setError(err.message);
      })
      .finally(() => {
        if (active) setLoading(false);
      });
    return () => {
      active = false;
    };
  }, [apply, tenant]);

  const save = async (event: FormEvent) => {
    event.preventDefault();
    setSaving(true);
    setError("");
    setStatus("");
    const serviceList = services
      .split(/[,\n]/)
      .map((value) => value.trim())
      .filter(Boolean);
    const languageList = languages
      .split(",")
      .map((value) => value.trim())
      .filter(Boolean);
    try {
      const res = await fetch(`${API}/api/tenant?tenant=${encodeURIComponent(tenant)}`, {
        method: "PATCH",
        credentials: "include",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          name,
          services: serviceList,
          windows,
          persona: { agent_name: agentName, tone, languages: languageList },
          disclosure,
          manager_number: managerNumber,
        }),
      });
      if (!res.ok) throw new Error((await res.text()) || "Could not save settings.");
      const saved = (await res.json()) as TenantConfig;
      apply(saved);
      setStatus("Saved. New calls use this configuration.");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Could not save settings.");
    } finally {
      setSaving(false);
    }
  };

  const toggleWindow = (value: string) => {
    setWindows((current) =>
      current.includes(value) ? current.filter((item) => item !== value) : [...current, value],
    );
  };

  return (
    <div className="space-y-6">
      <header className="flex flex-wrap items-end justify-between gap-4">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight text-[#EAE5DC]">Business</h1>
          <p className="mt-1 text-sm text-[#A8A092]">
            What callers hear, which services the agent can book, and where transfers go.
          </p>
        </div>
        {canSwitchTenant && (
          <select
            value={tenant}
            onChange={(event) => onTenantChange(event.target.value)}
            aria-label="Business"
            className="min-h-10 rounded-lg border border-[#3A3A3A] bg-[#242424] px-3 py-2 text-sm font-medium text-[#D8D2C7]"
          >
            {tenants.map((item) => (
              <option key={item.id} value={item.id}>
                {item.name}
              </option>
            ))}
          </select>
        )}
      </header>

      {error && (
        <Alert tone="error">{error}</Alert>
      )}
      {status && (
        <Alert tone="success">{status}</Alert>
      )}

      {loading || !config ? (
        <p className="text-sm text-[#A8A092]">Loading business settings…</p>
      ) : (
        <form onSubmit={save} className="space-y-5">
          <Card className="p-5">
            <h2 className="text-base font-semibold text-[#EAE5DC]">Identity</h2>
            <div className="mt-3 grid gap-4 sm:grid-cols-2">
              <label className="block">
                <span className="mb-1 block text-xs font-medium text-[#A8A092]">Business name</span>
                <input
                  required
                  value={name}
                  onChange={(event) => setName(event.target.value)}
                  className="w-full rounded-xl border border-[#3A3A3A] bg-[#242424] px-3 py-2.5 text-sm text-[#EAE5DC] outline-none focus:border-[#C87941]"
                />
              </label>
              <label className="block">
                <span className="mb-1 block text-xs font-medium text-[#A8A092]">Transfer number</span>
                <input
                  value={managerNumber}
                  onChange={(event) => setManagerNumber(event.target.value)}
                  placeholder="+91..."
                  className="w-full rounded-xl border border-[#3A3A3A] bg-[#242424] px-3 py-2.5 text-sm text-[#EAE5DC] outline-none focus:border-[#C87941]"
                />
              </label>
              <label className="block">
                <span className="mb-1 block text-xs font-medium text-[#A8A092]">Agent name</span>
                <input
                  value={agentName}
                  onChange={(event) => setAgentName(event.target.value)}
                  className="w-full rounded-xl border border-[#3A3A3A] bg-[#242424] px-3 py-2.5 text-sm text-[#EAE5DC] outline-none focus:border-[#C87941]"
                />
              </label>
              <label className="block">
                <span className="mb-1 block text-xs font-medium text-[#A8A092]">Tone</span>
                <input
                  value={tone}
                  onChange={(event) => setTone(event.target.value)}
                  className="w-full rounded-xl border border-[#3A3A3A] bg-[#242424] px-3 py-2.5 text-sm text-[#EAE5DC] outline-none focus:border-[#C87941]"
                />
              </label>
              <label className="block sm:col-span-2">
                <span className="mb-1 block text-xs font-medium text-[#A8A092]">
                  Languages (comma separated)
                </span>
                <input
                  value={languages}
                  onChange={(event) => setLanguages(event.target.value)}
                  placeholder="en, hi"
                  className="w-full rounded-xl border border-[#3A3A3A] bg-[#242424] px-3 py-2.5 text-sm text-[#EAE5DC] outline-none focus:border-[#C87941]"
                />
              </label>
            </div>
          </Card>

          <Card className="p-5">
            <h2 className="text-base font-semibold text-[#EAE5DC]">Services</h2>
            <p className="mt-1 text-xs text-[#8A8379]">
              One per line or separated by commas. The agent can only book these.
            </p>
            <textarea
              required
              rows={4}
              value={services}
              onChange={(event) => setServices(event.target.value)}
              className="mt-3 w-full rounded-xl border border-[#3A3A3A] bg-[#242424] px-3 py-2.5 text-sm text-[#EAE5DC] outline-none focus:border-[#C87941]"
            />
            <div className="mt-4">
              <span className="mb-2 block text-xs font-medium text-[#A8A092]">Visit windows</span>
              <div className="flex flex-wrap gap-2">
                {WINDOWS.map(([value, label]) => (
                  <button
                    key={value}
                    type="button"
                    onClick={() => toggleWindow(value)}
                    className={`rounded-lg border px-3 py-1.5 text-xs font-medium transition-colors ${
                      windows.includes(value)
                        ? "border-[#C87941] bg-[#C87941]/15 text-[#E3B28D]"
                        : "border-[#3A3A3A] text-[#A8A092] hover:bg-[#2A2A2A]"
                    }`}
                  >
                    {label}
                  </button>
                ))}
              </div>
            </div>
          </Card>

          <Card className="p-5">
            <h2 className="text-base font-semibold text-[#EAE5DC]">Call disclosure</h2>
            <p className="mt-1 text-xs text-[#8A8379]">
              Spoken before the first turn. It should say the assistant is AI and ask
              about keeping a transcript. Leave empty to use the default wording.
            </p>
            <textarea
              rows={4}
              value={disclosure}
              onChange={(event) => setDisclosure(event.target.value)}
              className="mt-3 w-full rounded-xl border border-[#3A3A3A] bg-[#242424] px-3 py-2.5 text-sm leading-relaxed text-[#EAE5DC] outline-none focus:border-[#C87941]"
            />
          </Card>

          <div className="flex flex-wrap items-center gap-3">
            <button
              type="submit"
              disabled={saving}
              className="rounded-xl bg-[#C87941] px-5 py-2.5 text-sm font-semibold text-[#111111] hover:bg-[#D98E5A] disabled:opacity-60"
            >
              {saving ? "Saving…" : "Save settings"}
            </button>
            <Badge tone="slate">Tenant {config.id}</Badge>
          </div>
        </form>
      )}

      <BudgetCard tenant={tenant} />

      <DocumentsCard tenant={tenant} />
    </div>
  );
}
