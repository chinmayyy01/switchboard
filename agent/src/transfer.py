from __future__ import annotations

import logging
import os

from livekit.agents.beta.workflows import WarmTransferTask

logger = logging.getLogger("switchboard.transfer")

SUMMARY_INSTRUCTIONS = (
    "You are briefing a human teammate who is taking over a home-services call. "
    "In under 20 seconds, say who is calling, what they need, the address if known, "
    "the requested visit window, and anything already promised. Do not repeat the "
    "whole transcript."
)


async def warm_transfer(agent, manager_number: str):
    """Dial the manager, brief them, then merge the caller. Returns a result.

    Requires LIVEKIT_SIP_OUTBOUND_TRUNK and LIVEKIT_SIP_NUMBER. If they are not
    configured we raise so the caller can fall back to a browser supervisor
    instead of being dropped.
    """
    trunk_id = os.getenv("LIVEKIT_SIP_OUTBOUND_TRUNK", "")
    sip_number = os.getenv("LIVEKIT_SIP_NUMBER", "")
    if not trunk_id or not sip_number:
        raise RuntimeError("outbound SIP trunk is not configured")
    if not manager_number:
        raise RuntimeError("no manager number is configured for this tenant")

    logger.info("starting warm transfer", extra={"manager": manager_number})
    return await WarmTransferTask(
        sip_call_to=manager_number,
        sip_trunk_id=trunk_id,
        sip_number=sip_number,
        chat_ctx=agent.chat_ctx,
        ringing_timeout=30.0,
        extra_instructions=SUMMARY_INSTRUCTIONS,
    )
