from __future__ import annotations

import asyncio
import json
import logging
import os
import time

import aiohttp
from dotenv import load_dotenv
from livekit import rtc
from livekit.agents import AgentServer, JobContext, UserInputTranscribedEvent, utils

from agent import Receptionist
from config import SessionConfig, session_config, settings
from goclient import GoClient
from recording import stop_recording
from session import build_session
from turns import TurnRecorder

load_dotenv()
logging.basicConfig(level=os.getenv("LOG_LEVEL", "INFO"))
logger = logging.getLogger("switchboard.main")

AGENT_NAME = os.getenv("LIVEKIT_AGENT_NAME", "switchboard-receptionist")
server = AgentServer()


def _job_metadata(ctx: JobContext) -> dict:
    try:
        return json.loads(ctx.job.metadata or "{}")
    except json.JSONDecodeError:
        return {}


async def _bootstrap(metadata: dict, room_name: str) -> dict:
    cfg = settings()
    if not cfg.agent_api_key:
        raise RuntimeError("AGENT_API_KEY is not set")
    payload = {
        "tenant_id": metadata.get("tenant_id", ""),
        "call_id": metadata.get("call_id") or room_name,
        "direction": metadata.get("direction", "inbound"),
        "from_number": metadata.get("from_number", ""),
        "to_number": metadata.get("to_number", ""),
        "sip_call_id": metadata.get("sip_call_id", ""),
        "source": metadata.get("source", ""),
        "provider": metadata.get("provider", ""),
        "agent_version": os.getenv("AGENT_VERSION", ""),
        "room_name": room_name,
    }
    session = utils.http_context.http_session()
    async with session.post(
        f"{cfg.go_api_url}/internal/agent/session",
        json=payload,
        headers={"X-Agent-Key": cfg.agent_api_key},
        timeout=aiohttp.ClientTimeout(total=5),
    ) as response:
        if response.status != 200:
            body = await response.text()
            raise RuntimeError(f"agent session failed ({response.status}): {body}")
        return await response.json()


@server.rtc_session(agent_name=AGENT_NAME)
async def entrypoint(ctx: JobContext) -> None:
    await ctx.connect()
    metadata = _job_metadata(ctx)

    # The dispatch metadata does not carry the caller's number. Read it from the
    # SIP participant once it is actually in the room, otherwise the bootstrap
    # would race the caller and store an empty from_number.
    participant = await ctx.wait_for_participant()
    attributes = participant.attributes or {}
    if participant.kind == rtc.ParticipantKind.PARTICIPANT_KIND_SIP:
        metadata["source"] = "sip"
        metadata["from_number"] = attributes.get("sip.phoneNumber") or metadata.get("from_number", "")
        metadata["to_number"] = attributes.get("sip.trunkPhoneNumber") or metadata.get("to_number", "")
        metadata["sip_call_id"] = attributes.get("sip.callID") or metadata.get("sip_call_id", "")
    else:
        metadata.setdefault("source", "browser")

    tenant_id = str(metadata.get("tenant_id") or "")
    if not tenant_id and os.getenv("ALLOW_DEV_TENANT", "false").lower() == "true":
        # Lets the LiveKit Agent Console start a session without our token
        # endpoint. Keep this off in production.
        tenant_id = os.getenv("DEV_TENANT", "rivera")
        metadata["tenant_id"] = tenant_id
    if not tenant_id:
        logger.error("job metadata has no tenant_id; refusing to answer")
        ctx.shutdown()
        return

    try:
        payload = await _bootstrap(metadata, ctx.room.name)
    except Exception:
        logger.exception("could not start session")
        ctx.shutdown()
        return

    cfg: SessionConfig = session_config(payload, ctx.room.name)
    client = GoClient(settings().go_api_url, cfg.token)
    turns = TurnRecorder(client)
    receptionist = Receptionist(cfg, client, ctx, turns=turns)
    session = build_session(cfg)

    def watch_fallbacks() -> None:
        def watch(stage: str, adapter, event_name: str, attr: str) -> None:
            if adapter is None or not hasattr(adapter, "on"):
                return
            def handler(ev) -> None:
                if getattr(ev, "available", True):
                    return
                provider = getattr(getattr(ev, attr, None), "label", "")
                asyncio.create_task(
                    client.report_metric("fallback", stage=stage, provider=str(provider))
                )
            try:
                adapter.on(event_name, handler)
            except Exception:
                logger.debug("could not watch %s fallbacks", stage, exc_info=True)
        watch("stt", session.stt, "stt_availability_changed", "stt")
        watch("llm", session.llm, "llm_availability_changed", "llm")
        watch("tts", session.tts, "tts_availability_changed", "tts")

    watch_fallbacks()

    # LiveKit Cloud audio and transcripts stay off. Recording only starts after
    # the caller explicitly consents, and goes to our own India bucket.
    started_at = time.monotonic()
    await session.start(
        agent=receptionist,
        room=ctx.room,
        record={"audio": False, "transcript": False, "traces": True, "logs": True, "redaction": True},
    )

    max_task = None
    max_seconds = int(os.getenv("MAX_CALL_SECONDS", "900"))
    if max_seconds > 0:
        async def _max_duration() -> None:
            await asyncio.sleep(max_seconds)
            logger.warning("max call duration reached", extra={"call_id": cfg.call_id, "seconds": max_seconds})
            ctx.shutdown()
        max_task = asyncio.create_task(_max_duration())

    @ctx.room.on("participant_connected")
    def on_participant(participant) -> None:
        if participant.identity.startswith("supervisor-"):
            logger.info("supervisor joined; handing off", extra={"call_id": cfg.call_id})
            receptionist.transferred = True
            ctx.shutdown()

    @session.on("conversation_item_added")
    def on_item(ev) -> None:
        item = ev.item
        metrics = getattr(item, "metrics", None) or {}
        role = getattr(item, "role", "")

        def observe(stage: str, key: str) -> None:
            value = metrics.get(key)
            if value is not None:
                asyncio.create_task(
                    client.report_metric("turn", stage=stage, ms=int(float(value) * 1000))
                )

        if role == "user" and metrics.get("end_of_turn_delay") is not None:
            logger.info(
                "latency.user eou=%.2fs stt=%.2fs",
                metrics.get("end_of_turn_delay", 0),
                metrics.get("transcription_delay", 0),
            )
            observe("eou", "end_of_turn_delay")
            observe("stt", "transcription_delay")
        if role == "assistant" and metrics.get("e2e_latency") is not None:
            logger.info(
                "latency.assistant e2e=%.2fs llm_ttft=%.2fs tts_ttfb=%.2fs",
                metrics.get("e2e_latency", 0),
                metrics.get("llm_node_ttft", 0),
                metrics.get("tts_node_ttfb", 0),
            )
            observe("e2e", "e2e_latency")
            observe("llm_ttft", "llm_node_ttft")
            observe("tts_ttfb", "tts_node_ttfb")
        if getattr(item, "type", "") == "message" and role in ("user", "assistant"):
            text = getattr(item, "text_content", None)
            if text:
                turns.add(role, text)

    @session.on("user_input_transcribed")
    def on_transcribed(ev: UserInputTranscribedEvent) -> None:
        if not ev.is_final:
            return
        if receptionist.transcript_allowed and os.getenv("TRANSCRIPT_DEBUG", "false").lower() == "true":
            logger.info(
                "caller said: %s",
                ev.transcript,
                extra={"call_id": cfg.call_id, "tenant": cfg.tenant_id},
            )
        else:
            logger.info(
                "caller transcript",
                extra={
                    "call_id": cfg.call_id,
                    "tenant": cfg.tenant_id,
                    "chars": len(ev.transcript),
                    "lang": str(ev.language or ""),
                },
            )

    if cfg.disclosure:
        session.say(cfg.disclosure, allow_interruptions=True)

    async def report_usage() -> None:
        try:
            items: list[dict] = []
            for usage in session.usage.model_usage:
                data = usage.model_dump()
                model = f"{data.get('provider', '')}/{data.get('model', '')}".strip("/")
                kind = data.get("type")
                if kind == "llm_usage":
                    items.append({"kind": "llm_input", "model": model, "units": int(data.get("input_tokens", 0))})
                    items.append({"kind": "llm_output", "model": model, "units": int(data.get("output_tokens", 0))})
                elif kind == "tts_usage":
                    items.append({"kind": "tts_characters", "model": model, "units": int(data.get("characters_count", 0))})
                elif kind == "stt_usage":
                    items.append({"kind": "stt_seconds", "model": model, "units": int(round(data.get("audio_duration", 0)))})
            duration = max(1, int(time.monotonic() - started_at))
            items.append({"kind": "call_seconds", "model": "default", "units": duration})
            await client.post("/internal/tools/usage", {"items": items})
        except Exception:
            logger.warning("could not report usage", exc_info=True)

    async def finish() -> None:
        if max_task is not None:
            max_task.cancel()
        if receptionist.egress_id:
            stopped = await stop_recording(ctx, receptionist.egress_id)
            if not stopped:
                asyncio.create_task(client.report_metric("recording_failure"))
            try:
                await client.post(
                    "/internal/tools/recording/stopped",
                    {
                        "egress_id": receptionist.egress_id,
                        "error": "" if stopped else "stop failed",
                    },
                )
            except Exception:
                logger.warning("could not persist recording stop", exc_info=True)
        await turns.close()
        await report_usage()
        try:
            await client.post("/internal/tools/end", {"outcome": "transferred" if receptionist.transferred else "completed"})
        except Exception:
            logger.warning("could not close call record", exc_info=True)

    ctx.add_shutdown_callback(finish)


if __name__ == "__main__":
    from livekit.agents import cli

    cli.run_app(server)
