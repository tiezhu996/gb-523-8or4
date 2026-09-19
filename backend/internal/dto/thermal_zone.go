package dto

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"datacenter-thermal-capacity-planner/backend/internal/model"
)

type CreateThermalZoneRequest struct {
	ZoneCode          string             `json:"zone_code" binding:"required,min=2,max=32"`
	Name              string             `json:"name" binding:"required,min=2,max=120"`
	CoolingCapacityKW float64            `json:"cooling_capacity_kw" binding:"required,gt=0,lte=10000"`
	SupplyTempC       float64            `json:"supply_temp_c" binding:"required,gte=10,lte=30"`
	MaxReturnTempC    float64            `json:"max_return_temp_c" binding:"required,gte=18,lte=60"`
	Adjacency         map[string]float64 `json:"adjacency" binding:"required"`
	ZoneStatus        string             `json:"zone_status" binding:"required,oneof=active constrained offline"`
}

type UpdateThermalZoneRequest struct {
	Name              string             `json:"name" binding:"required,min=2,max=120"`
	CoolingCapacityKW float64            `json:"cooling_capacity_kw" binding:"required,gt=0,lte=10000"`
	SupplyTempC       float64            `json:"supply_temp_c" binding:"required,gte=10,lte=30"`
	MaxReturnTempC    float64            `json:"max_return_temp_c" binding:"required,gte=18,lte=60"`
	Adjacency         map[string]float64 `json:"adjacency" binding:"required"`
	ZoneStatus        string             `json:"zone_status" binding:"required,oneof=active constrained offline"`
}

type ThermalZoneResponse struct {
	ID                  uint               `json:"id"`
	ZoneCode            string             `json:"zone_code"`
	Name                string             `json:"name"`
	CoolingCapacityKW   float64            `json:"cooling_capacity_kw"`
	SupplyTempC         float64            `json:"supply_temp_c"`
	MaxReturnTempC      float64            `json:"max_return_temp_c"`
	Adjacency           map[string]float64 `json:"adjacency"`
	ZoneStatus          string             `json:"zone_status"`
	RackCount           int64              `json:"rack_count"`
	AllocatedPowerKW    float64            `json:"allocated_power_kw"`
	CapacityUtilization float64            `json:"capacity_utilization"`
	TemperatureHeadroom float64            `json:"temperature_headroom_c"`
}

func (r CreateThermalZoneRequest) ValidateBusiness() error {
	if strings.TrimSpace(r.ZoneCode) == "" {
		return errors.New("zone code is required")
	}
	if r.MaxReturnTempC <= r.SupplyTempC {
		return errors.New("max return temperature must exceed supply temperature")
	}
	return validateAdjacency(r.ZoneCode, r.Adjacency)
}

func (r UpdateThermalZoneRequest) ValidateBusiness(zoneCode string) error {
	if r.MaxReturnTempC <= r.SupplyTempC {
		return errors.New("max return temperature must exceed supply temperature")
	}
	return validateAdjacency(zoneCode, r.Adjacency)
}

func validateAdjacency(self string, adjacency map[string]float64) error {
	for code, weight := range adjacency {
		if strings.EqualFold(strings.TrimSpace(code), strings.TrimSpace(self)) {
			return errors.New("a zone cannot be adjacent to itself")
		}
		if weight < 0 || weight > 1 {
			return fmt.Errorf("adjacency weight for %s must be between 0 and 1", code)
		}
	}
	return nil
}

func NewThermalZone(req CreateThermalZoneRequest) (model.ThermalZone, error) {
	if err := req.ValidateBusiness(); err != nil {
		return model.ThermalZone{}, err
	}
	adjacency, err := json.Marshal(req.Adjacency)
	if err != nil {
		return model.ThermalZone{}, fmt.Errorf("encode zone adjacency: %w", err)
	}
	return model.ThermalZone{
		ZoneCode:          strings.ToUpper(strings.TrimSpace(req.ZoneCode)),
		Name:              strings.TrimSpace(req.Name),
		CoolingCapacityKW: req.CoolingCapacityKW,
		SupplyTempC:       req.SupplyTempC,
		MaxReturnTempC:    req.MaxReturnTempC,
		AdjacencyJSON:     string(adjacency),
		ZoneStatus:        req.ZoneStatus,
	}, nil
}

func DecodeAdjacency(raw string) map[string]float64 {
	value := map[string]float64{}
	if err := json.Unmarshal([]byte(raw), &value); err != nil {
		return map[string]float64{}
	}
	return value
}
