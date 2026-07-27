-- AIMOC - Migration 000008 (down)
DROP INDEX IF EXISTS idx_orders_queue;
ALTER TABLE orders DROP COLUMN IF EXISTS queue_no;
-- Catatan: material yang dihapus tidak dikembalikan.
