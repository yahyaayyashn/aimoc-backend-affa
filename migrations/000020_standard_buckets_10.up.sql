-- Migration: 000020_standard_buckets_10
-- Keputusan tim (22-23 Juli 2026, lihat Dokumen_Penjelasan_Dashboard_AIMOC.pdf): 1 truk
-- diasumsikan SEMENTARA = 10 bucket untuk semua excavator (nilai asumsi, bukan hasil
-- pengukuran lapangan), dipakai sebagai acuan "Dashboard Produktivitas & Revenue" (AI
-- Only). Sebelumnya semua excavator seed di 20 (lihat migrations 000009/000010).

UPDATE excavators SET standard_buckets = 10 WHERE standard_buckets = 20;
