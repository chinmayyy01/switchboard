from __future__ import annotations

import asyncio
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
        tool = path.rstrip("/").rsplit("/", 1)[-1] or "unknown"
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
                    asyncio.create_task(
                        self.report_metric("tool_error", tool=tool, code=f"http_{response.status}")
                    )
                    raise ToolError("The booking system is unavailable right now.")
                data = await response.json()
        except aiohttp.ClientError as exc:
            asyncio.create_task(self.report_metric("tool_error", tool=tool, code="network"))
            raise ToolError("I can't reach the booking system right now.") from exc

        logger.info("tool %s %.0fms", path, (time.perf_counter() - started) * 1000)
        if not data.get("ok"):
            asyncio.create_task(
                self.report_metric("tool_error", tool=tool, code=str(data.get("code") or "tool_error"))
            )
            raise ToolError(str(data.get("message") or "I couldn't complete that."))
        return data

    async def report_metric(self, event: str, **fields) -> None:
        """Best-effort metric event. Never affects the call."""
        payload = {"event": event, **fields}
        try:
            session = utils.http_context.http_session()
            async with session.post(
                f"{self._base_url}/internal/metrics/agent",
                json=payload,
                headers={"Authorization": f"Bearer {self._token}"},
                timeout=aiohttp.ClientTimeout(total=2),
            ):
                pass
        except Exception:
            logger.debug("metric report failed: %s", event, exc_info=True)
