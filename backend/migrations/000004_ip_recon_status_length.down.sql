UPDATE ip_recon SET status = LEFT(status, 20) WHERE LENGTH(status) > 20;
ALTER TABLE ip_recon ALTER COLUMN status TYPE VARCHAR(20);
