-- Migration 000030: hasil analisa dari pipeline AI Vision baru (excavator_vlm, tim AI
-- Gracia BCS) -- YOLO+BoT-SORT+LSTM asli yang mendeteksi truk secara visual, beda dari
-- loading_cycles yang cuma mengelompokkan bucket_events excavator dari jeda waktu.
-- 1 baris = 1 job selesai diproses service Python eksternal (upload+async GPU queue).

CREATE TABLE ai_vision_analyses (
    id uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    -- NULL kalau trigger manual (video yang dipilih/diupload user, bukan dari 1 siklus
    -- loading otomatis).
    loading_cycle_id uuid REFERENCES loading_cycles(id) ON DELETE CASCADE,
    trigger_source varchar(20) NOT NULL DEFAULT 'auto', -- auto | manual
    label varchar(120), -- nama/keterangan opsional, dipakai jalur manual
    external_job_id varchar(64), -- job_id di service Python (service_data/jobs/<id>)
    status varchar(20) NOT NULL DEFAULT 'queued', -- queued|running|completed|failed
    unit_id varchar(40),
    dashboard_summary jsonb,
    annotated_video_path varchar(255), -- artifact key relatif, bukan URL penuh
    error_message text,
    submitted_at timestamptz NOT NULL DEFAULT now(),
    finished_at timestamptz
);

CREATE UNIQUE INDEX ai_vision_analyses_loading_cycle_id_idx
    ON ai_vision_analyses(loading_cycle_id) WHERE loading_cycle_id IS NOT NULL;
