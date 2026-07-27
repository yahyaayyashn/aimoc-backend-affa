-- Migration: 000022_bucket_event_source
-- Tambah kolom `source` pada bucket_events, konsisten dengan loading_cycles.source
-- (migration 000018) -- menandai asal event bucket: 'live' (dari live feed dashcam)
-- atau 'recording' (dari file VOD rekaman, jalur fail-safe saat live putus).

ALTER TABLE bucket_events
    ADD COLUMN IF NOT EXISTS source varchar(20) NOT NULL DEFAULT 'live';

COMMENT ON COLUMN bucket_events.source IS
    'live (dari live feed dashcam) | recording (dari file VOD rekaman, fail-safe saat live feed putus).';
