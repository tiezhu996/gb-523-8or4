package planner

import (
	"sort"

	"datacenter-thermal-capacity-planner/backend/internal/dto"
	"datacenter-thermal-capacity-planner/backend/internal/model"
)

// propagateThermal is an intentionally simplified planning model. It estimates
// return temperature from direct and weighted neighboring heat, not sensor data.
func propagateThermal(zones []model.ThermalZone, directHeat map[uint]float64) ([]dto.ZoneThermalResult, []dto.ConstraintViolation, float64) {
	ordered := append([]model.ThermalZone(nil), zones...)
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].ZoneCode < ordered[j].ZoneCode })
	byCode := make(map[string]model.ThermalZone, len(zones))
	for _, zone := range zones {
		byCode[zone.ZoneCode] = zone
	}
	results := make([]dto.ZoneThermalResult, 0, len(zones))
	violations := []dto.ConstraintViolation{}
	peak := 0.0
	for _, zone := range ordered {
		neighborHeat := 0.0
		for code, weight := range dto.DecodeAdjacency(zone.AdjacencyJSON) {
			if neighbor, ok := byCode[code]; ok {
				neighborHeat += directHeat[neighbor.ID] * weight
			}
		}
		effectiveHeat := directHeat[zone.ID] + neighborHeat
		capacity := zone.CoolingCapacityKW
		utilization := 0.0
		if capacity > 0 {
			utilization = effectiveHeat / capacity
		}
		// A calibrated planning assumption: full envelope adds 12 C to supply.
		estimated := zone.SupplyTempC + utilization*12
		margin := zone.MaxReturnTempC - estimated
		coolingMargin := capacity - effectiveHeat
		results = append(results, dto.ZoneThermalResult{
			ZoneID: zone.ID, ZoneCode: zone.ZoneCode, AssignedHeatKW: round2(directHeat[zone.ID]),
			NeighborHeatKW: round2(neighborHeat), EstimatedReturnC: round2(estimated),
			TemperatureMarginC: round2(margin), CoolingMarginKW: round2(coolingMargin),
		})
		if estimated > peak {
			peak = estimated
		}
		if coolingMargin < 0 {
			violations = append(violations, violation("ZONE_COOLING_LIMIT", zone.ID, "thermal_zone", "effective heat including adjacency exceeds cooling capacity", effectiveHeat, capacity))
		}
		if margin < 0 {
			violations = append(violations, violation("ZONE_RETURN_TEMP", zone.ID, "thermal_zone", "estimated return temperature exceeds configured limit", estimated, zone.MaxReturnTempC))
		} else if margin < 2 {
			violations = append(violations, dto.ConstraintViolation{
				Code: "ZONE_TEMP_HEADROOM_LOW", Severity: "warning", EntityType: "thermal_zone", EntityID: zone.ID,
				Message: "estimated return temperature has less than 2 C headroom", Actual: round2(estimated), Limit: zone.MaxReturnTempC,
			})
		}
	}
	return results, violations, round2(peak)
}
