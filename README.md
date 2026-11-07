# Switchboard

[![CI](https://github.com/chinmayyy01/switchboard/actions/workflows/ci.yml/badge.svg)](https://github.com/chinmayyy01/switchboard/actions/workflows/ci.yml)

**An AI voice receptionist for home services.**

Switchboard handles browser-based voice calls, books visits against its own staff schedule, answers policy questions from tenant documents, and hands the call to a supervisor with the full transcript. It runs on a Go realtime service, a Python AI service, a Next.js console, and configurable speech and language models.

![Switchboard landing page](docs/screenshots/landing.png)

> **Status:** portfolio demo snapshot.
> Browser only. No telephony, recordings, or production data.
> Auth is temporarily disabled in the local preview with `AUTH_DISABLED=true`; the owner/trial policy remains in code and can be re-enabled.
> Bookings and staff schedules persist in PostgreSQL. Auth uses SQLite; call records and labels remain local JSON files.
> Production hardening is tracked separately from this demo snapshot.

## Contents

- [What it does](#what-it-does)
- [Case study](docs/case-study.md)
- [Screenshots](#screenshots)
- [Quick start](#quick-start)
- [Architecture](#architecture)
+- [LiveKit agent and telephony](docs/telephony.md)
 - [Measurements](#measurements)
- [Configuration](#configuration)
- [Commands](#commands)
- [Project layout](#project-layout)
- [Limitations](#limitations)
- [Production readiness](#production-readiness)
- [Deployment](deploy/README.md)
- [Credits](#credits)

## What it does

- **Natural voice calls.** Sarvam Bulbul at 22.05 kHz, sentence by sentence TTS, barge in, and short backchannels such as "mm-hmm" and "okay".
- **Durable demo bookings.** The agent checks staff availability, assigns a qualified technician, and stores the visit in PostgreSQL. It is not connected to an external calendar or field-service system.
- **Alternatives when full.** If no technician is available, the tool returns specific dates and windows and the agent asks the caller to choose.
- **Grounded policy.** Hours, pricing, warranty, and service area are answered from tenant documents with citations; it refuses when the answer is not grounded.
- **Warm handoff.** Emergencies and human requests hold the call live for a supervisor instead of hanging up. The supervisor joins a browser audio bridge with the transcript.
- **Photo triage.** A caller can upload a photo; local vision (`qwen2.5vl`) writes a short description into the conversation.
- **Tenant aware.** Services, docs, staff, and the console are separated per tenant.
- **Measured.** Escalation, booking, retrieval, latency, and usage metrics are collected and scored.

## Screenshots

### Landing

![Landing page with Aero Shards](docs/screenshots/landing.png)

### Console

![Call floor and KPIs](docs/screenshots/console.png)

### Test call

![Browser test call, booking card, and photo triage](docs/screenshots/test-call.png)

### Measured call

![Transcript and booked visit](docs/screenshots/proof.png)

## Quick start

### Requirements

- Go 1.26+
- Node.js 20+
- Python 3.12
- Ollama
- PostgreSQL 16+ or Docker for the local database
- Make
- A Sarvam API key, recommended
- Optional: OpenAI or Deepgram keys

### 1. Environment

```bash
cp .env.example .env
```

Set at least:

```bash
SARVAM_API_KEY=sk_...
```

The example environment sets `AUTH_DISABLED=false`. This local preview sets `AUTH_DISABLED=true` in `.env`, so the console opens without sign-in. Set it back to `false` to use accounts again.

Without a key, set `STT_PROVIDER=local` and `TTS_PROVIDER=piper` to use Whisper and Piper, and the brain falls back to the hybrid rule state machine. The intended demo path is Sarvam for speech and chat, with Ollama for embeddings and vision.

### 2. Start PostgreSQL

The local compose file creates a development database matching `DATABASE_URL` in `.env.example`:

```bash
docker compose -f deploy/development/postgres-compose.yml up -d
```

Switchboard applies its booking and staff schema migrations on startup. Existing tenant staff files seed the database once; future changes belong in the Team screen.

### 3. Accounts for when auth is enabled

```bash
go run ./cmd/account -owner -email you@example.com -password 'change-me' -tenant rivera
```

`-prune` deletes every non-owner account and session. Create a standard account with 2 free calls:

```bash
go run ./cmd/account -email teammate@example.com -password 'change-me' -tenant rivera
```

The owner can use every tenant and has no call limit. Every other account is limited to 2 total calls and one active call at a time. These steps are only needed when `AUTH_DISABLED=false`.

### 4. Install the web app

```bash
cd web
npm install
cd ..
```

### 5. Install the AI service

```bash
python3.12 -m venv inference/.venv
inference/.venv/bin/pip install -r inference/requirements.txt
```

### 6. Ollama models

```bash
ollama pull nomic-embed-text
ollama pull qwen2.5vl:3b
```

Optional, for the local fallback and the experimental LLM brain:

```bash
ollama pull qwen3:1.7b
```

### 7. Piper voice for the local fallback

The voice files are not committed. Download a Piper medium voice and point `PIPER_VOICE` at it:

```bash
mkdir -p inference/voices
curl -L -o inference/voices/en_US-lessac-medium.onnx \
  https://huggingface.co/rhasspy/piper-voices/resolve/main/en/en_US/lessac/medium/en_US-lessac-medium.onnx
curl -L -o inference/voices/en_US-lessac-medium.onnx.json \
  https://huggingface.co/rhasspy/piper-voices/resolve/main/en/en_US/lessac/medium/en_US-lessac-medium.onnx.json
```

```bash
PIPER_VOICE=/absolute/path/to/en_US-lessac-medium.onnx
TTS_RATE=22050
```

### 8. Build the knowledge base

```bash
make ingest
inference/.venv/bin/python inference/ingest.py --tenant acme --docs tenants/acme/docs
```

Ollama must be running for embeddings.

### 9. Run

The LiveKit agent needs Python 3.12 or 3.13; the agent dependencies do not yet support 3.15.

```bash
make agent-install
```

In separate terminals:

```bash
make ollama
make ai
make go
make web
make agent
```

Telephony needs a Plivo DID and reviewed LiveKit SIP plans; see [docs/telephony.md](docs/telephony.md). Without them, the browser console still works.

Open:

- Landing: http://localhost:3000
- Console: http://localhost:3000/console
- Test call: http://localhost:3000/console?tab=test (choose WebRTC or the legacy transport)
- Sign in, when auth is enabled: http://localhost:3000/login

With auth disabled, open the console and start a call. With auth enabled, sign in with the owner or a trial account. Allow the microphone, use headphones, and press **Start call**. The AI service loads Whisper and Piper at startup; a local model downloads on first run if it is not cached.

### 10. Check

```bash
make check
```

Runs Go build, vet, and tests plus the web type check.

## Architecture

```mermaid
flowchart LR
  Browser["Next.js landing and console"] <-->|"16 kHz PCM + JSON events"| Go["Go realtime :8090"]
  Go <-->|"HTTP"| AI["Python AI :8100"]
  Go <-->|"bookings, staff, availability"| PG[("PostgreSQL")]
  Go <-->|"tenant config + policy docs"| Tenants["Tenant files"]
  Go <-->|"chat + tools"| Sarvam["Sarvam 105B"]
  AI <-->|"STT + TTS"| Sarvam
  AI <-->|"embeddings + vision"| Ollama["Ollama"]
  Go -->|"metrics and traces"| Obs["Prometheus, Grafana, OTel"]
  Supervisor["Supervisor browser"] <-->|"warm transfer bridge"| Go
```

### Call flow

1. The browser streams 16 kHz mono PCM in 20 ms frames.
2. Energy VAD with a 300 ms pre-roll finds the end of the caller's turn.
3. The AI service transcribes the utterance with Sarvam Saaras, Deepgram, or local Whisper.
4. The brain runs with tools: check service, save address, check staff availability, set a window, save a phone number, book, reschedule, cancel, or look up policy.
5. Policy questions are retrieved from tenant documents, answered with citations, and refused when ungrounded.
6. The reply is split into sentences and synthesized as it is produced.
7. On escalation, the call is held and flagged; a supervisor can join the live bridge.
8. Bookings and staff schedules persist in PostgreSQL; call transcripts and outcomes remain in local call storage.

### Protocol

One WebSocket per call.

- Binary frames: 16 kHz mono Int16 PCM, 320 samples per 20 ms frame.
- Text frames: JSON control and events such as `call.start`, `interrupt`, `stt.final`, `agent.text`, `tool.call`, `agent.speaking`, `agent.done`, `latency`, `escalation`, and `call.end`.
- TTS is returned at 22.05 kHz and played at its native rate.

### LiveKit agent and telephony

Real inbound calls run through LiveKit Cloud and a Python agent worker in `agent/`. The agent uses Deepgram for STT, OpenAI/Gemini for the LLM, and ElevenLabs for TTS, each behind a fallback adapter. Turn detection combines LiveKit's semantic turn detector with Silero VAD. The agent reads the tenant from SIP job metadata, exchanges its bootstrap key for a per-call token, and calls Go tools for availability, booking, policy, consent, and escalation.

Plivo is the primary SIP carrier. `cmd/sip-setup` writes reviewed LiveKit SIP plans for the operator to apply with `lk`; nothing is purchased automatically. Exotel sits behind the same `telephony.Provider` interface but is intentionally unimplemented until its inbound routing, KYC, and REFER support are confirmed. See [docs/telephony.md](docs/telephony.md).

LiveKit Cloud audio and transcript recording stay off. After the disclosure, the agent records the caller's choice; only an explicit yes starts an audio-only Egress recording to the configured India bucket. Declining or being unclear leaves recording off, and only the consent decision, booking, and minimal call metadata are stored.

### Staff and availability

Existing employees are seeded from `tenants/<id>/staff.json` the first time a tenant is initialized. After that, manage them in the Team screen. Each employee has a work role, contact details, services, weekdays, shifts, and an active status. Only active technicians with matching skills can be assigned visits; work roles do not grant app login permissions. Bookings, assignments, cancellations, and reschedules persist in PostgreSQL; booking transactions lock the schedule and use an idempotency key so retries return the original visit instead of creating duplicates. This is Switchboard's own job store, not a connected calendar or dispatch system.

### Tenants

Each tenant has a config, documents, and staff:

```text
tenants/rivera/
  config.json
  docs/
  staff.json
tenants/acme/
  config.json
  docs/
  staff.json
```

## Measurements

Measured on this Mac on the live demo. These are demo numbers, not a benchmark.

| Area | Result |
| --- | --- |
| Booking | 5 of 5 scripted scenarios; one prior live Sarvam smoke booking (requires API credentials/credits) |
| Escalation | precision 0.909, recall 0.909 on 22 hand-authored scenarios; not a labeled real-call sample |
| Retrieval | recall@1 1.00, recall@3 1.00, MRR 1.00 on 18 questions |
| Grounding | grounded 0.944, abstention 0.833 on 18 questions |
| First audio | 0.65 s to 1.3 s server side on the Sarvam smoke call |
| Local fallback p95 | 1.50 s at 5 calls, 2.83 s at 10 calls, Whisper base and Piper on CPU |
| Load | `make load` runs 50 synthetic callers; the earlier 500-caller figure is not reproducible from a checked-in report |
| Usage per 9 turn call | about 20.8k prompt tokens, 415 completion tokens, 920 TTS characters, 24.7 s STT |

Cost depends on current Sarvam rates. Usage is measured; a rupee cost is not calculated in this demo.
`make eval` reproduces the booking, 22-scenario handoff, and 18-question RAG results using the local tenant corpus. Latency and usage are one-off smoke measurements. `cmd/report` currently has no labeled-call data to reproduce a real-call escalation score.

## Configuration

| Variable | Purpose | Default |
| --- | --- | --- |
| `SARVAM_API_KEY` | Sarvam STT, TTS, and chat | empty |
| `SARVAM_MODEL` | Chat model | `sarvam-105b-conversations` |
| `SARVAM_STT_MODEL` | Speech to text model | `saaras:v4` |
| `SARVAM_TTS_MODEL` | Text to speech model | `bulbul:v3` |
| `SARVAM_TTS_SPEAKER` | Voice | `shubh` |
| `STT_PROVIDER` | `sarvam`, `deepgram`, or `local` | `sarvam` |
| `TTS_PROVIDER` | `sarvam` or `piper` | `sarvam` |
| `DEEPGRAM_API_KEY` | Optional streaming grade STT | empty |
| `WHISPER_MODEL` | Local Whisper model | `base` |
| `PIPER_VOICE` | Absolute path to a Piper voice | required for Piper |
| `TTS_RATE` | Playback sample rate | `22050` |
| `OPENAI_API_KEY` | Optional full LLM brain | empty |
| `BRAIN` | Local brain mode | `hybrid` |
| `OLLAMA_URL` | Ollama endpoint | `http://localhost:11434` |
| `OLLAMA_MODEL` | Local fallback model | `qwen3:1.7b` |
| `VISION_MODEL` | Local vision model | `qwen2.5vl:3b` |
| `INFERENCE_URL` | Python AI service | `http://127.0.0.1:8100` |
| `NEXT_PUBLIC_WS_URL` | Realtime WebSocket | `ws://localhost:8090/ws` |
| `NEXT_PUBLIC_API_URL` | Go API | `http://localhost:8090` |
| `NEXT_PUBLIC_AI_URL` | Python AI | `http://127.0.0.1:8100` |
| `NEXT_PUBLIC_SITE_URL` | Canonical and social metadata | `http://localhost:3000` |
| `AUTH_DB` | SQLite accounts and sessions | `data/auth.db` |
| `DATABASE_URL` | PostgreSQL bookings, employees, and schedules | required |
| `AUTH_DISABLED` | Bypass login and use a local owner account | `false` |
| `DEV_TENANT` | Tenant used by the local owner when auth is disabled | `rivera` |
| `NEXT_PUBLIC_AUTH_DISABLED` | Show the local-preview banner and copy | `false` |
| `APP_ORIGIN` | Allowed browser origin for cookies and CORS | `http://localhost:3000,http://127.0.0.1:3000` |
| `SESSION_TTL` | Session lifetime | `12h` |
| `COOKIE_SECURE` | Require HTTPS cookies | `false` |
| `KNOWLEDGE_DIR` | Ingested knowledge directory | `data/knowledge` |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | Optional OTLP endpoint | empty |
| `CHAT_MODEL_LARGE` | Optional larger model route | empty |

Accounts and sessions live in SQLite. The owner account is unlimited; other accounts have a 2-call lifetime quota and one active call at a time. While `AUTH_DISABLED=true`, every request uses a local unlimited owner account, so the quota is bypassed.

The LLM brain selection is automatic: Sarvam if `SARVAM_API_KEY` is set, then OpenAI if `OPENAI_API_KEY` is set, then Ollama via `BRAIN=llm` or the hybrid rule brain.

## Commands

| Command | What it does |
| --- | --- |
| `make ollama` | Start Ollama if it is not already running |
| `make ai` | Python AI service on `:8100` |
| `make go` | Go realtime service on `:8090` |
| `make web` | Next.js landing and console on `:3000` |
| `make ingest` | Ingest Rivera docs into `data/knowledge` |
| `make eval` | Booking, escalation, and RAG evals |
| `make smoke` | Live Sarvam call that asserts a booking; needs `SMOKE_EMAIL` and `SMOKE_PASSWORD` |
| `make load` | 50 concurrent fake callers; needs `LOAD_EMAIL` and `LOAD_PASSWORD` |
| `make sip-plan` | Write reviewed LiveKit SIP plans for Plivo |
| `make agent-install` | Create the agent venv and install LiveKit Agents |
| `make agent` | Run the LiveKit agent worker in dev mode |
| `make check` | Go build, vet, tests, and web type check |

`make smoke` uses real Sarvam credits. It signs in with `SMOKE_EMAIL` and `SMOKE_PASSWORD`, defaults to the `acme` tenant, and picks a random future date. Use the owner account: standard accounts are limited to 2 calls.

```bash
SMOKE_EMAIL=you@example.com SMOKE_PASSWORD='...' make smoke
LOAD_EMAIL=you@example.com LOAD_PASSWORD='...' make load
```

Restart `make go` first for a clean in-memory job store.

Optional observability:

```bash
docker compose -f deploy/observability/docker-compose.yml up -d
```

Prometheus runs on `:9090`, Grafana on `:3001`, and Jaeger on `:16686`. This stack is not required for the demo.

## Project layout

```text
agent/                 LiveKit Python agent worker (STT, LLM, TTS, tools)
cmd/switchboard/       Go entry point and HTTP routes
cmd/account/           Create the owner or trial accounts
cmd/eval/              Booking and escalation eval runner
cmd/loadgen/           Fake concurrent callers
cmd/sip-setup/         Emit reviewed LiveKit SIP plans for Plivo
internal/agent/        Session, VAD, brains, providers
internal/booking/      Rule and LLM brains, tools, evals
internal/jobs/         Job store, availability, staff assignment
internal/staff/        Staff skills, days, shifts, assignments
internal/tenant/       Tenant config loader
internal/ws/           WebSocket handler, hub feed, audio bridge
internal/api/          Calls, labels, report, staff, geocode, agent tools
internal/calls/        Durable call records and consent decisions
internal/livekit/      Browser room tokens (JWT HS256)
internal/telephony/    Provider abstraction: Plivo primary, Exotel stub
internal/auth/         Accounts, sessions, quotas, login
internal/store/        Call records and summaries
internal/inference/    AI service clients
inference/             FastAPI AI service, RAG, caching, TTS, STT, evals
web/                   Next.js landing and single page console
tenants/               Per tenant config, docs, and staff
scripts/               Live smoke test
docs/screenshots/      README screenshots
docs/case-study.md     Portfolio write-up
deploy/                Caddy, systemd, and single-VM guide
deploy/observability/  Prometheus, Grafana, Jaeger
.github/workflows/     CI
```

## Limitations

- **Telephony needs carrier setup.** LiveKit inbound SIP and the Plivo provider are implemented, but a live number requires Plivo KYC, a DID, and applying the generated SIP plans. Exotel is intentionally not implemented yet.
- **Auth is disabled in the local preview.** `AUTH_DISABLED=true` bypasses login and quota; do not deploy with it enabled. When enabled, accounts are invite-only, with no public signup, password reset, or email verification. Standard accounts get 2 lifetime calls; the owner is unlimited.
- **Call records remain local.** Bookings and staff persist in PostgreSQL; call records are appended to `data/calls.jsonl` and labels live in `data/labels.json`.
- **No recordings.** Transcripts are kept; audio is not.
- **No cost caps.** Usage is measured; hard budgets are not enforced.
- **Single node.** Redis, a queue, and automatic failover are not included. The one-call limiter is per process, so multiple Go instances would each allow a call.
- **Sarvam dependency.** Speech and chat quality and latency depend on the Sarvam API.
- **Demo data.** Staff, tenants, addresses, and calls are fictional.
- **Not for emergencies.** Do not point a real emergency line at this demo.
- **WebGPU.** The landing shards need WebGPU; without it the hero falls back to a flat background.

## Production readiness

This branch is a demo. Before a real phone product:

- [ ] Inbound DIDs and SIP or WebRTC media.
- [ ] Real transfer with ring groups, paging, hold timeout, and fallback.
- [ ] Public signup, email verification, password reset, RBAC, and distributed quotas.
- [x] Postgres for bookings and staff.
- [ ] Redis for distributed quotas and object storage for recordings.
- [ ] Call recordings with consent and retention.
- [ ] Cost metering, per-tenant budgets, and hard caps.
- [ ] SLOs, alerts, on call, and incident runbooks.
- [ ] Compliance: consent, recording disclosure, DPDP, and GDPR.

Production hardening is tracked separately; `main` stays as the portfolio demo snapshot.

## Portfolio

- [Case study](docs/case-study.md): problem, decisions, measured results, tradeoffs.
- [Deployment guide](deploy/README.md): single VM with Caddy, systemd, and Ollama.
- [CI](.github/workflows/ci.yml): Go build/vet/race tests, web typecheck/build, Python compile.
- Live KPI dashboard: the console Overview, plus the Grafana dashboard in `deploy/observability`.

## Credits

- [Sarvam AI](https://www.sarvam.ai) for Saaras, Bulbul, and Sarvam 105B.
- [React Bits Aero Shards](https://www.reactbits.dev/backgrounds/aero-shards) for the hero background, MIT licensed.
- [Phosphor Icons](https://phosphoricons.com) for the console and landing icons.
- [Manrope](https://fonts.google.com/specimen/Manrope) and [Geist Mono](https://fonts.google.com/specimen/Geist+Mono) through `next/font`.
- Go, Next.js, FastAPI, and Ollama.
