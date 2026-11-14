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
  slate: "border border-[#3A3A3A] bg-[#2A2A2A] text-[#A8A092]",
  indigo: "border border-[#C87941]/30 bg-[#2A2A2A] text-[#D98E5A]",
  emerald: "border border-[#3A3A3A] bg-[#2A2A2A] text-[#7FBF95]",
  amber: "border border-[#3A3A3A] bg-[#2A2A2A] text-[#D9A45B]",
  rose: "border border-[#3A3A3A] bg-[#2A2A2A] text-[#E08A80]",
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
      <div className="text-sm font-medium text-[#A8A092]">
        {label}
      </div>
      <div className="mt-1 text-2xl font-semibold tabular-nums tracking-tight text-[#EAE5DC]">
        {value}
      </div>
      {hint && <div className="mt-0.5 text-xs text-[#A8A092]">{hint}</div>}
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
        <h2 className="text-base font-semibold tracking-tight text-[#EAE5DC]">
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
    <div className="rounded-2xl border border-dashed border-[#3A3A3A] bg-[#EAE5DC]/[0.03] px-4 py-8 text-center text-sm leading-relaxed text-[#B5AEA3]">
      {children}
    </div>
  );
}

type AlertTone = "error" | "success" | "warning" | "info";

const alertIcons: Record<AlertTone, ReactNode> = {
  error: (
    <>
      <circle cx="12" cy="12" r="9" />
      <path d="M12 7.5v5" />
      <path d="M12 16.2h.01" />
    </>
  ),
  success: (
    <>
      <circle cx="12" cy="12" r="9" />
      <path d="m8.3 12.4 2.6 2.6 4.8-5.2" />
    </>
  ),
  warning: (
    <>
      <path d="M12 4.2 21 20H3L12 4.2Z" />
      <path d="M12 9.8v4.4" />
      <path d="M12 17.6h.01" />
    </>
  ),
  info: (
    <>
      <circle cx="12" cy="12" r="9" />
      <path d="M12 10.8v5.4" />
      <path d="M12 7.8h.01" />
    </>
  ),
};

const alertColors: Record<AlertTone, string> = {
  error: "text-[#E08A80]",
  success: "text-[#7FBF95]",
  warning: "text-[#D9A45B]",
  info: "text-[#A8A092]",
};

// Alert is a neutral message surface: a plain border and icon, no tinted
// background. Tone is carried by the icon and optional title only.
export function Alert({
  tone = "info",
  title,
  children,
  className = "",
}: {
  tone?: AlertTone;
  title?: string;
  children: ReactNode;
  className?: string;
}) {
  return (
    <div
      role={tone === "error" ? "alert" : "status"}
      className={`flex items-start gap-3 rounded-lg border border-[#3A3A3A] bg-[#1D1D1D] px-4 py-3 text-sm leading-relaxed text-[#D8D2C7] ${className}`}
    >
      <svg
        viewBox="0 0 24 24"
        className={`mt-0.5 h-4 w-4 shrink-0 ${alertColors[tone]}`}
        fill="none"
        stroke="currentColor"
        strokeWidth="1.7"
        strokeLinecap="round"
        strokeLinejoin="round"
        aria-hidden="true"
      >
        {alertIcons[tone]}
      </svg>
      <div className="min-w-0">
        {title && <div className="font-medium text-[#EAE5DC]">{title}</div>}
        <div className={title ? "mt-0.5 text-[#A8A092]" : ""}>{children}</div>
      </div>
    </div>
  );
}
