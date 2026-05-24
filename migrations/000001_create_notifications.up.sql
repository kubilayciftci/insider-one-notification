CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE notifications (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    batch_id       UUID,
    idempotency_key VARCHAR(255),
    recipient      VARCHAR(255) NOT NULL,
    channel        VARCHAR(20)  NOT NULL,
    content        TEXT         NOT NULL,
    priority       VARCHAR(10)  NOT NULL DEFAULT 'normal',
    status         VARCHAR(20)  NOT NULL DEFAULT 'pending',
    retry_count    INTEGER      NOT NULL DEFAULT 0,
    max_retries    INTEGER      NOT NULL DEFAULT 3,
    scheduled_at   TIMESTAMPTZ,
    payload        JSONB        NOT NULL DEFAULT '{}',
    created_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX idx_notifications_idempotency_key
    ON notifications (idempotency_key) WHERE idempotency_key IS NOT NULL AND idempotency_key != '';

CREATE INDEX idx_notifications_status     ON notifications (status);
CREATE INDEX idx_notifications_channel    ON notifications (channel);
CREATE INDEX idx_notifications_batch_id   ON notifications (batch_id) WHERE batch_id IS NOT NULL;
CREATE INDEX idx_notifications_priority   ON notifications (priority);
CREATE INDEX idx_notifications_created_at ON notifications (created_at);
CREATE INDEX idx_notifications_scheduled  ON notifications (scheduled_at) WHERE scheduled_at IS NOT NULL;

CREATE OR REPLACE FUNCTION update_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_notifications_updated_at
    BEFORE UPDATE ON notifications
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at();
