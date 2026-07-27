package service

import (
	"time"

	"aimoc-backend/internal/domain"
)

// TruckGroupGapSec — jeda (detik) tanpa bucket baru yang menandai truk sebelumnya
// selesai (Kondisi I, lihat Dokumen_Penjelasan_Dashboard_AIMOC.pdf 23 Juli 2026).
const TruckGroupGapSec = 300

// Nilai FillStatus — status pengisian 1 TruckGroup:
//
//	PENDING   kelompok baru / belum genap standardBuckets DAN belum ada bukti jeda
//	          penutup (boundary belum terkonfirmasi). Tidak menambah revenue tercatat.
//	UNDERLOAD ditutup oleh jeda >=gapThresholdSec SEBELUM genap standardBuckets.
//	SESUAI    genap standardBuckets, ditutup bersih oleh jeda >=gapThresholdSec.
//	OVERLOAD  sudah genap standardBuckets tapi bucket berikutnya datang dengan jeda
//	          <gapThresholdSec (Kondisi II) — batas truk jadi ambigu; bucket kelebihan
//	          dialihkan ke kelompok baru berstatus PENDING dan dihitung Unvalidated Volume.
const (
	FillPending   = "PENDING"
	FillUnderload = "UNDERLOAD"
	FillSesuai    = "SESUAI"
	FillOverload  = "OVERLOAD"
)

// TruckGroup — 1 kelompok bucket (calon 1 truk), dihitung on-the-fly dari BucketEvent
// mentah oleh ComputeTruckGroups. Sengaja tidak dipersist — lihat catatan arsitektur
// di internal/handler/dashboard_ai.go.
type TruckGroup struct {
	Buckets     []domain.BucketEvent
	StartTS     time.Time
	EndTS       time.Time
	BucketCount int
	FillStatus  string
	// Unvalidated — true bila kelompok ini lahir dari Kondisi II (overflow bucket
	// setelah kelompok sebelumnya genap tanpa jeda pemisah yang jelas). Bucket-nya
	// masuk hitungan "Unvalidated Volume", bukan "Truk Teridentifikasi".
	Unvalidated bool
}

// IsIdentified — "Truk Teridentifikasi" di dokumen: genap standardBuckets DAN
// boundary-nya sudah terkonfirmasi (SESUAI atau OVERLOAD). Menambah Revenue Tercatat.
func (g TruckGroup) IsIdentified() bool {
	return g.FillStatus == FillSesuai || g.FillStatus == FillOverload
}

// ComputeTruckGroups mengelompokkan BucketEvent (harus sudah terurut waktu ASC, untuk
// SATU excavator) menjadi TruckGroup sesuai aturan dokumen. `now`, bila non-zero,
// dipakai untuk menutup kelompok terbuka terakhir bila sudah >=gapThresholdSec sejak
// bucket terakhirnya walau belum ada bucket berikutnya (real-time report) — zero value
// (untuk pengetesan murni/deterministik) membiarkan kelompok terbuka apa adanya (PENDING).
func ComputeTruckGroups(events []domain.BucketEvent, standardBuckets, gapThresholdSec int, now time.Time) []TruckGroup {
	if standardBuckets <= 0 {
		standardBuckets = 10
	}
	if gapThresholdSec <= 0 {
		gapThresholdSec = TruckGroupGapSec
	}
	threshold := time.Duration(gapThresholdSec) * time.Second

	var groups []TruckGroup
	var cur *TruckGroup
	nextUnvalidated := false

	startGroup := func(e domain.BucketEvent) {
		cur = &TruckGroup{StartTS: e.DetectedAt, Unvalidated: nextUnvalidated}
		nextUnvalidated = false
	}
	appendToGroup := func(e domain.BucketEvent) {
		cur.Buckets = append(cur.Buckets, e)
		cur.BucketCount = len(cur.Buckets)
		cur.EndTS = e.DetectedAt
	}
	closeGroup := func(fill string) {
		cur.FillStatus = fill
		groups = append(groups, *cur)
		cur = nil
	}

	for _, e := range events {
		if cur == nil {
			startGroup(e)
			appendToGroup(e)
			continue
		}
		gap := e.DetectedAt.Sub(cur.EndTS)
		if cur.BucketCount < standardBuckets {
			if gap >= threshold {
				closeGroup(FillUnderload)
				startGroup(e)
			}
			appendToGroup(e)
			continue
		}
		// cur.BucketCount == standardBuckets, sudah penuh — menunggu konfirmasi boundary.
		if gap >= threshold {
			closeGroup(FillSesuai)
		} else {
			closeGroup(FillOverload)
			nextUnvalidated = true
		}
		startGroup(e)
		appendToGroup(e)
	}

	if cur != nil {
		if !now.IsZero() && now.Sub(cur.EndTS) >= threshold {
			if cur.BucketCount >= standardBuckets {
				closeGroup(FillSesuai)
			} else {
				closeGroup(FillUnderload)
			}
		} else {
			closeGroup(FillPending)
		}
	}

	return groups
}
