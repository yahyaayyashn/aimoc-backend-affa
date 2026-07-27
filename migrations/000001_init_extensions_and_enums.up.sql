-- AIMOC - Migration 000001: Extensions & ENUM types
-- Fase 1: Basic Sales & Gate Control

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pgcrypto";
CREATE EXTENSION IF NOT EXISTS "pg_trgm";

DO $$ BEGIN
    CREATE TYPE order_status AS ENUM (
        'NEW', 'PAID', 'READY', 'IN_PROGRESS', 'COMPLETED', 'CLOSED', 'CANCELLED'
    );
EXCEPTION WHEN duplicate_object THEN null; END $$;

DO $$ BEGIN
    CREATE TYPE payment_status AS ENUM ('UNPAID', 'PENDING', 'PAID', 'REFUNDED', 'FAILED');
EXCEPTION WHEN duplicate_object THEN null; END $$;

DO $$ BEGIN
    CREATE TYPE payment_method AS ENUM ('CASH', 'TRANSFER', 'VA', 'QRIS');
EXCEPTION WHEN duplicate_object THEN null; END $$;

DO $$ BEGIN
    CREATE TYPE gate_type AS ENUM ('GATE_IN', 'GATE_OUT');
EXCEPTION WHEN duplicate_object THEN null; END $$;

DO $$ BEGIN
    CREATE TYPE general_status AS ENUM ('AKTIF', 'NONAKTIF', 'BLOKIR');
EXCEPTION WHEN duplicate_object THEN null; END $$;

DO $$ BEGIN
    CREATE TYPE customer_type AS ENUM ('INDIVIDU', 'PERUSAHAAN');
EXCEPTION WHEN duplicate_object THEN null; END $$;

DO $$ BEGIN
    CREATE TYPE alert_severity AS ENUM ('INFO', 'WARNING', 'CRITICAL');
EXCEPTION WHEN duplicate_object THEN null; END $$;

DO $$ BEGIN
    CREATE TYPE notif_type AS ENUM (
        'ORDER_CREATED','ORDER_PAID','GATE_IN','LOADING_START','LOADING_END',
        'GATE_OUT','ORDER_COMPLETED','ALERT','SYSTEM'
    );
EXCEPTION WHEN duplicate_object THEN null; END $$;
