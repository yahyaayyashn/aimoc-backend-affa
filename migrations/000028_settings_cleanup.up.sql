-- Migration 000028: konsolidasi Pengaturan Sistem (01 Agu 2026).
-- Seed key baru untuk nilai yang sebelumnya hardcoded di source Go, hapus key lama
-- peninggalan sistem transaksi pra-pivot yang sudah tidak dibaca kode manapun.

INSERT INTO system_settings (key, value_jsonb, description)
VALUES
    ('REVENUE_PER_TRUCK', '250000', 'Revenue per truk penuh (flat, bukan per m3) -- dulu hardcoded 250000 di source'),
    ('TRUCK_GROUP_GAP_SEC', '300', 'Jeda (detik) tanpa bucket baru yang menandai truk selesai -- ubah ini mengubah klasifikasi Overload/Underload/Unvalidated, bukan cuma angka harga')
ON CONFLICT (key) DO NOTHING;

DELETE FROM system_settings
WHERE key IN ('TAX_PERCENT', 'ORDER_EXPIRED_HOURS', 'CCTV_MIN_CONFIDENCE', 'COMPANY_NAME', 'COMPANY_ADDRESS', 'COMPANY_PHONE', 'TAMBANG_LAT', 'TAMBANG_LNG', 'CURRENCY');
