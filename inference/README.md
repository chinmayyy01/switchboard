# Switchboard AI service (Python)

FastAPI service for **embeddings** (Ollama `nomic-embed-text`), **retrieval**
(hybrid BM25 + vector with a grounding gate), and **vision**. The LiveKit agent
uses its own STT/TTS plugins; the `/stt` and `/tts` routes here are legacy and
not used by the phone/browser agent. The Go service calls this service for RAG.

## Endpoints

| Endpoint | In | Out |
|----------|----|-----|
| `POST /stt?lang=` (legacy) | raw PCM s16le mono 16k | `{text, lang, confidence}` |
| `POST /tts` (legacy) | `{text, lang}` | raw PCM s16le mono 16k |
| `POST /embed` | `{text}` | `{embedding: [768 floats]}` |
| `POST /retrieve` | `{tenant, query, k, types?}` | `{grounded, top_score, results[]}` |
| `POST /answer` | `{tenant, query, k, lang}` | `{grounded, answer, citations, top_score}` |
| `GET /health` | — | model + config info |

## Setup

```bash
cd inference
python3.14 -m venv .venv
.venv/bin/pip install -r requirements.txt
export EMBED_MODEL=nomic-embed-text
export KNOWLEDGE_DIR=../data/knowledge
ollama pull nomic-embed-text
```

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
curl 127.0.0.1:8100/health
```

## Wire the Go service to it

```bash
ollama pull qwen3:1.7b
export INFERENCE_URL=http://127.0.0.1:8100
export OLLAMA_URL=http://localhost:11434
export OLLAMA_MODEL=qwen3:1.7b
go run ./cmd/switchboard
```

Without `INFERENCE_URL`, the Go service falls back to the fakes.

## Retrieval behaviour

- `/answer` is the grounded answering path: retrieve → extractive prompt to
  Ollama → deterministic best-sentence fallback → citations. Go only calls it.
- Tenant is a **hard filter**; an unknown tenant returns no results.
- Score = `0.6·cosine + 0.4·bm25`; `grounded` requires
  `vector ≥ 0.55` and `score ≥ 0.45` (BM25-only fallback: `score ≥ 0.5`).
- Below the gate the agent must **abstain and escalate**, never guess.
- nomic embeddings are 768-dim; numpy cosine is fine for a tenant corpus, swap
  to FAISS/Chroma at a few thousand chunks.

## Notes

- Models lazy-load on first request.
- The legacy `/tts` route needs `PIPER_VOICE`; the LiveKit agent does not call it.
- The nomic/vision APIs can shift between versions; `app.py` and `rag.py` are
  the files to adjust.
