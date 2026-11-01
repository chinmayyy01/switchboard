import base64
import io
import json
import logging
import os
import threading
import urllib.parse
import urllib.request
import uuid
import wave
from contextlib import asynccontextmanager
from functools import lru_cache

import numpy as np
from fastapi import FastAPI, HTTPException, Request
from fastapi.middleware.cors import CORSMiddleware
from fastapi.responses import Response
from prometheus_client import CONTENT_TYPE_LATEST, Counter, generate_latest
from starlette.concurrency import run_in_threadpool

from answer import answer as answer_query
from rag import EMBED_MODEL, KNOWLEDGE_DIR, embed_text, retrieve

SAMPLE_RATE = 16000
TTS_RATE = int(os.getenv("TTS_RATE", "22050"))
WHISPER_MODEL = os.getenv("WHISPER_MODEL", "base")
PIPER_VOICE = os.getenv("PIPER_VOICE", "")
STT_PROVIDER = os.getenv("STT_PROVIDER", "local")
TTS_PROVIDER = os.getenv("TTS_PROVIDER", "piper")
DEEPGRAM_API_KEY = os.getenv("DEEPGRAM_API_KEY", "")
DEEPGRAM_MODEL = os.getenv("DEEPGRAM_MODEL", "nova-3")
SARVAM_API_KEY = os.getenv("SARVAM_API_KEY", "")
SARVAM_STT_MODEL = os.getenv("SARVAM_STT_MODEL", "saaras:v4")
SARVAM_TTS_MODEL = os.getenv("SARVAM_TTS_MODEL", "bulbul:v3")
SARVAM_TTS_SPEAKER = os.getenv("SARVAM_TTS_SPEAKER", "shubh")
ASR_PROMPT = os.getenv(
    "ASR_PROMPT",
    "The caller may speak phone numbers and addresses. Transcribe digits accurately.",
)

logger = logging.getLogger("switchboard-ai")
PIPER_LOCK = threading.Lock()
TTS_CHARS = Counter(
    "switchboard_tts_characters_total", "Characters synthesized.", ["provider"]
)
STT_SECONDS = Counter(
    "switchboard_stt_seconds_total", "Audio seconds transcribed.", ["provider"]
)


@asynccontextmanager
async def lifespan(_app):
    def warm():
        for name, load in (("whisper", whisper), ("piper", piper)):
            try:
                load()
                logger.info("%s warm", name)
            except Exception as e:
                logger.warning("%s warmup failed: %s", name, e)

    threading.Thread(target=warm, daemon=True).start()
    yield


app = FastAPI(title="switchboard-inference", lifespan=lifespan)

app.add_middleware(
    CORSMiddleware,
    allow_origins=["http://localhost:3000", "http://127.0.0.1:3000"],
    allow_methods=["GET", "POST", "OPTIONS"],
    allow_headers=["*"],
)


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
    try:
        import audioop

        raw, _ = audioop.ratecv(
            pcm.astype(np.int16).tobytes(), 2, 1, int(sr), target, None
        )
        return np.frombuffer(raw, dtype=np.int16)
    except Exception:
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


def _sarvam_lang(lang):
    if not lang:
        return "unknown"
    if "-" in lang:
        return lang
    return lang + "-IN"


def _sarvam_tts(text, lang):
    code = _sarvam_lang(lang)
    if code == "unknown":
        code = "en-IN"
    payload = {
        "text": text,
        "language_code": code,
        "speaker": SARVAM_TTS_SPEAKER,
        "model": SARVAM_TTS_MODEL,
        "speech_sample_rate": TTS_RATE,
        "output_audio_codec": "wav",
    }
    req = urllib.request.Request(
        "https://api.sarvam.ai/text-to-speech",
        data=json.dumps(payload).encode(),
        headers={
            "api-subscription-key": SARVAM_API_KEY,
            "Content-Type": "application/json",
        },
        method="POST",
    )
    with urllib.request.urlopen(req, timeout=30) as resp:
        data = json.loads(resp.read())
    raw = base64.b64decode(data["audios"][0])
    with wave.open(io.BytesIO(raw), "rb") as w:
        sr = w.getframerate()
        pcm = np.frombuffer(w.readframes(w.getnframes()), dtype=np.int16)
    return pcm, sr


def _multipart(fields, files):
    boundary = "----switchboard" + uuid.uuid4().hex
    body = bytearray()
    for name, value in fields.items():
        body += f"--{boundary}\r\n".encode()
        body += f'Content-Disposition: form-data; name="{name}"\r\n\r\n'.encode()
        body += str(value).encode() + b"\r\n"
    for name, (filename, data, ctype) in files.items():
        body += f"--{boundary}\r\n".encode()
        body += (
            f'Content-Disposition: form-data; name="{name}"; filename="{filename}"\r\n'
        ).encode()
        body += f"Content-Type: {ctype}\r\n\r\n".encode()
        body += data + b"\r\n"
    body += f"--{boundary}--\r\n".encode()
    return bytes(body), f"multipart/form-data; boundary={boundary}"


def _sarvam_stt(pcm_bytes, lang):
    code = _sarvam_lang(lang)
    buf = io.BytesIO()
    with wave.open(buf, "wb") as w:
        w.setnchannels(1)
        w.setsampwidth(2)
        w.setframerate(SAMPLE_RATE)
        w.writeframes(pcm_bytes)
    body, ctype = _multipart(
        {"model": SARVAM_STT_MODEL, "language_code": code},
        {"file": ("audio.wav", buf.getvalue(), "audio/wav")},
    )
    req = urllib.request.Request(
        "https://api.sarvam.ai/speech-to-text",
        data=body,
        headers={"api-subscription-key": SARVAM_API_KEY, "Content-Type": ctype},
        method="POST",
    )
    with urllib.request.urlopen(req, timeout=30) as resp:
        data = json.loads(resp.read())
    return {
        "text": (data.get("transcript") or "").strip(),
        "lang": data.get("language_code") or code,
        "confidence": float(data.get("language_probability") or 1.0),
    }


def _deepgram_stt(pcm_bytes, lang):
    params = {
        "model": DEEPGRAM_MODEL,
        "smart_format": "true",
        "punctuate": "true",
        "encoding": "linear16",
        "sample_rate": str(SAMPLE_RATE),
        "channels": "1",
    }
    if lang:
        params["language"] = lang
    else:
        params["detect_language"] = "true"

    url = "https://api.deepgram.com/v1/listen?" + urllib.parse.urlencode(params)
    req = urllib.request.Request(
        url,
        data=pcm_bytes,
        headers={
            "Authorization": "Token " + DEEPGRAM_API_KEY,
            "Content-Type": "audio/raw",
        },
        method="POST",
    )
    with urllib.request.urlopen(req, timeout=30) as resp:
        data = json.loads(resp.read())

    channel = data["results"]["channels"][0]
    alt = channel["alternatives"][0]
    return {
        "text": (alt.get("transcript") or "").strip(),
        "lang": channel.get("detected_language") or lang or "en",
        "confidence": float(alt.get("confidence") or 0.0),
    }


def _synthesize(text):
    voice = piper()
    with PIPER_LOCK:
        chunks = [chunk.audio_int16_bytes for chunk in voice.synthesize(text)]
    return b"".join(chunks), int(getattr(voice.config, "sample_rate", SAMPLE_RATE))


@app.get("/health")
def health():
    return {
        "ok": True,
        "whisper_model": WHISPER_MODEL,
        "stt_provider": STT_PROVIDER if STT_PROVIDER != "sarvam" or SARVAM_API_KEY else "local",
        "tts_provider": TTS_PROVIDER if TTS_PROVIDER != "sarvam" or SARVAM_API_KEY else "piper",
        "deepgram": bool(DEEPGRAM_API_KEY),
        "sarvam": bool(SARVAM_API_KEY),
        "piper_voice": PIPER_VOICE or None,
        "sample_rate": SAMPLE_RATE,
        "tts_sample_rate": TTS_RATE,
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

    if STT_PROVIDER == "sarvam" and SARVAM_API_KEY:
        try:
            result = await run_in_threadpool(_sarvam_stt, body, lang)
            STT_SECONDS.labels("sarvam").inc(len(body) / 2 / SAMPLE_RATE)
            return result
        except Exception as e:
            logger.warning("sarvam stt failed, falling back to local: %s", e)

    if STT_PROVIDER == "deepgram" and DEEPGRAM_API_KEY:
        try:
            result = await run_in_threadpool(_deepgram_stt, body, lang)
            STT_SECONDS.labels("deepgram").inc(len(body) / 2 / SAMPLE_RATE)
            return result
        except Exception as e:
            logger.warning("deepgram failed, falling back to local: %s", e)

    STT_SECONDS.labels("local").inc(len(body) / 2 / SAMPLE_RATE)
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

    if TTS_PROVIDER == "sarvam" and SARVAM_API_KEY:
        try:
            pcm, sr = await run_in_threadpool(
                _sarvam_tts, text, data.get("lang") or "en"
            )
            TTS_CHARS.labels("sarvam").inc(len(text))
            pcm = _resample(pcm, sr, TTS_RATE)
            return Response(
                content=pcm.tobytes(), media_type="application/octet-stream"
            )
        except Exception as e:
            logger.warning("sarvam tts failed, falling back to piper: %s", e)

    TTS_CHARS.labels("piper").inc(len(text))
    raw, sr = await run_in_threadpool(_synthesize, text)
    pcm = np.frombuffer(raw, dtype=np.int16)
    pcm = _resample(pcm, sr, TTS_RATE)
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
