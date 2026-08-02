package main

import (
	"fmt"
	"strings"

	"aimoc-backend/internal/config"
	"aimoc-backend/internal/handler"
	"aimoc-backend/internal/middleware"
	"aimoc-backend/internal/repository"
	"aimoc-backend/internal/service"
	wshub "aimoc-backend/internal/websocket"
	"aimoc-backend/pkg/aivision"
	jwtpkg "aimoc-backend/pkg/jwt"
	"aimoc-backend/pkg/utils"

	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"go.uber.org/zap"
)

func main() {
	cfg := config.Load()
	utils.InitLogger(cfg.AppEnv)
	defer utils.Log.Sync()

	db, err := repository.NewDB(cfg)
	if err != nil {
		utils.Log.Fatal("Gagal koneksi DB: " + err.Error())
	}
	repository.EnsureSchema(db)

	jwtSvc := jwtpkg.New(cfg.JWTAccessSecret, cfg.JWTRefreshSecret, cfg.JWTAccessTTLMin, cfg.JWTRefreshTTLDay)

	wshub.InitDefault()

	authSvc := service.NewAuthService(db, jwtSvc)
	cctvSvc := service.NewCCTVService(db)

	// VOD Sync worker — menarik rekaman VOD dashcam ke RecordingsDir agar AI service
	// bisa fail-over ke rekaman saat live feed putus. Reuse pkg/blackvue.
	if cfg.VODSyncEnabled {
		vodSync := service.NewVODSyncService(db, cfg.RecordingsDir, cfg.VODSyncIntervalSec)
		vodSync.Start()
	} else {
		utils.Log.Info("VOD sync worker dinonaktifkan (VOD_SYNC_ENABLED=false)")
	}

	// Device Health poller — query command-socket dashcam (SD card/RTC/GPS/SIM) secara
	// berkala, dipakai halaman "02 - Detail Excavator" & "03 - Pemeriksaan Unit".
	if cfg.DeviceHealthEnabled {
		healthPoller := service.NewDeviceHealthPoller(db, cfg.DeviceHealthIntervalSec)
		healthPoller.Start()
	} else {
		utils.Log.Info("Device health poller dinonaktifkan (DEVICE_HEALTH_ENABLED=false)")
	}

	// AI Vision — pipeline baru tim AI (excavator_vlm), deteksi truk visual asli.
	// Service Python eksternal, dipanggil lewat pkg/aivision. Nonaktif secara default
	// (AI_VISION_ENABLED=false) sampai systemd service-nya tervalidasi di VPS.
	aiVisionClient := aivision.NewClient(cfg.AIVisionURL, cfg.AIVisionAPIKey)
	aiVisionSvc := service.NewAIVisionService(db, aiVisionClient, cfg.RecordingsDir, cfg.AIVisionEnabled, cfg.AIVisionMinDurationSec)
	cctvSvc.AIVision = aiVisionSvc
	if cfg.AIVisionEnabled {
		utils.Log.Info("AI Vision aktif", zap.String("url", cfg.AIVisionURL), zap.Int("min_duration_sec", cfg.AIVisionMinDurationSec))
	} else {
		utils.Log.Info("AI Vision dinonaktifkan (AI_VISION_ENABLED=false)")
	}

	authH := handler.NewAuthHandler(authSvc)
	masterH := handler.NewMasterHandler(db)
	cctvH := handler.NewCCTVHandler(cctvSvc)
	miscH := handler.NewMiscHandler(db)
	uploadH := handler.NewUploadHandler(cfg.UploadDir)
	camStreamH := handler.NewCameraStreamHandler(db, cfg.TestVideosDir, cfg.AIServiceDebugURL, cfg.RecordingsDir)
	incidentH := handler.NewCameraIncidentHandler(service.NewCameraIncidentService(db))
	aiVisionH := handler.NewAIVisionHandler(db, aiVisionSvc, aiVisionClient, cfg.TestVideosDir, cfg.UploadDir)

	app := fiber.New(fiber.Config{
		AppName:               cfg.AppName,
		DisableStartupMessage: false,
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			return utils.ServerError(c, err.Error(), nil)
		},
	})

	app.Use(recover.New())
	app.Use(logger.New())
	app.Use(cors.New(cors.Config{
		AllowOrigins:     strings.Join(cfg.CORSOrigins, ","),
		AllowCredentials: false,
		AllowHeaders:     "Origin, Content-Type, Accept, Authorization, X-Webhook-Secret",
		AllowMethods:     "GET,POST,PUT,DELETE,PATCH,OPTIONS",
	}))

	app.Static("/uploads", cfg.UploadDir)

	app.Get("/healthz", miscH.Health)

	// ============== WebSocket ==============
	app.Use("/ws", func(c *fiber.Ctx) error {
		if websocket.IsWebSocketUpgrade(c) {
			return c.Next()
		}
		return fiber.ErrUpgradeRequired
	})
	app.Get("/ws/:channel", websocket.New(wshub.Serve))

	v1 := app.Group("/api/v1")

	// ============== Auth ==============
	v1.Post("/auth/login", authH.Login)
	v1.Post("/auth/refresh", authH.Refresh)
	v1.Post("/auth/logout", authH.Logout)
	v1.Get("/auth/me", middleware.AuthRequired(jwtSvc), authH.Me)

	// ============== CCTV AI Receiver (Webhook) ==============
	// Hanya yang dipakai AI service versi AI-only: daftar kamera untuk dipoll +
	// penerimaan siklus loading terdeteksi AI secara mandiri.
	cctv := v1.Group("/cctv", middleware.WebhookSecret(cfg.CCTVWebhookSecret))
	cctv.Get("/cameras", cctvH.Cameras)
	cctv.Post("/loading-cycle-detected", cctvH.LoadingCycleDetected)
	// bucket-detected — 1 bucket individual dengan timestamp presisi, dipakai backend
	// mengelompokkan bucket jadi truk (lihat internal/service/truck_grouping.go).
	cctv.Post("/bucket-detected", cctvH.BucketDetected)
	cctv.Post("/camera-heartbeat", cctvH.CameraHeartbeat)

	// ============== Authenticated routes ==============
	auth := v1.Use(middleware.AuthRequired(jwtSvc))

	// Upload gambar (semua user terautentikasi internal)
	auth.Post("/uploads/image", uploadH.UploadImage)

	// Master Kamera
	auth.Get("/cameras", masterH.ListCameras)
	auth.Post("/cameras", middleware.RequireRole("SUPER_ADMIN"), masterH.CreateCamera)
	auth.Put("/cameras/:id", middleware.RequireRole("SUPER_ADMIN"), masterH.UpdateCamera)
	auth.Delete("/cameras/:id", middleware.RequireRole("SUPER_ADMIN"), masterH.DeleteCamera)
	auth.Get("/cameras/:id/live", camStreamH.Live)
	auth.Post("/cameras/:id/restart-video", camStreamH.RestartVideo)
	auth.Get("/cameras/:id/video-status", camStreamH.VideoStatus)
	auth.Get("/cameras/:id/recordings", camStreamH.Recordings)
	auth.Get("/cameras/:id/recordings/:filename", camStreamH.RecordingFile)

	// Pemeriksaan Unit — gangguan koneksi dashcam + tindak lanjut operator.
	auth.Get("/cameras/:id/incident", incidentH.ActiveIncident)
	auth.Get("/cameras/:id/incidents", incidentH.IncidentHistory)
	auth.Get("/camera-incidents", middleware.RequireRole("SUPER_ADMIN", "MANAJEMEN"), incidentH.ListActiveIncidents)
	auth.Put("/camera-incidents/:id", middleware.RequireRole("SUPER_ADMIN", "MANAJEMEN"), incidentH.UpdateIncident)

	// Master Excavator
	auth.Get("/excavators", masterH.ListExcavators)
	auth.Post("/excavators", middleware.RequireRole("SUPER_ADMIN"), masterH.CreateExcavator)
	auth.Put("/excavators/:id", middleware.RequireRole("SUPER_ADMIN"), masterH.UpdateExcavator)
	auth.Delete("/excavators/:id", middleware.RequireRole("SUPER_ADMIN"), masterH.DeleteExcavator)

	// Master Material
	auth.Get("/materials", masterH.ListMaterials)
	auth.Post("/materials", middleware.RequireRole("SUPER_ADMIN"), masterH.CreateMaterial)
	auth.Put("/materials/:id", middleware.RequireRole("SUPER_ADMIN"), masterH.UpdateMaterial)
	auth.Delete("/materials/:id", middleware.RequireRole("SUPER_ADMIN"), masterH.DeleteMaterial)

	// Aktivitas AI (siklus loading terdeteksi mandiri)
	auth.Get("/loading-cycles", middleware.RequireRole("SUPER_ADMIN", "MANAJEMEN"), miscH.ListLoadingCycles)
	auth.Get("/loading-cycles/:id/buckets", middleware.RequireRole("SUPER_ADMIN", "MANAJEMEN"), miscH.LoadingCycleBuckets)
	auth.Delete("/loading-cycles/:id", middleware.RequireRole("SUPER_ADMIN"), miscH.DeleteLoadingCycle)
	auth.Get("/loading-cycles/:id/ai-vision", middleware.RequireRole("SUPER_ADMIN", "MANAJEMEN"), aiVisionH.GetByLoadingCycle)

	// AI Vision (pipeline baru tim AI, deteksi truk visual -- lihat internal/service/ai_vision.go)
	auth.Get("/ai-vision", middleware.RequireRole("SUPER_ADMIN", "MANAJEMEN"), aiVisionH.List)
	auth.Get("/ai-vision/test-videos", middleware.RequireRole("SUPER_ADMIN", "MANAJEMEN"), aiVisionH.TestVideos)
	auth.Post("/ai-vision/manual", middleware.RequireRole("SUPER_ADMIN", "MANAJEMEN"), aiVisionH.Manual)
	auth.Get("/ai-vision/:id", middleware.RequireRole("SUPER_ADMIN", "MANAJEMEN"), aiVisionH.Get)
	auth.Get("/ai-vision/:id/video", middleware.RequireRole("SUPER_ADMIN", "MANAJEMEN"), aiVisionH.Video)

	// Alert & Notif
	auth.Get("/alerts", miscH.ListAlerts)
	auth.Put("/alerts/:id/acknowledge", miscH.AckAlert)
	auth.Get("/notifications", miscH.ListNotifs)
	auth.Put("/notifications/:id/read", miscH.MarkNotifRead)
	auth.Put("/notifications/read-all", miscH.MarkAllNotifRead)

	// Dashboard & Laporan aktivitas AI
	auth.Get("/dashboard/manajemen", miscH.DashboardManajemen)
	auth.Get("/reports/activity", middleware.RequireRole("SUPER_ADMIN", "MANAJEMEN"), miscH.ActivityReport)
	auth.Get("/reports/produktivitas-revenue", middleware.RequireRole("SUPER_ADMIN", "MANAJEMEN"), miscH.ProduktivitasRevenue)

	// Super Admin
	auth.Get("/users", middleware.RequireRole("SUPER_ADMIN"), miscH.ListUsers)
	auth.Put("/users/:id", middleware.RequireRole("SUPER_ADMIN"), miscH.UpdateUser)
	auth.Get("/roles", middleware.RequireRole("SUPER_ADMIN"), miscH.ListRoles)
	auth.Get("/permissions", middleware.RequireRole("SUPER_ADMIN"), miscH.ListPermissions)
	auth.Get("/audit-logs", middleware.RequireRole("SUPER_ADMIN"), miscH.ListAuditLogs)
	auth.Get("/settings", middleware.RequireRole("SUPER_ADMIN"), miscH.ListSettings)
	auth.Put("/settings/:key", middleware.RequireRole("SUPER_ADMIN"), miscH.UpdateSettings)

	addr := fmt.Sprintf(":%s", cfg.AppPort)
	utils.Log.Info("AIMOC backend (AI-only) listening on " + addr)
	if err := app.Listen(addr); err != nil {
		utils.Log.Fatal("Listen error: " + err.Error())
	}
}
