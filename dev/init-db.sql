-- Platform control-plane schema — Phase 0
-- These are the tables the API service reads/writes directly.
-- Neon-backed tenant databases are separate and managed by storage-cp.

CREATE EXTENSION IF NOT EXISTS "pgcrypto";
CREATE EXTENSION IF NOT EXISTS "pg_stat_statements";

-- ── Core domain tables ───────────────────────────────────────────────────────

CREATE TABLE orgs (
    id           TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
    slug         TEXT UNIQUE NOT NULL,
    display_name TEXT NOT NULL,
    plan_tier    TEXT NOT NULL DEFAULT 'shared',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE teams (
    id      TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
    org_id  TEXT NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    slug    TEXT NOT NULL,
    UNIQUE (org_id, slug)
);

CREATE TABLE projects (
    id         TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
    org_id     TEXT NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    team_id    TEXT REFERENCES teams(id),
    slug       TEXT NOT NULL,
    region_id  TEXT NOT NULL DEFAULT 'in-mum-1',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (org_id, slug)
);

CREATE TABLE environments (
    id         TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name       TEXT NOT NULL,
    is_preview BOOLEAN NOT NULL DEFAULT false,
    UNIQUE (project_id, name)
);

CREATE TABLE services (
    id           TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
    project_id   TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name         TEXT NOT NULL,
    type         TEXT NOT NULL DEFAULT 'app',
    build_config JSONB NOT NULL DEFAULT '{}',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (project_id, name)
);

CREATE TABLE deployments (
    id          TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
    service_id  TEXT NOT NULL REFERENCES services(id),
    env_id      TEXT NOT NULL REFERENCES environments(id),
    image       TEXT NOT NULL,
    status      TEXT NOT NULL DEFAULT 'pending',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    finished_at TIMESTAMPTZ
);

CREATE INDEX ON deployments (service_id, created_at DESC);

-- ── Metering / billing ledger ─────────────────────────────────────────────────

CREATE TABLE metering_events (
    id           BIGSERIAL PRIMARY KEY,
    org_id       TEXT NOT NULL,
    service_id   TEXT,
    event_type   TEXT NOT NULL,   -- compute_seconds | ram_gb_hours | egress_gb | ...
    quantity     NUMERIC NOT NULL,
    unit         TEXT NOT NULL,
    occurred_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    lago_tx_id   TEXT          -- foreign key to Lago event ID once ingested
);

CREATE INDEX ON metering_events (org_id, occurred_at DESC);

-- ── Regions & jurisdiction profiles ──────────────────────────────────────────

CREATE TABLE regions (
    id                    TEXT PRIMARY KEY,     -- e.g. "in-mum-1"
    display_name          TEXT NOT NULL,
    country_code          TEXT NOT NULL,        -- ISO 3166-1 alpha-2
    jurisdiction_profile  JSONB NOT NULL,       -- see domain.JurisdictionProfile
    status                TEXT NOT NULL DEFAULT 'active'
);

INSERT INTO regions (id, display_name, country_code, jurisdiction_profile) VALUES
('in-mum-1', 'India — Mumbai (Primary)', 'IN', '{
  "residency_rule": "hard",
  "transfer_mechanism": "adequacy",
  "breach_authority": "DPB",
  "breach_window_hrs": 72,
  "payment_gateway": "razorpay",
  "currency": "INR",
  "tax_model": "gst",
  "data_class_pinning": [
    {"class": "payment",   "residency": "hard"},
    {"class": "personal",  "residency": "hard"},
    {"class": "sensitive", "residency": "hard"},
    {"class": "telemetry", "residency": "none"}
  ]
}'::jsonb);
