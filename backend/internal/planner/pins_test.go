package planner

import (
	"testing"

	"datacenter-thermal-capacity-planner/backend/internal/constants"
	"datacenter-thermal-capacity-planner/backend/internal/dto"
	"datacenter-thermal-capacity-planner/backend/internal/model"
)

func pinFixture() ([]model.ThermalZone, []model.Rack) {
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

func readyLoad(id uint, name string, group string) model.EquipmentLoad {
	return model.EquipmentLoad{ID: id, Name: name, PowerKW: 5, HeatKW: 4.8, AirflowCFM: 1200, RackUnits: 4, RedundancyGroup: group, LoadStatus: "ready"}
}

func TestEvaluatePinnedLoadHonoured(t *testing.T) {
	zones, racks := pinFixture()
	loads := []model.EquipmentLoad{readyLoad(1, "Compute A", "G1"), readyLoad(2, "Compute B", "G2")}
	pins := []dto.RackPin{{LoadID: 1, RackID: 2}}

	result := NewEngine(100).Evaluate(zones, racks, loads, pins)
	if len(result.PinConflicts) != 0 {
		t.Fatalf("expected no pin conflicts, got %+v", result.PinConflicts)
	}
	if len(result.Assignments) != 2 {
		t.Fatalf("expected two assignments, got %d (%+v)", len(result.Assignments), result.Violations)
	}
	var pinned *dto.RackAssignment
	for index := range result.Assignments {
		if result.Assignments[index].LoadID == 1 {
			pinned = &result.Assignments[index]
		}
	}
	if pinned == nil || pinned.RackID != 2 || !pinned.Pinned {
		t.Fatalf("load 1 must be pinned to rack 2, got %+v", pinned)
	}
}

func TestEvaluatePinnedDeterministic(t *testing.T) {
	zones, racks := pinFixture()
	loads := []model.EquipmentLoad{readyLoad(1, "Compute A", "G1"), readyLoad(2, "Compute B", "G2")}
	pins := []dto.RackPin{{LoadID: 2, RackID: 1}}
	first := NewEngine(100).Evaluate(zones, racks, loads, pins)
	second := NewEngine(100).Evaluate(zones, racks, loads, pins)
	if len(first.PinConflicts) != 0 || len(second.PinConflicts) != 0 {
		t.Fatalf("pins must not conflict: %+v %+v", first.PinConflicts, second.PinConflicts)
	}
	if len(first.Assignments) != len(second.Assignments) {
		t.Fatal("pinned evaluation must be deterministic")
	}
	for index := range first.Assignments {
		a, b := first.Assignments[index], second.Assignments[index]
		if a.LoadID != b.LoadID || a.RackID != b.RackID || a.Pinned != b.Pinned {
			t.Fatalf("pinned placement differs: %+v vs %+v", a, b)
		}
	}
}

func TestEvaluatePinnedLoadReservesCapacity(t *testing.T) {
	zones, racks := pinFixture()
	// The big load is fixed to rack 1 in zone A; its redundancy peer must then
	// land in zone B, and remaining capacity is respected by free placement.
	big := model.EquipmentLoad{ID: 1, Name: "Big", PowerKW: 20, HeatKW: 19, AirflowCFM: 6000, RackUnits: 30, RedundancyGroup: "PAIR", LoadStatus: "ready"}
	peer := model.EquipmentLoad{ID: 2, Name: "Peer", PowerKW: 5, HeatKW: 4.8, AirflowCFM: 1200, RackUnits: 4, RedundancyGroup: "PAIR", LoadStatus: "ready"}
	result := NewEngine(100).Evaluate(zones, racks, []model.EquipmentLoad{big, peer}, []dto.RackPin{{LoadID: 1, RackID: 1}})
	if len(result.PinConflicts) != 0 {
		t.Fatalf("unexpected pin conflicts: %+v", result.PinConflicts)
	}
	placement := map[uint]uint{}
	for _, assignment := range result.Assignments {
		placement[assignment.LoadID] = assignment.RackID
	}
	if placement[1] != 1 {
		t.Fatalf("pinned load must reserve rack 1, got %d", placement[1])
	}
	if placement[2] != 2 {
		t.Fatalf("redundancy peer must be isolated to rack 2, got %d", placement[2])
	}
}

func TestEvaluatePinConflictTable(t *testing.T) {
	zones, racks := pinFixture()
	tests := []struct {
		name      string
		load      model.EquipmentLoad
		pin       dto.RackPin
		wantCode  string
		extraLoad []model.EquipmentLoad
		extraPins []dto.RackPin
	}{
		{name: "rack power exceeded", load: model.EquipmentLoad{ID: 10, Name: "Huge power", PowerKW: 30, HeatKW: 28, AirflowCFM: 1000, RackUnits: 4, RedundancyGroup: "X", LoadStatus: "ready"}, pin: dto.RackPin{LoadID: 10, RackID: 1}, wantCode: "RACK_POWER_LIMIT"},
		{name: "rack airflow exceeded", load: model.EquipmentLoad{ID: 11, Name: "High airflow", PowerKW: 5, HeatKW: 4.8, AirflowCFM: 9000, RackUnits: 4, RedundancyGroup: "X", LoadStatus: "ready"}, pin: dto.RackPin{LoadID: 11, RackID: 1}, wantCode: "RACK_AIRFLOW_LIMIT"},
		{name: "rack units exceeded", load: model.EquipmentLoad{ID: 12, Name: "Tall", PowerKW: 5, HeatKW: 4.8, AirflowCFM: 1000, RackUnits: 45, RedundancyGroup: "X", LoadStatus: "ready"}, pin: dto.RackPin{LoadID: 12, RackID: 1}, wantCode: "RACK_UNIT_LIMIT"},
		{name: "redundancy peers same zone", load: readyLoad(14, "Peer B", "PAIR"), pin: dto.RackPin{LoadID: 14, RackID: 1}, wantCode: "REDUNDANCY_ZONE_COLLISION", extraLoad: []model.EquipmentLoad{readyLoad(15, "Peer A", "PAIR")}, extraPins: []dto.RackPin{{LoadID: 15, RackID: 1}}},
		{name: "load not ready", load: model.EquipmentLoad{ID: 16, Name: "Held", PowerKW: 2, HeatKW: 2, AirflowCFM: 400, RackUnits: 2, RedundancyGroup: "X", LoadStatus: "held"}, pin: dto.RackPin{LoadID: 16, RackID: 1}, wantCode: "LOAD_NOT_READY"},
		{name: "rack missing", load: readyLoad(17, "Ghost rack", "X"), pin: dto.RackPin{LoadID: 17, RackID: 999}, wantCode: "PINNED_RACK_MISSING"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			loads := append([]model.EquipmentLoad{tt.load}, tt.extraLoad...)
			pins := append([]dto.RackPin{tt.pin}, tt.extraPins...)
			result := NewEngine(100).Evaluate(zones, racks, loads, pins)
			if len(result.PinConflicts) == 0 {
				t.Fatalf("expected pin conflict %s, got none", tt.wantCode)
			}
			found := false
			for _, conflict := range result.PinConflicts {
				if conflict.Code == tt.wantCode {
					found = true
					if conflict.RackID == 0 && tt.wantCode != "PINNED_RACK_MISSING" && tt.wantCode != "LOAD_NOT_READY" {
						// rack reference is attached for rack/zone level conflicts
						if tt.pin.RackID != 0 {
							t.Errorf("conflict %s missing rack reference", tt.wantCode)
						}
					}
				}
			}
			if !found {
				t.Fatalf("expected conflict code %s, got %+v", tt.wantCode, result.PinConflicts)
			}
			if len(result.Assignments) != 0 {
				t.Fatalf("failed pin evaluation must not return partial assignments, got %+v", result.Assignments)
			}
		})
	}
}

func TestEvaluatePinRackUnavailable(t *testing.T) {
	zones := []model.ThermalZone{{ID: 1, ZoneCode: "A", CoolingCapacityKW: 40, SupplyTempC: 18, MaxReturnTempC: 31, AdjacencyJSON: `{}`, ZoneStatus: "active"}}
	racks := []model.Rack{{ID: 7, ZoneID: 1, RackCode: "A-09", PowerLimitKW: 24, AirflowLimitCFM: 7000, RackUnits: 42, RackStatus: constants.RackUnavailable}}
	load := readyLoad(20, "Blocked", "X")
	result := NewEngine(100).Evaluate(zones, racks, []model.EquipmentLoad{load}, []dto.RackPin{{LoadID: 20, RackID: 7}})
	if len(result.PinConflicts) == 0 || result.PinConflicts[0].Code != "RACK_UNAVAILABLE" {
		t.Fatalf("expected RACK_UNAVAILABLE pin conflict, got %+v", result.PinConflicts)
	}
}

func TestEvaluatePinZoneCoolingConflict(t *testing.T) {
	// Rack envelope allows the load, but a low-capacity active zone cannot cool it.
	zones := []model.ThermalZone{{ID: 1, ZoneCode: "A", CoolingCapacityKW: 8, SupplyTempC: 18, MaxReturnTempC: 31, AdjacencyJSON: `{}`, ZoneStatus: "active"}}
	racks := []model.Rack{{ID: 1, ZoneID: 1, RackCode: "A-01", PowerLimitKW: 24, AirflowLimitCFM: 7000, RackUnits: 42, RackStatus: constants.RackAvailable}}
	load := model.EquipmentLoad{ID: 30, Name: "Zone breaker", PowerKW: 10, HeatKW: 9.5, AirflowCFM: 2000, RackUnits: 8, RedundancyGroup: "Z", LoadStatus: "ready"}
	result := NewEngine(100).Evaluate(zones, racks, []model.EquipmentLoad{load}, []dto.RackPin{{LoadID: 30, RackID: 1}})
	if len(result.PinConflicts) == 0 {
		t.Fatalf("expected a zone-level pin conflict, got assignments=%+v", result.Assignments)
	}
	found := false
	for _, conflict := range result.PinConflicts {
		if conflict.Code == "ZONE_COOLING_LIMIT" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected ZONE_COOLING_LIMIT conflict, got %+v", result.PinConflicts)
	}
}

func TestEvaluateWithoutPinsUnchanged(t *testing.T) {
	zones, racks := pinFixture()
	loads := []model.EquipmentLoad{readyLoad(1, "Compute A", "G1"), readyLoad(2, "Compute B", "G2")}
	without := NewEngine(100).Evaluate(zones, racks, loads)
	empty := NewEngine(100).Evaluate(zones, racks, loads, nil, []dto.RackPin{})
	if len(without.Assignments) != len(empty.Assignments) || len(without.PinConflicts) != len(empty.PinConflicts) {
		t.Fatal("nil pins and empty pins must match the pin-free evaluation")
	}
	for index := range without.Assignments {
		if without.Assignments[index].RackID != empty.Assignments[index].RackID {
			t.Fatalf("unpinned deterministic layout changed: %+v vs %+v", without.Assignments, empty.Assignments)
		}
		if empty.Assignments[index].Pinned {
			t.Fatal("free placement must not be marked pinned")
		}
	}
}
