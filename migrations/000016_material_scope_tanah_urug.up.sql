-- AIMOC - Migration 000016: Scope produk dipersempit ke Tanah Urug saja
-- Bisnis saat ini cuma benar-benar menjual Tanah Urug; material lain masih seed
-- demo dari awal project. Yang masih direferensikan order_items tidak bisa
-- di-DELETE (FK), jadi dinonaktifkan -- pola sama persis dgn migration 000008
-- (Pasir Halus/Tanah Halus).

UPDATE materials SET status = 'NONAKTIF'
WHERE name NOT ILIKE 'tanah urug'
  AND EXISTS (SELECT 1 FROM order_items oi WHERE oi.material_id = materials.id);

DELETE FROM materials
WHERE name NOT ILIKE 'tanah urug'
  AND NOT EXISTS (SELECT 1 FROM order_items oi WHERE oi.material_id = materials.id);
