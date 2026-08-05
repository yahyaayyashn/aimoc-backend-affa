-- revenue_rate_history — versi harga per truckload dari waktu ke waktu (05 Agu 2026).
-- Sebelumnya REVENUE_PER_TRUCK di system_settings cuma 1 nilai yang ditimpa langsung
-- (UPDATE in-place) -- akibatnya ganti harga hari ini ikut mengubah ulang Revenue
-- Tercatat untuk truk yang SUDAH selesai bertahun-tahun lalu (dihitung ulang tiap
-- request pakai rate SEKARANG). Rate seharusnya "efektif sejak kapan", bukan nilai
-- tunggal yang berlaku surut.
CREATE TABLE revenue_rate_history (
    id uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    rate numeric NOT NULL,
    effective_from timestamptz NOT NULL DEFAULT now(),
    created_at timestamptz NOT NULL DEFAULT now()
);

-- Seed 1 baris pakai nilai REVENUE_PER_TRUCK yang sudah ada (atau default 250000),
-- effective_from di masa lampau (epoch) supaya SEMUA data historis sebelum migration
-- ini tetap konsisten pakai rate yang sama seperti sebelumnya -- tidak ada data lama
-- yang tiba-tiba "kehilangan" rate.
INSERT INTO revenue_rate_history (rate, effective_from)
SELECT COALESCE(
    (SELECT (value_jsonb#>>'{}')::numeric FROM system_settings WHERE key = 'REVENUE_PER_TRUCK'),
    250000
), 'epoch'::timestamptz;
