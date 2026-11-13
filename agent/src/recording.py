from __future__ import annotations

import logging
import os

from livekit import api

logger = logging.getLogger("switchboard.recording")


def recording_configured() -> bool:
    return bool(os.getenv("RECORDING_S3_BUCKET"))


async def start_consented_recording(ctx, call_id: str) -> str:
    """Start an audio-only, two-channel Egress recording after consent.

    Returns the Egress ID, or an empty string when storage is not configured or
    Egress fails. Nothing is recorded before this is called.
    """
    if not recording_configured():
        logger.warning("recording skipped: RECORDING_S3_BUCKET is not set")
        return ""

    request = api.StartEgressRequest(
        room_name=ctx.room.name,
        media=api.MediaSource(
            audio=api.AudioConfig(
                routes=[
                    api.AudioRoute(
                        participant_kind=api.ParticipantInfo.Kind.AGENT,
                        channel=api.AudioChannel.AUDIO_CHANNEL_LEFT,
                    ),
                    api.AudioRoute(
                        participant_kind=api.ParticipantInfo.Kind.STANDARD,
                        channel=api.AudioChannel.AUDIO_CHANNEL_RIGHT,
                    ),
                ]
            )
        ),
        outputs=[
            api.Output(
                file=api.FileOutput(
                    file_type=api.EncodedFileType.OGG,
                    filepath=f"recordings/{call_id}.ogg",
                )
            )
        ],
        storage=api.StorageConfig(
            s3=api.S3Upload(
                bucket=os.getenv("RECORDING_S3_BUCKET", ""),
                region=os.getenv("RECORDING_S3_REGION", "ap-south-1"),
                access_key=os.getenv("RECORDING_S3_ACCESS_KEY", ""),
                secret=os.getenv("RECORDING_S3_SECRET", ""),
                endpoint=os.getenv("RECORDING_S3_ENDPOINT", ""),
                force_path_style=os.getenv("RECORDING_S3_FORCE_PATH_STYLE", "false").lower()
                == "true",
            )
        ),
    )
    try:
        info = await ctx.api.egress.start_egress(request)
        logger.info("recording started", extra={"call_id": call_id, "egress_id": info.egress_id})
        return info.egress_id
    except Exception:
        logger.exception("recording failed to start", extra={"call_id": call_id})
        return ""


async def stop_recording(ctx, egress_id: str) -> bool:
    """Stop an active Egress job. Safe to call if it already ended."""
    if not egress_id:
        return True
    try:
        await ctx.api.egress.stop_egress(api.StopEgressRequest(egress_id=egress_id))
        logger.info("recording stopped", extra={"egress_id": egress_id})
        return True
    except Exception:
        logger.warning("recording stop failed", extra={"egress_id": egress_id}, exc_info=True)
        return False
