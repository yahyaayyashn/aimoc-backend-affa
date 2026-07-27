-- Migration: 000017_drop_transaction_tables
-- Pivot AI-only (19 Jul 2026): buang seluruh tabel lapisan transaksi/kasir/admin.
-- Versi AI-only hanya butuh master kamera/excavator + loading_cycles (deteksi AI mandiri)
-- + auth/alert/notif/audit/settings. Tabel di bawah tidak lagi dipakai kode manapun.
--
-- Urutan drop memperhatikan foreign key (anak dulu, induk belakangan). CASCADE dipakai
-- sebagai pengaman terhadap constraint sisa. Idempotent (IF EXISTS).

DROP TABLE IF EXISTS loading_log_clips CASCADE;
DROP TABLE IF EXISTS surat_jalan CASCADE;
DROP TABLE IF EXISTS invoices CASCADE;
DROP TABLE IF EXISTS payments CASCADE;
DROP TABLE IF EXISTS order_items CASCADE;
DROP TABLE IF EXISTS gate_logs CASCADE;
DROP TABLE IF EXISTS loading_logs CASCADE;
DROP TABLE IF EXISTS orders CASCADE;

-- Master data transaksi-adjacent yang tidak dipakai jalur AI (dashboard AI hanya
-- memakai cameras & excavators). Dibuang agar skema bersih.
DROP TABLE IF EXISTS trucks CASCADE;
DROP TABLE IF EXISTS drivers CASCADE;
DROP TABLE IF EXISTS vendors CASCADE;
DROP TABLE IF EXISTS materials CASCADE;
DROP TABLE IF EXISTS categories CASCADE;
DROP TABLE IF EXISTS customers CASCADE;
