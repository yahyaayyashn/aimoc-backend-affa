package service

import (
	"testing"
	"time"

	"aimoc-backend/internal/domain"
)

// events membangun n BucketEvent berjarak `step` mulai dari `start`, lalu menambahkan
// satu event tambahan setelah `extraGap` (jika extraGap > 0).
func events(start time.Time, n int, step, extraGap time.Duration) []domain.BucketEvent {
	var out []domain.BucketEvent
	ts := start
	for i := 0; i < n; i++ {
		out = append(out, domain.BucketEvent{DetectedAt: ts})
		ts = ts.Add(step)
	}
	if extraGap > 0 {
		out = append(out, domain.BucketEvent{DetectedAt: ts.Add(extraGap - step)})
	}
	return out
}

func TestComputeTruckGroups_10BucketBersih(t *testing.T) {
	start := time.Date(2026, 7, 23, 8, 0, 0, 0, time.UTC)
	ev := events(start, 10, 20*time.Second, 0)
	// "now" cuma 60s setelah bucket terakhir -> belum ada bukti jeda penutup -> PENDING.
	now := ev[len(ev)-1].DetectedAt.Add(60 * time.Second)

	groups := ComputeTruckGroups(ev, 10, TruckGroupGapSec, now)
	if len(groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(groups))
	}
	if groups[0].BucketCount != 10 || groups[0].FillStatus != FillPending || groups[0].IsIdentified() {
		t.Fatalf("unexpected group: %+v", groups[0])
	}
}

func TestComputeTruckGroups_11BucketGapGE300(t *testing.T) {
	start := time.Date(2026, 7, 23, 8, 0, 0, 0, time.UTC)
	ev := events(start, 10, 20*time.Second, 310*time.Second) // ke-11 datang >=300s setelah ke-10
	now := ev[len(ev)-1].DetectedAt.Add(60 * time.Second)

	groups := ComputeTruckGroups(ev, 10, TruckGroupGapSec, now)
	if len(groups) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(groups))
	}
	if groups[0].BucketCount != 10 || groups[0].FillStatus != FillSesuai || !groups[0].IsIdentified() {
		t.Fatalf("group 0 unexpected: %+v", groups[0])
	}
	if groups[1].BucketCount != 1 || groups[1].FillStatus != FillPending || groups[1].Unvalidated {
		t.Fatalf("group 1 unexpected: %+v", groups[1])
	}
}

func TestComputeTruckGroups_11BucketGapLT300(t *testing.T) {
	start := time.Date(2026, 7, 23, 8, 0, 0, 0, time.UTC)
	ev := events(start, 10, 20*time.Second, 100*time.Second) // ke-11 datang <300s setelah ke-10 (overload)
	now := ev[len(ev)-1].DetectedAt.Add(60 * time.Second)

	groups := ComputeTruckGroups(ev, 10, TruckGroupGapSec, now)
	if len(groups) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(groups))
	}
	if groups[0].BucketCount != 10 || groups[0].FillStatus != FillOverload || !groups[0].IsIdentified() {
		t.Fatalf("group 0 unexpected: %+v", groups[0])
	}
	if groups[1].BucketCount != 1 || groups[1].FillStatus != FillPending || !groups[1].Unvalidated {
		t.Fatalf("group 1 unexpected (harus Unvalidated): %+v", groups[1])
	}
}

func TestComputeTruckGroups_8BucketGapGE300(t *testing.T) {
	start := time.Date(2026, 7, 23, 8, 0, 0, 0, time.UTC)
	ev := events(start, 8, 20*time.Second, 0)
	// "now" sudah >=300s sejak bucket terakhir, TANPA bucket ke-9 -> tetap harus
	// ditutup Underload berdasarkan waktu berjalan, bukan cuma menunggu event baru.
	now := ev[len(ev)-1].DetectedAt.Add(305 * time.Second)

	groups := ComputeTruckGroups(ev, 10, TruckGroupGapSec, now)
	if len(groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(groups))
	}
	if groups[0].BucketCount != 8 || groups[0].FillStatus != FillUnderload || groups[0].IsIdentified() {
		t.Fatalf("unexpected group: %+v", groups[0])
	}
}

func TestComputeTruckGroups_MiningOnly(t *testing.T) {
	groups := ComputeTruckGroups(nil, 10, TruckGroupGapSec, time.Now())
	if len(groups) != 0 {
		t.Fatalf("expected 0 groups for mining-only (no loading buckets), got %d", len(groups))
	}
}
