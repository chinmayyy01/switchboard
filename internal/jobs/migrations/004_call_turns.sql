-- Phase 1: caller identity, separate consent, and durable transcript turns.
--
-- from_number/to_number keep the normalized E.164 values. The *_raw columns
-- keep what the carrier sent, for debugging and for numbers we cannot
-- normalize. Existing rows are left untouched.

ALTER TABLE calls ADD COLUMN IF NOT EXISTS from_number_raw TEXT NOT NULL DEFAULT '';
ALTER TABLE calls ADD COLUMN IF NOT EXISTS to_number_raw TEXT NOT NULL DEFAULT '';
ALTER TABLE calls ADD COLUMN IF NOT EXISTS sip_call_id TEXT NOT NULL DEFAULT '';
ALTER TABLE calls ADD COLUMN IF NOT EXISTS source TEXT NOT NULL DEFAULT '';
ALTER TABLE calls ADD COLUMN IF NOT EXISTS agent_version TEXT NOT NULL DEFAULT '';
ALTER TABLE calls ADD COLUMN IF NOT EXISTS transcript_consent BOOLEAN;
ALTER TABLE calls ADD COLUMN IF NOT EXISTS consent_version TEXT NOT NULL DEFAULT '';
ALTER TABLE calls ADD COLUMN IF NOT EXISTS consent_language TEXT NOT NULL DEFAULT '';
ALTER TABLE calls ADD COLUMN IF NOT EXISTS end_reason TEXT NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS calls_room_name
    ON calls (room_name);

CREATE TABLE IF NOT EXISTS call_turns (
    call_id TEXT NOT NULL REFERENCES calls (id) ON DELETE CASCADE,
    tenant_id TEXT NOT NULL,
    seq INTEGER NOT NULL,
    role TEXT NOT NULL CHECK (role IN ('user', 'assistant')),
    text TEXT NOT NULL,
    latency_ms INTEGER NOT NULL DEFAULT 0,
    tool_name TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (call_id, seq)
);

CREATE INDEX IF NOT EXISTS call_turns_tenant_created
    ON call_turns (tenant_id, created_at DESC);
