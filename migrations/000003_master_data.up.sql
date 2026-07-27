-- AIMOC - Migration 000003: Master Data

CREATE TABLE customers (
    id           UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    code         VARCHAR(30) UNIQUE NOT NULL,
    name         VARCHAR(180) NOT NULL,
    type         customer_type NOT NULL DEFAULT 'INDIVIDU',
    npwp         VARCHAR(30),
    phone        VARCHAR(25),
    email        VARCHAR(160),
    address      TEXT,
    city         VARCHAR(80),
    province     VARCHAR(80),
    notes        TEXT,
    status       general_status NOT NULL DEFAULT 'AKTIF',
    created_by   UUID REFERENCES users(id),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_customers_name_trgm ON customers USING gin (name gin_trgm_ops);
CREATE INDEX idx_customers_status ON customers(status);

ALTER TABLE users
    ADD CONSTRAINT fk_users_customer FOREIGN KEY (customer_id) REFERENCES customers(id) ON DELETE SET NULL;

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

CREATE TABLE vendors (
    id           UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    code         VARCHAR(30) UNIQUE NOT NULL,
    name         VARCHAR(180) NOT NULL,
    phone        VARCHAR(25),
    email        VARCHAR(160),
    address      TEXT,
    pic_name     VARCHAR(120),
    pic_phone    VARCHAR(25),
    status       general_status NOT NULL DEFAULT 'AKTIF',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE trucks (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    plate_number    VARCHAR(20) UNIQUE NOT NULL,
    vendor_id       UUID REFERENCES vendors(id) ON DELETE SET NULL,
    type            VARCHAR(50),
    brand           VARCHAR(80),
    capacity_ton    NUMERIC(10,2) NOT NULL DEFAULT 0,
    year            INT,
    status          general_status NOT NULL DEFAULT 'AKTIF',
    notes           TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_trucks_plate ON trucks(plate_number);
CREATE INDEX idx_trucks_vendor ON trucks(vendor_id);

CREATE TABLE drivers (
    id             UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name           VARCHAR(150) NOT NULL,
    phone          VARCHAR(25),
    license_no     VARCHAR(50),
    vendor_id      UUID REFERENCES vendors(id) ON DELETE SET NULL,
    photo_url      TEXT,
    status         general_status NOT NULL DEFAULT 'AKTIF',
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE cameras (
    id           UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    code         VARCHAR(30) UNIQUE NOT NULL,
    name         VARCHAR(120) NOT NULL,
    area         VARCHAR(50) NOT NULL,    -- GATE_IN | GATE_OUT | LOADING | EQUIPMENT
    ip_address   VARCHAR(60),
    stream_url   TEXT,
    location     VARCHAR(150),
    lat          NUMERIC(10,7),
    lng          NUMERIC(10,7),
    status       general_status NOT NULL DEFAULT 'AKTIF',
    last_seen_at TIMESTAMPTZ,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
