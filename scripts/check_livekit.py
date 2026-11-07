#!/usr/bin/env python3
"""Validate LiveKit Cloud credentials before starting the agent.

Reads LIVEKIT_URL, LIVEKIT_API_KEY, and LIVEKIT_API_SECRET from the
environment. Never prints the secret.
"""
from __future__ import annotations

import asyncio
import os
import sys

from livekit import api


async def main() -> int:
    url = os.getenv("LIVEKIT_URL", "")
    key = os.getenv("LIVEKIT_API_KEY", "")
    secret = os.getenv("LIVEKIT_API_SECRET", "")
    missing = [
        name
        for name, value in (
            ("LIVEKIT_URL", url),
            ("LIVEKIT_API_KEY", key),
            ("LIVEKIT_API_SECRET", secret),
        )
        if not value
    ]
    if missing:
        print("missing:", ", ".join(missing))
        return 2

    print(f"url:  {url}")
    print(f"key:  {key[:6]}…  secret length: {len(secret)}")
    try:
        async with api.LiveKitAPI() as lk:
            rooms = await lk.room.list_rooms(api.ListRoomsRequest())
        print(f"ok: credentials valid ({len(rooms.rooms)} active rooms)")
        return 0
    except Exception as exc:
        print(f"fail: {type(exc).__name__}: {exc}")
        print()
        print("Check that the key and secret belong to the project in LIVEKIT_URL")
        print("(LiveKit Cloud → Project → Settings → Keys), then update .env.")
        return 1


if __name__ == "__main__":
    sys.exit(asyncio.run(main()))
