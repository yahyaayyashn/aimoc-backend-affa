-- AIMOC - Migration 000005: Gate Logs, Loading Logs, Surat Jalan, Invoice

CREATE TABLE gate_logs (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    order_id        UUID REFERENCES orders(id) ON DELETE SET NULL,
    truck_id        UUID REFERENCES trucks(id) ON DELETE SET NULL,
    plate_number    VARCHAR(20) NOT NULL,
    type            gate_type NOT NULL,
    camera_id       UUID REFERENCES cameras(id) ON DELETE SET NULL,
    snapshot_url    TEXT,
    confidence      NUMERIC(5,4),
    is_valid        BOOLEAN NOT NULL DEFAULT TRUE,
    alert_reason    TEXT,
    validated_by    UUID REFERENCES users(id),
    validated_at    TIMESTAMPTZ,
    ts              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_gate_logs_order ON gate_logs(order_id);
CREATE INDEX idx_gate_logs_plate ON gate_logs(plate_number);
CREATE INDEX idx_gate_logs_ts ON gate_logs(ts);
CREATE INDEX idx_gate_logs_type ON gate_logs(type);

CREATE TABLE loading_logs (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    order_id        UUID REFERENCES orders(id) ON DELETE SET NULL,
    truck_id        UUID REFERENCES trucks(id) ON DELETE SET NULL,
    plate_number    VARCHAR(20) NOT NULL,
    camera_id       UUID REFERENCES cameras(id) ON DELETE SET NULL,
    jenis_muatan    VARCHAR(80),
    est_tonase      NUMERIC(15,2),
    snapshot_url    TEXT,
    start_ts        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    end_ts          TIMESTAMPTZ,
    duration_sec    INT,
    is_valid        BOOLEAN NOT NULL DEFAULT TRUE,
    alert_reason    TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_loading_logs_order ON loading_logs(order_id);
CREATE INDEX idx_loading_logs_start ON loading_logs(start_ts);
CREATE INDEX idx_loading_logs_plate ON loading_logs(plate_number);

CREATE TABLE surat_jalan (
    id                  UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    sj_number           VARCHAR(40) UNIQUE NOT NULL,
    order_id            UUID NOT NULL REFERENCES orders(id),
    gate_out_log_id     UUID REFERENCES gate_logs(id) ON DELETE SET NULL,
    pdf_url             TEXT,
    signature_url       TEXT,
    signed_at           TIMESTAMPTZ,
    signed_by           UUID REFERENCES users(id),
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_sj_order ON surat_jalan(order_id);

CREATE TABLE invoices (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    invoice_no      VARCHAR(40) UNIQUE NOT NULL,
    order_id        UUID NOT NULL REFERENCES orders(id),
    customer_id     UUID NOT NULL REFERENCES customers(id),
    amount          NUMERIC(15,2) NOT NULL,
    tax             NUMERIC(15,2) NOT NULL DEFAULT 0,
    total           NUMERIC(15,2) NOT NULL,
    due_date        DATE,
    status          payment_status NOT NULL DEFAULT 'UNPAID',
    pdf_url         TEXT,
    paid_at         TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_invoices_order ON invoices(order_id);
CREATE INDEX idx_invoices_customer ON invoices(customer_id);
CREATE INDEX idx_invoices_status ON invoices(status);
