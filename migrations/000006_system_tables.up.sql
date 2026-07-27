-- AIMOC - Migration 000006: System Tables (alerts, notifications, audit, settings)

CREATE TABLE alerts (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    type            VARCHAR(50) NOT NULL,
    severity        alert_severity NOT NULL DEFAULT 'INFO',
    title           VARCHAR(200) NOT NULL,
    message         TEXT NOT NULL,
    related_table   VARCHAR(50),
    related_id      UUID,
    snapshot_url    TEXT,
    is_read         BOOLEAN NOT NULL DEFAULT FALSE,
    acknowledged_by UUID REFERENCES users(id),
    acknowledged_at TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_alerts_severity ON alerts(severity);
CREATE INDEX idx_alerts_created ON alerts(created_at);

CREATE TABLE notifications (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    type        notif_type NOT NULL,
    title       VARCHAR(200) NOT NULL,
    body        TEXT,
    data_jsonb  JSONB,
    is_read     BOOLEAN NOT NULL DEFAULT FALSE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_notifications_user ON notifications(user_id);
CREATE INDEX idx_notifications_is_read ON notifications(is_read);

CREATE TABLE audit_logs (
    id              BIGSERIAL PRIMARY KEY,
    user_id         UUID REFERENCES users(id) ON DELETE SET NULL,
    action          VARCHAR(80) NOT NULL,
    entity          VARCHAR(80) NOT NULL,
    entity_id       VARCHAR(80),
    old_value       JSONB,
    new_value       JSONB,
    ip              VARCHAR(64),
    user_agent      TEXT,
    ts              TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_audit_user ON audit_logs(user_id);
CREATE INDEX idx_audit_ts ON audit_logs(ts);
CREATE INDEX idx_audit_entity ON audit_logs(entity, entity_id);

CREATE TABLE system_settings (
    key         VARCHAR(80) PRIMARY KEY,
    value_jsonb JSONB NOT NULL,
    description TEXT,
    updated_by  UUID REFERENCES users(id),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE shifts (
    id           UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name         VARCHAR(60) NOT NULL,
    start_time   TIME NOT NULL,
    end_time     TIME NOT NULL,
    is_active    BOOLEAN NOT NULL DEFAULT TRUE
);
