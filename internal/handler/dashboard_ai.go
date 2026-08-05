package handler

import (
	"encoding/json"
	"math"
	"sort"
	"time"

	"aimoc-backend/internal/domain"
	"aimoc-backend/internal/service"
	"aimoc-backend/pkg/utils"

	"github.com/gofiber/fiber/v2"
)

// Revenue per truk (dulu const RevenuePerTruck=250000, keputusan tim 23 Jul 2026) kini
// disimpan di system_settings key REVENUE_PER_TRUCK -- lihat (*MiscHandler).getRevenuePerTruck
// di misc.go, default tetap Rp250.000 kalau key belum di-set.

type produktivitasExcavatorRow struct {
	ExcavatorID   string `json:"excavator_id"`
	ExcavatorCode string `json:"excavator_code"`
	ExcavatorName string `json:"excavator_name"`
	// VolumeM3/TruckIdentified -- basis AI Vision sejak 03 Agu 2026 (keputusan user:
	// "dashboard produktivitas pakai AI Vision aja"), BUKAN lagi pengelompokan bucket.
	// Diambil LANGSUNG dari field trucks_identified/productivity_loading_m3 punya
	// Gracia (05 Agu 2026 -- lihat komentar aiVisionKPIAgg), BUKAN lagi hitungan sendiri
	// (TruckIdentified*standard_buckets, salah karena buang truk underfilled/pending).
	// trigger_source 'auto' saja untuk excavator produksi (manual/halaman Analisa Video
	// AI dikecualikan -- TAPI ikut dihitung untuk excavator is_test=true, lihat
	// getAIVisionKPI). Cakupannya cuma siklus yang KEBETULAN sudah dianalisa AI Vision,
	// bukan seluruh aktivitas -- lihat getAIVisionKPI.
	VolumeM3        int `json:"volume_m3"`
	TruckIdentified int `json:"truck_identified"`
	// RevenueTercatat/EstimasiRevenue -- basis AI Vision sejak 04 Agu 2026 (keputusan
	// user: revenue harus konsisten dgn m3/truk yg sudah AI Vision-based). Rate tetap
	// dari REVENUE_PER_TRUCK (Settings kita), BUKAN dari revenue_recorded_idr bawaan
	// AI Vision -- lihat komentar getAIVisionKPI.
	RevenueTercatat float64 `json:"revenue_tercatat"`
	EstimasiRevenue float64 `json:"estimasi_revenue"`
	// Sesuai/Underload/Overload/Pending -- basis AI Vision sejak 04 Agu 2026 (sebelumnya
	// basis bucket/TruckGroup, keputusan user: chart Overload Analysis konsisten dgn
	// Revenue/Volume yg sudah AI Vision-based). Dipakai chart Overload Analysis &
	// MiniStat di DetailExcavator. PendingBucketM3 TETAP basis bucket (lihat komentar di
	// loop) -- itu volume mentah, beda concern dari 4 count di atas.
	Sesuai          int `json:"sesuai"`
	Underload       int `json:"underload"`
	Overload        int `json:"overload"`
	Pending         int `json:"pending"`
	PendingBucketM3 int `json:"pending_bucket_m3"`
}

type produktivitasLogRow struct {
	ExcavatorID   string    `json:"excavator_id"`
	ExcavatorCode string    `json:"excavator_code"`
	ExcavatorName string    `json:"excavator_name"`
	StartTS       time.Time `json:"start_ts"`
	EndTS         time.Time `json:"end_ts"`
	BucketCount   int       `json:"bucket_count"`
	VolumeM3      int       `json:"volume_m3"`
	FillStatus    string    `json:"fill_status"`
	Unvalidated   bool      `json:"unvalidated"`
	Revenue       float64   `json:"revenue"`
}

type produktivitasRevenueResp struct {
	ProduktivitasLoadingM3 int     `json:"produktivitas_loading_m3"`
	TruckIdentified        int     `json:"truck_identified"`
	UnvalidatedVolumeM3    int     `json:"unvalidated_volume_m3"`
	RevenueTercatat        float64 `json:"revenue_tercatat"`
	EstimasiTotalRevenue   float64 `json:"estimasi_total_revenue"`
	// RevenuePerTruck -- nilai REVENUE_PER_TRUCK yang lagi dipakai (system_settings,
	// bisa diubah admin di Pengaturan). Diekspos supaya FE bisa tampilkan caption
	// "× Rp{revenue_per_truck}" yang akurat, bukan hardcoded Rp250.000.
	RevenuePerTruck float64                     `json:"revenue_per_truck"`
	PerExcavator    []produktivitasExcavatorRow `json:"per_excavator"`
	LogAktivitas    []produktivitasLogRow       `json:"log_aktivitas"`
}

// ProduktivitasRevenue — "01 - Dashboard Produktivitas & Revenue" (AI Only), lihat
// Dokumen_Penjelasan_Dashboard_AIMOC.pdf. KPI utama (Volume/Truk/Revenue/Unvalidated)
// basis AI Vision (getAIVisionKPI). Log Aktivitas & chart Overload Analysis TETAP
// menghitung pengelompokan truk on-the-fly dari bucket_events mentah (lihat
// internal/service/truck_grouping.go) -- TIDAK ada state tersimpan selain event
// mentahnya sendiri, jadi selalu bisa direkonsiliasi ulang kalau
// parameter algoritma (standard_buckets, gap threshold) berubah.
func (h *MiscHandler) ProduktivitasRevenue(c *fiber.Ctx) error {
	from := c.Query("from", "")
	to := c.Query("to", "")
	excavatorID := c.Query("excavator_id", "")

	var excavators []domain.Excavator
	excTx := h.DB.Model(&domain.Excavator{}).Where("is_test = ?", isTestViewer(h.DB, c))
	if excavatorID != "" {
		excTx = excTx.Where("id = ?", excavatorID)
	}
	if err := excTx.Find(&excavators).Error; err != nil {
		return utils.BadRequest(c, err.Error(), nil)
	}

	var events []domain.BucketEvent
	evTx := h.DB.Model(&domain.BucketEvent{}).Order("excavator_id, detected_at ASC")
	if from != "" {
		evTx = evTx.Where("detected_at::date >= ?", from)
	}
	if to != "" {
		evTx = evTx.Where("detected_at::date <= ?", to)
	}
	if excavatorID != "" {
		evTx = evTx.Where("excavator_id = ?", excavatorID)
	}
	if err := evTx.Find(&events).Error; err != nil {
		return utils.BadRequest(c, err.Error(), nil)
	}

	eventsByExc := map[string][]domain.BucketEvent{}
	for _, e := range events {
		if e.ExcavatorID == nil {
			continue // belum tertaut excavator manapun di Master Data -- tidak masuk laporan per-unit
		}
		key := e.ExcavatorID.String()
		eventsByExc[key] = append(eventsByExc[key], e)
	}

	now := time.Now()
	revenuePerTruck := h.getRevenuePerTruck()
	resp := produktivitasRevenueResp{RevenuePerTruck: revenuePerTruck}
	gapSec := h.getTruckGroupGapSec()

	for _, exc := range excavators {
		excEvents := eventsByExc[exc.ID.String()]
		groups := service.ComputeTruckGroups(excEvents, exc.StandardBuckets, gapSec, now)

		row := produktivitasExcavatorRow{
			ExcavatorID:   exc.ID.String(),
			ExcavatorCode: exc.Code,
			ExcavatorName: exc.Name,
		}

		// Basis BUCKET -- HANYA dipakai untuk Log Aktivitas (audit per-siklus) & chart
		// Overload Analysis (Sesuai/Underload/Overload/Pending), BUKAN lagi untuk
		// Revenue/Unvalidated Volume (lihat basis AI Vision di bawah).
		for _, g := range groups {
			// logRow.VolumeM3 SELALU bucket count aktual (log/audit per-siklus).
			logRow := produktivitasLogRow{
				ExcavatorID:   exc.ID.String(),
				ExcavatorCode: exc.Code,
				ExcavatorName: exc.Name,
				StartTS:       g.StartTS,
				EndTS:         g.EndTS,
				BucketCount:   g.BucketCount,
				VolumeM3:      g.BucketCount,
				FillStatus:    g.FillStatus,
				Unvalidated:   g.Unvalidated,
			}
			if g.IsIdentified() {
				// Rate pada SAAT truk ini selesai (g.EndTS), bukan rate sekarang -- sama
				// alasan dengan sumHistoricalRevenue di getAIVisionKPI.
				logRow.Revenue = h.getRevenuePerTruckAt(g.EndTS)
			}
			// PendingBucketM3 TETAP basis bucket (volume mentah menunggu terbentuk jadi
			// truk) -- tidak ada padanan langsung di AI Vision, beda concern dari
			// Sesuai/Underload/Overload/Pending di bawah (sekarang basis AI Vision).
			if g.FillStatus == service.FillPending {
				row.PendingBucketM3 += g.BucketCount
			}
			resp.LogAktivitas = append(resp.LogAktivitas, logRow)
		}

		// Basis AI VISION -- Volume/Truk/Revenue/Unvalidated Volume (keputusan user
		// 03-04 Agu 2026, lihat komentar getAIVisionKPI).
		aiKPI := h.getAIVisionKPI(exc.Code, exc.IsTest, from, to)
		row.TruckIdentified = aiKPI.TruckIdentified
		row.VolumeM3 = int(math.Round(aiKPI.ProductivityLoadingM3))
		// RevenueTercatat -- sudah dihitung per-baris pakai rate historisnya masing-
		// masing (lihat sumHistoricalRevenue), BUKAN count*rate-sekarang lagi.
		row.RevenueTercatat = aiKPI.RevenueTercatat
		// EstimasiRevenue tetap pakai rate SEKARANG -- ini proyeksi/ceiling truk yang
		// belum final, wajar pakai asumsi harga hari ini (beda dari RevenueTercatat
		// yang harus jadi fakta terkunci begitu truk selesai).
		row.EstimasiRevenue = float64(aiKPI.RevenueBearingTrucks+aiKPI.PendingTrucks) * revenuePerTruck
		row.Sesuai = aiKPI.Sesuai
		row.Overload = aiKPI.Overload
		row.Underload = aiKPI.Underfilled
		row.Pending = aiKPI.Pending

		resp.ProduktivitasLoadingM3 += row.VolumeM3
		resp.TruckIdentified += row.TruckIdentified
		resp.RevenueTercatat += row.RevenueTercatat
		resp.EstimasiTotalRevenue += row.EstimasiRevenue
		resp.UnvalidatedVolumeM3 += aiKPI.UnvalidatedVolumeM3
		resp.PerExcavator = append(resp.PerExcavator, row)
	}

	sort.Slice(resp.PerExcavator, func(i, j int) bool {
		return resp.PerExcavator[i].ExcavatorCode < resp.PerExcavator[j].ExcavatorCode
	})
	sort.Slice(resp.LogAktivitas, func(i, j int) bool {
		return resp.LogAktivitas[i].StartTS.After(resp.LogAktivitas[j].StartTS)
	})

	return utils.OK(c, "OK", resp)
}

// aiVisionKPIAgg — agregat dari dashboard_summary.kpi semua analisa AI Vision completed
// (trigger_source='auto' saja, lihat getAIVisionKPI) milik 1 excavator dalam rentang
// tanggal. Sesuai/Overload/Underfilled/Pending = 4 kategori load_condition mentah dari
// AI Vision, dipakai chart Overload Analysis.
//
// TruckIdentified & ProductivityLoadingM3 -- diambil LANGSUNG dari field
// trucks_identified/productivity_loading_m3 Gracia (05 Agu 2026, dikonfirmasi lewat WA:
// truk yang cuma kegali sebagian, mis. 3 bucket, TETAP dihitung "teridentifikasi" &
// produktivitasnya = jumlah bucket aktual -- BUKAN cuma truk yang statusnya
// completed/overloaded). Sebelumnya kita re-derive TruckIdentified dari
// Sesuai+Overload saja & VolumeM3 dari TruckIdentified*standard_buckets -- salah, truk
// underfilled/pending jadi hilang dari hitungan (dibuktikan 05 Agu: dashboard kita
// akan nunjukkin 0 truk/0 m3 utk siklus yg Gracia sendiri bilang 3 truk/6 m3).
//
// RevenueBearingTrucks = Sesuai+Overload -- basis RevenueTercatat SAJA (uang yang
// sudah pasti/tervalidasi penuh, beda dari TruckIdentified yang masukin semua truk
// termasuk yang belum penuh). PendingTrucks = Underfilled+Pending, dipakai bareng
// RevenueBearingTrucks buat EstimasiRevenue (proyeksi ceiling kalau semua truk yang
// teridentifikasi akhirnya nyampe revenue penuh).
type aiVisionKPIAgg struct {
	Sesuai                int
	Overload              int
	Underfilled           int
	Pending               int
	TruckIdentified       int
	RevenueBearingTrucks  int
	PendingTrucks         int
	ProductivityLoadingM3 float64
	UnvalidatedVolumeM3   int
	// RevenueTercatat -- SUDAH dalam Rupiah (bukan count lagi), dijumlah per-baris
	// pakai rate yang berlaku SAAT baris itu selesai (getRevenuePerTruckAt, lihat
	// getAIVisionKPI) -- BUKAN RevenueBearingTrucks * rate-sekarang. Kalau admin ubah
	// REVENUE_PER_TRUCK hari ini, truk yang sudah selesai kemarin TETAP pakai rate
	// kemarin (05 Agu 2026, keluhan tim: "harusnya cuma truk yang akan datang yang
	// berubah").
	RevenueTercatat float64
}

// getAIVisionKPI — dashboard Produktivitas & Revenue 100% basis AI Vision sejak 04 Agu
// 2026 (keputusan user: revenue harus konsisten/"inline" dengan m3 & truk yg sudah AI
// Vision-based, bukan basis bucket_events/TruckGroup terpisah -- 2 sumber data paralel
// yang beda sebelumnya bikin bingung, lihat histori chat). Gracia's pipeline SEBENARNYA
// sudah kirim revenue_recorded_idr/estimated_total_revenue_idr sendiri di dashboard_summary,
// TAPI kita SENGAJA tidak pakai field itu -- rate di dalamnya kemungkinan hardcode di
// pipeline Python-nya, di luar kendali kita. Kita cuma pakai COUNT truk dari dia
// (completed/overloaded/pending/underfilled) + unvalidated_volume_bucket, dikali rate
// REVENUE_PER_TRUCK kita sendiri (getRevenuePerTruck, Settings kita) -- supaya revenue
// tetap bisa diubah admin kapan saja tanpa bergantung ke sistem tim AI.
// trigger_source='auto' SENGAJA difilter untuk excavator PRODUKSI (isTest=false) --
// halaman Analisa Video AI (manual) tetap ruang uji coba model, bukan data produksi
// (periode "manual ikut dihitung sementara", 03-04 Agu 2026, sudah berakhir seiring
// finalisasi dashboard). TAPI untuk excavator is_test=true (unit demo/testing, lihat
// migration 000031), manual JUSTRU ikut dihitung -- itu satu-satunya cara user demo
// menguji dashboard-nya sendiri (dashboard-nya sudah terisolasi total dari data
// produksi lewat is_test_viewer, jadi tidak ada risiko mencemari angka produksi lagi).
func (h *MiscHandler) getAIVisionKPI(unitCode string, isTestExcavator bool, from, to string) aiVisionKPIAgg {
	q := h.DB.Where("unit_id = ? AND status = 'completed'", unitCode)
	if isTestExcavator {
		q = q.Where("trigger_source IN ('auto', 'manual')")
	} else {
		q = q.Where("trigger_source = 'auto'")
	}
	if from != "" {
		q = q.Where("submitted_at::date >= ?", from)
	}
	if to != "" {
		q = q.Where("submitted_at::date <= ?", to)
	}
	var rows []domain.AIVisionAnalysis
	q.Find(&rows)
	agg := aggregateAIVisionKPI(rows)
	agg.RevenueTercatat = h.sumHistoricalRevenue(rows)
	return agg
}

// sumHistoricalRevenue -- terpisah dari aggregateAIVisionKPI (yang murni/tanpa DB,
// lihat komentarnya) karena butuh getRevenuePerTruckAt (query DB). Setiap baris
// dihitung pakai rate yang berlaku PERSIS saat baris itu selesai (FinishedAt, atau
// SubmittedAt kalau belum ada FinishedAt), bukan 1 rate sekarang dikali total count --
// itu yang bikin ganti harga hari ini diam-diam menghitung ulang revenue truk lama.
func (h *MiscHandler) sumHistoricalRevenue(rows []domain.AIVisionAnalysis) float64 {
	var total float64
	for _, r := range rows {
		if r.DashboardSummary == nil {
			continue
		}
		var parsed struct {
			KPI struct {
				CompletedTruckLoads  int `json:"completed_truck_loads"`
				OverloadedTruckLoads int `json:"overloaded_truck_loads"`
			} `json:"kpi"`
		}
		if err := json.Unmarshal([]byte(*r.DashboardSummary), &parsed); err != nil {
			continue
		}
		bearing := parsed.KPI.CompletedTruckLoads + parsed.KPI.OverloadedTruckLoads
		if bearing == 0 {
			continue
		}
		at := r.SubmittedAt
		if r.FinishedAt != nil {
			at = *r.FinishedAt
		}
		total += float64(bearing) * h.getRevenuePerTruckAt(at)
	}
	return total
}

// aggregateAIVisionKPI -- dipisah dari getAIVisionKPI supaya bisa diuji tanpa DB
// sungguhan (lihat dashboard_ai_test.go).
func aggregateAIVisionKPI(rows []domain.AIVisionAnalysis) aiVisionKPIAgg {
	var agg aiVisionKPIAgg
	for _, r := range rows {
		if r.DashboardSummary == nil {
			continue
		}
		var parsed struct {
			KPI struct {
				TrucksIdentified        int     `json:"trucks_identified"`
				ProductivityLoadingM3   float64 `json:"productivity_loading_m3"`
				CompletedTruckLoads     int     `json:"completed_truck_loads"`
				OverloadedTruckLoads    int     `json:"overloaded_truck_loads"`
				PendingTruckLoads       int     `json:"pending_truck_loads"`
				UnderfilledTruckLoads   int     `json:"underfilled_truck_loads"`
				UnvalidatedVolumeBucket int     `json:"unvalidated_volume_bucket"`
			} `json:"kpi"`
		}
		if err := json.Unmarshal([]byte(*r.DashboardSummary), &parsed); err != nil {
			continue
		}
		agg.Sesuai += parsed.KPI.CompletedTruckLoads
		agg.Overload += parsed.KPI.OverloadedTruckLoads
		agg.Underfilled += parsed.KPI.UnderfilledTruckLoads
		agg.Pending += parsed.KPI.PendingTruckLoads
		agg.UnvalidatedVolumeM3 += parsed.KPI.UnvalidatedVolumeBucket
		agg.TruckIdentified += parsed.KPI.TrucksIdentified
		agg.ProductivityLoadingM3 += parsed.KPI.ProductivityLoadingM3
		agg.RevenueBearingTrucks += parsed.KPI.CompletedTruckLoads + parsed.KPI.OverloadedTruckLoads
	}
	agg.PendingTrucks = agg.Underfilled + agg.Pending
	return agg
}

// loadingCycleBucketsResp — breakdown truk untuk 1 loading_cycle spesifik (halaman "04
// - Detail Aktivitas Loading"). Dihitung dari bucket_events milik excavator yang sama
// dalam window waktu cycle ini (+-60 detik toleransi), lewat mesin yang sama persis
// dengan ProduktivitasRevenue (ComputeTruckGroups) -- bukan algoritma baru.
type loadingCycleBucketsResp struct {
	CycleID      string           `json:"cycle_id"`
	BucketCount  int              `json:"bucket_count"`
	TruckSelesai int              `json:"truck_selesai"`
	TruckIdeal   int              `json:"truck_ideal"`
	Groups       []TruckGroupDTO  `json:"groups"`
	Events       []BucketEventDTO `json:"events"`
}

type TruckGroupDTO struct {
	StartTS     time.Time `json:"start_ts"`
	EndTS       time.Time `json:"end_ts"`
	BucketCount int       `json:"bucket_count"`
	FillStatus  string    `json:"fill_status"`
	Unvalidated bool      `json:"unvalidated"`
}

// BucketEventDTO — event bucket mentah per-segmen digging, dipakai FE untuk render
// timeline granular (Halaman "04 - Detail Aktivitas Loading"). StartedAt bisa nil
// untuk event lama (sebelum kolom ini ada) -- FE harus toleransi ini.
type BucketEventDTO struct {
	StartedAt         *time.Time `json:"started_at"`
	DetectedAt        time.Time  `json:"detected_at"`
	DiggingConfidence float64    `json:"digging_confidence"`
}

// LoadingCycleBuckets — breakdown truk (selesai/pending) untuk 1 rekaman/siklus
// spesifik, dipakai halaman "04 - Detail Aktivitas Loading". Window pencarian
// bucket_events diberi toleransi 60 detik di kedua sisi karena cycle_detector (idle-
// timeout) dan truck-grouping (gap 300 detik) punya batas penutupan yang berbeda --
// bucket yang secara wajar "milik" siklus ini bisa saja tercatat sedikit sebelum/
// sesudah start_ts/end_ts persis siklusnya.
func (h *MiscHandler) LoadingCycleBuckets(c *fiber.Ctx) error {
	var cycle domain.LoadingCycle
	if err := h.DB.First(&cycle, "id = ?", c.Params("id")).Error; err != nil {
		return utils.NotFound(c, "Siklus tidak ditemukan")
	}
	if cycle.ExcavatorID == nil {
		return utils.OK(c, "OK", loadingCycleBucketsResp{CycleID: cycle.ID.String()})
	}
	var exc domain.Excavator
	if err := h.DB.First(&exc, "id = ?", *cycle.ExcavatorID).Error; err != nil {
		return utils.BadRequest(c, err.Error(), nil)
	}
	// Isolasi dashboard demo/testing (lihat isTestViewer) -- siklus milik excavator di
	// luar scope user yang login dibalas 404, sama pola dengan AIVisionHandler.
	if exc.IsTest != isTestViewer(h.DB, c) {
		return utils.NotFound(c, "Siklus tidak ditemukan")
	}

	pad := 60 * time.Second
	var events []domain.BucketEvent
	err := h.DB.Model(&domain.BucketEvent{}).
		Where("excavator_id = ? AND detected_at >= ? AND detected_at <= ?",
			*cycle.ExcavatorID, cycle.StartTS.Add(-pad), cycle.EndTS.Add(pad)).
		Order("detected_at ASC").
		Find(&events).Error
	if err != nil {
		return utils.BadRequest(c, err.Error(), nil)
	}

	standard := exc.StandardBuckets
	if standard <= 0 {
		standard = h.getTruckCapacityM3()
	}
	groups := service.ComputeTruckGroups(events, standard, h.getTruckGroupGapSec(), time.Now())

	resp := loadingCycleBucketsResp{
		CycleID:     cycle.ID.String(),
		BucketCount: len(events),
		TruckIdeal:  int(math.Ceil(float64(len(events)) / float64(standard))),
	}
	for _, g := range groups {
		if g.IsIdentified() {
			resp.TruckSelesai++
		}
		resp.Groups = append(resp.Groups, TruckGroupDTO{
			StartTS: g.StartTS, EndTS: g.EndTS, BucketCount: g.BucketCount,
			FillStatus: g.FillStatus, Unvalidated: g.Unvalidated,
		})
	}
	for _, ev := range events {
		resp.Events = append(resp.Events, BucketEventDTO{
			StartedAt: ev.StartedAt, DetectedAt: ev.DetectedAt, DiggingConfidence: ev.DiggingConfidence,
		})
	}
	return utils.OK(c, "OK", resp)
}
