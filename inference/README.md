# Switchboard AI service (Python)

FastAPI service that owns all model inference: **STT** (faster-whisper),
**TTS** (Piper), **embeddings** (Ollama `nomic-embed-text`), and **retrieval**
(hybrid BM25 + vector with a grounding gate). The Go service talks to it over
HTTP via `internal/inference` and `internal/ai`.

## Endpoints

| Endpoint | In | Out |
|----------|----|-----|
| `POST /stt?lang=` | raw PCM s16le mono 16k | `{text, lang, confidence}` |
| `POST /tts` | `{text, lang}` | raw PCM s16le mono 16k |
| `POST /embed` | `{text}` | `{embedding: [768 floats]}` |
| `POST /retrieve` | `{tenant, query, k, types?}` | `{grounded, top_score, results[]}` |
| `GET /health` | — | model + config info |

## Setup

```bash
cd inference
python3.14 -m venv .venv
.venv/bin/pip install -r requirements.txt
export PIPER_VOICE=/absolute/path/to/en_US-amy-low.onnx
export WHISPER_MODEL=base
export EMBED_MODEL=nomic-embed-text
export KNOWLEDGE_DIR=../data/knowledge
ollama pull nomic-embed-text
```

Piper voices: https://huggingface.co/rhasspy/piper-voices

## Index a tenant

```bash
# from the repo root
inference/.venv/bin/python inference/ingest.py \
  --tenant rivera --docs tenants/rivera/docs
# -> data/knowledge/rivera.json  (chunks + 768-dim vectors)
```

## Run

```bash
# from the repo root
inference/.venv/bin/uvicorn app:app --app-dir inference --host 127.0.0.1 --port 8100
curl localhost:8100/health
```

## Wire the Go service to it

```bash
ollama pull qwen3:1.7b
export INFERENCE_URL=http://localhost:8100
export OLLAMA_URL=http://localhost:11434
export OLLAMA_MODEL=qwen3:1.7b
go run ./cmd/switchboard
```

Without `INFERENCE_URL`, the Go service falls back to the fakes.

## Retrieval behaviour

- Tenant is a **hard filter**; an unknown tenant returns no results.
- Score = `0.6·cosine + 0.4·bm25`; `grounded` requires
  `vector ≥ 0.55` and `score ≥ 0.45` (BM25-only fallback: `score ≥ 0.5`).
- Below the gate the agent must **abstain and escalate**, never guess.
- nomic embeddings are 768-dim; numpy cosine is fine for a tenant corpus, swap
  to FAISS/Chroma at a few thousand chunks.

## Notes

- Models lazy-load on first request.
- `/tts` resamples the voice output to 16 kHz to match the pipeline.
- The Piper/faster-whisper/nomic APIs can shift between versions; `app.py` and
  `rag.py` are the files to adjust.
