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
AUTH_DB=data/auth.db
AUTH_DISABLED=false
SARVAM_API_KEY=sk_...
STT_PROVIDER=sarvam
TTS_PROVIDER=sarvam
INFERENCE_URL=http://127.0.0.1:8100
PIPER_VOICE=/opt/switchboard/inference/voices/en_US-lessac-medium.onnx
TTS_RATE=22050
KNOWLEDGE_DIR=/opt/switchboard/data/knowledge
OLLAMA_URL=http://127.0.0.1:11434
```

`AUTH_DISABLED=true` is only for a local preview. It gives every visitor an
unlimited owner account and will burn Sarvam credits in public.

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

Download the Piper fallback voice:

```bash
sudo -u switchboard mkdir -p inference/voices
sudo -u switchboard curl -L -o inference/voices/en_US-lessac-medium.onnx \
  https://huggingface.co/rhasspy/piper-voices/resolve/main/en/en_US/lessac/medium/en_US-lessac-medium.onnx
sudo -u switchboard curl -L -o inference/voices/en_US-lessac-medium.onnx.json \
  https://huggingface.co/rhasspy/piper-voices/resolve/main/en/en_US/lessac/medium/en_US-lessac-medium.onnx.json
```

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

Build the web app with public URLs baked in:

```bash
cd /opt/switchboard/web
sudo -u switchboard env \
  NEXT_PUBLIC_SITE_URL=https://voice.example.com \
  NEXT_PUBLIC_API_URL=https://voice.example.com \
  NEXT_PUBLIC_WS_URL=wss://voice.example.com/ws \
  NEXT_PUBLIC_SUPERVISOR_WS_URL=wss://voice.example.com/ws/supervisor \
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

- `/api/*` and `/ws*` to Go on `:8090`
- `/metrics` to Go
- `/ai/*` to Python on `:8100`
- everything else to Next on `:3000`

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

- The Go call limiter and auth sessions are single-instance. Use one Go process,
  or move quotas and sessions to Redis/Postgres before scaling out.
- Jobs and staff are in memory and reset on restart. Call records persist in
  `data/`.
- Set `AUTH_DISABLED=false` for any public deployment.
- SMS and telephony are not part of this version.
