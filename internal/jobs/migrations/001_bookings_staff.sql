CREATE TABLE employees (
    tenant_id TEXT NOT NULL,
    id TEXT NOT NULL,
    name TEXT NOT NULL,
    role TEXT NOT NULL CHECK (role IN ('technician', 'dispatcher', 'manager', 'other')),
    email TEXT NOT NULL DEFAULT '',
    phone TEXT NOT NULL DEFAULT '',
    skills JSONB NOT NULL DEFAULT '[]'::jsonb,
    days JSONB NOT NULL DEFAULT '[]'::jsonb,
    shifts JSONB NOT NULL DEFAULT '[]'::jsonb,
    active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (tenant_id, id)
);

CREATE UNIQUE INDEX employees_tenant_email_unique
    ON employees (tenant_id, LOWER(email))
    WHERE email <> '';

CREATE TABLE jobs (
    id TEXT PRIMARY KEY,
    tenant_id TEXT NOT NULL,
    idempotency_key TEXT NOT NULL,
    service TEXT NOT NULL,
    address TEXT NOT NULL,
    service_date DATE NOT NULL,
    time_window TEXT NOT NULL CHECK (time_window IN ('morning', 'afternoon', 'evening')),
    phone TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL CHECK (status IN ('booked', 'cancelled')),
    employee_id TEXT,
    employee_name TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT jobs_employee_fk
        FOREIGN KEY (tenant_id, employee_id)
        REFERENCES employees (tenant_id, id)
        ON DELETE RESTRICT,
    CONSTRAINT jobs_tenant_idempotency_unique
        UNIQUE (tenant_id, idempotency_key)
);

CREATE UNIQUE INDEX jobs_employee_slot_unique
    ON jobs (tenant_id, employee_id, service_date, time_window)
    WHERE status = 'booked' AND employee_id IS NOT NULL;

CREATE INDEX jobs_phone_lookup
    ON jobs (tenant_id, phone, service_date DESC)
    WHERE status = 'booked';

CREATE INDEX jobs_tenant_schedule
    ON jobs (tenant_id, service_date, time_window)
    WHERE status = 'booked';
