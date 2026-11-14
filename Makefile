KNOWLEDGE_DIR ?= $(PWD)/data/knowledge
OLLAMA_MODEL ?= qwen3:1.7b
INFERENCE_URL ?= http://127.0.0.1:8100
PY := inference/.venv/bin/python
PY312 ?= python3.12
UV := inference/.venv/bin/uvicorn

-include .env
export

.PHONY: ai ollama go go-fake web ingest eval check sip-plan agent-install agent check-livekit retention backfill

agent-install:
	$(PY312) -m venv agent/.venv
	agent/.venv/bin/pip install -r agent/requirements.txt

agent:
	cd agent && set -a && . ../.env && set +a && .venv/bin/python src/main.py dev

check-livekit:
	set -a && . ./.env && set +a && agent/.venv/bin/python scripts/check_livekit.py

sip-plan:
	GOTOOLCHAIN=local go run ./cmd/sip-setup

retention:
	go run ./cmd/retention

backfill:
	go run ./cmd/backfill

ai:
	KNOWLEDGE_DIR=$(KNOWLEDGE_DIR) EMBED_MODEL=nomic-embed-text $(UV) app:app --app-dir inference --host 127.0.0.1 --port 8100

ollama:
	@pgrep -f "ollama serve" >/dev/null && echo "ollama already running on :11434" || ollama serve

go:
	INFERENCE_URL=$(INFERENCE_URL) OLLAMA_MODEL=$(OLLAMA_MODEL) go run ./cmd/switchboard

go-fake:
	go run ./cmd/switchboard

web:
	cd web && npm run dev

ingest:
	$(PY) inference/ingest.py --tenant rivera --docs tenants/rivera/docs

eval:
	go run ./cmd/eval
	$(PY) inference/eval_rag.py --tenant rivera --k 3 --gate

check:
	go build ./...
	go vet ./...
	go test ./...
	cd web && npm run typecheck
