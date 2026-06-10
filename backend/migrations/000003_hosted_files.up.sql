CREATE TABLE hosted_files (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    engagement_id UUID NOT NULL REFERENCES engagements(id) ON DELETE CASCADE,
    path          VARCHAR(512) NOT NULL,
    content_type  VARCHAR(128) NOT NULL,
    content       BYTEA NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (engagement_id, path)
);

CREATE INDEX idx_hosted_files_engagement ON hosted_files (engagement_id);
