-- Phase 2: recording lifecycle, consent evidence, and retention.
--
-- The object key is stored, never a public URL. Playback goes through a
-- short-lived presigned URL. Deleting the call cascades to its recording row;
-- the retention job deletes the S3 object and marks the row deleted.

CREATE TABLE IF NOT EXISTS call_recordings (
    call_id TEXT PRIMARY KEY REFERENCES calls (id) ON DELETE CASCADE,
    tenant_id TEXT NOT NULL,
    egress_id TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'started'
        CHECK (status IN ('started', 'completed', 'failed', 'aborted', 'deleted')),
    object_key TEXT NOT NULL DEFAULT '',
    error TEXT NOT NULL DEFAULT '',
    started_at TIMESTAMPTZ,
    ended_at TIMESTAMPTZ,
    duration_ms BIGINT NOT NULL DEFAULT 0,
    channels INTEGER NOT NULL DEFAULT 2,
    consent_at TIMESTAMPTZ,
    disclosure_version TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS call_recordings_egress
    ON call_recordings (egress_id);

CREATE INDEX IF NOT EXISTS call_recordings_tenant_ended
    ON call_recordings (tenant_id, ended_at DESC);
