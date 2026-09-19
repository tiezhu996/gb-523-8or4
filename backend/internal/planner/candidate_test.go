package planner

import (
	"reflect"
	"testing"

	"datacenter-thermal-capacity-planner/backend/internal/constants"
	"datacenter-thermal-capacity-planner/backend/internal/model"
)

func TestEngineEvaluateDeterministic(t *testing.T) {
	zones, racks := plannerFixture()
	zoneA := zones[0].ID
	zoneB := zones[1].ID
	loads := []model.EquipmentLoad{
		{ID: 2, Name: "Compute B", PowerKW: 9, HeatKW: 8.5, AirflowCFM: 1800, RackUnits: 8, RedundancyGroup: "PAIR", PreferredZoneID: &zoneB, LoadStatus: "ready"},
		{ID: 1, Name: "Compute A", PowerKW: 10, HeatKW: 9.4, AirflowCFM: 2000, RackUnits: 8, RedundancyGroup: "PAIR", PreferredZoneID: &zoneA, LoadStatus: "ready"},
	}
	first := NewEngine(100).Evaluate(zones, racks, loads)
	second := NewEngine(100).Evaluate(zones, racks, loads)
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("same input produced different results:\nfirst=%+v\nsecond=%+v", first, second)
	}
	if len(first.Assignments) != 2 {
		t.Fatalf("expected two assignments, got %d violations=%+v", len(first.Assignments), first.Violations)
	}
	if first.Assignments[0].ZoneID == first.Assignments[1].ZoneID {
		t.Fatal("redundancy peers were not isolated across zones")
	}
}

func TestEngineConstraintEvidence(t *testing.T) {
	zones, racks := plannerFixture()
	tests := []struct {
		name string
		load model.EquipmentLoad
		code string
	}{
		{name: "power and airflow too large", load: model.EquipmentLoad{ID: 3, Name: "Oversize", PowerKW: 100, HeatKW: 90, AirflowCFM: 30000, RackUnits: 10, RedundancyGroup: "X", LoadStatus: "ready"}, code: "LOAD_UNPLACED"},
		{name: "not ready", load: model.EquipmentLoad{ID: 4, Name: "Held", PowerKW: 2, HeatKW: 2, AirflowCFM: 500, RackUnits: 2, RedundancyGroup: "Y", LoadStatus: "held"}, code: "LOAD_NOT_READY"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NewEngine(100).Evaluate(zones, racks, []model.EquipmentLoad{tt.load})
			if len(result.Violations) == 0 || result.Violations[0].Code != tt.code {
				t.Fatalf("expected %s evidence, got %+v", tt.code, result.Violations)
			}
			if len(result.Assignments) != 0 {
				t.Fatalf("invalid load should not be assigned: %+v", result.Assignments)
			}
		})
	}
}

func plannerFixture() ([]model.ThermalZone, []model.Rack) {
	zones := []model.ThermalZone{
		{ID: 1, ZoneCode: "A", CoolingCapacityKW: 40, SupplyTempC: 18, MaxReturnTempC: 31, AdjacencyJSON: `{"B":0.2}`, ZoneStatus: "active"},
		{ID: 2, ZoneCode: "B", CoolingCapacityKW: 40, SupplyTempC: 18, MaxReturnTempC: 31, AdjacencyJSON: `{"A":0.2}`, ZoneStatus: "active"},
	}
	racks := []model.Rack{
		{ID: 1, ZoneID: 1, RackCode: "A-01", PowerLimitKW: 24, AirflowLimitCFM: 7000, RackUnits: 42, RackStatus: constants.RackAvailable},
		{ID: 2, ZoneID: 2, RackCode: "B-01", PowerLimitKW: 24, AirflowLimitCFM: 7000, RackUnits: 42, RackStatus: constants.RackAvailable},
	}
	return zones, racks
}
