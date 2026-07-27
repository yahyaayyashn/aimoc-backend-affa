-- AIMOC - Migration 000009 (down): Lepas excavator Operator Pos 1 & hapus excavatornya.

-- 1) Lepaskan kaitan operator dari excavator.
UPDATE users
SET excavator_id = NULL
WHERE email = 'operator@aimoc.id'
  AND excavator_id = (SELECT id FROM excavators WHERE code = 'EXC-PC200-1' LIMIT 1);

-- 2) Hapus excavator (hanya bila belum pernah dipakai pada loading_logs, agar lolos FK).
DELETE FROM excavators
WHERE code = 'EXC-PC200-1'
  AND NOT EXISTS (
    SELECT 1 FROM loading_logs ll WHERE ll.excavator_id = excavators.id
  );
