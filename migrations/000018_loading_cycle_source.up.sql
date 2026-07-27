-- Migration: 000018_loading_cycle_source
-- Tambah kolom `source` pada loading_cycles: menandai asal siklus deteksi AI —
-- 'live' (dari live feed dashcam) atau 'recording' (dari file VOD rekaman, jalur
-- fail-safe saat live putus). Dipakai dashboard untuk menampilkan "data dari rekaman".

ALTER TABLE loading_cycles
    ADD COLUMN IF NOT EXISTS source varchar(20) NOT NULL DEFAULT 'live';

COMMENT ON COLUMN loading_cycles.source IS
    'live (dari live feed dashcam) | recording (dari file VOD rekaman, fail-safe saat live feed putus).';
