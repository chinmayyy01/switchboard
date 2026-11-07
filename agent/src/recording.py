from __future__ import annotations

import logging
import os

from livekit import api

logger = logging.getLogger("switchboard.recording")


def recording_configured() -> bool:
    return bool(os.getenv("RECORDING_S3_BUCKET"))


async def start_consented_recording(ctx, call_id: str) -> bool:
    """Start an audio-only, two-channel Egress recording after consent.

    Nothing is recorded before this is called. If storage is not configured we
    log and skip rather than silently recording somewhere unexpected.
    """
    if not recording_configured():
        logger.warning("recording skipped: RECORDING_S3_BUCKET is not set")
        return False

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
        return True
    except Exception:
        logger.exception("recording failed to start", extra={"call_id": call_id})
        return False
