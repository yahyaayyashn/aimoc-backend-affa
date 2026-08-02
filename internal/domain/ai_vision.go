package domain

import (
	"time"

	"github.com/google/uuid"
)

// AIVisionAnalysis — 1 hasil analisa dari pipeline AI Vision baru (excavator_vlm, tim AI
// Gracia BCS): YOLO+BoT-SORT+LSTM asli yang mendeteksi truk secara visual, dijalankan
// sebagai job upload+async GPU queue di service Python eksternal (lihat
// internal/service/ai_vision.go). Beda dari LoadingCycle yang cuma mengelompokkan
// bucket_events dari jeda waktu, tanpa pernah "melihat" truknya.
type AIVisionAnalysis struct {
	ID uuid.UUID `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()" json:"id"`
	// LoadingCycleID — NULL kalau trigger manual (video yang dipilih/diupload user).
	LoadingCycleID *uuid.UUID    `gorm:"type:uuid" json:"loading_cycle_id"`
	LoadingCycle   *LoadingCycle `json:"loading_cycle,omitempty"`
	// TriggerSource — "auto" (dari 1 loading_cycles selesai) | "manual" (halaman
	// Analisa Video AI).
	TriggerSource string `gorm:"size:20;column:trigger_source;default:'auto'" json:"trigger_source"`
	Label         string `gorm:"size:120" json:"label"`
	// ExternalJobID — job_id di service Python (service_data/jobs/<id>).
	ExternalJobID string `gorm:"size:64;column:external_job_id" json:"external_job_id"`
	// Status — queued|running|completed|failed.
	Status string `gorm:"size:20;default:'queued'" json:"status"`
	UnitID string `gorm:"size:40;column:unit_id" json:"unit_id"`
	// DashboardSummary — JSON mentah dari `dashboard_summary` job (kpi, activity,
	// sessions[], dst — lihat DOCUMENTATION.md service Python), disimpan apa adanya
	// (string, pola sama SystemSetting.ValueJSONB) supaya tidak perlu migration ulang
	// tiap skema mereka berubah.
	DashboardSummary string `gorm:"type:jsonb;column:dashboard_summary" json:"dashboard_summary"`
	// AnnotatedVideoPath — artifact key relatif (mis. "annotated_video"), dipakai
	// handler proxy video, BUKAN URL penuh ke service Python.
	AnnotatedVideoPath string     `gorm:"size:255;column:annotated_video_path" json:"annotated_video_path"`
	ErrorMessage       string     `gorm:"column:error_message" json:"error_message"`
	SubmittedAt        time.Time  `gorm:"column:submitted_at" json:"submitted_at"`
	FinishedAt         *time.Time `gorm:"column:finished_at" json:"finished_at"`
}

func (AIVisionAnalysis) TableName() string { return "ai_vision_analyses" }
