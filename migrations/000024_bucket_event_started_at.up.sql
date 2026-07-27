-- Migration: 000024_bucket_event_started_at
-- Tambah kolom `started_at` pada bucket_events — momen segmen digging ini MULAI
-- (bukan cuma `detected_at` yang sudah ada, yang menandai momen SELESAI). Dipakai
-- untuk timeline granular digging/non-digging di halaman Detail Aktivitas Loading.
-- Nullable: data historis sebelum fitur ini tidak punya nilai ini.

ALTER TABLE bucket_events
    ADD COLUMN IF NOT EXISTS started_at timestamptz NULL;

COMMENT ON COLUMN bucket_events.started_at IS
    'Momen segmen digging dimulai (nullable — kosong untuk data sebelum fitur timeline granular ada). detected_at tetap berarti momen selesai.';
