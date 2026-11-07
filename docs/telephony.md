# Telephony: Plivo now, Exotel later

Switchboard uses **Plivo** as the SIP carrier and **LiveKit Cloud** for the media
path. Exotel is behind the same abstraction but intentionally unimplemented until
inbound SIP routing, trial limits, KYC, and SIP REFER support are confirmed.

Nothing in this flow purchases numbers or changes provider configuration
automatically.

## Provider abstraction

`internal/telephony` defines:

```go
type Provider interface {
    Name() string
    Plan(tenant.Tenant) (Plan, error)
}
```

`Plan` is a reviewed description of the LiveKit objects a tenant needs:

- an inbound trunk for the tenant's number
- an individual dispatch rule that puts each caller in `call-*`
- an agent dispatch with `{"tenant_id": "..."}` metadata
- an outbound trunk for warm transfer

`cmd/sip-setup` writes those as `lk`-compatible JSON files. An operator reviews
and applies them. Exotel returns `ErrNotConfigured` rather than pretending to
work.

## 1. Plivo prerequisites

1. Business KYC completed with Plivo.
2. One Indian DID purchased and attached to a SIP trunk.
3. Plivo trunk configured to send inbound calls to:

   ```text
   <sip-subdomain>.india.sip.livekit.cloud
   ```

4. Record the Plivo SIP domain, username, and password.

## 2. LiveKit objects

Set the environment from `.env.example`, then generate the plans:

```bash
TELEPHONY_PROVIDER=plivo \
LIVEKIT_SIP_SUBDOMAIN=<subdomain> \
PLIVO_SIP_DOMAIN=<plivo-sip-domain> \
PLIVO_USERNAME=<username> \
PLIVO_PASSWORD=<password> \
PLIVO_DEFAULT_NUMBER=+91... \
go run ./cmd/sip-setup -tenant rivera
```

This writes `deploy/livekit/rivera-{inbound,dispatch,outbound}.json`. Apply them
after review:

```bash
lk sip inbound create  deploy/livekit/rivera-inbound.json
lk sip dispatch create deploy/livekit/rivera-dispatch.json
lk sip outbound create deploy/livekit/rivera-outbound.json   # warm transfer only
```

Check them with `lk sip inbound list`, `lk sip dispatch list`, and
`lk sip outbound list`.

## 3. Agent

The agent registers as `switchboard-receptionist` and is dispatched by the
rule's metadata. It reads the tenant from job metadata, exchanges the bootstrap
key for a per-call token, and calls the Go tool API.

```bash
cd agent
python3.12 -m venv .venv
.venv/bin/pip install -r requirements.txt
set -a; source ../.env; set +a
.venv/bin/python src/main.py dev
```

For production, deploy to LiveKit Cloud in `ap-south`:

```bash
lk agent create --region ap-south
```

### Browser test call

Open `/console/livekit` after starting the Go service and the agent. The console
requests a room token from `GET /api/livekit/token?tenant=<id>`. The token embeds
`roomConfig.agents` with the agent name and `{tenant_id, call_id}` metadata, so
LiveKit dispatches the agent when the room is created. No separate dispatch call
is needed.

The browser receives live transcriptions on the `lk.transcription` text stream.
Booking, consent, and escalation events are published by the agent on the
`switchboard.events` data topic.

## 4. Consent and recording

LiveKit Cloud audio and transcript recording are **off** for every session:

```python
record={"audio": False, "transcript": False, "traces": True, "logs": True, "redaction": True}
```

The agent speaks the approved disclosure, then calls `set_recording_consent`.
Only when the caller agrees does it start an audio-only, two-channel Egress
recording to the bucket configured by `RECORDING_S3_*` (India region). If the
caller declines or is unclear, no recording is started and only the consent
decision, booking, and minimal call metadata are stored.

Because LiveKit fixes recording options when the session starts, this is the
only design that can honour a mid-call consent decision.

## 5. Voice input accuracy

Phone audio is the weakest link in the demo. Three controls matter:

- **Keyterms.** The agent sends the tenant's service names, the business name,
  and common address words to Deepgram as keyterms, so "drain cleaning" is not
  transcribed as "train cleaning".
- **Language.** `STT_LANGUAGE` defaults to `en-IN`, which outperforms `en-US`
  for Indian callers. `STT_ENDPOINTING_MS` defaults to 200 to avoid cutting
  callers off mid-address.
- **Server-side draft.** `save_address` and `save_callback_number` persist those
  fields on the call. `confirm_booking_details` reads them back from the
  database instead of accepting them from the model, so a phone number or
  address cannot be invented to satisfy a tool schema.
- **Read-back enforcement.** `confirm_booking_details` canonicalizes the service,
  rejects past dates, and returns a signed `confirmation_id`. `book_visit`
  rejects anything without that token, so the caller hears the details first.

If accuracy is still poor on real calls, capture the transcript log lines
(`caller said: ...`) and tune keyterms and endpointing before changing models.

## 6. Warm transfer

`transfer_to_human` calls Go to persist the escalation and read the tenant's
manager number, then uses LiveKit's `WarmTransferTask` with the outbound trunk.
The manager is briefed with a short summary before being merged with the caller.

Required environment:

```bash
LIVEKIT_SIP_OUTBOUND_TRUNK=<outbound trunk id>
LIVEKIT_SIP_NUMBER=<caller ID shown to the manager>
MANAGER_NUMBER=+91...          # or MANAGER_NUMBER_<TENANT>
```

If the outbound trunk is missing, the transfer fails safely: the agent tells the
caller it cannot reach a teammate and offers a callback. It never drops the call.

## 7. Exotel

When Exotel responds, implement `internal/telephony/exotel.go` with the same
`Plan` shape and select it with `TELEPHONY_PROVIDER=exotel`. Nothing else in the
agent or Go API changes. Until then, `FromEnv()` returns the stub, and
`sip-setup` skips Exotel tenants with `ErrNotConfigured`.

## 8. Tests

- `go test ./internal/telephony` — plan shape and India endpoint.
- `TEST_DATABASE_URL=... go test ./internal/api -run TestAgentTools` — session
  bootstrap, idempotent booking, consent persistence, token rejection.
- `TEST_DATABASE_URL=... go test ./internal/jobs -run TestPostgres` — durable
  bookings, retries, cancellation, staff seeding.

Real-call checks still require Plivo credentials and a DID.
