# Local observability stack

Jaeger (traces) + Prometheus (metrics) + Grafana (dashboards).

## Start
```bash
docker compose -f deploy/observability/docker-compose.yml up -d
```
- Jaeger: http://localhost:16686 (search services `switchboard`, `switchboard-ai`)
- Prometheus: http://localhost:9090
- Grafana: http://localhost:3001 (admin / admin) → **Import** `grafana-dashboard.json`

## Enable traces
Both services must point at the OTLP HTTP endpoint:
```bash
export OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318
```
- Go reads it at startup (`internal/obs`).
- Python reads it at import (`inference/app.py` → `_setup_otel`).

No collector? Debug spans to stdout instead:
```bash
export OTEL_TRACES=stdout
```

## What you get
- **Go spans:** `call` → `turn` → `stt` / `brain` / `tts`, plus client spans for
  `/stt`, `/tts`, `/answer` and Ollama.
- **Python spans:** FastAPI request spans + urllib (Ollama) spans, as children of
  the Go client span via W3C `traceparent`.
- **Logs:** JSON via `log/slog`, every line carries `call_id`.
- **Metrics:** `/metrics` scraped by Prometheus; dashboard + alerts in this folder.

## Notes
- Prometheus scrapes `host.docker.internal:8090`; on Docker Desktop for macOS this
  resolves to the host. With Colima, add `--add-host` or use the host IP.
- Alerts fire in Prometheus but do not notify until you add Alertmanager.
- Grafana runs on 3001 so it doesn't collide with Next on 3000.
