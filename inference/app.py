import os
from functools import lru_cache

import numpy as np
from fastapi import FastAPI, HTTPException, Request
from fastapi.responses import Response
from prometheus_client import CONTENT_TYPE_LATEST, generate_latest
from starlette.concurrency import run_in_threadpool

from answer import answer as answer_query
from rag import EMBED_MODEL, KNOWLEDGE_DIR, embed_text, retrieve

SAMPLE_RATE = 16000
WHISPER_MODEL = os.getenv("WHISPER_MODEL", "base")
PIPER_VOICE = os.getenv("PIPER_VOICE", "")
ASR_PROMPT = os.getenv(
    "ASR_PROMPT",
    "The caller may speak phone numbers and addresses. Transcribe digits accurately.",
)

app = FastAPI(title="switchboard-inference")


def _setup_otel(app):
    endpoint = os.getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
    mode = os.getenv("OTEL_TRACES")
    if not endpoint and mode != "stdout":
        return

    from opentelemetry import trace
    from opentelemetry.instrumentation.fastapi import FastAPIInstrumentor
    from opentelemetry.instrumentation.urllib import URLLibInstrumentor
    from opentelemetry.sdk.resources import Resource
    from opentelemetry.sdk.trace import TracerProvider
    from opentelemetry.sdk.trace.export import BatchSpanProcessor, ConsoleSpanExporter

    provider = TracerProvider(resource=Resource.create({"service.name": "switchboard-ai"}))
    if endpoint:
        from opentelemetry.exporter.otlp.proto.http.trace_exporter import OTLPSpanExporter

        provider.add_span_processor(BatchSpanProcessor(OTLPSpanExporter()))
    else:
        provider.add_span_processor(BatchSpanProcessor(ConsoleSpanExporter()))
    trace.set_tracer_provider(provider)
    FastAPIInstrumentor.instrument_app(app)
    URLLibInstrumentor().instrument()


_setup_otel(app)


@lru_cache(maxsize=1)
def whisper():
    try:
        from faster_whisper import WhisperModel
    except ImportError as e:
        raise HTTPException(503, f"faster-whisper not installed: {e}")
    return WhisperModel(WHISPER_MODEL, device="cpu", compute_type="int8")


@lru_cache(maxsize=1)
def piper():
    if not PIPER_VOICE:
        raise HTTPException(503, "PIPER_VOICE env var not set (path to a .onnx voice)")
    try:
        from piper import PiperVoice
    except ImportError as e:
        raise HTTPException(503, f"piper-tts not installed: {e}")
    return PiperVoice.load(PIPER_VOICE)


def _resample(pcm, sr, target=SAMPLE_RATE):
    if sr == target or len(pcm) == 0:
        return pcm
    xi = np.linspace(0, len(pcm) - 1, int(len(pcm) * target / sr))
    return np.interp(xi, np.arange(len(pcm)), pcm).astype(np.int16)


def _transcribe(audio, lang):
    segments, info = whisper().transcribe(
        audio,
        language=lang,
        beam_size=1,
        initial_prompt=ASR_PROMPT,
        condition_on_previous_text=False,
        vad_filter=True,
    )
    text = "".join(s.text for s in segments).strip()
    return text, info


def _synthesize(text):
    voice = piper()
    chunks = [chunk.audio_int16_bytes for chunk in voice.synthesize(text)]
    return b"".join(chunks), int(getattr(voice.config, "sample_rate", SAMPLE_RATE))


@app.get("/health")
def health():
    return {
        "ok": True,
        "whisper_model": WHISPER_MODEL,
        "piper_voice": PIPER_VOICE or None,
        "sample_rate": SAMPLE_RATE,
        "embed_model": EMBED_MODEL,
        "knowledge_dir": KNOWLEDGE_DIR,
    }


@app.get("/metrics")
def metrics():
    return Response(content=generate_latest(), media_type=CONTENT_TYPE_LATEST)


@app.post("/stt")
async def stt(request: Request):
    lang = request.query_params.get("lang") or None
    body = await request.body()
    audio = np.frombuffer(body, dtype=np.int16).astype(np.float32) / 32768.0

    text, info = await run_in_threadpool(_transcribe, audio, lang)
    return {
        "text": text,
        "lang": info.language,
        "confidence": float(getattr(info, "language_probability", 0.0)),
    }


@app.post("/tts")
async def tts(request: Request):
    data = await request.json()
    text = data.get("text", "")
    if not text:
        raise HTTPException(400, "text required")

    raw, sr = await run_in_threadpool(_synthesize, text)
    pcm = np.frombuffer(raw, dtype=np.int16)
    pcm = _resample(pcm, sr)
    return Response(content=pcm.tobytes(), media_type="application/octet-stream")


@app.post("/embed")
async def embed_endpoint(request: Request):
    data = await request.json()
    text = data.get("text", "")
    if not text:
        raise HTTPException(400, "text required")
    vec = await run_in_threadpool(embed_text, text)
    if vec is None:
        raise HTTPException(503, "embedding unavailable (is ollama running?)")
    return {"embedding": vec}


@app.post("/retrieve")
async def retrieve_endpoint(request: Request):
    data = await request.json()
    tenant = data.get("tenant", "")
    query = data.get("query", "")
    if not tenant or not query:
        raise HTTPException(400, "tenant and query required")
    return await run_in_threadpool(retrieve, tenant, query, int(data.get("k", 3)), data.get("types"))


@app.post("/answer")
async def answer_endpoint(request: Request):
    data = await request.json()
    tenant = data.get("tenant", "")
    query = data.get("query", "")
    if not tenant or not query:
        raise HTTPException(400, "tenant and query required")
    return await run_in_threadpool(answer_query, tenant, query, int(data.get("k", 3)))
