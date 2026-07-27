-- Migration: 000021_bucket_events
-- Event mentah PER-BUCKET (1 baris = 1 siklus digging->non_digging selesai) yang
-- dideteksi AI service, dengan timestamp individual -- primitif baru yang dibutuhkan
-- untuk pengelompokan "1 truk = standard_buckets excavator, jeda >=300 detik = truk
-- baru" pada Dashboard Produktivitas & Revenue (lihat
-- Dokumen_Penjelasan_Dashboard_AIMOC.pdf).
-- Beda dengan loading_cycles (agregat per-siklus idle-timeout/max-duration, tanpa cap
-- bucket/truk) -- tabel ini APPEND-ONLY, pengelompokan truk dihitung on-the-fly dari
-- baris-baris ini (lihat internal/service/truck_grouping.go), tidak disimpan di sini.

CREATE TABLE IF NOT EXISTS bucket_events (
    id                  uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    camera_id           uuid REFERENCES cameras(id) ON DELETE SET NULL,
    excavator_id        uuid REFERENCES excavators(id) ON DELETE SET NULL,
    detected_at         timestamptz NOT NULL,
    digging_confidence  numeric(5,4) NOT NULL DEFAULT 0,
    created_at          timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_bucket_events_excavator_ts
    ON bucket_events (excavator_id, detected_at);

CREATE INDEX IF NOT EXISTS idx_bucket_events_camera_ts
    ON bucket_events (camera_id, detected_at);

COMMENT ON TABLE bucket_events IS
    'Event mentah 1-bucket-1-baris dengan timestamp individual, dipakai untuk menghitung pengelompokan truk (1 truk = standard_buckets excavator, jeda >=300 detik = truk baru) secara on-the-fly di internal/service/truck_grouping.go. Append-only, tidak pernah di-update.';
