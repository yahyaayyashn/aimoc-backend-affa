-- AIMOC - Migration 000004: Orders & Payments

CREATE TABLE orders (
    id                  UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    order_no            VARCHAR(40) UNIQUE NOT NULL,
    customer_id         UUID NOT NULL REFERENCES customers(id),
    vendor_id           UUID REFERENCES vendors(id) ON DELETE SET NULL,
    truck_id            UUID REFERENCES trucks(id) ON DELETE SET NULL,
    driver_id           UUID REFERENCES drivers(id) ON DELETE SET NULL,
    plate_number        VARCHAR(20),
    delivery_address    TEXT,
    delivery_lat        NUMERIC(10,7),
    delivery_lng        NUMERIC(10,7),
    status              order_status NOT NULL DEFAULT 'NEW',
    payment_status      payment_status NOT NULL DEFAULT 'UNPAID',
    qr_token            VARCHAR(80) UNIQUE NOT NULL,
    target_ton          NUMERIC(15,2) NOT NULL DEFAULT 0,
    actual_ton          NUMERIC(15,2) NOT NULL DEFAULT 0,
    subtotal            NUMERIC(15,2) NOT NULL DEFAULT 0,
    tax                 NUMERIC(15,2) NOT NULL DEFAULT 0,
    discount            NUMERIC(15,2) NOT NULL DEFAULT 0,
    total               NUMERIC(15,2) NOT NULL DEFAULT 0,
    notes               TEXT,
    cancel_reason       TEXT,
    created_by          UUID REFERENCES users(id),
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expired_at          TIMESTAMPTZ
);
CREATE INDEX idx_orders_status ON orders(status);
CREATE INDEX idx_orders_customer ON orders(customer_id);
CREATE INDEX idx_orders_plate ON orders(plate_number);
CREATE INDEX idx_orders_created ON orders(created_at);

CREATE TABLE order_items (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    order_id        UUID NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    material_id     UUID NOT NULL REFERENCES materials(id),
    ton             NUMERIC(15,2) NOT NULL,
    price_per_unit  NUMERIC(15,2) NOT NULL,
    subtotal        NUMERIC(15,2) NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_order_items_order ON order_items(order_id);

CREATE TABLE payments (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    order_id        UUID NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    method          payment_method NOT NULL,
    amount          NUMERIC(15,2) NOT NULL,
    status          payment_status NOT NULL DEFAULT 'PENDING',
    reference       VARCHAR(120),
    paid_at         TIMESTAMPTZ,
    confirmed_by    UUID REFERENCES users(id),
    proof_url       TEXT,
    notes           TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_payments_order ON payments(order_id);
