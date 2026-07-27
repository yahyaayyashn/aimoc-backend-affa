-- AIMOC - Migration 000009: Tambah excavator untuk Operator Pos 1 (operator@aimoc.id)
-- Melengkapi data: setiap operator memiliki excavator masing-masing.
-- Idempotent: aman dijalankan berulang (tidak duplikat).

-- 1) Tambah excavator baru, terhubung ke kamera area loading (CAM-LOAD-01).
INSERT INTO excavators (code, name, brand, model, standard_buckets, status, camera_id, image_url, notes)
SELECT 'EXC-PC200-1', 'Excavator Komatsu PC 200 - 1', 'Komatsu', 'PC 200', 20, 'AKTIF',
       (SELECT id FROM cameras WHERE code = 'CAM-LOAD-01' LIMIT 1), '', ''
WHERE NOT EXISTS (SELECT 1 FROM excavators WHERE code = 'EXC-PC200-1');

-- 2) Hubungkan operator@aimoc.id (Operator Pos 1) ke excavator tersebut.
UPDATE users
SET excavator_id = (SELECT id FROM excavators WHERE code = 'EXC-PC200-1' LIMIT 1)
WHERE email = 'operator@aimoc.id'
  AND excavator_id IS NULL;
