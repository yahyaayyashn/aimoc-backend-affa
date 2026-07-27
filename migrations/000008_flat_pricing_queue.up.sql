-- AIMOC - Migration 000008: Flat pricing per truk + antrian (queue) + hapus material

-- Nomor antrian loading per order.
ALTER TABLE orders ADD COLUMN IF NOT EXISTS queue_no INT NOT NULL DEFAULT 0;
CREATE INDEX IF NOT EXISTS idx_orders_queue ON orders(queue_no);

-- Hapus material "Pasir Halus" & "Tanah Halus" dari sistem.
-- Yang masih direferensikan order_items tidak bisa di-DELETE (FK), jadi dinonaktifkan.
UPDATE materials SET status = 'NONAKTIF'
WHERE (name ILIKE 'pasir halus' OR name ILIKE 'tanah halus')
  AND EXISTS (SELECT 1 FROM order_items oi WHERE oi.material_id = materials.id);

DELETE FROM materials
WHERE (name ILIKE 'pasir halus' OR name ILIKE 'tanah halus')
  AND NOT EXISTS (SELECT 1 FROM order_items oi WHERE oi.material_id = materials.id);
