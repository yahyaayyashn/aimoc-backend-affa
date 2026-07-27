-- Migration: 000014_loading_cycles
-- Siklus loading yang dideteksi AI service SECARA MANDIRI (cycle_detector.py di
-- aimoc-ai-service), lepas dari status sesi transaksi aplikasi (loading_logs). Dipakai
-- untuk membandingkan jumlah siklus terdeteksi AI vs jumlah transaksi tercatat manual
-- (lihat endpoint /api/v1/reports/ai-vs-manual), guna mendeteksi loading yang tidak
-- direkam/di-scan operator -- konsep pembanding independen yang tidak tercakup oleh
-- evaluateLoading() (yang cuma menilai kewajaran 1 sesi yang SUDAH dibuka).

CREATE TABLE IF NOT EXISTS loading_cycles (
    id                  uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    camera_id           uuid REFERENCES cameras(id) ON DELETE SET NULL,
    excavator_id        uuid REFERENCES excavators(id) ON DELETE SET NULL,
    bucket_count        int NOT NULL DEFAULT 0,
    duration_sec        int NOT NULL DEFAULT 0,
    start_ts            timestamptz NOT NULL,
    end_ts              timestamptz NOT NULL,
    close_reason        varchar(20) NOT NULL DEFAULT 'idle',
    digging_confidence  numeric(5,4) NOT NULL DEFAULT 0,
    created_at          timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_loading_cycles_excavator_start
    ON loading_cycles (excavator_id, start_ts);

CREATE INDEX IF NOT EXISTS idx_loading_cycles_camera_start
    ON loading_cycles (camera_id, start_ts);

COMMENT ON TABLE loading_cycles IS
    'Siklus loading (1 gerakan angkat bucket sampai kembali idle atau kena batas durasi maksimum) yang dideteksi AI service SECARA MANDIRI, independen dari sesi transaksi aplikasi (loading_logs) -- dipakai untuk membandingkan jumlah siklus terdeteksi AI vs jumlah transaksi tercatat manual, guna mendeteksi loading yang tidak direkam/di-scan operator.';
COMMENT ON COLUMN loading_cycles.excavator_id IS
    'Bisa NULL kalau kamera belum ditautkan ke excavator manapun di Master Data -- baris tetap disimpan (tidak dibuang) supaya data tidak hilang.';
COMMENT ON COLUMN loading_cycles.close_reason IS
    'idle (tidak ada aktivitas digging melebihi CYCLE_IDLE_TIMEOUT_SEC) | max_duration (durasi siklus melebihi CYCLE_MAX_DURATION_SEC, batas pengaman keras).';
