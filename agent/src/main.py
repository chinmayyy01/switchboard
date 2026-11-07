from __future__ import annotations

import json
import logging
import os

import aiohttp
from dotenv import load_dotenv
from livekit.agents import AgentServer, JobContext, UserInputTranscribedEvent, utils

from agent import Receptionist
from config import SessionConfig, session_config, settings
from goclient import GoClient
from session import build_session

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
    tenant_id = str(metadata.get("tenant_id") or "")
    if not tenant_id:
        logger.error("job metadata has no tenant_id; refusing to answer")
        await ctx.shutdown()
        return

    try:
        payload = await _bootstrap(metadata, ctx.room.name)
    except Exception:
        logger.exception("could not start session")
        await ctx.shutdown()
        return

    cfg: SessionConfig = session_config(payload, ctx.room.name)
    client = GoClient(settings().go_api_url, cfg.token)
    receptionist = Receptionist(cfg, client, ctx)
    session = build_session(cfg)

    # LiveKit Cloud audio and transcripts stay off. Recording only starts after
    # the caller explicitly consents, and goes to our own India bucket.
    await session.start(
        agent=receptionist,
        room=ctx.room,
        record={"audio": False, "transcript": False, "traces": True, "logs": True, "redaction": True},
    )

    @session.on("user_input_transcribed")
    def on_transcribed(ev: UserInputTranscribedEvent) -> None:
        if ev.is_final:
            logger.info(
                "caller said: %s",
                ev.transcript,
                extra={"call_id": cfg.call_id, "tenant": cfg.tenant_id},
            )

    if cfg.disclosure:
        session.say(cfg.disclosure, allow_interruptions=True)

    async def finish() -> None:
        try:
            await client.post("/internal/tools/end", {"outcome": "transferred" if receptionist.transferred else "completed"})
        except Exception:
            logger.warning("could not close call record", exc_info=True)

    ctx.add_shutdown_callback(finish)


if __name__ == "__main__":
    from livekit.agents import cli

    cli.run_app(server)
