package planner

import (
	"fmt"
	"sort"

	"datacenter-thermal-capacity-planner/backend/internal/dto"
	"datacenter-thermal-capacity-planner/backend/internal/model"
)

func checkCandidate(load model.EquipmentLoad, rack model.Rack, zone model.ThermalZone, usage *rackUsage, zoneHeat float64, zoneGroups map[string]bool) []dto.ConstraintViolation {
	violations := []dto.ConstraintViolation{}
	if !rack.IsUsable() {
		violations = append(violations, violation("RACK_UNAVAILABLE", rack.ID, "rack", "rack status does not allow placement", 1, 0))
		return violations
	}
	if !zone.IsActive() {
		violations = append(violations, violation("ZONE_UNAVAILABLE", zone.ID, "thermal_zone", "thermal zone is not active", 1, 0))
	}
	if usage.powerKW+load.PowerKW > rack.PowerLimitKW {
		violations = append(violations, violation("RACK_POWER_LIMIT", rack.ID, "rack", "placement exceeds rack power limit", usage.powerKW+load.PowerKW, rack.PowerLimitKW))
	}
	if usage.airflowCFM+load.AirflowCFM > rack.AirflowLimitCFM {
		violations = append(violations, violation("RACK_AIRFLOW_LIMIT", rack.ID, "rack", "placement exceeds rack airflow limit", usage.airflowCFM+load.AirflowCFM, rack.AirflowLimitCFM))
	}
	if usage.rackUnits+load.RackUnits > rack.RackUnits {
		violations = append(violations, violation("RACK_UNIT_LIMIT", rack.ID, "rack", "placement exceeds available rack units", float64(usage.rackUnits+load.RackUnits), float64(rack.RackUnits)))
	}
	if zoneHeat+load.HeatKW > zone.CoolingCapacityKW {
		violations = append(violations, violation("ZONE_COOLING_LIMIT", zone.ID, "thermal_zone", "placement exceeds zone cooling capacity", zoneHeat+load.HeatKW, zone.CoolingCapacityKW))
	}
	if zoneGroups[load.RedundancyGroup] {
		violations = append(violations, violation("REDUNDANCY_ZONE_COLLISION", load.ID, "equipment_load", "redundancy peers must be isolated across thermal zones", 1, 0))
	}
	return violations
}

func summarizeUnplaced(load model.EquipmentLoad, evidence []dto.ConstraintViolation) []dto.ConstraintViolation {
	counts := map[string]int{}
	for _, item := range evidence {
		counts[item.Code]++
	}
	codes := make([]string, 0, len(counts))
	for code := range counts {
		codes = append(codes, code)
	}
	sort.Strings(codes)
	message := "no rack satisfied all placement constraints"
	if len(codes) > 0 {
		message = fmt.Sprintf("no rack satisfied all placement constraints; evidence: %v", codes)
	}
	return []dto.ConstraintViolation{{
		Code: "LOAD_UNPLACED", Severity: "critical", EntityType: "equipment_load", EntityID: load.ID,
		Message: message, Actual: float64(len(evidence)), Limit: 0,
	}}
}

func validateFinalAssignments(racks []model.Rack, usage map[uint]*rackUsage, zones []model.ThermalZone, zonePower map[uint]float64, assignments []dto.RackAssignment) []dto.ConstraintViolation {
	violations := []dto.ConstraintViolation{}
	for _, rack := range racks {
		u := usage[rack.ID]
		if u == nil {
			continue
		}
		if u.powerKW > rack.PowerLimitKW {
			violations = append(violations, violation("RACK_POWER_LIMIT", rack.ID, "rack", "rack power limit exceeded", u.powerKW, rack.PowerLimitKW))
		}
		if u.airflowCFM > rack.AirflowLimitCFM {
			violations = append(violations, violation("RACK_AIRFLOW_LIMIT", rack.ID, "rack", "rack airflow limit exceeded", u.airflowCFM, rack.AirflowLimitCFM))
		}
		if u.rackUnits > rack.RackUnits {
			violations = append(violations, violation("RACK_UNIT_LIMIT", rack.ID, "rack", "rack unit limit exceeded", float64(u.rackUnits), float64(rack.RackUnits)))
		}
	}
	for _, zone := range zones {
		if zonePower[zone.ID] > zone.CoolingCapacityKW {
			violations = append(violations, violation("ZONE_POWER_ENVELOPE", zone.ID, "thermal_zone", "assigned power exceeds zone cooling envelope", zonePower[zone.ID], zone.CoolingCapacityKW))
		}
	}
	return violations
}

func violation(code string, id uint, entity, message string, actual, limit float64) dto.ConstraintViolation {
	return dto.ConstraintViolation{Code: code, Severity: "critical", EntityType: entity, EntityID: id, Message: message, Actual: actual, Limit: limit}
}
