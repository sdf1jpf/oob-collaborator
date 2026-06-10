CREATE TABLE ip_recon (
    ip VARCHAR(45) PRIMARY KEY,
    reverse_dns TEXT NOT NULL DEFAULT '',
    country VARCHAR(100) NOT NULL DEFAULT '',
    country_code VARCHAR(2) NOT NULL DEFAULT '',
    region VARCHAR(100) NOT NULL DEFAULT '',
    city VARCHAR(100) NOT NULL DEFAULT '',
    lat DOUBLE PRECISION,
    lon DOUBLE PRECISION,
    isp VARCHAR(255) NOT NULL DEFAULT '',
    org VARCHAR(255) NOT NULL DEFAULT '',
    asn VARCHAR(100) NOT NULL DEFAULT '',
    status VARCHAR(20) NOT NULL DEFAULT '',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_ip_recon_updated_at ON ip_recon (updated_at);
