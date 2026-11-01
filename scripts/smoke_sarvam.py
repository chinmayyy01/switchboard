#!/usr/bin/env python3
"""Live Sarvam smoke test: place a scripted booking call through the real stack.

Requires the Go server, the AI service, and SARVAM_API_KEY. Caller audio is
synthesized with Sarvam TTS (priya) at 16 kHz, then streamed to /ws. The agent
uses Saaras STT, sarvam-105b-conversations, and Bulbul TTS.

Run: make smoke
"""

import asyncio
import base64
import io
import json
import os
import random
import sys
import time
import urllib.request
import wave
from datetime import date, timedelta

import websockets

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))


def env(key, default=""):
    value = os.environ.get(key)
    if value:
        return value
    path = os.path.join(ROOT, ".env")
    if os.path.exists(path):
        for line in open(path):
            if line.startswith(key + "="):
                return line.split("=", 1)[1].strip()
    return default


API_KEY = env("SARVAM_API_KEY")
WS_URL = env("NEXT_PUBLIC_WS_URL", "ws://localhost:8090/ws")
TENANT = env("SMOKE_TENANT", "acme")
SPEAKER = env("SMOKE_CALLER_SPEAKER", "priya")
MODEL = env("SARVAM_TTS_MODEL", "bulbul:v3")


def synth(text):
    payload = {
        "text": text,
        "language_code": "en-IN",
        "speaker": SPEAKER,
        "model": MODEL,
        "speech_sample_rate": 16000,
        "output_audio_codec": "wav",
    }
    req = urllib.request.Request(
        "https://api.sarvam.ai/text-to-speech",
        data=json.dumps(payload).encode(),
        headers={
            "api-subscription-key": API_KEY,
            "Content-Type": "application/json",
        },
        method="POST",
    )
    with urllib.request.urlopen(req, timeout=60) as resp:
        data = json.loads(resp.read())
    raw = base64.b64decode(data["audios"][0])
    with wave.open(io.BytesIO(raw), "rb") as w:
        if w.getframerate() != 16000:
            raise SystemExit(f"caller TTS rate {w.getframerate()} != 16000")
        return w.readframes(w.getnframes())


async def send_utterance(ws, pcm):
    pcm = pcm + b"\x00" * 32000
    for i in range(0, len(pcm), 640):
        await ws.send(pcm[i : i + 640])
        await asyncio.sleep(0.01)


async def read_turn(ws):
    events = []
    tools = []
    audio_bytes = 0
    first_audio = None
    started = time.time()
    while True:
        msg = await asyncio.wait_for(ws.recv(), timeout=120)
        if isinstance(msg, bytes):
            audio_bytes += len(msg)
            if first_audio is None:
                first_audio = time.time() - started
            continue
        data = json.loads(msg)
        kind = data.get("type")
        events.append(data)
        if kind == "tool.call":
            tools.append(data.get("name"))
            print(f"  tool   {data.get('name')} {data.get('args')}")
        elif kind == "stt.final":
            print(f"  caller {data.get('text')}")
        elif kind == "agent.text":
            print(f"  agent  {data.get('text')}")
        elif kind == "escalation":
            print(f"  ESCALATION {data.get('reason')}")
        elif kind == "latency":
            print(
                "  latency "
                f"stt={data.get('stt_ms')}ms brain={data.get('brain_ms')}ms "
                f"tts={data.get('tts_ms')}ms first_audio={data.get('first_audio_ms')}ms"
            )
        if kind == "agent.done":
            return events, tools, audio_bytes, first_audio


async def main():
    if not API_KEY:
        raise SystemExit("SARVAM_API_KEY is not set in .env")

    target = date.today() + timedelta(days=14 + random.randint(0, 7))
    day = f"{target.strftime('%B')} {target.day}"
    lines = [
        "My kitchen sink is leaking and I need a plumber.",
        "Yes, that's right.",
        "It's 123 Main Street.",
        "Yes, that's right.",
        f"{day} in the morning at 10.",
        "Yes, that's right.",
        "My phone number is 555 123 4567.",
        "Yes, that's right.",
        "Yes, book it.",
    ]

    print(f"tenant={TENANT} date={day} caller_voice={SPEAKER}")
    print("synthesizing caller audio...")
    utterances = [synth(line) for line in lines]

    url = f"{WS_URL}?tenant={TENANT}"
    async with websockets.connect(url, max_size=None) as ws:
        await ws.send(json.dumps({"type": "call.start", "lang": "en"}))
        print("waiting for greeting...")
        try:
            while True:
                msg = await asyncio.wait_for(ws.recv(), timeout=10)
                if isinstance(msg, bytes):
                    continue
                data = json.loads(msg)
                if data.get("type") == "agent.text":
                    print(f"greeting: {data.get('text')}")
                if data.get("type") == "agent.done":
                    break
        except asyncio.TimeoutError:
            print("no greeting (continuing)")

        all_tools = []
        all_events = []
        first_audios = []
        started = time.time()

        for n, (line, pcm) in enumerate(zip(lines, utterances), 1):
            print(f"turn {n}: {line}")
            await send_utterance(ws, pcm)
            events, tools, audio, first = await read_turn(ws)
            all_events.extend(events)
            all_tools.extend(tools)
            lat = next((e for e in events if e.get("type") == "latency"), None)
            if lat and lat.get("first_audio_ms", -1) >= 0:
                first_audios.append(lat["first_audio_ms"])
            await asyncio.sleep(0.3)

        try:
            while True:
                msg = await asyncio.wait_for(ws.recv(), timeout=3)
                if isinstance(msg, str):
                    data = json.loads(msg)
                    all_events.append(data)
                    if data.get("type") in ("call.end", "latency"):
                        print(f"  {data.get('type')} {data}")
                    if data.get("type") == "call.end":
                        break
        except asyncio.TimeoutError:
            pass

    text = " ".join(
        e.get("text", "") for e in all_events if e.get("type") == "agent.text"
    ).lower()
    errors = [e for e in all_events if e.get("type") == "escalation"]
    booked = "book" in all_tools and "book" in text
    ok = booked and not errors
    prompt_tokens = sum(
        e.get("prompt_tokens", 0) for e in all_events if e.get("type") == "latency"
    )
    completion_tokens = sum(
        e.get("completion_tokens", 0) for e in all_events if e.get("type") == "latency"
    )

    print("\n--- result ---")
    print(f"tools: {all_tools}")
    print(f"escalations: {[e.get('reason') for e in errors]}")
    print(f"booked: {booked}")
    print(f"first_audio_ms: {first_audios}")
    print(f"tokens: prompt={prompt_tokens} completion={completion_tokens}")
    print(f"wall_seconds: {time.time() - started:.1f}")
    print("PASS" if ok else "FAIL")
    sys.exit(0 if ok else 1)


if __name__ == "__main__":
    asyncio.run(main())
