package dto

import (
	"errors"
	"fmt"
	"strings"

	"datacenter-thermal-capacity-planner/backend/internal/model"
)

type CreateEquipmentLoadRequest struct {
	Name            string  `json:"name" binding:"required,min=2,max=120"`
	PowerKW         float64 `json:"power_kw" binding:"required,gt=0,lte=500"`
	HeatKW          float64 `json:"heat_kw" binding:"required,gt=0,lte=500"`
	AirflowCFM      float64 `json:"airflow_cfm" binding:"required,gt=0,lte=100000"`
	RackUnits       int     `json:"rack_units" binding:"required,gte=1,lte=60"`
	RedundancyGroup string  `json:"redundancy_group" binding:"required,min=1,max=64"`
	PreferredZoneID *uint   `json:"preferred_zone_id"`
	LoadStatus      string  `json:"load_status" binding:"required,oneof=draft ready held placed"`
}

type UpdateEquipmentLoadRequest struct {
	Name            string  `json:"name" binding:"required,min=2,max=120"`
	PowerKW         float64 `json:"power_kw" binding:"required,gt=0,lte=500"`
	HeatKW          float64 `json:"heat_kw" binding:"required,gt=0,lte=500"`
	AirflowCFM      float64 `json:"airflow_cfm" binding:"required,gt=0,lte=100000"`
	RackUnits       int     `json:"rack_units" binding:"required,gte=1,lte=60"`
	RedundancyGroup string  `json:"redundancy_group" binding:"required,min=1,max=64"`
	PreferredZoneID *uint   `json:"preferred_zone_id"`
	LoadStatus      string  `json:"load_status" binding:"required,oneof=draft ready held placed"`
}

type EquipmentLoadResponse struct {
	ID                uint    `json:"id"`
	Name              string  `json:"name"`
	PowerKW           float64 `json:"power_kw"`
	HeatKW            float64 `json:"heat_kw"`
	AirflowCFM        float64 `json:"airflow_cfm"`
	RackUnits         int     `json:"rack_units"`
	RedundancyGroup   string  `json:"redundancy_group"`
	PreferredZoneID   *uint   `json:"preferred_zone_id"`
	PreferredZoneCode string  `json:"preferred_zone_code,omitempty"`
	LoadStatus        string  `json:"load_status"`
	HeatRatio         float64 `json:"heat_ratio"`
	ValidationState   string  `json:"validation_state"`
}

type LoadValidationResult struct {
	LoadID   uint     `json:"load_id"`
	LoadName string   `json:"load_name"`
	Valid    bool     `json:"valid"`
	Issues   []string `json:"issues"`
}

type BatchValidateResponse struct {
	Total      int                    `json:"total"`
	ValidCount int                    `json:"valid_count"`
	Results    []LoadValidationResult `json:"results"`
}

func (r CreateEquipmentLoadRequest) ValidateBusiness() error {
	if r.HeatKW > r.PowerKW*1.15 {
		return errors.New("heat output cannot exceed power by more than 15 percent")
	}
	if strings.TrimSpace(r.RedundancyGroup) == "" {
		return errors.New("redundancy group is required")
	}
	if r.PreferredZoneID != nil && *r.PreferredZoneID == 0 {
		return fmt.Errorf("preferred zone id must be positive")
	}
	return nil
}

func NewEquipmentLoad(req CreateEquipmentLoadRequest) (model.EquipmentLoad, error) {
	if err := req.ValidateBusiness(); err != nil {
		return model.EquipmentLoad{}, err
	}
	return model.EquipmentLoad{
		Name:            strings.TrimSpace(req.Name),
		PowerKW:         req.PowerKW,
		HeatKW:          req.HeatKW,
		AirflowCFM:      req.AirflowCFM,
		RackUnits:       req.RackUnits,
		RedundancyGroup: strings.ToUpper(strings.TrimSpace(req.RedundancyGroup)),
		PreferredZoneID: req.PreferredZoneID,
		LoadStatus:      req.LoadStatus,
	}, nil
}
