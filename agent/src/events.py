from __future__ import annotations

import json
import logging
from typing import Any

logger = logging.getLogger("switchboard.events")

TOPIC = "switchboard.events"


class EventPublisher:
    """Publishes small JSON events for the console.

    These are UI events, not the source of truth: bookings and consent are
    already persisted by the Go API before they are emitted here.
    """

    def __init__(self, room) -> None:
        self._room = room

    async def emit(self, event_type: str, **fields: Any) -> None:
        payload = json.dumps({"type": event_type, **fields}, separators=(",", ":"))
        try:
            await self._room.local_participant.publish_data(
                payload.encode("utf-8"),
                reliable=True,
                topic=TOPIC,
            )
        except Exception:
            logger.warning("could not publish %s event", event_type, exc_info=True)
