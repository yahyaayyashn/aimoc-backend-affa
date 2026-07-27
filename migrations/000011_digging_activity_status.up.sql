-- Migration: 000011_digging_activity_status
-- Menambah kolom activity_status dan digging_confidence ke tabel loading_logs
-- untuk menyimpan hasil deteksi ONNX (digging / non_digging) dari AI CCTV.

ALTER TABLE loading_logs
    ADD COLUMN IF NOT EXISTS activity_status    VARCHAR(20)      DEFAULT '' NOT NULL,
    ADD COLUMN IF NOT EXISTS digging_confidence NUMERIC(5, 4)    DEFAULT 0  NOT NULL;

-- Index untuk query cepat excavator yang sedang digging
CREATE INDEX IF NOT EXISTS idx_loading_logs_activity_status
    ON loading_logs (activity_status);

COMMENT ON COLUMN loading_logs.activity_status    IS 'Status aktivitas excavator dari AI ONNX: digging | non_digging | (kosong = belum terdeteksi)';
COMMENT ON COLUMN loading_logs.digging_confidence IS 'Confidence score (0.0–1.0) dari model LSTM untuk prediksi activity_status';
