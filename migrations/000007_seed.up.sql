-- AIMOC - Migration 000007: Seed initial data
-- Password default semua user: Admin@123  (bcrypt hash di bawah)
-- Bcrypt hash dari "Admin@123" cost 10
--   $2a$10$N8H/Qe6S4kZGrFqW5n9oZuP8wQbS6QmM3MpYi8sJgM3p7nQpQ9KOC
-- (akan diregenerasi oleh seeder Go agar konsisten)

INSERT INTO roles (code, name, description, is_system) VALUES
    ('SUPER_ADMIN', 'Super Admin', 'Akses penuh ke seluruh sistem & semua role', TRUE),
    ('MANAJEMEN',   'Manajemen / Kepala Tambang', 'Eksekutif - dashboard, laporan, approval', TRUE),
    ('ADMIN_SALES', 'Admin Sales & Dispatch', 'Kelola order, master data, pembayaran', TRUE),
    ('OPERATOR',    'Operator Lapangan', 'Validasi gate, loading, cetak SJ', TRUE),
    ('CUSTOMER',    'Customer / Pembeli', 'Self-service order & tracking', TRUE)
ON CONFLICT (code) DO NOTHING;

INSERT INTO permissions (code, name, module) VALUES
    ('USER_MANAGE',      'Kelola User',                  'USER'),
    ('ROLE_MANAGE',      'Kelola Role & Permission',     'USER'),
    ('CUSTOMER_VIEW',    'Lihat Customer',               'MASTER'),
    ('CUSTOMER_MANAGE',  'Kelola Customer',              'MASTER'),
    ('MATERIAL_VIEW',    'Lihat Material',               'MASTER'),
    ('MATERIAL_MANAGE',  'Kelola Material',              'MASTER'),
    ('VENDOR_MANAGE',    'Kelola Vendor',                'MASTER'),
    ('TRUCK_MANAGE',     'Kelola Truck',                 'MASTER'),
    ('DRIVER_MANAGE',    'Kelola Driver',                'MASTER'),
    ('CAMERA_MANAGE',    'Kelola Kamera',                'MASTER'),
    ('ORDER_VIEW',       'Lihat Order',                  'ORDER'),
    ('ORDER_CREATE',     'Buat Order',                   'ORDER'),
    ('ORDER_EDIT',       'Edit Order',                   'ORDER'),
    ('ORDER_CANCEL',     'Batalkan Order',               'ORDER'),
    ('PAYMENT_CONFIRM',  'Konfirmasi Pembayaran',        'ORDER'),
    ('GATE_VIEW',        'Lihat Gate Log',               'GATE'),
    ('GATE_OVERRIDE',    'Override Gate manual',         'GATE'),
    ('LOADING_VIEW',     'Lihat Loading',                'LOADING'),
    ('LOADING_OVERRIDE', 'Override Loading manual',      'LOADING'),
    ('SJ_VIEW',          'Lihat Surat Jalan',            'DOCUMENT'),
    ('SJ_GENERATE',      'Generate Surat Jalan',         'DOCUMENT'),
    ('INVOICE_VIEW',     'Lihat Invoice',                'DOCUMENT'),
    ('REPORT_VIEW',      'Lihat Laporan',                'REPORT'),
    ('REPORT_EXPORT',    'Export Laporan',               'REPORT'),
    ('DASHBOARD_EXEC',   'Dashboard Eksekutif',          'DASHBOARD'),
    ('DASHBOARD_ADMIN',  'Dashboard Admin',              'DASHBOARD'),
    ('DASHBOARD_OPERATOR','Dashboard Operator',          'DASHBOARD'),
    ('CCTV_RECEIVE',     'Terima webhook CCTV AI',       'SYSTEM'),
    ('ALERT_VIEW',       'Lihat Alert',                  'SYSTEM'),
    ('AUDIT_VIEW',       'Lihat Audit Log',              'SYSTEM'),
    ('SETTINGS_MANAGE',  'Kelola Pengaturan Sistem',     'SYSTEM')
ON CONFLICT (code) DO NOTHING;

-- SUPER_ADMIN: semua permission
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r CROSS JOIN permissions p
WHERE r.code = 'SUPER_ADMIN'
ON CONFLICT DO NOTHING;

-- MANAJEMEN
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r, permissions p
WHERE r.code = 'MANAJEMEN' AND p.code IN (
    'DASHBOARD_EXEC','REPORT_VIEW','REPORT_EXPORT','ORDER_VIEW','GATE_VIEW',
    'LOADING_VIEW','SJ_VIEW','INVOICE_VIEW','ALERT_VIEW','AUDIT_VIEW',
    'CUSTOMER_VIEW','MATERIAL_VIEW'
)
ON CONFLICT DO NOTHING;

-- ADMIN_SALES
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r, permissions p
WHERE r.code = 'ADMIN_SALES' AND p.code IN (
    'DASHBOARD_ADMIN','CUSTOMER_VIEW','CUSTOMER_MANAGE','MATERIAL_VIEW',
    'MATERIAL_MANAGE','VENDOR_MANAGE','TRUCK_MANAGE','DRIVER_MANAGE',
    'ORDER_VIEW','ORDER_CREATE','ORDER_EDIT','ORDER_CANCEL','PAYMENT_CONFIRM',
    'SJ_VIEW','SJ_GENERATE','INVOICE_VIEW','REPORT_VIEW','REPORT_EXPORT',
    'GATE_VIEW','LOADING_VIEW','ALERT_VIEW'
)
ON CONFLICT DO NOTHING;

-- OPERATOR
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r, permissions p
WHERE r.code = 'OPERATOR' AND p.code IN (
    'DASHBOARD_OPERATOR','ORDER_VIEW','GATE_VIEW','GATE_OVERRIDE',
    'LOADING_VIEW','LOADING_OVERRIDE','SJ_VIEW','SJ_GENERATE','ALERT_VIEW',
    'MATERIAL_VIEW'
)
ON CONFLICT DO NOTHING;

-- CUSTOMER
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r, permissions p
WHERE r.code = 'CUSTOMER' AND p.code IN (
    'ORDER_VIEW','ORDER_CREATE','SJ_VIEW','INVOICE_VIEW','MATERIAL_VIEW'
)
ON CONFLICT DO NOTHING;

-- Super Admin user default (password: Admin@123 bcrypt cost 10)
INSERT INTO users (role_id, email, phone, password_hash, full_name, status)
SELECT r.id, 'admin@aimoc.id', '081234567890',
       '$2a$10$Yy7CtbI9Op0gC2g6cWN7XOaDdM7bb2T4rWcN5LqU3HsrYwO/2eNHi',
       'Super Admin AIMOC', 'AKTIF'
FROM roles r WHERE r.code = 'SUPER_ADMIN'
ON CONFLICT (email) DO NOTHING;

-- Demo user per role (password: Admin@123)
INSERT INTO users (role_id, email, phone, password_hash, full_name, status)
SELECT r.id, 'manajemen@aimoc.id', '081234500001',
       '$2a$10$Yy7CtbI9Op0gC2g6cWN7XOaDdM7bb2T4rWcN5LqU3HsrYwO/2eNHi',
       'Pak Manajer', 'AKTIF'
FROM roles r WHERE r.code = 'MANAJEMEN'
ON CONFLICT (email) DO NOTHING;

INSERT INTO users (role_id, email, phone, password_hash, full_name, status)
SELECT r.id, 'sales@aimoc.id', '081234500002',
       '$2a$10$Yy7CtbI9Op0gC2g6cWN7XOaDdM7bb2T4rWcN5LqU3HsrYwO/2eNHi',
       'Admin Sales', 'AKTIF'
FROM roles r WHERE r.code = 'ADMIN_SALES'
ON CONFLICT (email) DO NOTHING;

INSERT INTO users (role_id, email, phone, password_hash, full_name, status)
SELECT r.id, 'operator@aimoc.id', '081234500003',
       '$2a$10$Yy7CtbI9Op0gC2g6cWN7XOaDdM7bb2T4rWcN5LqU3HsrYwO/2eNHi',
       'Operator Pos 1', 'AKTIF'
FROM roles r WHERE r.code = 'OPERATOR'
ON CONFLICT (email) DO NOTHING;

-- Material contoh (harga flat per truk Rp 200.000 — kolom price_per_unit tidak dipakai untuk harga order baru)
INSERT INTO materials (code, name, unit, price_per_unit, stock, min_stock, description, status) VALUES
    ('MAT-002','Pasir Kasar','TON',130000,4000,500,'Pasir kasar untuk pondasi & beton','AKTIF'),
    ('MAT-003','Tanah Urug','TON',75000,10000,1000,'Tanah urug pengisi area','AKTIF'),
    ('MAT-004','Batu Split 1-2','TON',180000,3000,300,'Batu split ukuran 1-2 cm','AKTIF'),
    ('MAT-005','Batu Split 2-3','TON',185000,2500,300,'Batu split ukuran 2-3 cm','AKTIF'),
    ('MAT-006','Batu Belah','TON',160000,2000,200,'Batu belah konstruksi','AKTIF')
ON CONFLICT (code) DO NOTHING;

-- Vendor contoh
INSERT INTO vendors (code, name, phone, address, pic_name, pic_phone, status) VALUES
    ('VND-001','PT Truck Sejahtera','021555000','Jl. Industri No. 1','Budi','081200001','AKTIF'),
    ('VND-002','CV Angkutan Mandiri','021555001','Jl. Raya Tambang KM 5','Sari','081200002','AKTIF')
ON CONFLICT (code) DO NOTHING;

-- Camera contoh
INSERT INTO cameras (code, name, area, ip_address, location, status) VALUES
    ('CAM-IN-01','Kamera Gate IN 1','GATE_IN','192.168.1.10','Pintu Masuk Utama','AKTIF'),
    ('CAM-LOAD-01','Kamera Loading 1','LOADING','192.168.1.11','Area Loading A','AKTIF'),
    ('CAM-OUT-01','Kamera Gate OUT 1','GATE_OUT','192.168.1.12','Pintu Keluar Utama','AKTIF')
ON CONFLICT (code) DO NOTHING;

-- Shift contoh
INSERT INTO shifts (name, start_time, end_time, is_active) VALUES
    ('Pagi','06:00','14:00',TRUE),
    ('Siang','14:00','22:00',TRUE),
    ('Malam','22:00','06:00',TRUE);

-- System settings default
INSERT INTO system_settings (key, value_jsonb, description) VALUES
    ('TAX_PERCENT',     '11',                        'PPN dalam persen'),
    ('CURRENCY',        '"IDR"',                      'Mata uang sistem'),
    ('ORDER_EXPIRED_HOURS','24',                      'Order expired berapa jam setelah dibuat'),
    ('CCTV_MIN_CONFIDENCE','0.75',                    'Confidence minimum AI CCTV'),
    ('COMPANY_NAME',    '"AIMOC Quarry Brown Canyon"','Nama perusahaan tampil di SJ/DO'),
    ('COMPANY_ADDRESS', '"Jl. Tambang Brown Canyon, Semarang"','Alamat perusahaan'),
    ('COMPANY_PHONE',   '"021-555-0000"',             'Telepon perusahaan'),
    ('TAMBANG_LAT',     '-7.052',                     'Latitude lokasi tambang'),
    ('TAMBANG_LNG',     '110.450',                    'Longitude lokasi tambang')
ON CONFLICT (key) DO NOTHING;
