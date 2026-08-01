-- Migration 000027: Material dikembalikan sebagai master data (permintaan user 01 Agu 2026).
-- Skema sama persis dengan yang di-drop di 000017 (lihat 000003_master_data.up.sql).

CREATE TABLE materials (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    code            VARCHAR(30) UNIQUE NOT NULL,
    name            VARCHAR(150) NOT NULL,
    unit            VARCHAR(20) NOT NULL DEFAULT 'TON',
    price_per_unit  NUMERIC(15,2) NOT NULL DEFAULT 0,
    stock           NUMERIC(15,2) NOT NULL DEFAULT 0,
    min_stock       NUMERIC(15,2) NOT NULL DEFAULT 0,
    description     TEXT,
    image_url       TEXT,
    status          general_status NOT NULL DEFAULT 'AKTIF',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_materials_name_trgm ON materials USING gin (name gin_trgm_ops);
