-- Migration: 000012_loading_log_clips
-- Tabel child untuk menyimpan segmen video mentah onboard dashcam (BlackVue VOD)
-- sebagai backup rekaman sesi loading, independen dari live-stream/AI-detection
-- pipeline. Segmen disimpan terpisah per file (tanpa merge/ffmpeg untuk saat ini).

CREATE TABLE IF NOT EXISTS loading_log_clips (
    id              uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    loading_log_id  uuid NOT NULL REFERENCES loading_logs(id) ON DELETE CASCADE,
    camera_id       uuid REFERENCES cameras(id) ON DELETE SET NULL,
    filename        varchar(120) NOT NULL,
    url             text NOT NULL DEFAULT '',
    size_bytes      bigint NOT NULL DEFAULT 0,
    segment_ts      timestamptz,
    status          varchar(20) NOT NULL DEFAULT 'DONE',
    error_message   text NOT NULL DEFAULT '',
    created_at      timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_loading_log_clips_loading_log
    ON loading_log_clips (loading_log_id);

CREATE UNIQUE INDEX IF NOT EXISTS uq_loading_log_clips_log_filename
    ON loading_log_clips (loading_log_id, filename);

COMMENT ON TABLE loading_log_clips IS
    'Segmen video mentah (~1 menit) dari dashcam BlackVue (VOD), backup independen dari live-stream/AI pipeline, per sesi loading.';
COMMENT ON COLUMN loading_log_clips.segment_ts IS
    'Timestamp mulai segmen sesuai nama file dashcam (waktu device lokal).';
COMMENT ON COLUMN loading_log_clips.status IS
    'DONE | FAILED -- FAILED tetap disimpan agar kegagalan tidak hilang diam-diam (lihat juga tabel alerts, type=CLIP_BACKUP_FAILED).';
