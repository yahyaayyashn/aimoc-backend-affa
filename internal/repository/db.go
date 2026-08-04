package repository

import (
	"fmt"
	"time"

	"aimoc-backend/internal/config"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func NewDB(cfg *config.Config) (*gorm.DB, error) {
	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s TimeZone=%s",
		cfg.DBHost, cfg.DBPort, cfg.DBUser, cfg.DBPassword, cfg.DBName, cfg.DBSSLMode, cfg.AppTimezone,
	)
	gormLog := logger.Default.LogMode(logger.Warn)
	if cfg.AppEnv == "production" {
		gormLog = logger.Default.LogMode(logger.Error)
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: gormLog})
	if err != nil {
		return nil, err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(cfg.DBMaxOpen)
	sqlDB.SetMaxIdleConns(cfg.DBMaxIdle)
	sqlDB.SetConnMaxLifetime(time.Hour)
	return db, nil
}

// EnsureSchema memastikan kolom & seed yang dibutuhkan versi AI-only ada (idempotent,
// aman dijalankan tiap start). Seluruh penyesuaian tabel transaksi (orders, order_items,
// loading_logs, materials-pricing, categories) sudah dibuang pada pivot AI-only.
func EnsureSchema(db *gorm.DB) {
	db.Exec(`ALTER TABLE cameras ADD COLUMN IF NOT EXISTS image_url text NOT NULL DEFAULT ''`)

	// Kolom excavator assignment pada users (excavator yang dioperasikan oleh operator).
	db.Exec(`ALTER TABLE users ADD COLUMN IF NOT EXISTS excavator_id uuid REFERENCES excavators(id) ON DELETE SET NULL`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_users_excavator ON users(excavator_id)`)

	// Master Excavator (alat berat pemuat) — idempotent.
	db.Exec(`CREATE TABLE IF NOT EXISTS excavators (
		id uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
		code varchar(30) NOT NULL UNIQUE,
		name varchar(120) NOT NULL,
		brand varchar(80) NOT NULL DEFAULT '',
		model varchar(80) NOT NULL DEFAULT '',
		standard_buckets int NOT NULL DEFAULT 20,
		image_url text NOT NULL DEFAULT '',
		status varchar(20) NOT NULL DEFAULT 'AKTIF',
		notes text NOT NULL DEFAULT '',
		created_at timestamptz DEFAULT now(),
		updated_at timestamptz DEFAULT now()
	)`)
	db.Exec(`ALTER TABLE excavators ADD COLUMN IF NOT EXISTS standard_buckets int NOT NULL DEFAULT 20`)
	// Kolom camera_id pada excavators — setiap excavator punya CCTV area loading.
	db.Exec(`ALTER TABLE excavators ADD COLUMN IF NOT EXISTS camera_id uuid REFERENCES cameras(id) ON DELETE SET NULL`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_excavators_camera ON excavators(camera_id)`)
	db.Exec(`INSERT INTO excavators (code, name, brand, model, standard_buckets, status) VALUES
		('EXC-PC200', 'Excavator Komatsu PC 200', 'Komatsu', 'PC 200', 20, 'AKTIF')
		ON CONFLICT (code) DO NOTHING`)

	// Kolom source pada loading_cycles — asal siklus (live | recording), untuk fail-safe VOD.
	db.Exec(`ALTER TABLE loading_cycles ADD COLUMN IF NOT EXISTS source varchar(20) NOT NULL DEFAULT 'live'`)

	// Kolom status real-time kamera — diisi tiap heartbeat AI service (POST
	// /cctv/camera-heartbeat), dasar badge Aktif/Idle/Offline di dashboard.
	db.Exec(`ALTER TABLE cameras ADD COLUMN IF NOT EXISTS last_activity_status varchar(20) NOT NULL DEFAULT ''`)
	db.Exec(`ALTER TABLE cameras ADD COLUMN IF NOT EXISTS last_activity_source varchar(20) NOT NULL DEFAULT 'live'`)

	ensureAdminAccounts(db)
	ensureOperatorAccounts(db)
	ensureCameraExcavatorLinks(db)

	// Gambar CCTV/kamera (path tersimpan di DB).
	cameraImages := map[string]string{
		"CAM-IN-01":   "/uploads/images/cctv/CAM-IN-01.jpg",
		"CAM-LOAD-01": "/uploads/images/cctv/CAM-LOAD-01.jpg",
		"CAM-OUT-01":  "/uploads/images/cctv/CAM-OUT-01.jpg",
	}
	for code, url := range cameraImages {
		db.Exec(`UPDATE cameras SET image_url = ? WHERE code = ? AND (image_url IS NULL OR image_url = '')`, url, code)
	}
}

// ensureCameraExcavatorLinks menghubungkan setiap excavator ke kamera CCTV area loading-nya.
// Idempotent: aman dijalankan setiap start.
func ensureCameraExcavatorLinks(db *gorm.DB) {
	// Semua excavator di area loading dipantau oleh kamera loading.
	// Mapping code excavator → code kamera.
	links := []struct{ excCode, camCode string }{
		{"EXC-PC200-7", "CAM-LOAD-01"},
		{"EXC-PC200-8", "CAM-LOAD-01"},
		{"EXC-PC200", "CAM-LOAD-01"},
	}
	for _, l := range links {
		db.Exec(`UPDATE excavators SET camera_id = (
			SELECT id FROM cameras WHERE code = ? LIMIT 1
		) WHERE code = ? AND camera_id IS NULL`, l.camCode, l.excCode)
	}
}

// ensureOperatorAccounts membuat akun operator lapangan — satu per excavator yang ada.
// Setiap akun langsung terhubung ke excavator-nya via kolom excavator_id.
// Password default: Admin@123 (sama dengan akun admin).
// Idempotent: aman dijalankan setiap start.
func ensureOperatorAccounts(db *gorm.DB) {
	const pwHash = `$2a$10$iHnKWYB7LrtZ4WQMop7PJ.kxz.Rv2FXXn1G5qo4dLXI0DWefy.XqG`

	type opSeed struct {
		email   string
		phone   string
		name    string
		excCode string
	}
	seeds := []opSeed{
		{"operator.pc200.7@aimoc.id", "081234500010", "Operator PC200-7", "EXC-PC200-7"},
		{"operator.pc200.8@aimoc.id", "081234500011", "Operator PC200-8", "EXC-PC200-8"},
		{"operator.pc200@aimoc.id", "081234500012", "Operator PC200", "EXC-PC200"},
	}
	for _, s := range seeds {
		db.Exec(`INSERT INTO users (role_id, email, phone, password_hash, full_name, status)
			SELECT r.id, ?, ?, ?, ?, 'AKTIF'
			FROM roles r WHERE r.code = 'OPERATOR'
			ON CONFLICT (email) DO NOTHING`, s.email, s.phone, pwHash, s.name)
		// Hubungkan ke excavator yang sesuai (hanya bila belum tersambung).
		db.Exec(`UPDATE users SET excavator_id = (
			SELECT id FROM excavators WHERE code = ? LIMIT 1
		) WHERE email = ? AND excavator_id IS NULL`, s.excCode, s.email)
	}
}

// ensureAdminAccounts menyelaraskan akun admin operasional: mengganti nama lama
// "Super Admin AIMOC" menjadi "Admin Utara", serta menambahkan akun kedua
// "Admin Selatan" — dua admin (utara & selatan) agar audit trail jelas. Istilah
// "Gate" sengaja dibuang dari nama (permintaan user 04 Agu 2026) -- sisa istilah
// alur transaksi lama, tidak relevan lagi di versi AI-only.
// Idempotent: aman dijalankan setiap start.
func ensureAdminAccounts(db *gorm.DB) {
	// Hash bcrypt cost 10 dari "Admin@123" (diverifikasi cocok — lihat catatan bug
	// 20 Jul 2026: konstanta lama TIDAK benar-benar match "Admin@123" walau namanya
	// mengklaim begitu, ditemukan saat verifikasi login pivot AI-only).
	const pwHash = `$2a$10$RYOYfhZw0V/xSh3dp87Sq.3xQzesv9mJzsok19NXiyMYriMdIOg3C`

	// Admin Utara — rename akun super admin lama (hanya bila masih bernama bawaan).
	db.Exec(`UPDATE users SET full_name = 'Admin Utara'
		WHERE email = 'admin@aimoc.id' AND full_name = 'Super Admin AIMOC'`)
	// Perbaiki password admin@aimoc.id kalau masih hash lama yang salah (idempotent —
	// hanya menimpa hash spesifik lama yang diketahui salah, tak menyentuh password
	// yang sudah diganti manual oleh admin).
	db.Exec(`UPDATE users SET password_hash = ?
		WHERE email = 'admin@aimoc.id' AND password_hash = '$2a$10$Yy7CtbI9Op0gC2g6cWN7XOaDdM7bb2T4rWcN5LqU3HsrYwO/2eNHi'`, pwHash)

	// Admin Selatan — akun admin kedua (role SUPER_ADMIN, sama seperti admin utara).
	db.Exec(`INSERT INTO users (role_id, email, phone, password_hash, full_name, status)
		SELECT r.id, 'admin.selatan@aimoc.id', '081234567891', ?, 'Admin Selatan', 'AKTIF'
		FROM roles r WHERE r.code = 'SUPER_ADMIN'
		ON CONFLICT (email) DO NOTHING`, pwHash)
}
