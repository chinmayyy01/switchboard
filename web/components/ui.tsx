import type { ReactNode } from "react";

export function Card({
  children,
  className = "",
}: {
  children: ReactNode;
  className?: string;
}) {
  return (
    <div
      className={`rounded-2xl border border-[#3A3A3A] bg-[#242424] shadow-none ${className}`}
    >
      {children}
    </div>
  );
}

type Tone = "slate" | "indigo" | "emerald" | "amber" | "rose";

const tones: Record<Tone, string> = {
  slate: "bg-[#EAE5DC]/5 text-[#A8A092]",
  indigo: "bg-[#C87941]/10 text-[#D98E5A]",
  emerald: "bg-emerald-500/10 text-emerald-300",
  amber: "bg-amber-500/10 text-amber-300",
  rose: "bg-rose-500/10 text-rose-300",
};

export function Badge({
  children,
  tone = "slate",
}: {
  children: ReactNode;
  tone?: Tone;
}) {
  return (
    <span
      className={`inline-flex items-center gap-1 rounded-full px-2.5 py-0.5 text-xs font-medium ${tones[tone]}`}
    >
      {children}
    </span>
  );
}

export function Stat({
  label,
  value,
  hint,
}: {
  label: string;
  value: string;
  hint?: string;
}) {
  return (
    <div className="rounded-2xl border border-[#3A3A3A] bg-[#2A2A2A] p-4">
      <div className="text-xs font-medium uppercase tracking-wide text-[#A8A092]">
        {label}
      </div>
      <div className="mt-1 text-2xl font-semibold tabular-nums tracking-tight text-[#EAE5DC]">
        {value}
      </div>
      {hint && <div className="mt-0.5 text-xs text-[#8A8379]">{hint}</div>}
    </div>
  );
}

export function SectionHeading({
  title,
  subtitle,
  action,
}: {
  title: string;
  subtitle?: string;
  action?: ReactNode;
}) {
  return (
    <div className="mb-3 flex items-end justify-between gap-4">
      <div>
        <h2 className="text-lg font-semibold tracking-tight text-[#EAE5DC]">
          {title}
        </h2>
        {subtitle && <p className="mt-0.5 text-sm text-[#A8A092]">{subtitle}</p>}
      </div>
      {action}
    </div>
  );
}

export function Empty({ children }: { children: ReactNode }) {
  return (
    <div className="rounded-2xl border border-dashed border-[#3A3A3A] bg-[#EAE5DC]/[0.03] px-4 py-8 text-center text-sm text-[#8A8379]">
      {children}
    </div>
  );
}
