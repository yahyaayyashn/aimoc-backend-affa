package domain

import (
	"time"

	"github.com/google/uuid"
)

// CATATAN (pivot AI-only, 19 Jul 2026): seluruh tipe transaksi (Order, OrderItem,
// Payment, GateLog, LoadingLog, LoadingLogClip, SuratJalan, Invoice) sudah DIBUANG.
// Versi AI-only tidak punya lapisan transaksi; dashboard & laporan bersumber murni
// dari LoadingCycle (jalur AI-mandiri, lihat cycle_detector.py di ai-service).

// LoadingCycle — 1 siklus loading yang dideteksi AI service SECARA MANDIRI (lihat
// cycle_detector.py). Inilah sumber utama data aktivitas di versi AI-only.
type LoadingCycle struct {
	ID          uuid.UUID  `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()" json:"id"`
	CameraID    *uuid.UUID `gorm:"type:uuid" json:"camera_id"`
	Camera      *Camera    `json:"camera,omitempty"`
	ExcavatorID *uuid.UUID `gorm:"type:uuid" json:"excavator_id"`
	Excavator   *Excavator `json:"excavator,omitempty"`
	BucketCount int        `gorm:"column:bucket_count" json:"bucket_count"`
	DurationSec int        `gorm:"column:duration_sec" json:"duration_sec"`
	StartTS     time.Time  `gorm:"column:start_ts" json:"start_ts"`
	EndTS       time.Time  `gorm:"column:end_ts" json:"end_ts"`
	// CloseReason — idle (idle-timeout terlampaui) | max_duration (batas pengaman keras).
	CloseReason       string  `gorm:"size:20;column:close_reason" json:"close_reason"`
	DiggingConfidence float64 `gorm:"type:numeric(5,4);column:digging_confidence" json:"digging_confidence"`
	// Source — "live" (dari live feed dashcam) | "recording" (dari file VOD rekaman,
	// fail-safe saat live putus). Dipakai dashboard menandai "data dari rekaman".
	Source            string    `gorm:"size:20;column:source;default:'live'" json:"source"`
	CreatedAt         time.Time `json:"created_at"`
}

func (LoadingCycle) TableName() string { return "loading_cycles" }

// BucketEvent — 1 bucket individual (1 baris = 1 siklus digging->non_digging selesai)
// dengan timestamp presisi, dikirim AI service tiap kali terdeteksi. Dipakai untuk
// mengelompokkan bucket jadi truk secara on-the-fly (lihat truck_grouping.go) — beda
// dengan LoadingCycle di atas yang hanya menyimpan agregat per-siklus idle-timeout,
// tanpa timestamp per-bucket dan tanpa cap bucket/truk.
type BucketEvent struct {
	ID                uuid.UUID  `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()" json:"id"`
	CameraID          *uuid.UUID `gorm:"type:uuid" json:"camera_id"`
	Camera            *Camera    `json:"camera,omitempty"`
	ExcavatorID       *uuid.UUID `gorm:"type:uuid" json:"excavator_id"`
	Excavator         *Excavator `json:"excavator,omitempty"`
	DetectedAt        time.Time  `gorm:"column:detected_at" json:"detected_at"`
	// StartedAt — momen segmen digging ini dimulai (nullable, data historis sebelum
	// fitur timeline granular tidak punya nilai ini). DetectedAt tetap momen selesai.
	StartedAt         *time.Time `gorm:"column:started_at" json:"started_at"`
	DiggingConfidence float64    `gorm:"type:numeric(5,4);column:digging_confidence" json:"digging_confidence"`
	// Source — "live" (dari live feed dashcam) | "recording" (dari file VOD rekaman,
	// fail-safe saat live putus), konsisten dengan LoadingCycle.Source di atas.
	Source    string    `gorm:"size:20;column:source;default:'live'" json:"source"`
	CreatedAt time.Time `json:"created_at"`
}

func (BucketEvent) TableName() string { return "bucket_events" }

// CameraIncident — 1 episode gangguan koneksi dashcam (dari terdeteksi putus sampai
// dikonfirmasi pulih) + tindak lanjut operator. Lihat internal/service/camera_incident.go.
type CameraIncident struct {
	ID          uuid.UUID  `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()" json:"id"`
	CameraID    uuid.UUID  `gorm:"type:uuid" json:"camera_id"`
	Camera      *Camera    `json:"camera,omitempty"`
	DetectedAt  time.Time  `gorm:"column:detected_at" json:"detected_at"`
	LastDataAt  *time.Time `gorm:"column:last_data_at" json:"last_data_at"`
	// ReasonCode — jenis gangguan: CONNECTION_LOST (default/lama, koneksi basi) |
	// SD_FULL | SD_WRITE_FAILURE | SD_WRITE_PROTECTED | SD_RW_FAILURE | SD_NOT_DETECTED |
	// RTC_LOW_BATTERY | GPS_NO_SIGNAL | SIM_INACTIVE | FLEET_DISCONNECTED. Nullable untuk
	// incident lama sebelum klasifikasi ini ada (diperlakukan sebagai CONNECTION_LOST).
	ReasonCode *string `gorm:"size:40;column:reason_code" json:"reason_code"`
	// Status — BELUM_DIPERIKSA (default) | SEDANG_DITANGANI | SUDAH_PULIH.
	Status     string     `gorm:"size:20" json:"status"`
	Notes      string     `gorm:"type:text" json:"notes"`
	CheckedBy  *uuid.UUID `gorm:"type:uuid" json:"checked_by"`
	ResolvedAt *time.Time `json:"resolved_at"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

func (CameraIncident) TableName() string { return "camera_incidents" }

type Alert struct {
	ID             uuid.UUID  `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()" json:"id"`
	Type           string     `gorm:"size:50" json:"type"`
	Severity       string     `gorm:"size:20" json:"severity"`
	Title          string     `gorm:"size:200" json:"title"`
	Message        string     `gorm:"type:text" json:"message"`
	RelatedTable   string     `gorm:"size:50" json:"related_table"`
	RelatedID      *uuid.UUID `gorm:"type:uuid" json:"related_id"`
	SnapshotURL    string     `json:"snapshot_url"`
	IsRead         bool       `json:"is_read"`
	AcknowledgedBy *uuid.UUID `gorm:"type:uuid" json:"acknowledged_by"`
	AcknowledgedAt *time.Time `json:"acknowledged_at"`
	CreatedAt      time.Time  `json:"created_at"`
}

func (Alert) TableName() string { return "alerts" }

type Notification struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()" json:"id"`
	UserID    uuid.UUID `gorm:"type:uuid" json:"user_id"`
	Type      string    `gorm:"size:30" json:"type"`
	Title     string    `gorm:"size:200" json:"title"`
	Body      string    `gorm:"type:text" json:"body"`
	DataJSONB string    `gorm:"column:data_jsonb;type:jsonb" json:"data"`
	IsRead    bool      `json:"is_read"`
	CreatedAt time.Time `json:"created_at"`
}

func (Notification) TableName() string { return "notifications" }

type AuditLog struct {
	ID        int64      `gorm:"primaryKey" json:"id"`
	UserID    *uuid.UUID `gorm:"type:uuid" json:"user_id"`
	Action    string     `gorm:"size:80" json:"action"`
	Entity    string     `gorm:"size:80" json:"entity"`
	EntityID  string     `gorm:"size:80" json:"entity_id"`
	OldValue  string     `gorm:"type:jsonb" json:"old_value"`
	NewValue  string     `gorm:"type:jsonb" json:"new_value"`
	IP        string     `gorm:"size:64" json:"ip"`
	UserAgent string     `json:"user_agent"`
	TS        time.Time  `gorm:"column:ts" json:"ts"`
}

func (AuditLog) TableName() string { return "audit_logs" }

type SystemSetting struct {
	Key         string    `gorm:"size:80;primaryKey" json:"key"`
	ValueJSONB  string    `gorm:"column:value_jsonb;type:jsonb" json:"value"`
	Description string    `gorm:"type:text" json:"description"`
	UpdatedBy   *uuid.UUID `gorm:"type:uuid" json:"updated_by"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (SystemSetting) TableName() string { return "system_settings" }
