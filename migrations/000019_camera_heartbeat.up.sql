-- Migration: 000019_camera_heartbeat
-- Kolom status real-time kamera, diisi tiap heartbeat masuk dari AI service
-- (POST /cctv/camera-heartbeat) — dasar badge Aktif/Idle/Offline di dashboard.
-- last_seen_at yang basi (lebih tua dari ambang, mis. >90 detik) menandakan Offline.

ALTER TABLE cameras
    ADD COLUMN IF NOT EXISTS last_activity_status varchar(20) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS last_activity_source varchar(20) NOT NULL DEFAULT 'live';

COMMENT ON COLUMN cameras.last_activity_status IS
    'digging | non_digging | '''' (belum pernah heartbeat) — status AI TERKINI, diupdate tiap heartbeat.';
COMMENT ON COLUMN cameras.last_activity_source IS
    'live | recording — asal data heartbeat terakhir (fail-safe auto-failover AI service).';
