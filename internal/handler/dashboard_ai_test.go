package handler

import (
	"testing"

	"aimoc-backend/internal/domain"
)

// Kasus nyata 05 Agu 2026: truk underfilled (cuma 2-3 bucket, load_condition
// "underfilled"/"pending") tetap harus dihitung "teridentifikasi" & masuk
// produktivitas -- sebelumnya TruckIdentified cuma dihitung dari
// completed+overloaded, jadi truk ini hilang dari dashboard (lihat komentar
// aiVisionKPIAgg).
func TestAggregateAIVisionKPIIncludesUnderfilledTrucks(t *testing.T) {
	summary := `{"kpi": {
		"trucks_identified": 3,
		"productivity_loading_m3": 6.0,
		"completed_truck_loads": 0,
		"overloaded_truck_loads": 0,
		"underfilled_truck_loads": 2,
		"pending_truck_loads": 1,
		"unvalidated_volume_bucket": 6
	}}`
	rows := []domain.AIVisionAnalysis{{DashboardSummary: &summary}}

	agg := aggregateAIVisionKPI(rows)

	if agg.TruckIdentified != 3 {
		t.Errorf("TruckIdentified = %d, want 3 (harus ikut trucks_identified Gracia, bukan cuma completed+overloaded)", agg.TruckIdentified)
	}
	if agg.ProductivityLoadingM3 != 6.0 {
		t.Errorf("ProductivityLoadingM3 = %v, want 6.0", agg.ProductivityLoadingM3)
	}
	if agg.RevenueBearingTrucks != 0 {
		t.Errorf("RevenueBearingTrucks = %d, want 0 (belum ada yang completed/overloaded)", agg.RevenueBearingTrucks)
	}
	if agg.PendingTrucks != 3 {
		t.Errorf("PendingTrucks = %d, want 3 (underfilled 2 + pending 1)", agg.PendingTrucks)
	}
}

func TestAggregateAIVisionKPISkipsNilOrInvalidSummary(t *testing.T) {
	invalid := "bukan json"
	rows := []domain.AIVisionAnalysis{
		{DashboardSummary: nil},
		{DashboardSummary: &invalid},
	}
	agg := aggregateAIVisionKPI(rows)
	if agg.TruckIdentified != 0 || agg.ProductivityLoadingM3 != 0 {
		t.Errorf("expected zero-value agg for nil/invalid rows, got %+v", agg)
	}
}
