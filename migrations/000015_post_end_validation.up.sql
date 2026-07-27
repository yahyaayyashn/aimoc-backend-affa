-- post_end_status — hasil pemantauan AI pasca operator klik "Selesai" (alur
-- "Validasi AI" dari PM, 16 Juli 2026): setelah sesi loading ditutup, AI tetap
-- dianggap memantau selama POST_END_WATCH_WINDOW_SEC (default 5 menit). Jika di
-- jendela itu masih ada siklus digging terdeteksi (loading_cycles), sesi ditandai
-- SUSPICIOUS ("Perlu Pengecekan Lanjutan" di dashboard).
-- Nilai: '' (baris lama/belum dievaluasi), 'PENDING' (menunggu jadwal cek),
--        'CLEAR' (tidak ada aktivitas pasca-Selesai), 'SUSPICIOUS' (ada).
ALTER TABLE loading_logs
    ADD COLUMN IF NOT EXISTS post_end_status VARCHAR(20) NOT NULL DEFAULT '';
