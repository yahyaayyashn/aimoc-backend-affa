package service

import (
	"time"

	"aimoc-backend/internal/domain"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// CameraOfflineThresholdSec — sama persis dengan heartbeatOfflineThresholdSec di
// internal/handler/misc.go (DashboardManajemen) -- kamera dianggap offline kalau
// last_seen_at lebih basi dari ini. Didefinisikan terpisah (bukan diimpor) karena
// package service tidak boleh bergantung ke package handler; kalau nilainya berubah,
// ubah di kedua tempat.
const CameraOfflineThresholdSec = 90

const (
	IncidentBelumDiperiksa  = "BELUM_DIPERIKSA"
	IncidentSedangDitangani = "SEDANG_DITANGANI"
	IncidentSudahPulih      = "SUDAH_PULIH"
)

type CameraIncidentService struct {
	DB *gorm.DB
}

func NewCameraIncidentService(db *gorm.DB) *CameraIncidentService {
	return &CameraIncidentService{DB: db}
}

// deviceHealthReasonCode menentukan reason_code dari device-health check (SD card/RTC/
// GPS/SIM, diisi DeviceHealthPoller -- lihat device_health.go), independen dari status
// koneksi (last_seen_at) -- kamera bisa "online" (masih kirim heartbeat) tapi tetap
// punya masalah SD card/RTC/GPS/SIM yang perlu ditindaklanjuti. Return "" kalau semua
// normal. Prioritas: SD card dulu (paling operasional-kritis -- VOD gagal tersimpan),
// lalu RTC, GPS, SIM.
func deviceHealthReasonCode(cam domain.Camera) string {
	if cam.SDStatus != nil && *cam.SDStatus != "NORMAL" {
		return "SD_" + *cam.SDStatus
	}
	if cam.RTCLowBattery {
		return "RTC_LOW_BATTERY"
	}
	if cam.GPSNoSignalSince != nil {
		return "GPS_NO_SIGNAL"
	}
	if cam.SIMStatus != nil && *cam.SIMStatus != "NORMAL" {
		return "SIM_INACTIVE"
	}
	return ""
}

// GetOrCreateActive mengembalikan gangguan yang SEDANG BERLANGSUNG untuk 1 kamera --
// dibuat otomatis di sini (bukan oleh AI service, yang justru berhenti mengirim saat
// gangguan terjadi) kalau cameras.last_seen_at sudah basi (CONNECTION_LOST) ATAU
// device-health check menemukan masalah (SD/RTC/GPS/SIM, lihat deviceHealthReasonCode)
// DAN belum ada baris camera_incidents yang masih terbuka (status != SUDAH_PULIH)
// untuk kamera ini. Return (nil, nil) kalau kamera sehat sepenuhnya -- tidak ada
// gangguan aktif.
func (s *CameraIncidentService) GetOrCreateActive(cameraID uuid.UUID) (*domain.CameraIncident, error) {
	var cam domain.Camera
	if err := s.DB.First(&cam, "id = ?", cameraID).Error; err != nil {
		return nil, err
	}

	offline := cam.LastSeenAt == nil || time.Since(*cam.LastSeenAt).Seconds() > CameraOfflineThresholdSec
	reasonCode := ""
	if offline {
		reasonCode = "CONNECTION_LOST"
	} else if hc := deviceHealthReasonCode(cam); hc != "" {
		reasonCode = hc
	}
	if reasonCode == "" {
		return nil, nil
	}

	var existing domain.CameraIncident
	err := s.DB.Where("camera_id = ? AND status <> ?", cameraID, IncidentSudahPulih).
		Order("created_at DESC").First(&existing).Error
	if err == nil {
		if existing.ReasonCode == nil || *existing.ReasonCode != reasonCode {
			if updErr := s.DB.Model(&existing).Update("reason_code", reasonCode).Error; updErr != nil {
				return nil, updErr
			}
			existing.ReasonCode = &reasonCode
		}
		return &existing, nil
	}
	if err != gorm.ErrRecordNotFound {
		return nil, err
	}

	inc := domain.CameraIncident{
		CameraID:   cameraID,
		DetectedAt: derefTimeOr(cam.LastSeenAt, time.Now()),
		LastDataAt: cam.LastSeenAt,
		ReasonCode: &reasonCode,
		Status:     IncidentBelumDiperiksa,
	}
	if err := s.DB.Create(&inc).Error; err != nil {
		return nil, err
	}
	return &inc, nil
}

func derefTimeOr(t *time.Time, fallback time.Time) time.Time {
	if t == nil {
		return fallback
	}
	return *t
}

// ListIncidents mengembalikan riwayat gangguan 1 kamera, terbaru dulu.
func (s *CameraIncidentService) ListIncidents(cameraID uuid.UUID) ([]domain.CameraIncident, error) {
	var rows []domain.CameraIncident
	err := s.DB.Where("camera_id = ?", cameraID).Order("created_at DESC").Limit(50).Find(&rows).Error
	return rows, err
}

// ListActive mengembalikan semua gangguan yang masih terbuka lintas kamera -- dipakai
// widget "Unit Memerlukan Tindakan" di dashboard. Men-sync dulu (scan SEMUA kamera,
// auto-create insiden untuk yang offline tapi belum pernah "dibuka" halaman
// Pemeriksaan Unit-nya) sebelum query -- tanpa ini, unit yang offline tapi belum
// pernah diklik siapa pun tidak akan pernah muncul di sini walau jelas offline.
func (s *CameraIncidentService) ListActive() ([]domain.CameraIncident, error) {
	var cams []domain.Camera
	if err := s.DB.Find(&cams).Error; err != nil {
		return nil, err
	}
	for _, cam := range cams {
		if _, err := s.GetOrCreateActive(cam.ID); err != nil {
			return nil, err
		}
	}

	var rows []domain.CameraIncident
	err := s.DB.Preload("Camera").Where("status <> ?", IncidentSudahPulih).
		Order("created_at DESC").Find(&rows).Error
	return rows, err
}

// UpdateIncident menyimpan catatan pemeriksaan + status penanganan. resolved_at
// diisi otomatis begitu status berubah jadi SUDAH_PULIH.
func (s *CameraIncidentService) UpdateIncident(id uuid.UUID, status, notes string, checkedBy *uuid.UUID) (*domain.CameraIncident, error) {
	var inc domain.CameraIncident
	if err := s.DB.First(&inc, "id = ?", id).Error; err != nil {
		return nil, err
	}
	updates := map[string]interface{}{
		"status": status, "notes": notes, "checked_by": checkedBy,
	}
	if status == IncidentSudahPulih && inc.ResolvedAt == nil {
		now := time.Now()
		updates["resolved_at"] = &now
	}
	if err := s.DB.Model(&inc).Updates(updates).Error; err != nil {
		return nil, err
	}
	s.DB.Preload("Camera").First(&inc, "id = ?", id)
	return &inc, nil
}
