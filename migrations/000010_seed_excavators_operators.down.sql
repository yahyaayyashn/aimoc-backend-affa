-- AIMOC - Migration 000010 (down): Lepas & hapus excavator PC200 (7, 8, & umum).

-- 1) Lepaskan kaitan operator dari excavator.
UPDATE users
SET excavator_id = NULL
WHERE email IN ('operator.pc200.7@aimoc.id', 'operator.pc200.8@aimoc.id', 'operator.pc200@aimoc.id');

-- 2) Hapus excavator (hanya yang belum pernah dipakai pada loading_logs, agar lolos FK).
DELETE FROM excavators
WHERE code IN ('EXC-PC200-7', 'EXC-PC200-8', 'EXC-PC200')
  AND NOT EXISTS (
    SELECT 1 FROM loading_logs ll WHERE ll.excavator_id = excavators.id
  );
