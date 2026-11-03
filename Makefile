PIPER_VOICE ?= $(PWD)/inference/voices/en_US-amy-low.onnx
KNOWLEDGE_DIR ?= $(PWD)/data/knowledge
OLLAMA_MODEL ?= qwen3:1.7b
INFERENCE_URL ?= http://127.0.0.1:8100
PY := inference/.venv/bin/python
UV := inference/.venv/bin/uvicorn

-include .env
export

.PHONY: ai ollama go go-fake web ingest eval load check smoke

ai:
	PIPER_VOICE=$(PIPER_VOICE) KNOWLEDGE_DIR=$(KNOWLEDGE_DIR) WHISPER_MODEL=$(WHISPER_MODEL) EMBED_MODEL=nomic-embed-text $(UV) app:app --app-dir inference --host 127.0.0.1 --port 8100

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

load:
	go run ./cmd/loadgen -n 50 -email "$(LOAD_EMAIL)" -password "$(LOAD_PASSWORD)"

smoke:
	$(PY) scripts/smoke_sarvam.py

check:
	go build ./...
	go vet ./...
	go test ./...
	cd web && npm run typecheck
