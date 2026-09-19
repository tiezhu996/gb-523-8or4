package planner

import (
	"fmt"
	"math"

	"datacenter-thermal-capacity-planner/backend/internal/dto"
	"datacenter-thermal-capacity-planner/backend/internal/model"
)

func placementScore(load model.EquipmentLoad, rack model.Rack, zone model.ThermalZone, usage *rackUsage, zoneHeat float64, zones []model.ThermalZone, allZoneHeat map[uint]float64) (float64, []string) {
	powerMargin := 1 - (usage.powerKW+load.PowerKW)/rack.PowerLimitKW
	airflowMargin := 1 - (usage.airflowCFM+load.AirflowCFM)/rack.AirflowLimitCFM
	unitMargin := 1 - float64(usage.rackUnits+load.RackUnits)/float64(rack.RackUnits)
	capacityMargin := 1 - (zoneHeat+load.HeatKW)/zone.CoolingCapacityKW
	neighborPenalty := adjacentHeatPenalty(zone, zones, allZoneHeat)
	hotspotPenalty := math.Max(0, (zoneHeat+load.HeatKW)/zone.CoolingCapacityKW-0.75)
	preference := 0.0
	if load.PreferredZoneID != nil && *load.PreferredZoneID == zone.ID {
		preference = 12
	}
	reservationPenalty := 0.0
	if rack.RackStatus == "reserved" {
		reservationPenalty = 8
	}
	score := 50 + powerMargin*15 + airflowMargin*10 + unitMargin*8 + capacityMargin*17 + preference - neighborPenalty*14 - hotspotPenalty*20 - reservationPenalty
	explanation := []string{
		fmt.Sprintf("power headroom %.0f%%", powerMargin*100),
		fmt.Sprintf("airflow headroom %.0f%%", airflowMargin*100),
		fmt.Sprintf("zone cooling headroom %.0f%%", capacityMargin*100),
	}
	if preference > 0 {
		explanation = append(explanation, "preferred thermal zone bonus applied")
	}
	if neighborPenalty > 0 {
		explanation = append(explanation, fmt.Sprintf("adjacent heat penalty %.1f", neighborPenalty*14))
	}
	return round2(score), explanation
}

func adjacentHeatPenalty(zone model.ThermalZone, zones []model.ThermalZone, heat map[uint]float64) float64 {
	byCode := make(map[string]model.ThermalZone, len(zones))
	for _, item := range zones {
		byCode[item.ZoneCode] = item
	}
	penalty := 0.0
	for code, weight := range dto.DecodeAdjacency(zone.AdjacencyJSON) {
		neighbor, ok := byCode[code]
		if !ok || neighbor.CoolingCapacityKW <= 0 {
			continue
		}
		penalty += (heat[neighbor.ID] / neighbor.CoolingCapacityKW) * weight
	}
	return penalty
}

func scenarioScore(assignments []dto.RackAssignment, zones []dto.ZoneThermalResult, violations []dto.ConstraintViolation) float64 {
	score := 100.0
	for _, violation := range violations {
		if violation.Severity == "critical" {
			score -= 24
		} else {
			score -= 6
		}
	}
	for _, zone := range zones {
		if zone.TemperatureMarginC < 3 {
			score -= (3 - zone.TemperatureMarginC) * 3
		}
	}
	if len(assignments) == 0 {
		score = 0
	}
	return round2(math.Max(0, math.Min(100, score)))
}

func round2(value float64) float64 { return math.Round(value*100) / 100 }

func maxFloat(values ...float64) float64 {
	maximum := values[0]
	for _, value := range values[1:] {
		if value > maximum {
			maximum = value
		}
	}
	return maximum
}
