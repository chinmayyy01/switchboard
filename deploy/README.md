# Deploy Switchboard on a single VM

Target: one Linux VM with Caddy, Go, Node, Python, and Ollama. Caddy serves the
web app and proxies API, WebSocket, metrics, and AI paths to the same origin.

This guide prepares the deployment. A public URL still needs a server, a domain,
and DNS.

## 1. Server prerequisites

- Ubuntu 24.04 or similar
- Go 1.26+
- Node.js 22+
- Python 3.12
- PostgreSQL 16+ (managed service recommended)
- Caddy 2
- Ollama
- A domain pointing at the VM
- A Sarvam API key

Create a service user:

```bash
sudo useradd --system --create-home --shell /usr/sbin/nologin switchboard
sudo mkdir -p /opt/switchboard
sudo chown -R switchboard:switchboard /opt/switchboard
```

## 2. Install the app

```bash
sudo -u switchboard git clone https://github.com/chinmayyy01/switchboard.git /opt/switchboard
cd /opt/switchboard
sudo -u switchboard cp .env.example .env
```

Set the environment:

```bash
APP_ENV=production
APP_ORIGIN=https://voice.example.com
COOKIE_SECURE=true
AUTH_DRIVER=postgres
AUTH_DISABLED=false
DATABASE_URL=postgres://switchboard:...@db.example.com:5432/switchboard?sslmode=require
REDIS_URL=redis://127.0.0.1:6379/0
TRUSTED_PROXIES=127.0.0.1/32
PRICING_FILE=/opt/switchboard/config/pricing.json
SMTP_HOST=smtp.example.com
SMTP_PORT=587
SMTP_USER=...
SMTP_PASSWORD=...
SMTP_FROM=no-reply@voice.example.com
APP_BASE_URL=https://voice.example.com
SARVAM_API_KEY=sk_...
INFERENCE_URL=http://127.0.0.1:8100
KNOWLEDGE_DIR=/opt/switchboard/data/knowledge
OLLAMA_URL=http://127.0.0.1:11434
```

`AUTH_DISABLED=true` is ignored outside `APP_ENV=development`; production always
requires auth. Create the owner before the first deploy with
`go run ./cmd/account -owner -email you@example.com -password '...' -tenant rivera`.

Add the LiveKit agent settings when the worker runs on the same VM or in Cloud:
`LIVEKIT_URL`, `LIVEKIT_API_KEY`, `LIVEKIT_API_SECRET`, `AGENT_API_KEY`, and a
distinct `AGENT_TOKEN_SECRET`.

## 3. Install dependencies

```bash
cd /opt/switchboard
sudo -u switchboard python3.12 -m venv inference/.venv
sudo -u switchboard inference/.venv/bin/pip install -r inference/requirements.txt
sudo -u switchboard bash -lc 'cd web && npm ci'
sudo -u ollama ollama pull nomic-embed-text
sudo -u ollama ollama pull qwen2.5vl:3b
```

Run the pulls as the user that runs Ollama. On Ubuntu that is usually
`ollama`; if Ollama runs as `switchboard`, use that user instead.

## 4. Build

```bash
cd /opt/switchboard
sudo -u switchboard mkdir -p bin
sudo -u switchboard go build -o bin/switchboard ./cmd/switchboard
sudo -u switchboard go run ./cmd/account -owner \
  -email owner@example.com -password 'change-me' -tenant rivera
sudo -u switchboard inference/.venv/bin/python inference/ingest.py \
  --tenant rivera --docs tenants/rivera/docs
sudo -u switchboard inference/.venv/bin/python inference/ingest.py \
  --tenant acme --docs tenants/acme/docs
```

The Go service applies versioned booking/staff schema migrations at startup. Keep a database backup before deploying a new version.

Build the web app with public URLs baked in:

```bash
cd /opt/switchboard/web
sudo -u switchboard env \
  NEXT_PUBLIC_SITE_URL=https://voice.example.com \
  NEXT_PUBLIC_API_URL=https://voice.example.com \
  NEXT_PUBLIC_AI_URL=https://voice.example.com/ai \
  NEXT_PUBLIC_AUTH_DISABLED=false \
  npm run build
```

## 5. Services

```bash
sudo cp deploy/switchboard.service /etc/systemd/system/
sudo cp deploy/switchboard-ai.service /etc/systemd/system/
sudo cp deploy/switchboard-web.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now switchboard switchboard-ai switchboard-web
```

## 6. Caddy

```bash
sudo cp deploy/Caddyfile /etc/caddy/Caddyfile
sudo systemctl edit caddy
```

Add the environment values:

```ini
[Service]
Environment=DOMAIN=voice.example.com
Environment=ACME_EMAIL=you@example.com
```

Then:

```bash
sudo systemctl restart caddy
```

Caddy obtains TLS automatically. It proxies:

- `/api/*` to Go on `:8090`
- `/internal/livekit/*` to Go on `:8090` for signed LiveKit webhooks
- `/metrics` to Go
- `/ai/*` to Python on `:8100`
- everything else to Next on `:3000`

Create a LiveKit Cloud webhook pointing at
`https://<your-domain>/internal/livekit/webhook` with the same signing API key
as `LIVEKIT_API_KEY`. Caller ID, call closure, and recording state depend on it.
Only the `/internal/livekit/*` prefix is public; the agent tool routes remain
private. If the agent worker runs in LiveKit Cloud rather than on this VM, it
must reach `/internal/agent/session` and `/internal/tools/*`. Put the worker on
the same private network as Go, or add explicit Caddy rules for those two
prefixes and treat `AGENT_API_KEY` as a high-value secret.

## 7. Smoke test

```bash
curl -fsS https://voice.example.com/healthz
curl -fsS https://voice.example.com/ai/health
```

Then open the console, run a browser call, and check the booking card.

## 8. Observability

The Prometheus, Grafana, and Jaeger stack is optional:

```bash
docker compose -f deploy/observability/docker-compose.yml up -d
```

Prometheus scrapes `host.docker.internal:8090`. On a Linux VM, either add
`--add-host=host.docker.internal:host-gateway` or change the target to the host
address.

## Notes

- Bookings, staff, calls, transcript turns, labels, recordings metadata, usage,
  accounts, and sessions are stored in PostgreSQL.
- Set `REDIS_URL` when running more than one Go process; call and login limits
  are shared through Redis. Without it they are per process.
- Run `make retention` (or `cmd/retention`) on a schedule to enforce the 30-day
  audio and 180-day transcript policy.
- Self-serve signup creates a new tenant under `tenants/<id>/`. Ensure
  `/opt/switchboard/tenants` is writable by the service user and backed up.
  Multiple Go instances need a shared volume or tenant config moved to a
  database/object store.
- Recording requires `RECORDING_S3_*` and `RECORDING_S3_BUCKET`; without it the
  agent tells the caller recording is unavailable.
- A real phone number still needs carrier KYC and the reviewed LiveKit SIP
  plans. SMS is not part of this version.
