-- Migration 000029: setting kapasitas truk default (02 Agu 2026).
-- Angka fallback yang sebelumnya hardcoded 10 di source Go (dashboard_ai.go), dipakai
-- kalau Excavator.StandardBuckets per-unit belum diisi admin di Master Data.

INSERT INTO system_settings (key, value_jsonb, description)
VALUES
    ('TRUCK_CAPACITY_M3', '10', 'Kapasitas truk default (m3) kalau Excavator.StandardBuckets belum diisi -- dulu hardcoded 10 di source')
ON CONFLICT (key) DO NOTHING;
