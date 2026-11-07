from __future__ import annotations

import logging
import os

from livekit.agents import AgentSession, TurnHandlingOptions, inference
from livekit.agents.voice.turn import EndpointingOptions, PreemptiveGenerationOptions
from livekit.agents.llm import FallbackAdapter as LLMFallback
from livekit.agents.stt import FallbackAdapter as STTFallback
from livekit.agents.tts import FallbackAdapter as TTSFallback
from livekit.plugins import cartesia, deepgram, elevenlabs, openai, silero

from config import SessionConfig

logger = logging.getLogger("switchboard.session")

# Generic words that are easy to mishear on a phone line. Service names come
# from the tenant; these cover the booking vocabulary around them.
BASE_KEYTERMS = [
    "morning",
    "afternoon",
    "evening",
    "callback",
    "landmark",
    "flat",
    "apartment",
    "society",
    "sector",
    "nagar",
    "colony",
    "road",
    "street",
]


def _llms():
    candidates = []
    gemini_key = os.getenv("GEMINI_API_KEY")
    if gemini_key:
        from livekit.plugins import google

        options = {
            "model": os.getenv("GEMINI_MODEL", "gemini-3.5-flash"),
            "api_key": gemini_key,
        }
        level = os.getenv("GEMINI_THINKING_LEVEL", "minimal")
        budget = os.getenv("GEMINI_THINKING_BUDGET", "")
        if level:
            options["thinking_config"] = {"thinking_level": level}
        elif budget:
            options["thinking_config"] = {"thinking_budget": int(budget)}
        logger.info("LLM primary: google/%s thinking=%s", options["model"], level or budget)
        candidates.append(google.LLM(**options))
    if os.getenv("OPENAI_API_KEY"):
        logger.info("LLM fallback: openai/%s", os.getenv("OPENAI_MODEL", "gpt-4.1-mini"))
        candidates.append(openai.LLM(model=os.getenv("OPENAI_MODEL", "gpt-4.1-mini")))
        candidates.append(
            openai.LLM(model=os.getenv("OPENAI_FALLBACK_MODEL", "gpt-4.1-nano"))
        )
    if not candidates:
        raise RuntimeError("no LLM provider is configured")
    # Sticky keeps the primary until it actually fails, so a recovered primary
    # does not pull traffic back mid-call.
    return LLMFallback(candidates, attempt_timeout=4.0, sticky=True)


def _stt(cfg: SessionConfig):
    keyterms = []
    for term in [*cfg.services, cfg.tenant_name, *BASE_KEYTERMS]:
        term = term.strip().lower()
        if term and term not in keyterms:
            keyterms.append(term)

    primary = deepgram.STT(
        model=os.getenv("STT_MODEL", "nova-3"),
        language=os.getenv("STT_LANGUAGE", "en-IN"),
        keyterm=keyterms,
        smart_format=True,
        numerals=True,
        filler_words=False,
        endpointing_ms=int(os.getenv("STT_ENDPOINTING_MS", "200")),
    )
    fallback = deepgram.STT(
        model=os.getenv("STT_FALLBACK_MODEL", "nova-2-phonecall"),
        language=os.getenv("STT_LANGUAGE", "en-IN"),
        smart_format=True,
        numerals=True,
        filler_words=False,
        endpointing_ms=int(os.getenv("STT_ENDPOINTING_MS", "200")),
    )
    return STTFallback([primary, fallback], attempt_timeout=6.0, max_retry_per_stt=1)


def _tts():
    candidates = []
    if os.getenv("CARTESIA_API_KEY"):
        options = {"model": os.getenv("CARTESIA_MODEL", "sonic-3")}
        if os.getenv("CARTESIA_VOICE_ID"):
            options["voice"] = os.environ["CARTESIA_VOICE_ID"]
        # Cartesia has the lowest time-to-first-audio in our benchmark.
        candidates.append(cartesia.TTS(**options))
    eleven_key = os.getenv("ELEVEN_API_KEY") or os.getenv("ELEVENLABS_API_KEY")
    if eleven_key:
        options = {
            "model": os.getenv("ELEVEN_MODEL", "eleven_turbo_v2_5"),
            "api_key": eleven_key,
        }
        if os.getenv("ELEVEN_VOICE_ID"):
            options["voice_id"] = os.environ["ELEVEN_VOICE_ID"]
        candidates.append(elevenlabs.TTS(**options))
    if not candidates:
        raise RuntimeError("no TTS provider is configured")
    return TTSFallback(candidates, max_retry_per_tts=2)


def build_session(cfg: SessionConfig) -> AgentSession:
    return AgentSession(
        stt=_stt(cfg),
        llm=_llms(),
        tts=_tts(),
        vad=silero.VAD.load(),
        turn_handling=TurnHandlingOptions(
            turn_detection=inference.TurnDetector(),
            # Allow callers to pause while dictating an address or phone number
            # instead of treating every pause as the end of a turn.
            endpointing=EndpointingOptions(
                mode="fixed",
                min_delay=float(os.getenv("ENDPOINTING_MIN_DELAY", "0.35")),
                max_delay=float(os.getenv("ENDPOINTING_MAX_DELAY", "2.0")),
            ),
            # Start the LLM before the turn is fully committed, then discard if
            # the caller keeps talking. Hides 200-400 ms on most turns.
            preemptive_generation=PreemptiveGenerationOptions(
                enabled=os.getenv("PREEMPTIVE_GENERATION", "true").lower() == "true",
                preemptive_tts=True,
            ),
        ),
    )
