package planner

import (
	"reflect"
	"testing"

	"datacenter-thermal-capacity-planner/backend/internal/constants"
	"datacenter-thermal-capacity-planner/backend/internal/dto"
	"datacenter-thermal-capacity-planner/backend/internal/model"
)

func TestEnginePinnedPlacement(t *testing.T) {
	zones, racks := plannerFixture()
	zoneA := zones[0].ID
	load := model.EquipmentLoad{ID: 11, Name: "Pinned node", PowerKW: 5, HeatKW: 4.8, AirflowCFM: 1200, RackUnits: 4, RedundancyGroup: "SOLO", PreferredZoneID: &zoneA, LoadStatus: "ready"}
	pin := dto.PinnedRack{LoadID: 11, RackID: 2, RackCode: "B-01", ZoneID: 2, ZoneCode: "B"}

	tests := []struct {
		name       string
		racks      []model.Rack
		pins       []dto.PinnedRack
		wantRackID uint
		wantFail   bool
		wantCode   string
	}{
		{name: "pin forces load into declared rack even against zone preference", racks: racks, pins: []dto.PinnedRack{pin}, wantRackID: 2},
		{name: "pin into missing rack fails", racks: racks, pins: []dto.PinnedRack{{LoadID: 11, RackID: 99}}, wantFail: true, wantCode: "PIN_RACK_NOT_FOUND"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine := NewEngine(100)
			result := engine.Evaluate(zones, tt.racks, []model.EquipmentLoad{load}, tt.pins...)
			if tt.wantFail {
				if !result.PinnedFailure || len(result.Violations) == 0 {
					t.Fatalf("expected pinned failure, got %+v", result)
				}
				if result.Violations[0].Code != tt.wantCode {
					t.Fatalf("expected %s evidence, got %+v", tt.wantCode, result.Violations)
				}
				if len(result.Assignments) != 0 {
					t.Fatalf("failed pin must not produce assignments: %+v", result.Assignments)
				}
				return
			}
			if result.PinnedFailure {
				t.Fatalf("unexpected pin failure: %+v", result.Violations)
			}
			if len(result.Assignments) != 1 || result.Assignments[0].RackID != tt.wantRackID {
				t.Fatalf("pinned assignment wrong: %+v", result.Assignments)
			}
			if !result.Assignments[0].Pinned {
				t.Fatal("assignment should be flagged as pinned")
			}
		})
	}
}

func TestEnginePinConstraintConflicts(t *testing.T) {
	zones, _ := plannerFixture()
	racks := []model.Rack{
		{ID: 1, ZoneID: 1, RackCode: "A-01", PowerLimitKW: 24, AirflowLimitCFM: 7000, RackUnits: 42, RackStatus: constants.RackAvailable},
		{ID: 2, ZoneID: 2, RackCode: "B-01", PowerLimitKW: 24, AirflowLimitCFM: 7000, RackUnits: 42, RackStatus: constants.RackAvailable},
		{ID: 3, ZoneID: 1, RackCode: "A-02", PowerLimitKW: 10, AirflowLimitCFM: 1000, RackUnits: 8, RackStatus: constants.RackReserved},
		{ID: 4, ZoneID: 1, RackCode: "A-03", PowerLimitKW: 24, AirflowLimitCFM: 7000, RackUnits: 42, RackStatus: constants.RackUnavailable},
	}
	pinRack := func(loadID, rackID uint) dto.PinnedRack { return dto.PinnedRack{LoadID: loadID, RackID: rackID} }
	tests := []struct {
		name     string
		loads    []model.EquipmentLoad
		pins     []dto.PinnedRack
		wantCode string
	}{
		{
			name: "power airflow and U limits on same rack",
			loads: []model.EquipmentLoad{
				{ID: 21, Name: "Huge", PowerKW: 20, HeatKW: 19, AirflowCFM: 1200, RackUnits: 10, RedundancyGroup: "X", LoadStatus: "ready"},
			},
			pins:     []dto.PinnedRack{pinRack(21, 3)},
			wantCode: "PIN_RACK_POWER_LIMIT",
		},
		{
			name: "unavailable rack",
			loads: []model.EquipmentLoad{
				{ID: 22, Name: "Small", PowerKW: 2, HeatKW: 2, AirflowCFM: 400, RackUnits: 2, RedundancyGroup: "X", LoadStatus: "ready"},
			},
			pins:     []dto.PinnedRack{pinRack(22, 4)},
			wantCode: "PIN_RACK_UNAVAILABLE",
		},
		{
			name: "redundancy peers pinned into one zone",
			loads: []model.EquipmentLoad{
				{ID: 23, Name: "Peer 1", PowerKW: 3, HeatKW: 3, AirflowCFM: 600, RackUnits: 2, RedundancyGroup: "PAIR", LoadStatus: "ready"},
				{ID: 24, Name: "Peer 2", PowerKW: 3, HeatKW: 3, AirflowCFM: 600, RackUnits: 2, RedundancyGroup: "PAIR", LoadStatus: "ready"},
			},
			pins:     []dto.PinnedRack{pinRack(23, 1), pinRack(24, 1)},
			wantCode: "PIN_REDUNDANCY_ZONE_COLLISION",
		},
		{
			name: "zone cooling capacity exceeded",
			loads: []model.EquipmentLoad{
				{ID: 25, Name: "Hot box", PowerKW: 45, HeatKW: 45, AirflowCFM: 4000, RackUnits: 10, RedundancyGroup: "HOT", LoadStatus: "ready"},
			},
			pins:     []dto.PinnedRack{pinRack(25, 2)},
			wantCode: "PIN_ZONE_COOLING_LIMIT",
		},
		{
			name: "pinned load missing from scenario",
			loads: []model.EquipmentLoad{
				{ID: 26, Name: "Present", PowerKW: 2, HeatKW: 2, AirflowCFM: 400, RackUnits: 2, RedundancyGroup: "X", LoadStatus: "ready"},
			},
			pins:     []dto.PinnedRack{pinRack(99, 1)},
			wantCode: "PIN_LOAD_NOT_IN_SCENARIO",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine := NewEngine(200)
			result := engine.Evaluate(zones, racks, tt.loads, tt.pins...)
			if !result.PinnedFailure {
				t.Fatalf("expected pinned failure, result=%+v", result)
			}
			found := false
			for _, violation := range result.Violations {
				if violation.Code == tt.wantCode {
					found = true
				}
			}
			if !found {
				t.Fatalf("expected evidence %s, got %+v", tt.wantCode, result.Violations)
			}
		})
	}
}

func TestEnginePinsFreeLoadsRemainDeterministic(t *testing.T) {
	zones, racks := plannerFixture()
	zoneA := zones[0].ID
	loads := []model.EquipmentLoad{
		{ID: 31, Name: "Free A", PowerKW: 6, HeatKW: 5.6, AirflowCFM: 1400, RackUnits: 4, RedundancyGroup: "G1", PreferredZoneID: &zoneA, LoadStatus: "ready"},
		{ID: 32, Name: "Free B", PowerKW: 7, HeatKW: 6.5, AirflowCFM: 1500, RackUnits: 5, RedundancyGroup: "G2", PreferredZoneID: &zoneA, LoadStatus: "ready"},
	}
	pins := []dto.PinnedRack{{LoadID: 31, RackID: 2, RackCode: "B-01", ZoneID: 2, ZoneCode: "B"}}
	engine := NewEngine(100)
	first := engine.Evaluate(zones, racks, loads, pins...)
	second := engine.Evaluate(zones, racks, loads, pins...)
	if first.PinnedFailure {
		t.Fatalf("unexpected pin failure: %+v", first.Violations)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("pinned evaluation is not deterministic:\n%+v\n%+v", first, second)
	}
	byLoad := map[uint]dto.RackAssignment{}
	for _, assignment := range first.Assignments {
		byLoad[assignment.LoadID] = assignment
	}
	if byLoad[31].RackID != 2 || !byLoad[31].Pinned {
		t.Fatalf("load 31 should occupy pinned rack B-01: %+v", byLoad[31])
	}
	if byLoad[32].Pinned || byLoad[32].RackID == 0 {
		t.Fatalf("free load should still be placed deterministically: %+v", byLoad[32])
	}
}

func TestEngineCheckPinsClearsWithoutPins(t *testing.T) {
	zones, racks := plannerFixture()
	loads := []model.EquipmentLoad{{ID: 41, Name: "Loose", PowerKW: 4, HeatKW: 4, AirflowCFM: 900, RackUnits: 3, RedundancyGroup: "Z", LoadStatus: "ready"}}
	if conflicts := NewEngine(100).CheckPins(zones, racks, loads, nil); len(conflicts) != 0 {
		t.Fatalf("empty pin set should have no conflicts: %+v", conflicts)
	}
}
