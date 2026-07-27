-- Migration: 000017_drop_transaction_tables (DOWN)
-- IRREVERSIBLE untuk versi AI-only: tabel transaksi tidak dipulihkan oleh down ini.
-- Kalau butuh lapisan transaksi kembali, gunakan migration asli 000003/000004/000005
-- dari repo lama (app/aimoc-brown-canyon-backend) — struktur & seed-nya lengkap di sana.
-- Sengaja no-op agar rollback ke 000016 tidak gagal, bukan berarti data kembali.
SELECT 1;
