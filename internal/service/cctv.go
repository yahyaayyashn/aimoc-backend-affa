package service

import (
	"errors"
	"strings"
	"time"

	"aimoc-backend/internal/domain"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// CCTVService (versi AI-only) — hanya menyediakan hal yang dipakai AI service &
// dashboard aktivitas: daftar kamera untuk dipoll AI, dan pencatatan siklus loading
// yang dideteksi AI secara mandiri (loading_cycles). Seluruh logika transaksi/gate/
// loading-session/scan/operator sudah dibuang pada pivot AI-only (19 Jul 2026).
type CCTVService struct {
	DB *gorm.DB
	// AIVision — opsional, di-set dari main.go setelah konstruksi (bukan parameter
	// constructor supaya tidak perlu ubah urutan wiring service lain). Nil-safe: kalau
	// belum di-set (mis. AI_VISION_ENABLED=false), RecordLoadingCycle tetap jalan normal
	// tanpa trigger AI Vision.
	AIVision *AIVisionService
}

func NewCCTVService(db *gorm.DB) *CCTVService {
	return &CCTVService{DB: db}
}

type AICameraInfo struct {
	Code      string `json:"code"`
	StreamURL string `json:"stream_url"`
}

// Cameras mengembalikan daftar kamera yang bisa dibaca AI service (stream_url
// berskema blackvue://, rtsp://, atau video://, status AKTIF) — dipakai
// camera_registry.py (AI service) polling berkala supaya kamera yang
// ditambah/diubah/dihapus lewat Master Kamera otomatis kepakai (start/stop
// pipeline), tanpa perlu edit .env CAMERAS manual + restart service.
func (s *CCTVService) Cameras() ([]AICameraInfo, error) {
	var rows []AICameraInfo
	err := s.DB.Model(&domain.Camera{}).
		Select("code, stream_url").
		Where("status = ? AND (stream_url LIKE ? OR stream_url LIKE ? OR stream_url LIKE ?)",
			"AKTIF", "blackvue://%", "rtsp://%", "video://%").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	return rows, nil
}

func (s *CCTVService) findCameraID(code string) *uuid.UUID {
	if code == "" {
		return nil
	}
	var cam domain.Camera
	if err := s.DB.Where("code = ?", code).First(&cam).Error; err != nil {
		return nil
	}
	return &cam.ID
}

// findExcavatorByCameraCode meresolusi excavator dari camera_code. Dipakai
// RecordLoadingCycle karena cycle_detector.py (AI service) mendeteksi siklus SECARA
// MANDIRI — hanya tahu camera_code, bukan excavator. Nil-safe: kalau kamera belum
// ditautkan ke excavator manapun, return nil (bukan error) supaya caller tetap bisa
// menyimpan baris dengan excavator_id NULL, bukan membuang datanya.
func (s *CCTVService) findExcavatorByCameraCode(cameraCode string) *domain.Excavator {
	camID := s.findCameraID(cameraCode)
	if camID == nil {
		return nil
	}
	var e domain.Excavator
	if err := s.DB.Where("camera_id = ?", *camID).First(&e).Error; err != nil {
		return nil
	}
	return &e
}

// LoadingCycleDetectedReq — 1 siklus loading yang dideteksi AI service SECARA MANDIRI
// (cycle_detector.py). Excavator di-resolve server-side dari camera_code (lihat
// findExcavatorByCameraCode). Source menandai apakah siklus berasal dari live feed
// atau dari file rekaman VOD (fail-safe) — dipakai dashboard untuk menampilkan
// "data dari rekaman".
type LoadingCycleDetectedReq struct {
	CameraCode        string    `json:"camera_code" validate:"required"`
	BucketCount       int       `json:"bucket_count"`
	DurationSec       int       `json:"duration_sec"`
	StartTS           time.Time `json:"start_ts"`
	EndTS             time.Time `json:"end_ts"`
	CloseReason       string    `json:"close_reason"`
	DiggingConfidence float64   `json:"digging_confidence"`
	Source            string    `json:"source"` // "live" | "recording"
}

// RecordLoadingCycle menyimpan 1 siklus loading terdeteksi AI ke tabel loading_cycles.
// Murni pencatatan agregat untuk dashboard/laporan aktivitas AI — tidak trigger
// alert/WebSocket, tidak menyentuh flow lain.
func (s *CCTVService) RecordLoadingCycle(req LoadingCycleDetectedReq) (*domain.LoadingCycle, error) {
	camID := s.findCameraID(req.CameraCode)
	var excID *uuid.UUID
	if exc := s.findExcavatorByCameraCode(req.CameraCode); exc != nil {
		excID = &exc.ID
	}
	lc := domain.LoadingCycle{
		CameraID:          camID,
		ExcavatorID:       excID,
		BucketCount:       req.BucketCount,
		DurationSec:       req.DurationSec,
		StartTS:           req.StartTS,
		EndTS:             req.EndTS,
		CloseReason:       ifEmpty(req.CloseReason, "idle"),
		Source:            ifEmpty(req.Source, "live"),
		DiggingConfidence: req.DiggingConfidence,
	}
	if err := s.DB.Create(&lc).Error; err != nil {
		return nil, err
	}
	if s.AIVision != nil {
		s.AIVision.TriggerAsync(lc)
	}
	return &lc, nil
}

// BucketDetectedReq — 1 bucket individual dengan timestamp presisi, dikirim AI service
// tiap kali cycle_bucket_counter naik. Excavator di-resolve server-side dari
// camera_code, sama seperti LoadingCycleDetectedReq di atas.
type BucketDetectedReq struct {
	CameraCode        string     `json:"camera_code" validate:"required"`
	DetectedAt        time.Time  `json:"detected_at"`
	StartedAt         *time.Time `json:"started_at"` // momen segmen digging dimulai, opsional
	DiggingConfidence float64    `json:"digging_confidence"`
	Source            string     `json:"source"` // "live" | "recording"
}

// RecordBucketEvent menyimpan 1 event bucket mentah ke tabel bucket_events. Append-only,
// murni pencatatan — pengelompokan jadi truk dihitung on-the-fly (lihat truck_grouping.go),
// tidak di sini.
func (s *CCTVService) RecordBucketEvent(req BucketDetectedReq) (*domain.BucketEvent, error) {
	camID := s.findCameraID(req.CameraCode)
	var excID *uuid.UUID
	if exc := s.findExcavatorByCameraCode(req.CameraCode); exc != nil {
		excID = &exc.ID
	}
	be := domain.BucketEvent{
		CameraID:          camID,
		ExcavatorID:       excID,
		DetectedAt:        req.DetectedAt,
		StartedAt:         req.StartedAt,
		DiggingConfidence: req.DiggingConfidence,
		Source:            ifEmpty(req.Source, "live"),
	}
	if err := s.DB.Create(&be).Error; err != nil {
		return nil, err
	}
	return &be, nil
}

func ifEmpty(v, fallback string) string {
	if strings.TrimSpace(v) == "" {
		return fallback
	}
	return v
}

// CameraHeartbeatReq — laporan status TERKINI dari AI service, dikirim berkala tiap
// beberapa detik per kamera (terpisah dari LoadingCycleDetectedReq yang cuma dikirim
// saat 1 siklus SELESAI). Ini satu-satunya sinyal "sedang apa sekarang" yang dipunya
// backend, dasar badge status real-time (Aktif Menggali/Idle) di dashboard.
type CameraHeartbeatReq struct {
	CameraCode     string `json:"camera_code" validate:"required"`
	ActivityStatus string `json:"activity_status"` // "digging" | "non_digging"
	Source         string `json:"source"`          // "live" | "recording"
}

// RecordHeartbeat menyimpan status AI terkini untuk 1 kamera. LastSeenAt di-set ke
// waktu sekarang setiap panggilan — dipakai backend menentukan "offline" (heartbeat
// terlalu basi = pipeline/kamera mati atau network putus), berbeda dari "idle" yang
// berarti AI tetap mengamati tapi excavator tidak sedang menggali.
func (s *CCTVService) RecordHeartbeat(req CameraHeartbeatReq) error {
	now := time.Now()
	res := s.DB.Model(&domain.Camera{}).Where("code = ?", req.CameraCode).Updates(map[string]interface{}{
		"last_activity_status": ifEmpty(req.ActivityStatus, "non_digging"),
		"last_activity_source": ifEmpty(req.Source, "live"),
		"last_seen_at":         &now,
	})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errors.New("kamera '" + req.CameraCode + "' tidak ditemukan")
	}
	return nil
}
