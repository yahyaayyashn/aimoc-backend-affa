-- AIMOC - Migration 000010: Seed excavator untuk operator PC200 (7, 8, & umum)
-- Membuat 3 excavator sesuai akun operator yang ada, lalu menghubungkannya.
-- Idempotent: aman dijalankan berulang (ON CONFLICT (code) DO NOTHING).

-- 0) SINKRONISASI SKEMA — tambah semua kolom yang dipakai backend terbaru.
--    Memperbaiki error berurutan: "column camera_id/excavator_id/operator_id does not exist".
--    Idempotent (IF NOT EXISTS). Urutan penting karena ada foreign key antar-tabel.

-- 0a) excavators.camera_id  (excavators -> cameras)  — fix error Edit Excavator
ALTER TABLE excavators  ADD COLUMN IF NOT EXISTS camera_id uuid REFERENCES cameras(id) ON DELETE SET NULL;
CREATE INDEX IF NOT EXISTS idx_excavators_camera ON excavators(camera_id);

-- 0b) users.excavator_id  (users -> excavators)  — fix error Accept order operator
ALTER TABLE users       ADD COLUMN IF NOT EXISTS excavator_id uuid REFERENCES excavators(id) ON DELETE SET NULL;
CREATE INDEX IF NOT EXISTS idx_users_excavator ON users(excavator_id);

-- 0c) orders.operator_id  (orders -> users)  — dipakai saat operator menerima order
ALTER TABLE orders      ADD COLUMN IF NOT EXISTS operator_id uuid REFERENCES users(id) ON DELETE SET NULL;
CREATE INDEX IF NOT EXISTS idx_orders_operator ON orders(operator_id);

-- 0d) loading_logs: excavator_id & bucket_count (dipakai pencatatan loading + AI CCTV)
ALTER TABLE loading_logs ADD COLUMN IF NOT EXISTS excavator_id uuid REFERENCES excavators(id) ON DELETE SET NULL;
ALTER TABLE loading_logs ADD COLUMN IF NOT EXISTS bucket_count int NOT NULL DEFAULT 0;

-- 1) Buat 3 excavator.
INSERT INTO excavators (code, name, brand, model, standard_buckets, status, image_url, notes)
VALUES
  ('EXC-PC200-7', 'Excavator Komatsu PC 200 - 7', 'Komatsu', 'PC 200', 20, 'AKTIF', '', ''),
  ('EXC-PC200-8', 'Excavator Komatsu PC 200 - 8', 'Komatsu', 'PC 200', 20, 'AKTIF', '', ''),
  ('EXC-PC200',   'Excavator Komatsu PC 200',     'Komatsu', 'PC 200', 20, 'AKTIF', '', '')
ON CONFLICT (code) DO NOTHING;

-- 2) Hubungkan tiap operator ke excavator-nya (cocokkan email -> code).
UPDATE users SET excavator_id = (SELECT id FROM excavators WHERE code = 'EXC-PC200-7' LIMIT 1)
  WHERE email = 'operator.pc200.7@aimoc.id';

UPDATE users SET excavator_id = (SELECT id FROM excavators WHERE code = 'EXC-PC200-8' LIMIT 1)
  WHERE email = 'operator.pc200.8@aimoc.id';

UPDATE users SET excavator_id = (SELECT id FROM excavators WHERE code = 'EXC-PC200' LIMIT 1)
  WHERE email = 'operator.pc200@aimoc.id';

-- 3) Hubungkan ketiga excavator ke kamera area loading (CAM-LOAD-01), bila ada.
UPDATE excavators
SET camera_id = (SELECT id FROM cameras WHERE code = 'CAM-LOAD-01' LIMIT 1)
WHERE code IN ('EXC-PC200-7', 'EXC-PC200-8', 'EXC-PC200')
  AND camera_id IS NULL;
