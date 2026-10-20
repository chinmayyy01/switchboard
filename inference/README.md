# Switchboard inference sidecar

FastAPI service wrapping the two local models:
**faster-whisper** (STT) and **Piper** (TTS). The Go service talks to it over
HTTP via `internal/inference`.

## Setup

```bash
cd inference
python3 -m venv .venv && source .venv/bin/activate
pip install -r requirements.txt

export PIPER_VOICE=/absolute/path/to/en_US-amy-low.onnx
export WHISPER_MODEL=base          # tiny | base | small | medium | large-v3
```

Piper voices: https://huggingface.co/rhasspy/piper-voices

## Run

```bash
uvicorn app:app --host 127.0.0.1 --port 8100
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

## Notes
- Models lazy-load on first request, so the service boots instantly and the
  first call is slower (model download/load).
- `/tts` resamples the voice output to 16 kHz so it matches the pipeline.
- The Piper/faster-whisper APIs change between versions; if an import or method
  name differs, this is the one file to adjust.
