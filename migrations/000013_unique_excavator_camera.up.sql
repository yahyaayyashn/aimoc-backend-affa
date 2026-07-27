-- Migration: 000013_unique_excavator_camera
-- Satu kamera/dashcam fisik cuma bisa dedicated ke SATU excavator. Sebelum ini,
-- semua excavator boleh (dan pernah) share camera_id yang sama tanpa dicegah --
-- risiko nyata: kalau 2 excavator dengan camera_id sama sama-sama punya sesi
-- loading aktif, AI service (session_registry.py, mapping camera_code->excavator_code)
-- diam-diam kehilangan observasi salah satunya (dict override), tanpa error apa pun.

-- Bersihkan data lama dulu: sebelumnya SEMUA excavator nunjuk ke kamera fisik yang
-- sama karena baru ada 1 dashcam. Pertahankan cuma di EXC-PC200-1 (yang dipakai di
-- test end-to-end terakhir), null-kan sisanya supaya constraint di bawah bisa apply.
UPDATE excavators SET camera_id = NULL
WHERE code <> 'EXC-PC200-1' AND camera_id IS NOT NULL;

ALTER TABLE excavators ADD CONSTRAINT uq_excavators_camera_id UNIQUE (camera_id);

COMMENT ON CONSTRAINT uq_excavators_camera_id ON excavators IS
    'Satu kamera cuma boleh dedicated ke satu excavator. NULL (belum terhubung kamera) boleh berulang -- perilaku default UNIQUE constraint Postgres.';
