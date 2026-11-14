-- Phase 4: per-tenant usage ledger and monthly budgets.
--
-- Costs are stored in minor currency units (paise for INR) and computed
-- server-side. The agent reports units, never a price.

CREATE TABLE IF NOT EXISTS tenant_budgets (
    tenant_id TEXT PRIMARY KEY,
    cap_minor BIGINT NOT NULL DEFAULT 0,
    currency TEXT NOT NULL DEFAULT 'INR',
    action TEXT NOT NULL DEFAULT 'transfer'
        CHECK (action IN ('transfer', 'refuse', 'degrade')),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS tenant_usage (
    id BIGSERIAL PRIMARY KEY,
    tenant_id TEXT NOT NULL,
    call_id TEXT NOT NULL,
    period TEXT NOT NULL,
    kind TEXT NOT NULL,
    model TEXT NOT NULL DEFAULT '',
    units BIGINT NOT NULL,
    cost_minor BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (call_id, kind, model)
);

CREATE INDEX IF NOT EXISTS tenant_usage_period
    ON tenant_usage (tenant_id, period);

CREATE INDEX IF NOT EXISTS tenant_usage_call
    ON tenant_usage (call_id);
