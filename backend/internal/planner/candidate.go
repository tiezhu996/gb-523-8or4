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
	Assignments []dto.RackAssignment
	ZoneResults []dto.ZoneThermalResult
	Violations  []dto.ConstraintViolation
	TotalPower  float64
	PeakTemp    float64
	Score       float64
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

func (e *Engine) Evaluate(zones []model.ThermalZone, racks []model.Rack, loads []model.EquipmentLoad) Result {
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
	orderedLoads := append([]model.EquipmentLoad(nil), loads...)
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
		u := usage[selected.rack.ID]
		u.powerKW += load.PowerKW
		u.heatKW += load.HeatKW
		u.airflowCFM += load.AirflowCFM
		u.rackUnits += load.RackUnits
		u.groups[load.RedundancyGroup] = true
		zoneGroups[selected.zone.ID][load.RedundancyGroup] = true
		zoneHeat[selected.zone.ID] += load.HeatKW
		zonePower[selected.zone.ID] += load.PowerKW
		result.TotalPower += load.PowerKW
		result.Assignments = append(result.Assignments, dto.RackAssignment{
			LoadID: load.ID, LoadName: load.Name, RackID: selected.rack.ID, RackCode: selected.rack.RackCode,
			ZoneID: selected.zone.ID, ZoneCode: selected.zone.ZoneCode, PowerKW: load.PowerKW,
			HeatKW: load.HeatKW, AirflowCFM: load.AirflowCFM, RackUnits: load.RackUnits,
			PlacementScore: selected.score, Explanation: selected.explanation,
		})
	}

	thermalResults, thermalViolations, peak := propagateThermal(zones, zoneHeat)
	result.ZoneResults = thermalResults
	result.Violations = append(result.Violations, thermalViolations...)
	result.PeakTemp = peak
	result.Violations = append(result.Violations, validateFinalAssignments(orderedRacks, usage, zones, zonePower, result.Assignments)...)
	result.Score = scenarioScore(result.Assignments, result.ZoneResults, result.Violations)
	return result
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
