package planner

import (
	"fmt"
	"sort"

	"datacenter-thermal-capacity-planner/backend/internal/dto"
	"datacenter-thermal-capacity-planner/backend/internal/model"
)

const AlgorithmVersion = "thermal-v1"

type Engine struct {
	maxIterations int
}

type Result struct {
	Assignments  []dto.RackAssignment
	ZoneResults  []dto.ZoneThermalResult
	Violations   []dto.ConstraintViolation
	PinConflicts []dto.ConstraintViolation
	TotalPower   float64
	PeakTemp     float64
	Score        float64
}

type rackUsage struct {
	powerKW    float64
	heatKW     float64
	airflowCFM float64
	rackUnits  int
	groups     map[string]bool
}

type candidate struct {
	rack        model.Rack
	zone        model.ThermalZone
	score       float64
	explanation []string
}

func NewEngine(maxIterations int) *Engine {
	if maxIterations < 1 {
		maxIterations = 1
	}
	return &Engine{maxIterations: maxIterations}
}

// Evaluate runs the deterministic placement. Optional rack pins are applied
// first with priority capacity reservation; any unsatisfied pin fails the
// whole evaluation through Result.PinConflicts.
func (e *Engine) Evaluate(zones []model.ThermalZone, racks []model.Rack, loads []model.EquipmentLoad, pins ...[]dto.RackPin) Result {
	zoneByID := make(map[uint]model.ThermalZone, len(zones))
	for _, zone := range zones {
		zoneByID[zone.ID] = zone
	}

	orderedRacks := append([]model.Rack(nil), racks...)
	sort.SliceStable(orderedRacks, func(i, j int) bool {
		if orderedRacks[i].RackCode == orderedRacks[j].RackCode {
			return orderedRacks[i].ID < orderedRacks[j].ID
		}
		return orderedRacks[i].RackCode < orderedRacks[j].RackCode
	})

	usage := make(map[uint]*rackUsage, len(orderedRacks))
	zoneHeat := make(map[uint]float64, len(zones))
	zonePower := make(map[uint]float64, len(zones))
	zoneGroups := make(map[uint]map[string]bool, len(zones))
	for _, zone := range zones {
		zoneGroups[zone.ID] = map[string]bool{}
	}
	for _, rack := range orderedRacks {
		usage[rack.ID] = &rackUsage{groups: map[string]bool{}}
	}

	result := Result{Assignments: []dto.RackAssignment{}, ZoneResults: []dto.ZoneThermalResult{}, Violations: []dto.ConstraintViolation{}, PinConflicts: []dto.ConstraintViolation{}}

	// Fixed loads reserve their rack before any free candidate is considered.
	var pinIndex map[uint]uint
	if len(pins) > 0 && len(pins[0]) > 0 {
		pinIndex = dto.PinIndex(pins[0])
	}
	var orderedLoads []model.EquipmentLoad
	if len(pinIndex) > 0 {
		var pinConflicts []dto.ConstraintViolation
		orderedLoads, pinConflicts = e.placePinned(pinIndex, zones, orderedRacks, loads, usage, zoneHeat, zoneGroups, &result)
		if len(pinConflicts) > 0 {
			result.PinConflicts = pinConflicts
			result.Assignments = []dto.RackAssignment{}
			return result
		}
	} else {
		orderedLoads = append([]model.EquipmentLoad(nil), loads...)
	}

	sort.SliceStable(orderedLoads, func(i, j int) bool {
		left := tightness(orderedLoads[i], orderedRacks)
		right := tightness(orderedLoads[j], orderedRacks)
		if left == right {
			if orderedLoads[i].PowerKW == orderedLoads[j].PowerKW {
				return orderedLoads[i].ID < orderedLoads[j].ID
			}
			return orderedLoads[i].PowerKW > orderedLoads[j].PowerKW
		}
		return left > right
	})

	iterations := 0
	for _, load := range orderedLoads {
		if !load.IsPlannable() {
			result.Violations = append(result.Violations, dto.ConstraintViolation{
				Code: "LOAD_NOT_READY", Severity: "critical", EntityType: "equipment_load", EntityID: load.ID,
				Message: "load is not in ready state and cannot be placed",
			})
			continue
		}
		candidates := make([]candidate, 0, len(orderedRacks))
		var evidence []dto.ConstraintViolation
		for _, rack := range orderedRacks {
			iterations++
			if iterations > e.maxIterations {
				evidence = append(evidence, dto.ConstraintViolation{
					Code: "ITERATION_LIMIT", Severity: "critical", EntityType: "equipment_load", EntityID: load.ID,
					Message: "candidate search reached the configured iteration limit",
				})
				break
			}
			zone, exists := zoneByID[rack.ZoneID]
			if !exists {
				continue
			}
			violations := checkCandidate(load, rack, zone, usage[rack.ID], zoneHeat[rack.ZoneID], zoneGroups[rack.ZoneID])
			if len(violations) > 0 {
				evidence = append(evidence, violations...)
				continue
			}
			score, explanation := placementScore(load, rack, zone, usage[rack.ID], zoneHeat[rack.ZoneID], zones, zoneHeat)
			candidates = append(candidates, candidate{rack: rack, zone: zone, score: score, explanation: explanation})
		}
		if len(candidates) == 0 {
			result.Violations = append(result.Violations, summarizeUnplaced(load, evidence)...)
			continue
		}
		sort.SliceStable(candidates, func(i, j int) bool {
			if candidates[i].score == candidates[j].score {
				return candidates[i].rack.RackCode < candidates[j].rack.RackCode
			}
			return candidates[i].score > candidates[j].score
		})
		selected := candidates[0]
		commitPlacement(load, selected.rack, selected.zone, zones, usage, zoneHeat, zoneGroups, &result, false)
	}

	for _, assignment := range result.Assignments {
		zonePower[assignment.ZoneID] += assignment.PowerKW
	}

	thermalResults, thermalViolations, peak := propagateThermal(zones, zoneHeat)
	result.ZoneResults = thermalResults
	result.Violations = append(result.Violations, thermalViolations...)
	result.PeakTemp = peak
	result.Violations = append(result.Violations, validateFinalAssignments(orderedRacks, usage, zones, zonePower, result.Assignments)...)
	result.Score = scenarioScore(result.Assignments, result.ZoneResults, result.Violations)
	return result
}

// placePinned applies fixed loads and rejects the evaluation when their forced
// racks cannot satisfy rack, zone cooling/return-temperature or redundancy
// isolation constraints.
func (e *Engine) placePinned(pinIndex map[uint]uint, zones []model.ThermalZone, orderedRacks []model.Rack, loads []model.EquipmentLoad,
	usage map[uint]*rackUsage, zoneHeat map[uint]float64, zoneGroups map[uint]map[string]bool, result *Result) ([]model.EquipmentLoad, []dto.ConstraintViolation) {
	freeLoads := applyPins(pinIndex, zones, orderedRacks, loads, usage, zoneHeat, zoneGroups, result)
	if len(result.PinConflicts) > 0 {
		return nil, result.PinConflicts
	}
	// Fixed loads alone must not breach zone cooling or return-temperature limits.
	_, thermalViolations, _ := propagateThermal(zones, zoneHeat)
	for _, violation := range thermalViolations {
		if violation.Severity != "critical" {
			continue
		}
		violation.EntityType = "rack_pin"
		violation.Message = "pinned racks alone breach thermal zone limits: " + violation.Message
		result.PinConflicts = append(result.PinConflicts, violation)
	}
	if len(result.PinConflicts) > 0 {
		return nil, result.PinConflicts
	}
	return freeLoads, nil
}

func tightness(load model.EquipmentLoad, racks []model.Rack) float64 {
	best := 0.0
	for _, rack := range racks {
		if !rack.IsUsable() {
			continue
		}
		value := maxFloat(load.PowerKW/rack.PowerLimitKW, load.AirflowCFM/rack.AirflowLimitCFM, float64(load.RackUnits)/float64(rack.RackUnits))
		if value > best {
			best = value
		}
	}
	return best
}

// applyPins forces the requested loads onto their pinned racks. Pinned loads
// reserve capacity before the deterministic candidate search runs. A single
// unsatisfied pin aborts the whole evaluation and reports the exact rack,
// thermal zone or load conflict. It returns the loads left for free placement
// (nil when a pin blocks the evaluation).
func applyPins(pinIndex map[uint]uint, zones []model.ThermalZone, racks []model.Rack, loads []model.EquipmentLoad,
	usage map[uint]*rackUsage, zoneHeat map[uint]float64, zoneGroups map[uint]map[string]bool,
	result *Result) []model.EquipmentLoad {
	rackByID := make(map[uint]model.Rack, len(racks))
	for _, rack := range racks {
		rackByID[rack.ID] = rack
	}
	zoneByID := make(map[uint]model.ThermalZone, len(zones))
	for _, zone := range zones {
		zoneByID[zone.ID] = zone
	}
	loadByID := make(map[uint]model.EquipmentLoad, len(loads))
	for _, load := range loads {
		loadByID[load.ID] = load
	}
	loadIDs := make([]uint, 0, len(pinIndex))
	for loadID := range pinIndex {
		loadIDs = append(loadIDs, loadID)
	}
	sort.Slice(loadIDs, func(i, j int) bool { return loadIDs[i] < loadIDs[j] })

	freeLoads := make([]model.EquipmentLoad, 0, len(loads))
	for _, load := range loads {
		if _, pinned := pinIndex[load.ID]; !pinned {
			freeLoads = append(freeLoads, load)
		}
	}

	blocked := false
	for _, loadID := range loadIDs {
		load, loadOK := loadByID[loadID]
		rack, rackOK := rackByID[pinIndex[loadID]]
		switch {
		case !loadOK:
			result.PinConflicts = append(result.PinConflicts, pinConflict(loadID, pinIndex[loadID], 0,
				"PINNED_LOAD_MISSING", "pinned load is not part of this scenario input", 0, 0))
			blocked = true
			continue
		case !rackOK:
			result.PinConflicts = append(result.PinConflicts, pinConflict(loadID, pinIndex[loadID], 0,
				"PINNED_RACK_MISSING", "pinned rack does not exist", 0, 0))
			blocked = true
			continue
		case !load.IsPlannable():
			result.PinConflicts = append(result.PinConflicts, pinConflict(loadID, rack.ID, rack.ZoneID,
				"LOAD_NOT_READY", "pinned load is not in ready state and cannot be placed", 1, 0))
			blocked = true
			continue
		}
		zone, zoneOK := zoneByID[rack.ZoneID]
		if !zoneOK {
			result.PinConflicts = append(result.PinConflicts, pinConflict(loadID, rack.ID, rack.ZoneID,
				"PINNED_ZONE_MISSING", "pinned rack is not linked to a known thermal zone", 0, 0))
			blocked = true
			continue
		}
		checks := checkCandidate(load, rack, zone, usage[rack.ID], zoneHeat[rack.ZoneID], zoneGroups[rack.ZoneID])
		if len(checks) > 0 {
			for _, check := range checks {
				check.EntityType = "rack_pin"
				check.Message = fmt.Sprintf("pinned rack %s rejected load %q: %s", rack.RackCode, load.Name, check.Message)
				check.RackID = rack.ID
				check.ZoneID = rack.ZoneID
				result.PinConflicts = append(result.PinConflicts, check)
			}
			blocked = true
			continue
		}
		commitPlacement(load, rack, zone, zones, usage, zoneHeat, zoneGroups, result, true)
	}
	if blocked {
		return nil
	}
	return freeLoads
}

// commitPlacement scores a placement against current spare capacity and then
// records it against the shared usage accumulators.
func commitPlacement(load model.EquipmentLoad, rack model.Rack, zone model.ThermalZone, zones []model.ThermalZone,
	usage map[uint]*rackUsage, zoneHeat map[uint]float64, zoneGroups map[uint]map[string]bool,
	result *Result, pinned bool) {
	u := usage[rack.ID]
	score, explanation := placementScore(load, rack, zone, u, zoneHeat[zone.ID], zones, zoneHeat)
	u.powerKW += load.PowerKW
	u.heatKW += load.HeatKW
	u.airflowCFM += load.AirflowCFM
	u.rackUnits += load.RackUnits
	u.groups[load.RedundancyGroup] = true
	zoneGroups[zone.ID][load.RedundancyGroup] = true
	zoneHeat[zone.ID] += load.HeatKW
	result.TotalPower += load.PowerKW
	if pinned {
		explanation = append([]string{"pinned rack fixed by planner"}, explanation...)
	}
	result.Assignments = append(result.Assignments, dto.RackAssignment{
		LoadID: load.ID, LoadName: load.Name, RackID: rack.ID, RackCode: rack.RackCode,
		ZoneID: zone.ID, ZoneCode: zone.ZoneCode, PowerKW: load.PowerKW,
		HeatKW: load.HeatKW, AirflowCFM: load.AirflowCFM, RackUnits: load.RackUnits,
		PlacementScore: score, Pinned: pinned, Explanation: explanation,
	})
}

func pinConflict(loadID, rackID, zoneID uint, code, message string, actual, limit float64) dto.ConstraintViolation {
	return dto.ConstraintViolation{
		Code: code, Severity: "critical", EntityType: "rack_pin", EntityID: loadID,
		RackID: rackID, ZoneID: zoneID, Message: message, Actual: actual, Limit: limit,
	}
}
