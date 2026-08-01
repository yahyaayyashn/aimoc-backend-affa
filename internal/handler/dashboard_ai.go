package handler

import (
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
	ExcavatorID     string  `json:"excavator_id"`
	ExcavatorCode   string  `json:"excavator_code"`
	ExcavatorName   string  `json:"excavator_name"`
	VolumeM3        int     `json:"volume_m3"`
	TruckIdentified int     `json:"truck_identified"`
	RevenueTercatat float64 `json:"revenue_tercatat"`
	EstimasiRevenue float64 `json:"estimasi_revenue"`
	Sesuai          int     `json:"sesuai"`
	Underload       int     `json:"underload"`
	Overload        int     `json:"overload"`
	Pending         int     `json:"pending"`
	PendingBucketM3 int     `json:"pending_bucket_m3"`
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
	ProduktivitasLoadingM3 int                         `json:"produktivitas_loading_m3"`
	TruckIdentified        int                         `json:"truck_identified"`
	UnvalidatedVolumeM3    int                         `json:"unvalidated_volume_m3"`
	RevenueTercatat        float64                     `json:"revenue_tercatat"`
	EstimasiTotalRevenue   float64                     `json:"estimasi_total_revenue"`
	PerExcavator           []produktivitasExcavatorRow `json:"per_excavator"`
	LogAktivitas           []produktivitasLogRow       `json:"log_aktivitas"`
}

// ProduktivitasRevenue — "01 - Dashboard Produktivitas & Revenue" (AI Only), lihat
// Dokumen_Penjelasan_Dashboard_AIMOC.pdf. Menghitung pengelompokan truk on-the-fly dari
// bucket_events mentah (lihat internal/service/truck_grouping.go) -- TIDAK ada state
// tersimpan selain event mentahnya sendiri, jadi selalu bisa direkonsiliasi ulang kalau
// parameter algoritma (standard_buckets, gap threshold) berubah.
func (h *MiscHandler) ProduktivitasRevenue(c *fiber.Ctx) error {
	from := c.Query("from", "")
	to := c.Query("to", "")
	excavatorID := c.Query("excavator_id", "")

	var excavators []domain.Excavator
	excTx := h.DB.Model(&domain.Excavator{})
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
	resp := produktivitasRevenueResp{}
	revenuePerTruck := h.getRevenuePerTruck()
	gapSec := h.getTruckGroupGapSec()

	for _, exc := range excavators {
		excEvents := eventsByExc[exc.ID.String()]
		groups := service.ComputeTruckGroups(excEvents, exc.StandardBuckets, gapSec, now)

		row := produktivitasExcavatorRow{
			ExcavatorID:   exc.ID.String(),
			ExcavatorCode: exc.Code,
			ExcavatorName: exc.Name,
			VolumeM3:      len(excEvents),
		}
		standard := exc.StandardBuckets
		if standard <= 0 {
			standard = 10
		}
		for _, g := range groups {
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
				row.TruckIdentified++
				logRow.Revenue = revenuePerTruck
			}
			if g.Unvalidated {
				resp.UnvalidatedVolumeM3 += g.BucketCount
			}
			switch g.FillStatus {
			case service.FillSesuai:
				row.Sesuai++
			case service.FillUnderload:
				row.Underload++
			case service.FillOverload:
				row.Overload++
			case service.FillPending:
				row.Pending++
				row.PendingBucketM3 += g.BucketCount
			}
			resp.LogAktivitas = append(resp.LogAktivitas, logRow)
		}
		row.RevenueTercatat = float64(row.TruckIdentified) * revenuePerTruck
		row.EstimasiRevenue = math.Ceil(float64(row.VolumeM3)/float64(standard)) * revenuePerTruck

		resp.ProduktivitasLoadingM3 += row.VolumeM3
		resp.TruckIdentified += row.TruckIdentified
		resp.RevenueTercatat += row.RevenueTercatat
		resp.EstimasiTotalRevenue += row.EstimasiRevenue
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
		standard = 10
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
