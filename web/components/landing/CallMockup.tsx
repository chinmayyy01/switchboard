import {
  CalendarCheck,
  CheckCircle,
  MapPin,
  PhoneCall,
  Waveform,
} from "@phosphor-icons/react/dist/ssr";
import { type } from "@/components/landing/type";

const bars = [2, 3, 4, 2, 5, 3, 4, 2, 5, 4, 3, 2, 4, 5, 3, 2, 4, 3, 5, 2];

export function CallMockup() {
  return (
    <div className="w-full min-w-0 overflow-hidden rounded-3xl border border-[#3A3A3A] bg-[#2A2A2A] p-4 text-[#EAE5DC] shadow-[0_24px_80px_rgba(20,17,15,0.24)]">
      <div className="flex items-center justify-between gap-3">
        <div className="flex min-w-0 items-center gap-2">
          <span className="grid h-9 w-9 shrink-0 place-items-center rounded-xl bg-[#2E2E2E] text-[#EAE5DC]">
            <PhoneCall size={18} weight="fill" aria-hidden="true" />
          </span>
          <div className="min-w-0">
            <div className="truncate text-sm font-semibold">
              Rivera Plumbing & HVAC
            </div>
            <div className="text-xs text-[#EAE5DC]/50">Demo call · 00:41</div>
          </div>
        </div>
        <span className="inline-flex shrink-0 items-center gap-2 rounded-full bg-[#2E2E2E] px-2 py-1 text-xs text-[#EAE5DC]/70">
          <span className="h-1.5 w-1.5 animate-pulse rounded-full bg-emerald-400 motion-reduce:animate-none" />
          live
        </span>
      </div>

      <div className="mt-4 rounded-lg bg-[#2E2E2E] p-3">
        <div className="flex items-center gap-2 text-xs text-[#EAE5DC]/40">
          <Waveform size={14} aria-hidden="true" />
          caller
        </div>
        <p className="mt-1 text-sm text-[#EAE5DC]/80">
          My kitchen sink is leaking.
        </p>
        <div className="mt-3 flex items-end gap-1" aria-hidden="true">
          {bars.map((height, i) => (
            <span
              key={i}
              className={`w-1 rounded-full bg-[#2A2A2A]/25 ${
                height === 2
                  ? "h-2"
                  : height === 3
                    ? "h-3"
                    : height === 4
                      ? "h-4"
                      : "h-5"
              }`}
            />
          ))}
        </div>
      </div>

      <div className="mt-3 rounded-lg bg-[#2E2E2E] p-3">
        <div className="text-xs text-[#EAE5DC]/40">agent</div>
        <p className="mt-1 text-sm text-[#EAE5DC]/80">
          Got it, leak repair. Is that right?
        </p>
      </div>

      <div className="mt-3 rounded-lg bg-[#2E2E2E] p-3">
        <div className="text-xs text-[#EAE5DC]/40">caller</div>
        <p className="mt-1 text-sm text-[#EAE5DC]/80">
          Yes. 123 Main Street, November 20 in the morning.
        </p>
      </div>

      <div className="mt-4 rounded-lg border border-[#3A3A3A] bg-[#2E2E2E] p-3">
        <div className="flex items-center justify-between gap-2">
          <div className="flex items-center gap-2">
            <CheckCircle
              size={16}
              weight="fill"
              className="text-emerald-400"
              aria-hidden="true"
            />
            <span className={`${type.caption3} tracking-wide text-[#EAE5DC]/80`}>
              Booked
            </span>
          </div>
          <span className="text-xs text-[#EAE5DC]/40">job confirmed</span>
        </div>

        <dl className="mt-3 space-y-2">
          <div className="flex items-center justify-between gap-3">
            <dt className="text-xs text-[#EAE5DC]/40">Service</dt>
            <dd className="text-right text-sm text-[#EAE5DC]/80">Leak repair</dd>
          </div>
          <div className="flex items-center justify-between gap-3">
            <dt className="text-xs text-[#EAE5DC]/40">When</dt>
            <dd className="flex min-w-0 items-center justify-end gap-2 text-right text-sm text-[#EAE5DC]/80">
              <CalendarCheck size={14} aria-hidden="true" />
              Nov 20, morning
            </dd>
          </div>
          <div className="flex items-center justify-between gap-3">
            <dt className="text-xs text-[#EAE5DC]/40">Address</dt>
            <dd className="flex min-w-0 items-center justify-end gap-2 text-right text-sm text-[#EAE5DC]/80">
              <MapPin size={14} aria-hidden="true" />
              123 Main Street
            </dd>
          </div>
          <div className="flex items-center justify-between gap-3">
            <dt className="text-xs text-[#EAE5DC]/40">Tech</dt>
            <dd className="text-right text-sm text-[#EAE5DC]/80">Emma Davis</dd>
          </div>
        </dl>
      </div>
    </div>
  );
}
