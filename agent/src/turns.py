from __future__ import annotations

import asyncio
import contextlib
import logging

from goclient import GoClient

logger = logging.getLogger("switchboard.turns")


class TurnRecorder:
    """Persists conversation turns incrementally through the Go API.

    The server is the consent authority: it rejects turns for calls without
    transcript consent. This class only buffers the first turn until the consent
    tool has run, so the caller's answer itself can be stored when allowed.
    """

    def __init__(self, client: GoClient, flush_timeout: float = 5.0) -> None:
        self._client = client
        self._seq = 0
        self._queue: asyncio.Queue[dict] = asyncio.Queue()
        self._consent_ready = asyncio.Event()
        self._flush_timeout = flush_timeout
        self._task = asyncio.create_task(self._run())

    def add(self, role: str, text: str, latency_ms: int = 0, tool_name: str = "") -> None:
        text = (text or "").strip()
        if not text:
            return
        self._seq += 1
        self._queue.put_nowait(
            {
                "seq": self._seq,
                "role": role,
                "text": text[:8000],
                "latency_ms": latency_ms,
                "tool_name": tool_name,
            }
        )

    def consent_set(self) -> None:
        self._consent_ready.set()

    async def _run(self) -> None:
        while True:
            item = await self._queue.get()
            try:
                await self._consent_ready.wait()
                for attempt in range(3):
                    try:
                        await self._client.post("/internal/tools/turn", item)
                        break
                    except Exception:
                        if attempt == 2:
                            logger.warning(
                                "dropping turn seq=%s after retries", item.get("seq"), exc_info=True
                            )
                        else:
                            await asyncio.sleep(0.15 * (attempt + 1))
            finally:
                self._queue.task_done()

    async def close(self) -> None:
        """Flush queued turns, then stop the worker. Safe to call twice."""
        self._consent_ready.set()
        try:
            await asyncio.wait_for(self._queue.join(), timeout=self._flush_timeout)
        except asyncio.TimeoutError:
            logger.warning("turn flush timed out")
        self._task.cancel()
        with contextlib.suppress(asyncio.CancelledError):
            await self._task
