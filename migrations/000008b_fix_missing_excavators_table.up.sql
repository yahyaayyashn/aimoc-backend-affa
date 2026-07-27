-- AIMOC - Migration 000008b (FIX, dibuat 11 Juli 2026, bukan dari tim backend):
-- Tabel `excavators` TIDAK PERNAH dibuat di migration manapun (0001-0008), padahal
-- 000009_excavator_operator_pos1.up.sql dan 000010_seed_excavators_operators.up.sql
-- keduanya langsung INSERT ke tabel ini tanpa CREATE TABLE terlebih dulu — migration
-- 000009 gagal dengan error "relation excavators does not exist" kalau dijalankan
-- urut dari awal database kosong. Skema di bawah disusun dari struct Go
-- `internal/domain/models.go` (`type Excavator struct`), mengikuti gaya penulisan
-- tabel `cameras` di 000003_master_data.up.sql untuk konsistensi (UUID PK,
-- general_status enum, FK ke cameras).
--
-- CATATAN BUAT TIM BACKEND: migration ini perlu ditambahkan resmi ke repo (idealnya
-- digabung/direnumber sebagai bagian dari urutan asli), supaya deployment lain dari
-- database kosong tidak kena error yang sama.

CREATE TABLE excavators (
    id               UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    code             VARCHAR(30) UNIQUE NOT NULL,
    name             VARCHAR(120) NOT NULL,
    brand            VARCHAR(80),
    model            VARCHAR(80),
    standard_buckets INT NOT NULL DEFAULT 0,
    camera_id        UUID REFERENCES cameras(id) ON DELETE SET NULL,
    image_url        TEXT,
    status           general_status NOT NULL DEFAULT 'AKTIF',
    notes            TEXT,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_excavators_camera ON excavators(camera_id);
CREATE INDEX idx_excavators_status ON excavators(status);
