import type { Metadata } from "next";
import Link from "next/link";
import {
  ArrowRight,
  BookOpenText,
  CalendarCheck,
  Camera,
  Check,
  CheckCircle,
  MapPin,
  PhoneCall,
  UsersThree,
  X,
} from "@phosphor-icons/react/dist/ssr";
import { CallMockup } from "@/components/landing/CallMockup";
import { HeroAero } from "@/components/landing/HeroAero";
import { LandingNav } from "@/components/landing/LandingNav";
import { Reveal } from "@/components/landing/Reveal";
import { TaglineReveal } from "@/components/landing/TaglineReveal";
import { type } from "@/components/landing/type";

export const metadata: Metadata = {
  alternates: { canonical: "/" },
};

const authDisabled = process.env.NEXT_PUBLIC_AUTH_DISABLED === "true";
const demo = "/console?tab=test";

const stats = [
  ["0.65s to 1.3s", "first audio in the live smoke call"],
  ["9 tools", "in one booking, end to end"],
  ["0.91", "escalation precision and recall, 22 labelled calls"],
  ["2", "demo tenants on the same stack"],
];

const oldWay = [
  "A caller hears voicemail.",
  "The office calls back after the job.",
  "The slot goes to whoever answers first.",
];

const newWay = [
  "Every call is answered in a natural voice.",
  "The job is booked with a real tech.",
  "The team gets the transcript before the handoff.",
];

const benefits = [
  {
    icon: PhoneCall,
    title: "Answers in a natural voice",
    body: "Sarvam Bulbul at 22.05 kHz. Callers can interrupt, and the agent stops.",
  },
  {
    icon: CalendarCheck,
    title: "Books the job, not a callback",
    body: "Checks skills, shifts, and existing jobs, assigns a tech, and offers alternatives when a time is full.",
  },
  {
    icon: BookOpenText,
    title: "Knows the business",
    body: "Answers hours, pricing, warranty, and service area from your documents, with citations, and refuses when it does not know.",
  },
  {
    icon: UsersThree,
    title: "Hands off with the whole story",
    body: "Emergencies and human requests hold the line live for a supervisor, with the transcript already on screen.",
  },
  {
    icon: Camera,
    title: "Sees the problem",
    body: "A photo upload runs local vision and writes the issue into the job.",
  },
];

const steps = [
  {
    title: "The call comes in",
    body: "A browser call streams 16 kHz audio into the Go realtime service.",
  },
  {
    title: "The agent listens and decides",
    body: "Sarvam transcribes, the model calls booking tools, and the code validates every slot.",
  },
  {
    title: "The job lands on the board",
    body: "A tech is assigned, the booking card fills in, and the supervisor sees the transcript.",
  },
];

const faq = [
  {
    q: "Is this a real product or a demo?",
    a: "It is a working demo. The console runs a real Go service, a real Python AI service, and Sarvam speech and language models.",
  },
  {
    q: "Can it book real appointments?",
    a: "It checks the tenant's staff, skills, shifts, and existing jobs, assigns a technician, and writes the booking to the job store.",
  },
  {
    q: "What happens when no one is available?",
    a: "It offers specific alternatives from the schedule. If there are none, it offers a callback or a human.",
  },
  {
    q: "What happens when the caller asks for a person?",
    a: "The agent holds the call live and flags it in the console, where a supervisor can join with the transcript. Phone and SIP transfer are not in this demo yet.",
  },
  {
    q: "Which languages does it speak?",
    a: "English and Hindi today, using Sarvam Saaras for speech and Bulbul for voice. The stack can support more.",
  },
  {
    q: "Does it replace the team?",
    a: "No. It handles routine booking and policy questions, and hands off when the call needs a person.",
  },
  {
    q: "What does it run on?",
    a: "Go for realtime audio, Python for speech and retrieval, Next.js for the console, and Sarvam for speech and the conversational model.",
  },
  {
    q: "How do I try it?",
    a: authDisabled
      ? "Open the console and start a call in your browser. Authentication is temporarily disabled in this local preview."
      : "Sign in with an account from the owner, choose a tenant, and start a call in your browser. Standard accounts include 2 free calls. The owner account has unlimited calls.",
  },
];

const primary = `inline-flex items-center justify-center gap-2 rounded-xl bg-[#C87941] px-3 py-2 ${type.buttonMd} text-[#111111] transition-[background-color,transform] duration-300 ease-[cubic-bezier(0.22,1,0.36,1)] hover:bg-[#D98E5A] active:scale-[0.98] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[#D98E5A] focus-visible:ring-offset-2 focus-visible:ring-offset-[#111111]`;

const card = "rounded-2xl border border-[#3A3A3A] bg-[#242424]";

export default function Page() {
  const schema = {
    "@context": "https://schema.org",
    "@type": "FAQPage",
    mainEntity: faq.map((item) => ({
      "@type": "Question",
      name: item.q,
      acceptedAnswer: { "@type": "Answer", text: item.a },
    })),
  };

  return (
    <div className="relative min-h-screen overflow-x-hidden bg-[#111111] text-[#EAE5DC]">
      <div className="pointer-events-none fixed inset-0 z-0">
        <HeroAero />
      </div>
      <div className="pointer-events-none fixed inset-0 z-0 bg-[#111111]/50" />
      <div className="relative z-10">
      <a
        href="#main"
        className="sr-only focus:not-sr-only focus:fixed focus:left-3 focus:top-3 focus:z-[60] focus:rounded-lg focus:bg-[#C87941] focus:px-3 focus:py-2 focus:text-sm focus:font-semibold focus:text-[#111111]"
      >
        Skip to content
      </a>

      <LandingNav />

      <main id="main">
        <section className="mx-auto max-w-6xl px-6 pb-16 pt-24">
          <div className="grid min-w-0 grid-cols-1 items-center gap-12 lg:grid-cols-[1.05fr_0.95fr]">
            <div className="min-w-0">
              <p
                className={`${type.caption3} uppercase tracking-wide text-[#A8A092]`}
              >
                AI voice receptionist for home services
              </p>
              <h1
                className={`${type.heading1} mt-3 max-w-[680px]`}
                style={{
                  backgroundImage: "linear-gradient(90deg, #EAE5DC, #9A948B)",
                  WebkitBackgroundClip: "text",
                  backgroundClip: "text",
                  color: "transparent",
                }}
              >
                Every call answered.
                <br />
                Every job booked.
              </h1>
              <p
                className={`${type.body3} mt-6 max-w-[680px] text-[#A8A092]`}
              >
                An AI voice receptionist for home services teams. It answers in
                a natural voice, books real staff and real slots, and hands the
                call to your team with the full story.
              </p>

              <div className="mt-8">
                <Link href={demo} className={primary}>
                  Talk to the agent
                  <ArrowRight size={16} aria-hidden="true" />
                </Link>
                <p className={`${type.caption2} mt-3 text-[#A8A092]`}>
                  {authDisabled
                    ? "Local preview. Auth is temporarily disabled; calls are unlimited."
                    : "Live in your browser. Standard accounts get 2 free calls; owner accounts are unlimited."}
                </p>
              </div>

              <div className="mt-8 flex flex-wrap gap-2">
                {[
                  "0.65s to 1.3s first audio",
                  "2 free calls",
                  "Sarvam speech",
                  "Live transcript",
                ].map((item) => (
                  <span
                    key={item}
                    className={`${type.caption4} rounded-full border border-[#3A3A3A] bg-[#242424] px-2 py-1 text-[#A8A092]`}
                  >
                    {item}
                  </span>
                ))}
              </div>
            </div>

            <div className="min-w-0"><CallMockup /></div>
          </div>
        </section>

        <section className="mx-auto max-w-6xl bg-[#111111]/25 px-6 pb-16">
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
            {stats.map(([value, label], i) => (
              <Reveal key={label} delay={i * 60} className="min-w-0">
                <div className={`${card} p-4`}>
                  <div className={type.heading4}>{value}</div>
                  <p className={`${type.caption4} mt-2 text-[#A8A092]`}>
                    {label}
                  </p>
                </div>
              </Reveal>
            ))}
          </div>
          <p className={`${type.caption5} mt-4 text-[#A8A092]`}>
            Demo tenants: Rivera Plumbing & HVAC and Acme Home Services.
          </p>
        </section>

        <section className="mx-auto max-w-6xl bg-[#111111]/35 px-6 py-16">
          <div className="max-w-[680px]">
            <h2 className={type.heading3}>Missed calls are missed jobs.</h2>
            <p className={`${type.body3} mt-4 text-[#A8A092]`}>
              A tech is under a sink, the office is on another line, and the
              caller books with the next plumber who picks up. Switchboard
              answers every call and puts the job on the board.
            </p>
          </div>

          <div className="mt-8 grid grid-cols-1 gap-4 md:grid-cols-2">
            <Reveal className="min-w-0">
              <div className={`${card} p-4`}>
                <h3 className={type.title3}>The old way</h3>
                <ul className="mt-4 space-y-2">
                  {oldWay.map((item) => (
                    <li
                      key={item}
                      className={`${type.body6} flex items-start gap-2 text-[#A8A092]`}
                    >
                      <X
                        size={16}
                        className="mt-1 shrink-0 text-[#8A8379]"
                        aria-hidden="true"
                      />
                      {item}
                    </li>
                  ))}
                </ul>
              </div>
            </Reveal>

            <Reveal delay={80} className="min-w-0">
              <div className="rounded-2xl border border-[#3A3A3A] bg-[#2A2A2A] p-4">
                <h3 className={type.title3}>With Switchboard</h3>
                <ul className="mt-4 space-y-2">
                  {newWay.map((item) => (
                    <li
                      key={item}
                      className={`${type.body6} flex items-start gap-2 text-[#D8D2C7]`}
                    >
                      <Check
                        size={16}
                        weight="bold"
                        className="mt-1 shrink-0 text-[#D98E5A]"
                        aria-hidden="true"
                      />
                      {item}
                    </li>
                  ))}
                </ul>
              </div>
            </Reveal>
          </div>
        </section>

        <section
          id="product"
          className="mx-auto max-w-6xl scroll-mt-24 bg-[#111111]/35 px-6 py-16"
        >
          <div className="max-w-[680px]">
            <h2 className={type.heading3}>
              Everything the front desk does, on every call.
            </h2>
            <p className={`${type.body3} mt-4 text-[#A8A092]`}>
              The agent is not a phone tree. It has tools, documents, staff, and
              a supervisor behind it.
            </p>
          </div>

          <div className="mt-8 grid grid-cols-1 gap-4 md:grid-cols-2 lg:grid-cols-3">
            {benefits.map((benefit, i) => {
              const Icon = benefit.icon;
              return (
                <Reveal
                  key={benefit.title}
                  delay={i * 60}
                  className={`min-w-0 ${i === 4 ? "lg:col-span-2" : ""}`}
                >
                  <div className={`${card} h-full p-4`}>
                    <span className="grid h-9 w-9 place-items-center rounded-xl bg-[#2E2E2E] text-[#D98E5A]">
                      <Icon size={18} aria-hidden="true" />
                    </span>
                    <h3 className={`${type.title3} mt-4`}>{benefit.title}</h3>
                    <p className={`${type.body6} mt-2 text-[#A8A092]`}>
                      {benefit.body}
                    </p>
                  </div>
                </Reveal>
              );
            })}
          </div>
        </section>

        <section className="mx-auto max-w-6xl bg-[#111111]/30 px-6 py-24">
          <TaglineReveal text="The phone is ringing. Your best receptionist is already on it." />
        </section>

        <section id="how" className="mx-auto max-w-6xl scroll-mt-24 bg-[#111111]/35 px-6 py-16">
          <div className="max-w-[680px]">
            <h2 className={type.heading3}>Three steps, one call.</h2>
            <p className={`${type.body3} mt-4 text-[#A8A092]`}>
              The same stack runs the live demo and the console behind it.
            </p>
          </div>

          <div className="mt-8 grid grid-cols-1 gap-8 md:grid-cols-3">
            {steps.map((step, i) => (
              <Reveal key={step.title} delay={i * 80} className="min-w-0">
                <div>
                  <span className="grid h-9 w-9 place-items-center rounded-full bg-[#C87941] text-sm font-semibold text-[#111111]">
                    {i + 1}
                  </span>
                  <h3 className={`${type.title3} mt-4`}>{step.title}</h3>
                  <p className={`${type.body6} mt-2 text-[#A8A092]`}>
                    {step.body}
                  </p>
                </div>
              </Reveal>
            ))}
          </div>
        </section>

        <section
          id="proof"
          className="mx-auto max-w-6xl scroll-mt-24 bg-[#111111]/35 px-6 py-16"
        >
          <div className="max-w-[680px]">
            <h2 className={type.heading3}>A real call, not a recording.</h2>
            <p className={`${type.body3} mt-4 text-[#A8A092]`}>
              This is the transcript from the live smoke test: one browser call,
              nine tools, one booked visit.
            </p>
          </div>

          <div className="mt-8 grid grid-cols-1 gap-6 lg:grid-cols-[1.1fr_0.9fr]">
            <Reveal className="min-w-0">
              <div className={`${card} p-6`}>
                <div className="flex items-center justify-between gap-3">
                  <h3 className={type.title3}>Sample call</h3>
                  <span
                    className={`${type.caption4} rounded-full border border-[#3A3A3A] px-2 py-1 text-[#A8A092]`}
                  >
                    Demo tenant
                  </span>
                </div>

                <div className="mt-6 space-y-4">
                  <div>
                    <div
                      className={`${type.caption3} uppercase tracking-wide text-[#A8A092]`}
                    >
                      Caller
                    </div>
                    <p className={`${type.body3} mt-1`}>
                      My kitchen sink is leaking.
                    </p>
                  </div>
                  <div>
                    <div
                      className={`${type.caption3} uppercase tracking-wide text-[#A8A092]`}
                    >
                      Agent
                    </div>
                    <p className={`${type.body3} mt-1`}>
                      Got it, leak repair. Is that right?
                    </p>
                  </div>
                  <div>
                    <div
                      className={`${type.caption3} uppercase tracking-wide text-[#A8A092]`}
                    >
                      Caller
                    </div>
                    <p className={`${type.body3} mt-1`}>
                      Yes. 123 Main Street, November 20 in the morning.
                    </p>
                  </div>
                  <div>
                    <div
                      className={`${type.caption3} uppercase tracking-wide text-[#A8A092]`}
                    >
                      Agent
                    </div>
                    <p className={`${type.body3} mt-1`}>
                      Just to confirm, November 20th in the morning. Does that
                      work?
                    </p>
                  </div>
                  <div>
                    <div
                      className={`${type.caption3} uppercase tracking-wide text-[#A8A092]`}
                    >
                      Agent
                    </div>
                    <p className={`${type.body3} mt-1`}>
                      Emma Davis will be out to 123 Main Street on November 20th
                      for the leak repair.
                    </p>
                  </div>
                </div>
              </div>
            </Reveal>

            <Reveal delay={80} className="min-w-0">
              <div className="h-full rounded-2xl border border-[#3A3A3A] bg-[#2A2A2A] p-6 text-[#EAE5DC]">
                <div className="flex items-center justify-between gap-3">
                  <h3 className={type.title3}>Booking</h3>
                  <span
                    className={`${type.caption4} inline-flex items-center gap-2 rounded-full bg-[#2E2E2E] px-2 py-1 text-[#EAE5DC]/70`}
                  >
                    <CheckCircle
                      size={14}
                      weight="fill"
                      className="text-emerald-400"
                      aria-hidden="true"
                    />
                    Booked
                  </span>
                </div>

                <dl className="mt-6 space-y-3">
                  <div className="flex items-center justify-between gap-4">
                    <dt className="text-xs text-[#EAE5DC]/40">Service</dt>
                    <dd className="text-sm text-[#EAE5DC]/80">Leak repair</dd>
                  </div>
                  <div className="flex items-center justify-between gap-4">
                    <dt className="text-xs text-[#EAE5DC]/40">When</dt>
                    <dd className="text-sm text-[#EAE5DC]/80">
                      November 20, morning
                    </dd>
                  </div>
                  <div className="flex items-center justify-between gap-4">
                    <dt className="text-xs text-[#EAE5DC]/40">Address</dt>
                    <dd className="text-sm text-[#EAE5DC]/80">123 Main Street</dd>
                  </div>
                  <div className="flex items-center justify-between gap-4">
                    <dt className="text-xs text-[#EAE5DC]/40">Tech</dt>
                    <dd className="text-sm text-[#EAE5DC]/80">Emma Davis</dd>
                  </div>
                </dl>

                <div className="mt-6 flex items-center gap-2 rounded-lg border border-[#3A3A3A] bg-[#2E2E2E] p-3">
                  <MapPin
                    size={16}
                    className="text-[#D98E5A]"
                    aria-hidden="true"
                  />
                  <span className="text-xs text-[#EAE5DC]/60">
                    123 Main Street, demo map
                  </span>
                </div>

                <p className="mt-4 text-xs text-[#EAE5DC]/40">
                  Names and addresses are demo data.
                </p>
              </div>
            </Reveal>
          </div>
        </section>

        <section className="mx-auto max-w-6xl bg-[#111111]/20 px-6 py-12">
          <div className="flex flex-wrap items-center gap-2">
            <span className={`${type.caption3} uppercase tracking-wide text-[#A8A092]`}>
              Built on
            </span>
            {["Sarvam", "Go", "Next.js", "OpenTelemetry"].map((item) => (
              <span
                key={item}
                className={`${type.caption4} rounded-full border border-[#3A3A3A] px-2 py-1 text-[#A8A092]`}
              >
                {item}
              </span>
            ))}
          </div>
        </section>

        <section id="faq" className="mx-auto max-w-6xl scroll-mt-24 bg-[#111111]/50 px-6 py-16">
          <div className="max-w-[680px]">
            <h2 className={type.heading3}>Questions, answered.</h2>
            <p className={`${type.body3} mt-4 text-[#A8A092]`}>
              What the demo does, what it does not, and what it runs on.
            </p>
          </div>

          <div className="mt-8 grid grid-cols-1 gap-8 md:grid-cols-2">
            {faq.map((item, i) => (
              <Reveal key={item.q} delay={(i % 2) * 60} className="min-w-0">
                <article>
                  <h3 className={type.title3}>{item.q}</h3>
                  <p className={`${type.body6} mt-2 text-[#A8A092]`}>
                    {item.a}
                  </p>
                </article>
              </Reveal>
            ))}
          </div>
        </section>

        <section className="mx-auto max-w-6xl bg-[#111111]/35 px-6 py-16">
          <div className="rounded-3xl bg-[#2A2A2A] p-8 text-[#EAE5DC]">
            <div className="max-w-[680px]">
              <h2 className={type.heading3}>Hear it answer your next call.</h2>
              <p className={`${type.body3} mt-4 text-[#EAE5DC]/60`}>
                Open the console, start a browser call, and watch the booking
                card fill in. The demo runs on the same stack as the product.
              </p>
              <div className="mt-8">
                <Link
                  href={demo}
                  className={`inline-flex items-center justify-center gap-2 rounded-xl bg-[#C87941] px-3 py-2 ${type.buttonMd} text-[#111111] transition-[background-color,transform] duration-300 ease-[cubic-bezier(0.22,1,0.36,1)] hover:bg-[#D98E5A] active:scale-[0.98] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[#EAE5DC] focus-visible:ring-offset-2 focus-visible:ring-offset-[#2A2A2A]`}
                >
                  Talk to the agent
                  <ArrowRight size={16} aria-hidden="true" />
                </Link>
                <p className={`${type.caption2} mt-3 text-[#EAE5DC]/50`}>
                  {authDisabled
                    ? "Local preview. Auth is temporarily disabled."
                    : "2 free calls for standard accounts. Owner accounts are unlimited."}
                </p>
              </div>
            </div>
          </div>
        </section>
      </main>

      <footer className="bg-[#0D0D0D]/80 py-12 backdrop-blur-sm">
        <div className="mx-auto max-w-6xl px-6">
          <div className="flex flex-wrap items-start justify-between gap-8">
            <div>
              <div className="flex items-center gap-2">
                <span className="grid h-9 w-9 place-items-center rounded-xl bg-[#C87941] text-[#111111]">
                  <svg
                    viewBox="0 0 24 24"
                    className="h-5 w-5"
                    fill="none"
                    stroke="currentColor"
                    strokeWidth="2"
                    strokeLinecap="round"
                    aria-hidden="true"
                  >
                    <path d="M4 12v1M8 8v8M12 5v14M16 8v8M20 11v2" />
                  </svg>
                </span>
                <span className="text-sm font-semibold tracking-tight">
                  Switchboard
                </span>
              </div>
              <p className={`${type.caption2} mt-3 max-w-[320px] text-[#A8A092]`}>
                AI voice reception for home services.
              </p>
            </div>

            <nav className="grid grid-cols-1 gap-8 sm:grid-cols-2">
              <div>
                <div
                  className={`${type.caption3} uppercase tracking-wide text-[#A8A092]`}
                >
                  Product
                </div>
                <div className="mt-3 flex flex-col gap-2">
                  {[
                    ["Product", "#product"],
                    ["How it works", "#how"],
                    ["Proof", "#proof"],
                    ["FAQ", "#faq"],
                  ].map(([label, href]) => (
                    <a
                      key={href}
                      href={href}
                      className={`${type.caption2} text-[#A8A092] transition-colors duration-300 hover:text-[#EAE5DC]`}
                    >
                      {label}
                    </a>
                  ))}
                </div>
              </div>
              <div>
                <div
                  className={`${type.caption3} uppercase tracking-wide text-[#A8A092]`}
                >
                  Elsewhere
                </div>
                <div className="mt-3 flex flex-col gap-2">
                  <Link
                    href="/console"
                    className={`${type.caption2} text-[#A8A092] transition-colors duration-300 hover:text-[#EAE5DC]`}
                  >
                    Console
                  </Link>
                  <Link
                    href="/privacy"
                    className={`${type.caption2} text-[#A8A092] transition-colors duration-300 hover:text-[#EAE5DC]`}
                  >
                    Privacy
                  </Link>
                  <Link
                    href="/terms"
                    className={`${type.caption2} text-[#A8A092] transition-colors duration-300 hover:text-[#EAE5DC]`}
                  >
                    Terms
                  </Link>
                  <a
                    href="https://github.com/chinmayyy01/switchboard"
                    target="_blank"
                    rel="noreferrer"
                    className={`${type.caption2} text-[#A8A092] transition-colors duration-300 hover:text-[#EAE5DC]`}
                  >
                    Source
                  </a>
                </div>
              </div>
            </nav>
          </div>

          <div
            className={`${type.caption5} mt-8 flex flex-wrap items-center justify-between gap-4 text-[#A8A092]`}
          >
            <span>© 2026 Switchboard. Built with Sarvam, Go and Next.js.</span>
            <span>Demo only. No production dispatch.</span>
          </div>
        </div>
      </footer>
      </div>

      <script
        type="application/ld+json"
        dangerouslySetInnerHTML={{ __html: JSON.stringify(schema) }}
      />
    </div>
  );
}
