DELETE FROM system_settings WHERE key IN ('REVENUE_PER_TRUCK', 'TRUCK_GROUP_GAP_SEC');

INSERT INTO system_settings (key, value_jsonb, description)
VALUES
    ('TAX_PERCENT', '11', 'PPN dalam persen'),
    ('CURRENCY', '"IDR"', 'Mata uang sistem'),
    ('ORDER_EXPIRED_HOURS', '24', 'Order expired berapa jam setelah dibuat'),
    ('CCTV_MIN_CONFIDENCE', '0.75', 'Confidence minimum AI CCTV'),
    ('COMPANY_NAME', '"AIMOC Quarry Brown Canyon"', 'Nama perusahaan tampil di SJ/DO'),
    ('COMPANY_ADDRESS', '"Jl. Tambang Brown Canyon, Semarang"', 'Alamat perusahaan'),
    ('COMPANY_PHONE', '"021-555-0000"', 'Telepon perusahaan'),
    ('TAMBANG_LAT', '-7.052', 'Latitude lokasi tambang'),
    ('TAMBANG_LNG', '110.450', 'Longitude lokasi tambang')
ON CONFLICT (key) DO NOTHING;
