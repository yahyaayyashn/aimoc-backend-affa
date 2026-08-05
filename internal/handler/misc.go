package handler

import (
	"encoding/json"
	"strings"
	"time"

	"aimoc-backend/internal/domain"
	"aimoc-backend/internal/middleware"
	"aimoc-backend/internal/service"
	wshub "aimoc-backend/internal/websocket"
	"aimoc-backend/pkg/utils"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type MiscHandler struct{ DB *gorm.DB }

func NewMiscHandler(db *gorm.DB) *MiscHandler { return &MiscHandler{DB: db} }

// ============== LOADING CYCLES (aktivitas AI mandiri) ==============

// ListLoadingCycles mengembalikan baris MENTAH loading_cycles (1 baris = 1 siklus
// loading yang dideteksi AI secara mandiri, lihat cycle_detector.py) -- sumber utama
// data aktivitas AI-only: dipakai dashboard & laporan untuk detail per siklus
// (bucket_count, durasi, confidence, alasan tutup, source live/recording).
func (h *MiscHandler) ListLoadingCycles(c *fiber.Ctx) error {
	p := utils.GetPagination(c)
	tx := h.DB.Model(&domain.LoadingCycle{}).
		Preload("Excavator").
		Preload("Camera").
		Preload("AIVisionAnalysis").
		Joins("JOIN excavators ON excavators.id = loading_cycles.excavator_id").
		Where("excavators.is_test = ?", isTestViewer(h.DB, c))
	if id := c.Query("id", ""); id != "" {
		tx = tx.Where("id = ?", id)
	}
	if from := c.Query("from", ""); from != "" {
		tx = tx.Where("start_ts::date >= ?", from)
	}
	if to := c.Query("to", ""); to != "" {
		tx = tx.Where("start_ts::date <= ?", to)
	}
	if excavatorID := c.Query("excavator_id", ""); excavatorID != "" {
		tx = tx.Where("excavator_id = ?", excavatorID)
	}
	var total int64
	tx.Count(&total)
	var data []domain.LoadingCycle
	tx.Limit(p.PerPage).Offset(p.Offset).Order("start_ts DESC").Find(&data)
	return utils.OKMeta(c, "OK", data, utils.MakeMeta(p, total))
}

func notifyCctvLog() {
	if wshub.Default == nil {
		return
	}
	wshub.Default.Broadcast([]string{"admin", "dashboard"}, wshub.Event{
		Type: "CCTV_LOG_DELETED", Channel: "admin",
		Data: map[string]interface{}{}, Timestamp: time.Now().Format(time.RFC3339),
	})
}

// DeleteLoadingCycle — hapus 1 baris siklus AI (mis. reset data testing).
func (h *MiscHandler) DeleteLoadingCycle(c *fiber.Ctx) error {
	if err := h.DB.Delete(&domain.LoadingCycle{}, "id = ?", c.Params("id")).Error; err != nil {
		return utils.BadRequest(c, err.Error(), nil)
	}
	notifyCctvLog()
	return utils.OK(c, "Siklus loading dihapus", nil)
}

// ============== ALERTS ==============

func (h *MiscHandler) ListAlerts(c *fiber.Ctx) error {
	p := utils.GetPagination(c)
	var data []domain.Alert
	var total int64
	h.DB.Model(&domain.Alert{}).Count(&total)
	h.DB.Limit(p.PerPage).Offset(p.Offset).Order("created_at DESC").Find(&data)
	return utils.OKMeta(c, "OK", data, utils.MakeMeta(p, total))
}

func (h *MiscHandler) AckAlert(c *fiber.Ctx) error {
	uid, _ := c.Locals(middleware.CtxUserID).(uuid.UUID)
	now := time.Now()
	h.DB.Model(&domain.Alert{}).Where("id = ?", c.Params("id")).Updates(map[string]interface{}{
		"is_read": true, "acknowledged_by": uid, "acknowledged_at": &now,
	})
	return utils.OK(c, "Alert ditandai dibaca", nil)
}

// ============== NOTIFICATIONS ==============

func (h *MiscHandler) ListNotifs(c *fiber.Ctx) error {
	uid, _ := c.Locals(middleware.CtxUserID).(uuid.UUID)
	p := utils.GetPagination(c)
	var data []domain.Notification
	var total int64
	h.DB.Model(&domain.Notification{}).Where("user_id = ?", uid).Count(&total)
	h.DB.Where("user_id = ?", uid).Limit(p.PerPage).Offset(p.Offset).
		Order("created_at DESC").Find(&data)
	return utils.OKMeta(c, "OK", data, utils.MakeMeta(p, total))
}

func (h *MiscHandler) MarkNotifRead(c *fiber.Ctx) error {
	h.DB.Model(&domain.Notification{}).Where("id = ?", c.Params("id")).Update("is_read", true)
	return utils.OK(c, "Notifikasi ditandai dibaca", nil)
}

func (h *MiscHandler) MarkAllNotifRead(c *fiber.Ctx) error {
	uid, _ := c.Locals(middleware.CtxUserID).(uuid.UUID)
	h.DB.Model(&domain.Notification{}).Where("user_id = ?", uid).Update("is_read", true)
	return utils.OK(c, "Semua notifikasi ditandai dibaca", nil)
}

// ============== DASHBOARD (aktivitas AI) ==============

// operationalHours — jam mulai/selesai operasi quarry, dipakai sebagai pembagi
// utilization %. Configurable lewat Pengaturan (system_settings key
// "operational_hours"), default 07:00-17:00 (asumsi awal, lihat riset metrik AI).
type operationalHours struct {
	StartHour int `json:"start_hour"`
	EndHour   int `json:"end_hour"`
}

func (h *MiscHandler) getOperationalHours() operationalHours {
	oh := operationalHours{StartHour: 7, EndHour: 17}
	var raw string
	h.DB.Raw(`SELECT value_jsonb::text FROM system_settings WHERE key = 'operational_hours'`).Scan(&raw)
	if raw == "" {
		return oh
	}
	_ = json.Unmarshal([]byte(raw), &oh)
	if oh.EndHour <= oh.StartHour || oh.StartHour < 0 || oh.EndHour > 24 {
		return operationalHours{StartHour: 7, EndHour: 17} // guard nilai rusak/tak masuk akal
	}
	return oh
}

// heartbeatOfflineThresholdSec — heartbeat AI service dikirim tiap ~10-15s (lihat
// HEARTBEAT_INTERVAL_SEC di ai-service); kamera dianggap offline kalau last_seen_at
// lebih basi dari ini (network putus / pipeline mati), bukan sekadar idle.
const heartbeatOfflineThresholdSec = 90

// getRevenuePerTruck — dulu hardcoded (const RevenuePerTruck), dipindah ke
// system_settings (01 Agu 2026) supaya bisa diubah tanpa deploy ulang. Default tetap
// Rp250.000 (keputusan tim 23 Jul 2026) kalau key belum pernah di-set.
func (h *MiscHandler) getRevenuePerTruck() float64 {
	v := 250000.0
	var raw string
	h.DB.Raw(`SELECT value_jsonb::text FROM system_settings WHERE key = 'REVENUE_PER_TRUCK'`).Scan(&raw)
	if raw == "" {
		return v
	}
	_ = json.Unmarshal([]byte(raw), &v)
	if v <= 0 {
		return 250000.0
	}
	return v
}

// getTruckGroupGapSec — dulu hardcoded (service.TruckGroupGapSec), dipindah ke
// system_settings (01 Agu 2026). Default tetap 300s/5 menit. Ubah nilai ini mengubah
// klasifikasi Overload/Underload/Unvalidated -- bukan cuma angka harga, lihat
// truck_grouping.go.
func (h *MiscHandler) getTruckGroupGapSec() int {
	v := service.TruckGroupGapSec
	var raw string
	h.DB.Raw(`SELECT value_jsonb::text FROM system_settings WHERE key = 'TRUCK_GROUP_GAP_SEC'`).Scan(&raw)
	if raw == "" {
		return v
	}
	_ = json.Unmarshal([]byte(raw), &v)
	if v <= 0 {
		return service.TruckGroupGapSec
	}
	return v
}

// DashboardManajemen — ringkasan aktivitas excavator hari ini, bersumber murni dari
// loading_cycles (deteksi AI mandiri) + alerts + heartbeat kamera. Metrik transaksi
// (revenue/truck/ton) sudah dibuang pada pivot AI-only. Headline: status real-time
// (aktif/idle/offline) + utilization % -- BUKAN bucket count (lihat riset metrik AI,
// bucket count cuma info sekunder di drill-down).
func (h *MiscHandler) DashboardManajemen(c *fiber.Ctx) error {
	today := time.Now().Format("2006-01-02")
	oh := h.getOperationalHours()

	var siklusHariIni int64
	h.DB.Model(&domain.LoadingCycle{}).Where("start_ts::date = ?", today).Count(&siklusHariIni)

	var bucketHariIni int64
	h.DB.Model(&domain.LoadingCycle{}).Where("start_ts::date = ?", today).
		Select("COALESCE(SUM(bucket_count),0)").Scan(&bucketHariIni)

	var durasiHariIni int64
	h.DB.Model(&domain.LoadingCycle{}).Where("start_ts::date = ?", today).
		Select("COALESCE(SUM(duration_sec),0)").Scan(&durasiHariIni)

	var fromRecording int64
	h.DB.Model(&domain.LoadingCycle{}).Where("start_ts::date = ? AND source = ?", today, "recording").Count(&fromRecording)

	var alertOpen int64
	h.DB.Model(&domain.Alert{}).Where("is_read = false").Count(&alertOpen)

	// Jam operasional yang SUDAH LEWAT hari ini, dalam detik -- pembagi utilization %.
	// Contoh: jam operasional 07-17, sekarang jam 12:00 -> 5 jam (18000 detik) sudah lewat.
	now := time.Now()
	opStart := time.Date(now.Year(), now.Month(), now.Day(), oh.StartHour, 0, 0, 0, now.Location())
	opEnd := time.Date(now.Year(), now.Month(), now.Day(), oh.EndHour, 0, 0, 0, now.Location())
	elapsedSec := 0.0
	if now.After(opStart) {
		cappedNow := now
		if now.After(opEnd) {
			cappedNow = opEnd
		}
		elapsedSec = cappedNow.Sub(opStart).Seconds()
	}

	// Status + utilization per excavator (headline dashboard).
	type excStatusRow struct {
		ID                 string     `json:"id"`
		Code               string     `json:"code"`
		Name               string     `json:"name"`
		LastActivityStatus string     `json:"last_activity_status"`
		LastActivitySource string     `json:"last_activity_source"`
		LastSeenAt         *time.Time `json:"last_seen_at"`
		DurasiAktifSec     int64      `json:"durasi_aktif_sec"`
		SiklusHariIni      int64      `json:"siklus_hari_ini"`
	}
	var excavators []excStatusRow
	h.DB.Raw(`SELECT e.id, e.code, e.name,
			COALESCE(cam.last_activity_status, '') AS last_activity_status,
			COALESCE(cam.last_activity_source, 'live') AS last_activity_source,
			cam.last_seen_at,
			COALESCE((SELECT SUM(lc.duration_sec) FROM loading_cycles lc
				WHERE lc.excavator_id = e.id AND lc.start_ts::date = ?), 0) AS durasi_aktif_sec,
			COALESCE((SELECT COUNT(*) FROM loading_cycles lc
				WHERE lc.excavator_id = e.id AND lc.start_ts::date = ?), 0) AS siklus_hari_ini
		FROM excavators e
		LEFT JOIN cameras cam ON cam.id = e.camera_id
		WHERE e.status = 'AKTIF'
		ORDER BY e.code ASC`, today, today).Scan(&excavators)

	type excStatus struct {
		ID             string  `json:"id"`
		Code           string  `json:"code"`
		Name           string  `json:"name"`
		Status         string  `json:"status"` // aktif | idle | offline
		Source         string  `json:"source"` // live | recording
		UtilizationPct float64 `json:"utilization_pct"`
		SiklusHariIni  int64   `json:"siklus_hari_ini"`
	}
	excStatuses := make([]excStatus, 0, len(excavators))
	aktifCount := 0
	var totalUtilization float64
	for _, e := range excavators {
		status := "offline"
		if e.LastSeenAt != nil && now.Sub(*e.LastSeenAt).Seconds() <= heartbeatOfflineThresholdSec {
			if e.LastActivityStatus == "digging" {
				status = "aktif"
				aktifCount++
			} else {
				status = "idle"
			}
		}
		util := 0.0
		if elapsedSec > 0 {
			util = float64(e.DurasiAktifSec) / elapsedSec * 100
			if util > 100 {
				util = 100
			}
		}
		totalUtilization += util
		source := e.LastActivitySource
		if strings.TrimSpace(source) == "" {
			source = "live"
		}
		excStatuses = append(excStatuses, excStatus{
			ID: e.ID, Code: e.Code, Name: e.Name,
			Status: status, Source: source,
			UtilizationPct: util, SiklusHariIni: e.SiklusHariIni,
		})
	}
	avgUtilization := 0.0
	if len(excStatuses) > 0 {
		avgUtilization = totalUtilization / float64(len(excStatuses))
	}

	// Siklus per jam hari ini.
	type bucket struct {
		Jam   int   `json:"jam"`
		Count int64 `json:"count"`
	}
	var perJam []bucket
	h.DB.Raw(`SELECT EXTRACT(HOUR FROM start_ts)::int AS jam, COUNT(*) AS count
		FROM loading_cycles WHERE start_ts::date = ?
		GROUP BY jam ORDER BY jam`, today).Scan(&perJam)

	return utils.OK(c, "OK", fiber.Map{
		"siklus_hari_ini":       siklusHariIni,
		"bucket_hari_ini":       bucketHariIni,
		"durasi_detik_hari_ini": durasiHariIni,
		"siklus_dari_rekaman":   fromRecording,
		"alert_open":            alertOpen,
		"excavator_aktif":       aktifCount,
		"excavator_total":       len(excStatuses),
		"utilization_avg_pct":   avgUtilization,
		"excavators":            excStatuses,
		"siklus_per_jam":        perJam,
		"operational_hours":     oh,
	})
}

// ============== REPORTS (aktivitas AI) ==============

// ActivityReport — laporan aktivitas AI per excavator, per hari/bulan/rentang custom,
// bersumber dari loading_cycles. Menggantikan report "AI vs Manual" lama: karena
// lapisan transaksi (manual) sudah dibuang, laporan ini murni statistik deteksi AI
// (jumlah siklus, total bucket, total durasi), sekaligus pecahan live vs recording.
func (h *MiscHandler) ActivityReport(c *fiber.Ctx) error {
	from := c.Query("from", "")
	to := c.Query("to", "")
	granularity := c.Query("granularity", "day")
	excavatorID := c.Query("excavator_id", "")

	dateFmt := "YYYY-MM-DD"
	if granularity == "month" {
		dateFmt = "YYYY-MM"
	}

	type row struct {
		ExcavatorID   string `json:"excavator_id"`
		ExcavatorCode string `json:"excavator_code"`
		ExcavatorName string `json:"excavator_name"`
		Bucket        string `json:"bucket"`
		CycleCount    int64  `json:"cycle_count"`
		BucketTotal   int64  `json:"bucket_total"`
		DurationSec   int64  `json:"duration_sec"`
		FromLive      int64  `json:"from_live"`
		FromRecording int64  `json:"from_recording"`
	}

	tx := h.DB.Table("loading_cycles t").
		Select(`COALESCE(t.excavator_id::text, '') AS excavator_id,
			COALESCE(e.code,'') AS excavator_code, COALESCE(e.name,'') AS excavator_name,
			TO_CHAR(t.start_ts, '` + dateFmt + `') AS bucket,
			COUNT(*) AS cycle_count,
			COALESCE(SUM(t.bucket_count),0) AS bucket_total,
			COALESCE(SUM(t.duration_sec),0) AS duration_sec,
			COALESCE(SUM(CASE WHEN t.source = 'recording' THEN 0 ELSE 1 END),0) AS from_live,
			COALESCE(SUM(CASE WHEN t.source = 'recording' THEN 1 ELSE 0 END),0) AS from_recording`).
		Joins("LEFT JOIN excavators e ON e.id = t.excavator_id").
		Group("t.excavator_id, e.code, e.name, bucket").
		Order("bucket ASC, excavator_code ASC")
	if from != "" {
		tx = tx.Where("t.start_ts::date >= ?", from)
	}
	if to != "" {
		tx = tx.Where("t.start_ts::date <= ?", to)
	}
	if excavatorID != "" {
		tx = tx.Where("t.excavator_id = ?", excavatorID)
	}

	var rows []row
	tx.Scan(&rows)
	return utils.OK(c, "OK", rows)
}

// ============== HEALTH ==============

func (h *MiscHandler) Health(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{"status": "ok", "time": time.Now().Format(time.RFC3339)})
}

// ============== ROLES / PERMISSIONS / USERS (Super Admin) ==============

func (h *MiscHandler) ListRoles(c *fiber.Ctx) error {
	var data []domain.Role
	h.DB.Preload("Permissions").Order("name").Find(&data)
	return utils.OK(c, "OK", data)
}

func (h *MiscHandler) ListPermissions(c *fiber.Ctx) error {
	var data []domain.Permission
	h.DB.Order("module, code").Find(&data)
	return utils.OK(c, "OK", data)
}

func (h *MiscHandler) ListUsers(c *fiber.Ctx) error {
	p := utils.GetPagination(c)
	var data []domain.User
	var total int64
	h.DB.Model(&domain.User{}).Count(&total)
	h.DB.Preload("Role").Limit(p.PerPage).Offset(p.Offset).Order("created_at DESC").Find(&data)
	return utils.OKMeta(c, "OK", data, utils.MakeMeta(p, total))
}

// UpdateUser memperbarui profil user (termasuk foto/avatar) — hanya field yang dikirim.
func (h *MiscHandler) UpdateUser(c *fiber.Ctx) error {
	var u domain.User
	if err := h.DB.First(&u, "id = ?", c.Params("id")).Error; err != nil {
		return utils.NotFound(c, "User tidak ditemukan")
	}
	var body struct {
		FullName  *string `json:"full_name"`
		Phone     *string `json:"phone"`
		Status    *string `json:"status"`
		AvatarURL *string `json:"avatar_url"`
	}
	if err := c.BodyParser(&body); err != nil {
		return utils.BadRequest(c, "Body tidak valid", nil)
	}
	updates := map[string]interface{}{}
	if body.FullName != nil {
		updates["full_name"] = *body.FullName
	}
	if body.Phone != nil {
		updates["phone"] = *body.Phone
	}
	if body.Status != nil {
		updates["status"] = *body.Status
	}
	if body.AvatarURL != nil {
		updates["avatar_url"] = *body.AvatarURL
	}
	if len(updates) > 0 {
		if err := h.DB.Model(&u).Updates(updates).Error; err != nil {
			return utils.BadRequest(c, err.Error(), nil)
		}
	}
	h.DB.Preload("Role").First(&u, "id = ?", u.ID)
	return utils.OK(c, "User diperbarui", u)
}

func (h *MiscHandler) ListAuditLogs(c *fiber.Ctx) error {
	p := utils.GetPagination(c)
	var data []domain.AuditLog
	var total int64
	h.DB.Model(&domain.AuditLog{}).Count(&total)
	h.DB.Limit(p.PerPage).Offset(p.Offset).Order("ts DESC").Find(&data)
	return utils.OKMeta(c, "OK", data, utils.MakeMeta(p, total))
}

// ============== SETTINGS ==============

func (h *MiscHandler) ListSettings(c *fiber.Ctx) error {
	var data []domain.SystemSetting
	h.DB.Order("key").Find(&data)
	return utils.OK(c, "OK", data)
}

// UpdateSettings — upsert 1 key system_settings (mis. "operational_hours"). Value
// dikirim sebagai JSON mentah (string), disimpan apa adanya ke value_jsonb.
func (h *MiscHandler) UpdateSettings(c *fiber.Ctx) error {
	uid, _ := c.Locals(middleware.CtxUserID).(uuid.UUID)
	var body struct {
		Value       string `json:"value" validate:"required"`
		Description string `json:"description"`
	}
	if err := c.BodyParser(&body); err != nil {
		return utils.BadRequest(c, "Body tidak valid", nil)
	}
	key := c.Params("key")
	res := h.DB.Exec(`INSERT INTO system_settings (key, value_jsonb, description, updated_by, updated_at)
		VALUES (?, ?::jsonb, ?, ?, NOW())
		ON CONFLICT (key) DO UPDATE SET value_jsonb = EXCLUDED.value_jsonb,
			description = EXCLUDED.description, updated_by = EXCLUDED.updated_by, updated_at = NOW()`,
		key, body.Value, body.Description, uid)
	if res.Error != nil {
		return utils.BadRequest(c, res.Error.Error(), nil)
	}
	var updated domain.SystemSetting
	h.DB.First(&updated, "key = ?", key)
	return utils.OK(c, "Pengaturan diperbarui", updated)
}
