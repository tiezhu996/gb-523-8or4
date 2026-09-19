package planner

import (
	"sort"

	"datacenter-thermal-capacity-planner/backend/internal/dto"
	"datacenter-thermal-capacity-planner/backend/internal/model"
)

const AlgorithmVersion = "thermal-v1"

type Engine struct {
	maxIterations int
}

type Result struct {
	Assignments   []dto.RackAssignment
	ZoneResults   []dto.ZoneThermalResult
	Violations    []dto.ConstraintViolation
	TotalPower    float64
	PeakTemp      float64
	Score         float64
	PinnedFailure bool
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

func (e *Engine) Evaluate(zones []model.ThermalZone, racks []model.Rack, loads []model.EquipmentLoad, pins ...dto.PinnedRack) Result {
	zoneByID := make(map[uint]model.ThermalZone, len(zones))
	for _, zone := range zones {
		zoneByID[zone.ID] = zone
	}
	rackByID := make(map[uint]model.Rack, len(racks))
	for _, rack := range racks {
		rackByID[rack.ID] = rack
	}
	loadByID := make(map[uint]model.EquipmentLoad, len(loads))
	for _, load := range loads {
		loadByID[load.ID] = load
	}

	orderedRacks := append([]model.Rack(nil), racks...)
	sort.SliceStable(orderedRacks, func(i, j int) bool {
		if orderedRacks[i].RackCode == orderedRacks[j].RackCode {
			return orderedRacks[i].ID < orderedRacks[j].ID
		}
		return orderedRacks[i].RackCode < orderedRacks[j].RackCode
	})
	orderedPins := orderedPins(pins, loadByID)

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

	result := Result{Assignments: []dto.RackAssignment{}, ZoneResults: []dto.ZoneThermalResult{}, Violations: []dto.ConstraintViolation{}}
	pinnedLoads := make(map[uint]bool, len(orderedPins))

	// Pinned loads consume their declared rack first. Any unmet constraint is a
	// hard, draft-preserving failure with concrete rack/zone/load evidence.
	for _, pin := range orderedPins {
		load, loadExists := loadByID[pin.LoadID]
		rack, rackExists := rackByID[pin.RackID]
		if !loadExists {
			result.Violations = append(result.Violations, pinViolation("PIN_LOAD_NOT_IN_SCENARIO", pin, "pinned load is not part of this scenario draft"))
			continue
		}
		pinnedLoads[load.ID] = true
		if !rackExists {
			result.Violations = append(result.Violations, pinViolation("PIN_RACK_NOT_FOUND", pin, "pinned rack does not exist"))
			continue
		}
		zone, exists := zoneByID[rack.ZoneID]
		if !exists {
			result.Violations = append(result.Violations, pinViolation("PIN_ZONE_NOT_FOUND", pin, "pinned rack is not linked to a thermal zone"))
			continue
		}
		violations := checkCandidate(load, rack, zone, usage[rack.ID], zoneHeat[rack.ZoneID], zoneGroups[rack.ZoneID])
		if len(violations) > 0 {
			result.Violations = append(result.Violations, decoratePinViolations(pin, violations)...)
			continue
		}
		score, explanation := placementScore(load, rack, zone, usage[rack.ID], zoneHeat[rack.ZoneID], zones, zoneHeat)
		applyPlacement(&result, usage, zoneHeat, zonePower, zoneGroups, load, rack, zone, score, true, explanation)
	}
	if len(result.Violations) > 0 {
		result.PinnedFailure = true
		// Thermal zone limits (including adjacency-driven return temperature)
		// still apply to the successfully consumed pins, so surface that evidence.
		thermalResults, thermalViolations, peak := propagateThermal(zones, zoneHeat)
		result.ZoneResults = thermalResults
		result.PeakTemp = peak
		for _, item := range thermalViolations {
			if item.Severity == "critical" {
				result.Violations = append(result.Violations, item)
			}
		}
		result.Score = 0
		return result
	}

	freeLoads := make([]model.EquipmentLoad, 0, len(loads))
	for _, load := range loads {
		if pinnedLoads[load.ID] {
			continue
		}
		freeLoads = append(freeLoads, load)
	}
	sort.SliceStable(freeLoads, func(i, j int) bool {
		left := tightness(freeLoads[i], orderedRacks)
		right := tightness(freeLoads[j], orderedRacks)
		if left == right {
			if freeLoads[i].PowerKW == freeLoads[j].PowerKW {
				return freeLoads[i].ID < freeLoads[j].ID
			}
			return freeLoads[i].PowerKW > freeLoads[j].PowerKW
		}
		return left > right
	})

	iterations := 0
	for _, load := range freeLoads {
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
		applyPlacement(&result, usage, zoneHeat, zonePower, zoneGroups, load, selected.rack, selected.zone, selected.score, false, selected.explanation)
	}

	thermalResults, thermalViolations, peak := propagateThermal(zones, zoneHeat)
	result.ZoneResults = thermalResults
	result.Violations = append(result.Violations, thermalViolations...)
	result.PeakTemp = peak
	result.Violations = append(result.Violations, validateFinalAssignments(orderedRacks, usage, zones, zonePower, result.Assignments)...)
	result.Score = scenarioScore(result.Assignments, result.ZoneResults, result.Violations)
	return result
}

// orderedPins keeps pin consumption deterministic: by load id, rack id as tie break.
func orderedPins(pins []dto.PinnedRack, loadByID map[uint]model.EquipmentLoad) []dto.PinnedRack {
	ordered := append([]dto.PinnedRack(nil), pins...)
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].LoadID == ordered[j].LoadID {
			return ordered[i].RackID < ordered[j].RackID
		}
		return ordered[i].LoadID < ordered[j].LoadID
	})
	return ordered
}

func applyPlacement(result *Result, usage map[uint]*rackUsage, zoneHeat, zonePower map[uint]float64, zoneGroups map[uint]map[string]bool, load model.EquipmentLoad, rack model.Rack, zone model.ThermalZone, score float64, pinned bool, explanation []string) {
	u := usage[rack.ID]
	u.powerKW += load.PowerKW
	u.heatKW += load.HeatKW
	u.airflowCFM += load.AirflowCFM
	u.rackUnits += load.RackUnits
	u.groups[load.RedundancyGroup] = true
	zoneGroups[zone.ID][load.RedundancyGroup] = true
	zoneHeat[zone.ID] += load.HeatKW
	zonePower[zone.ID] += load.PowerKW
	result.TotalPower += load.PowerKW
	notes := explanation
	if pinned {
		notes = append(append([]string{}, explanation...), "placed in planner-pinned rack")
	}
	result.Assignments = append(result.Assignments, dto.RackAssignment{
		LoadID: load.ID, LoadName: load.Name, RackID: rack.ID, RackCode: rack.RackCode,
		ZoneID: zone.ID, ZoneCode: zone.ZoneCode, PowerKW: load.PowerKW,
		HeatKW: load.HeatKW, AirflowCFM: load.AirflowCFM, RackUnits: load.RackUnits,
		PlacementScore: score, Pinned: pinned, Explanation: notes,
	})
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
