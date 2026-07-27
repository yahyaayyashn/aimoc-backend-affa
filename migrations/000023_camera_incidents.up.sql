-- Migration: 000023_camera_incidents
-- Riwayat gangguan koneksi dashcam + tindak lanjutnya (halaman "03 - Pemeriksaan Unit"
-- di Dokumen_Penjelasan_Dashboard_AIMOC.pdf). 1 baris = 1 episode gangguan (dari
-- terdeteksi putus sampai dikonfirmasi pulih), dibuat otomatis saat backend mendeteksi
-- cameras.last_seen_at basi (lihat CameraIncidentService.GetOrCreateActive) -- bukan
-- dikirim AI service, karena AI service sendiri yang berhenti mengirim saat ini terjadi.

CREATE TABLE IF NOT EXISTS camera_incidents (
    id           uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    camera_id    uuid NOT NULL REFERENCES cameras(id) ON DELETE CASCADE,
    detected_at  timestamptz NOT NULL,
    last_data_at timestamptz,
    status       varchar(20) NOT NULL DEFAULT 'BELUM_DIPERIKSA',
    notes        text NOT NULL DEFAULT '',
    checked_by   uuid REFERENCES users(id),
    resolved_at  timestamptz,
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_camera_incidents_camera
    ON camera_incidents (camera_id, created_at DESC);

COMMENT ON COLUMN camera_incidents.status IS
    'BELUM_DIPERIKSA (default, baru terdeteksi) | SEDANG_DITANGANI (operator sudah mulai menangani) | SUDAH_PULIH (dikonfirmasi normal kembali, resolved_at terisi).';
