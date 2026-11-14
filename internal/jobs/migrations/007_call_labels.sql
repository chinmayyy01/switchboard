-- Phase 6: call labels move from data/labels.json to Postgres.

CREATE TABLE IF NOT EXISTS call_labels (
    call_id TEXT PRIMARY KEY REFERENCES calls (id) ON DELETE CASCADE,
    tenant_id TEXT NOT NULL,
    label BOOLEAN NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS call_labels_tenant
    ON call_labels (tenant_id, updated_at DESC);
