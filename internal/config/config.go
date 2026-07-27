package config

import (
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	AppName     string
	AppEnv      string
	AppPort     string
	AppTimezone string

	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string
	DBSSLMode  string
	DBMaxOpen  int
	DBMaxIdle  int

	JWTAccessSecret  string
	JWTRefreshSecret string
	JWTAccessTTLMin  int
	JWTRefreshTTLDay int

	CORSOrigins []string

	UploadDir     string
	UploadBaseURL string

	CCTVWebhookSecret string

	// TestVideosDir — direktori video file lokal buat kamera yang dikonfigurasi skema
	// video:// (pengganti dashcam asli untuk testing/simulasi, lihat camera_stream.go
	// dan aimoc-ai-service/video_file_capture.py). Sama persis dengan TEST_VIDEOS_DIR
	// di ai-service, kedua sisi harus mount direktori host yang sama.
	TestVideosDir string

	// AIServiceDebugURL — base URL debug listener internal aimoc-ai-service (lihat
	// debug_listener.py, port default 8090, cuma reachable lewat jaringan Docker
	// internal). Dipakai CameraStreamHandler.RestartVideo buat relay permintaan
	// "putar ulang video dari awal" dari tombol FE -- backend BISA menjangkau ini
	// (satu jaringan Docker), browser TIDAK BISA (sengaja tidak dipublish ke host).
	AIServiceDebugURL string

	// RecordingsDir — direktori tempat VOD Sync worker menaruh file rekaman yang
	// ditarik dari dashcam (per kamera: <RecordingsDir>/<camera_code>/*.mp4). AI
	// service membaca folder yang SAMA sebagai sumber fail-safe saat live feed putus,
	// jadi kedua sisi harus mount direktori host yang sama.
	RecordingsDir string
	// VODSyncIntervalSec — periode worker menarik segmen VOD terbaru dari tiap dashcam.
	VODSyncIntervalSec int
	// VODSyncEnabled — matikan worker VOD sync per-deployment bila perlu.
	VODSyncEnabled bool

	// DeviceHealthIntervalSec — periode DeviceHealthPoller query command-socket
	// dashcam (SD card/RTC/GPS/SIM, lihat internal/service/device_health.go).
	DeviceHealthIntervalSec int
	// DeviceHealthEnabled — matikan poller device-health per-deployment bila perlu.
	DeviceHealthEnabled bool
}

func Load() *Config {
	v := viper.New()
	v.AutomaticEnv()
	v.SetConfigName(".env")
	v.SetConfigType("env")
	v.AddConfigPath(".")
	v.AddConfigPath("./..")
	_ = v.ReadInConfig()

	v.SetDefault("APP_NAME", "AIMOC Backend")
	v.SetDefault("APP_ENV", "development")
	v.SetDefault("APP_PORT", "8080")
	v.SetDefault("APP_TIMEZONE", "Asia/Jakarta")
	v.SetDefault("DB_HOST", "localhost")
	v.SetDefault("DB_PORT", "5432")
	v.SetDefault("DB_USER", "postgres")
	v.SetDefault("DB_PASSWORD", "postgres")
	v.SetDefault("DB_NAME", "aimoc")
	v.SetDefault("DB_SSLMODE", "disable")
	v.SetDefault("DB_MAX_OPEN", 50)
	v.SetDefault("DB_MAX_IDLE", 10)
	v.SetDefault("JWT_ACCESS_TTL_MIN", 15)
	v.SetDefault("JWT_REFRESH_TTL_DAYS", 7)
	v.SetDefault("CORS_ORIGINS", "http://localhost:5173")
	v.SetDefault("UPLOAD_DIR", "./uploads")
	v.SetDefault("UPLOAD_BASE_URL", "http://localhost:8080/uploads")
	v.SetDefault("TEST_VIDEOS_DIR", "./test-videos")
	v.SetDefault("AI_SERVICE_DEBUG_URL", "http://ai-service:8090")
	v.SetDefault("RECORDINGS_DIR", "./recordings")
	v.SetDefault("VOD_SYNC_INTERVAL_SEC", 300)
	v.SetDefault("VOD_SYNC_ENABLED", true)
	v.SetDefault("DEVICE_HEALTH_INTERVAL_SEC", 300)
	v.SetDefault("DEVICE_HEALTH_ENABLED", true)

	cfg := &Config{
		AppName:     v.GetString("APP_NAME"),
		AppEnv:      v.GetString("APP_ENV"),
		AppPort:     v.GetString("APP_PORT"),
		AppTimezone: v.GetString("APP_TIMEZONE"),

		DBHost:     v.GetString("DB_HOST"),
		DBPort:     v.GetString("DB_PORT"),
		DBUser:     v.GetString("DB_USER"),
		DBPassword: v.GetString("DB_PASSWORD"),
		DBName:     v.GetString("DB_NAME"),
		DBSSLMode:  v.GetString("DB_SSLMODE"),
		DBMaxOpen:  v.GetInt("DB_MAX_OPEN"),
		DBMaxIdle:  v.GetInt("DB_MAX_IDLE"),

		JWTAccessSecret:  v.GetString("JWT_ACCESS_SECRET"),
		JWTRefreshSecret: v.GetString("JWT_REFRESH_SECRET"),
		JWTAccessTTLMin:  v.GetInt("JWT_ACCESS_TTL_MIN"),
		JWTRefreshTTLDay: v.GetInt("JWT_REFRESH_TTL_DAYS"),

		CORSOrigins: strings.Split(v.GetString("CORS_ORIGINS"), ","),

		UploadDir:     v.GetString("UPLOAD_DIR"),
		UploadBaseURL: v.GetString("UPLOAD_BASE_URL"),

		CCTVWebhookSecret: v.GetString("CCTV_WEBHOOK_SECRET"),

		TestVideosDir: v.GetString("TEST_VIDEOS_DIR"),

		AIServiceDebugURL: v.GetString("AI_SERVICE_DEBUG_URL"),

		RecordingsDir:      v.GetString("RECORDINGS_DIR"),
		VODSyncIntervalSec: v.GetInt("VOD_SYNC_INTERVAL_SEC"),
		VODSyncEnabled:     v.GetBool("VOD_SYNC_ENABLED"),

		DeviceHealthIntervalSec: v.GetInt("DEVICE_HEALTH_INTERVAL_SEC"),
		DeviceHealthEnabled:     v.GetBool("DEVICE_HEALTH_ENABLED"),
	}

	if cfg.JWTAccessSecret == "" {
		cfg.JWTAccessSecret = "dev-access-secret-please-change"
	}
	if cfg.JWTRefreshSecret == "" {
		cfg.JWTRefreshSecret = "dev-refresh-secret-please-change"
	}
	return cfg
}
