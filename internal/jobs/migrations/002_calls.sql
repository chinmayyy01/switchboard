CREATE TABLE IF NOT EXISTS calls (
    id TEXT PRIMARY KEY,
    tenant_id TEXT NOT NULL,
    direction TEXT NOT NULL DEFAULT 'inbound',
    from_number TEXT NOT NULL DEFAULT '',
    to_number TEXT NOT NULL DEFAULT '',
    room_name TEXT NOT NULL DEFAULT '',
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    ended_at TIMESTAMPTZ,
    recording_consent BOOLEAN,
    consent_at TIMESTAMPTZ,
    outcome TEXT NOT NULL DEFAULT '',
    escalated BOOLEAN NOT NULL DEFAULT FALSE,
    escalation_reason TEXT NOT NULL DEFAULT '',
    manager_number TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS calls_tenant_started
    ON calls (tenant_id, started_at DESC);

CREATE INDEX IF NOT EXISTS calls_open
    ON calls (tenant_id, ended_at)
    WHERE ended_at IS NULL;
