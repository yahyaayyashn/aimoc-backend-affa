-- AIMOC - Migration 000016 (down)
-- Catatan: material yang dihapus tidak dikembalikan.
UPDATE materials SET status = 'AKTIF' WHERE name NOT ILIKE 'tanah urug';
