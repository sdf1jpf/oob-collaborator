CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE engagements (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    client_name VARCHAR(255) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE payloads (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    engagement_id UUID NOT NULL REFERENCES engagements(id) ON DELETE CASCADE,
    sub_domain VARCHAR(64) NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT payloads_sub_domain_unique UNIQUE (sub_domain)
);

CREATE INDEX idx_payloads_sub_domain ON payloads (sub_domain);
CREATE INDEX idx_payloads_engagement_id ON payloads (engagement_id);

CREATE TABLE interactions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    payload_id UUID REFERENCES payloads(id) ON DELETE SET NULL,
    protocol VARCHAR(10) NOT NULL,
    source_ip VARCHAR(45) NOT NULL,
    raw_data TEXT NOT NULL,
    interacted_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    delivered_at TIMESTAMPTZ,
    CONSTRAINT interactions_protocol_check CHECK (protocol IN ('HTTP', 'DNS', 'SMTP'))
);

CREATE INDEX idx_interactions_payload_id ON interactions (payload_id);
CREATE INDEX idx_interactions_interacted_at ON interactions (interacted_at DESC);
CREATE INDEX idx_interactions_delivered_at ON interactions (delivered_at) WHERE delivered_at IS NULL;
