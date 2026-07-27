package domain

import (
	"time"

	"github.com/google/uuid"
)

type BaseUUID struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()" json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Role struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()" json:"id"`
	Code        string    `gorm:"size:50;unique" json:"code"`
	Name        string    `gorm:"size:100" json:"name"`
	Description string    `gorm:"type:text" json:"description"`
	IsSystem    bool      `json:"is_system"`
	Permissions []Permission `gorm:"many2many:role_permissions;" json:"permissions,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (Role) TableName() string { return "roles" }

type Permission struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()" json:"id"`
	Code        string    `gorm:"size:100;unique" json:"code"`
	Name        string    `gorm:"size:150" json:"name"`
	Module      string    `gorm:"size:50" json:"module"`
	Description string    `gorm:"type:text" json:"description"`
}

func (Permission) TableName() string { return "permissions" }

type User struct {
	ID           uuid.UUID  `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()" json:"id"`
	RoleID       uuid.UUID  `gorm:"type:uuid" json:"role_id"`
	Role         *Role      `json:"role,omitempty"`
	Email        string     `gorm:"size:160;unique" json:"email"`
	Phone        string     `gorm:"size:25" json:"phone"`
	PasswordHash string     `gorm:"size:255" json:"-"`
	FullName     string     `gorm:"size:150" json:"full_name"`
	AvatarURL    string     `json:"avatar_url"`
	Status       string     `gorm:"size:20" json:"status"`
	LastLoginAt  *time.Time `json:"last_login_at"`
	CustomerID   *uuid.UUID `gorm:"type:uuid" json:"customer_id"`
	// ExcavatorID — excavator yang dioperasikan oleh user bertipe OPERATOR.
	ExcavatorID  *uuid.UUID `gorm:"type:uuid" json:"excavator_id"`
	Excavator    *Excavator `json:"excavator,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

func (User) TableName() string { return "users" }

type RefreshToken struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()" json:"id"`
	UserID    uuid.UUID `gorm:"type:uuid" json:"user_id"`
	TokenHash string    `gorm:"size:255" json:"-"`
	ExpiresAt time.Time `json:"expires_at"`
	Revoked   bool      `json:"revoked"`
	IP        string    `gorm:"size:64" json:"ip"`
	UserAgent string    `json:"user_agent"`
	CreatedAt time.Time `json:"created_at"`
}

func (RefreshToken) TableName() string { return "refresh_tokens" }

type Customer struct {
	ID         uuid.UUID `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()" json:"id"`
	Code       string    `gorm:"size:30;unique" json:"code"`
	Name       string    `gorm:"size:180" json:"name"`
	Type       string    `gorm:"size:20" json:"type"`
	NPWP       string    `gorm:"size:30" json:"npwp"`
	Phone      string    `gorm:"size:25" json:"phone"`
	Email      string    `gorm:"size:160" json:"email"`
	Address    string    `gorm:"type:text" json:"address"`
	City       string    `gorm:"size:80" json:"city"`
	Province   string    `gorm:"size:80" json:"province"`
	Notes      string    `gorm:"type:text" json:"notes"`
	ImageURL   string    `json:"image_url"`
	Status     string    `gorm:"size:20" json:"status"`
	CreatedBy  *uuid.UUID `gorm:"type:uuid" json:"created_by"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func (Customer) TableName() string { return "customers" }

type Material struct {
	ID           uuid.UUID `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()" json:"id"`
	Code         string    `gorm:"size:30;unique" json:"code"`
	Name         string    `gorm:"size:150" json:"name"`
	Unit         string    `gorm:"size:20" json:"unit"`
	PricePerUnit float64   `gorm:"type:numeric(15,2)" json:"price_per_unit"`
	Stock        float64   `gorm:"type:numeric(15,2)" json:"stock"`
	MinStock     float64   `gorm:"type:numeric(15,2)" json:"min_stock"`
	Description  string    `gorm:"type:text" json:"description"`
	ImageURL     string    `json:"image_url"`
	Status       string    `gorm:"size:20" json:"status"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func (Material) TableName() string { return "materials" }

type Category struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()" json:"id"`
	Name      string    `gorm:"size:80;unique" json:"name"`
	ImageURL  string    `json:"image_url"`
	Sort      int       `json:"sort"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (Category) TableName() string { return "categories" }

type Vendor struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()" json:"id"`
	Code      string    `gorm:"size:30;unique" json:"code"`
	Name      string    `gorm:"size:180" json:"name"`
	Phone     string    `gorm:"size:25" json:"phone"`
	Email     string    `gorm:"size:160" json:"email"`
	Address   string    `gorm:"type:text" json:"address"`
	PicName   string    `gorm:"size:120" json:"pic_name"`
	PicPhone  string    `gorm:"size:25" json:"pic_phone"`
	ImageURL  string    `json:"image_url"`
	Status    string    `gorm:"size:20" json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (Vendor) TableName() string { return "vendors" }

type Truck struct {
	ID          uuid.UUID  `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()" json:"id"`
	PlateNumber string     `gorm:"size:20;unique" json:"plate_number"`
	VendorID    *uuid.UUID `gorm:"type:uuid" json:"vendor_id"`
	Vendor      *Vendor    `json:"vendor,omitempty"`
	Type        string     `gorm:"size:50" json:"type"`
	Brand       string     `gorm:"size:80" json:"brand"`
	CapacityTon float64    `gorm:"type:numeric(10,2)" json:"capacity_ton"`
	Year        int        `json:"year"`
	ImageURL    string     `json:"image_url"`
	Status      string     `gorm:"size:20" json:"status"`
	Notes       string     `gorm:"type:text" json:"notes"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

func (Truck) TableName() string { return "trucks" }

type Driver struct {
	ID        uuid.UUID  `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()" json:"id"`
	Name      string     `gorm:"size:150" json:"name"`
	Phone     string     `gorm:"size:25" json:"phone"`
	LicenseNo string     `gorm:"size:50" json:"license_no"`
	VendorID  *uuid.UUID `gorm:"type:uuid" json:"vendor_id"`
	Vendor    *Vendor    `json:"vendor,omitempty"`
	PhotoURL  string     `json:"photo_url"`
	Status    string     `gorm:"size:20" json:"status"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

func (Driver) TableName() string { return "drivers" }

type Camera struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()" json:"id"`
	Code      string    `gorm:"size:30;unique" json:"code"`
	Name      string    `gorm:"size:120" json:"name"`
	Area      string    `gorm:"size:50" json:"area"`
	IPAddress string    `gorm:"size:60" json:"ip_address"`
	StreamURL string    `json:"stream_url"`
	Location  string    `gorm:"size:150" json:"location"`
	Lat       float64   `gorm:"type:numeric(10,7)" json:"lat"`
	Lng       float64   `gorm:"type:numeric(10,7)" json:"lng"`
	ImageURL  string    `json:"image_url"`
	Status    string    `gorm:"size:20" json:"status"`
	// LastSeenAt — jam heartbeat terakhir diterima dari AI service (lihat
	// CCTVService.RecordHeartbeat). Dipakai menentukan status "offline" (heartbeat
	// terlalu lama = pipeline/kamera mati atau network putus).
	LastSeenAt *time.Time `json:"last_seen_at"`
	// LastActivityStatus — "digging" | "non_digging", hasil klasifikasi AI TERKINI
	// dari heartbeat terakhir. Dasar badge status real-time (Aktif/Idle) di dashboard,
	// berbeda dari Offline yang ditentukan dari basi-tidaknya LastSeenAt.
	LastActivityStatus string `gorm:"size:20;column:last_activity_status" json:"last_activity_status"`
	// LastActivitySource — "live" | "recording", asal data heartbeat terakhir
	// (dari fail-safe auto-failover AI service).
	LastActivitySource string `gorm:"size:20;column:last_activity_source" json:"last_activity_source"`
	// Field device-health (diisi DeviceHealthPoller, lihat internal/service/device_health.go)
	// dari command-socket dashcam -- semua nullable/zero-value untuk kamera non-blackvue
	// (video:// test) atau sebelum siklus poll pertama jalan.
	SDStatus         *string    `gorm:"size:30;column:sd_status" json:"sd_status"`
	SDAvailKB        *int64     `gorm:"column:sd_avail_kb" json:"sd_avail_kb"`
	GPSNoSignalSince *time.Time `gorm:"column:gps_no_signal_since" json:"gps_no_signal_since"`
	RTCLowBattery    bool       `gorm:"column:rtc_low_battery;default:false" json:"rtc_low_battery"`
	SIMStatus        *string    `gorm:"size:30;column:sim_status" json:"sim_status"`
	HealthCheckedAt  *time.Time `gorm:"column:health_checked_at" json:"health_checked_at"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

func (Camera) TableName() string { return "cameras" }

// Excavator — alat berat pemuat (loader) di area loading. Setiap excavator punya
// kode unik agar scanner Loading Start/End dapat mencatat excavator mana yang memuat.
type Excavator struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()" json:"id"`
	Code      string    `gorm:"size:30;unique" json:"code"`
	Name      string    `gorm:"size:120" json:"name"`
	Brand     string    `gorm:"size:80" json:"brand"`
	Model     string    `gorm:"size:80" json:"model"`
	// StandardBuckets — jumlah bucket standar untuk memuat 1 truck penuh (dipakai
	// sebagai acuan deteksi anomali terhadap pembacaan AI CCTV).
	StandardBuckets int        `gorm:"column:standard_buckets" json:"standard_buckets"`
	// CameraID — kamera CCTV yang memantau area loading excavator ini.
	CameraID        *uuid.UUID `gorm:"type:uuid" json:"camera_id"`
	Camera          *Camera    `json:"camera,omitempty"`
	ImageURL        string     `json:"image_url"`
	Status          string     `gorm:"size:20" json:"status"`
	Notes           string     `gorm:"type:text" json:"notes"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (Excavator) TableName() string { return "excavators" }
