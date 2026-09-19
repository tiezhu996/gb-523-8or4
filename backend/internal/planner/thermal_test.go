package planner

import (
	"testing"

	"datacenter-thermal-capacity-planner/backend/internal/model"
)

func TestPropagateThermal(t *testing.T) {
	tests := []struct {
		name            string
		heat            map[uint]float64
		wantPeakAtLeast float64
		wantCritical    bool
	}{
		{name: "healthy margin", heat: map[uint]float64{1: 10, 2: 5}, wantPeakAtLeast: 20, wantCritical: false},
		{name: "adjacent overload", heat: map[uint]float64{1: 50, 2: 50}, wantPeakAtLeast: 33, wantCritical: true},
	}
	zones, _ := plannerFixture()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, violations, peak := propagateThermal(zones, tt.heat)
			if len(results) != len(zones) || peak < tt.wantPeakAtLeast {
				t.Fatalf("unexpected thermal result peak=%.2f results=%+v", peak, results)
			}
			hasCritical := false
			for _, violation := range violations {
				hasCritical = hasCritical || violation.Severity == "critical"
			}
			if hasCritical != tt.wantCritical {
				t.Fatalf("critical=%t, want %t; violations=%+v", hasCritical, tt.wantCritical, violations)
			}
		})
	}
}

func TestThermalOrderingStable(t *testing.T) {
	zones := []model.ThermalZone{
		{ID: 2, ZoneCode: "Z-B", CoolingCapacityKW: 20, SupplyTempC: 18, MaxReturnTempC: 30, AdjacencyJSON: `{}`, ZoneStatus: "active"},
		{ID: 1, ZoneCode: "Z-A", CoolingCapacityKW: 20, SupplyTempC: 18, MaxReturnTempC: 30, AdjacencyJSON: `{}`, ZoneStatus: "active"},
	}
	results, _, _ := propagateThermal(zones, map[uint]float64{})
	if results[0].ZoneCode != "Z-A" || results[1].ZoneCode != "Z-B" {
		t.Fatalf("thermal output should be sorted by zone code: %+v", results)
	}
}
