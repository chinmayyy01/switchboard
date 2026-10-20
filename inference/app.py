"""Switchboard inference sidecar: STT (faster-whisper) + TTS (Piper).

Exposes the two model seams the Go service needs, over plain HTTP:
  POST /stt?lang=   raw PCM s16le mono 16k body  ->  {text, lang, confidence}
  POST /tts         {text, lang}                 ->  raw PCM s16le mono 16k
  GET  /health

Models are lazy-loaded on first use so the service starts fast.
"""

import os
from functools import lru_cache

import numpy as np
from fastapi import FastAPI, HTTPException, Request
from fastapi.responses import Response

SAMPLE_RATE = 16000
WHISPER_MODEL = os.getenv("WHISPER_MODEL", "base")
PIPER_VOICE = os.getenv("PIPER_VOICE", "")

app = FastAPI(title="switchboard-inference")


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


def _resample(pcm: np.ndarray, sr: int, target: int = SAMPLE_RATE) -> np.ndarray:
    if sr == target or len(pcm) == 0:
        return pcm
    xi = np.linspace(0, len(pcm) - 1, int(len(pcm) * target / sr))
    return np.interp(xi, np.arange(len(pcm)), pcm).astype(np.int16)


@app.get("/health")
def health():
    return {
        "ok": True,
        "whisper_model": WHISPER_MODEL,
        "piper_voice": PIPER_VOICE or None,
        "sample_rate": SAMPLE_RATE,
    }


@app.post("/stt")
async def stt(request: Request):
    lang = request.query_params.get("lang") or None
    body = await request.body()
    audio = np.frombuffer(body, dtype=np.int16).astype(np.float32) / 32768.0

    segments, info = whisper().transcribe(audio, language=lang, beam_size=1)
    text = "".join(s.text for s in segments).strip()
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

    voice = piper()
    chunks = [chunk.audio_int16_bytes for chunk in voice.synthesize(text)]
    pcm = np.frombuffer(b"".join(chunks), dtype=np.int16)

    sr = int(getattr(voice.config, "sample_rate", SAMPLE_RATE))
    pcm = _resample(pcm, sr)
    return Response(content=pcm.tobytes(), media_type="application/octet-stream")
