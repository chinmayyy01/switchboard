from __future__ import annotations

import logging
import time

import aiohttp
from livekit.agents import utils
from livekit.agents.llm import ToolError

logger = logging.getLogger("switchboard.tools")


class GoClient:
    """Calls the Go API with the per-call token minted at session start.

    Tool failures become ToolError so the model can tell the caller what
    happened instead of crashing the pipeline.
    """

    def __init__(self, base_url: str, token: str, timeout_seconds: float = 3.0) -> None:
        self._base_url = base_url.rstrip("/")
        self._token = token
        self._timeout = aiohttp.ClientTimeout(total=timeout_seconds)

    async def post(self, path: str, payload: dict) -> dict:
        started = time.perf_counter()
        headers = {"Authorization": f"Bearer {self._token}"}
        session = utils.http_context.http_session()
        try:
            async with session.post(
                f"{self._base_url}{path}",
                json=payload,
                headers=headers,
                timeout=self._timeout,
            ) as response:
                if response.status != 200:
                    raise ToolError("The booking system is unavailable right now.")
                data = await response.json()
        except aiohttp.ClientError as exc:
            raise ToolError("I can't reach the booking system right now.") from exc

        logger.info("tool %s %.0fms", path, (time.perf_counter() - started) * 1000)
        if not data.get("ok"):
            raise ToolError(str(data.get("message") or "I couldn't complete that."))
        return data
