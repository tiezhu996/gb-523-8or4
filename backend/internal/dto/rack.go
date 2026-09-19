package dto

import (
	"errors"
	"fmt"
	"strings"

	"datacenter-thermal-capacity-planner/backend/internal/constants"
	"datacenter-thermal-capacity-planner/backend/internal/model"
)

type CreateRackRequest struct {
	ZoneID          uint                 `json:"zone_id" binding:"required"`
	RackCode        string               `json:"rack_code" binding:"required,min=2,max=32"`
	RowIndex        int                  `json:"row_index" binding:"gte=0,lte=100"`
	ColumnIndex     int                  `json:"column_index" binding:"gte=0,lte=100"`
	PowerLimitKW    float64              `json:"power_limit_kw" binding:"required,gt=0,lte=500"`
	AirflowLimitCFM float64              `json:"airflow_limit_cfm" binding:"required,gt=0,lte=100000"`
	RackUnits       int                  `json:"rack_units" binding:"required,gte=12,lte=60"`
	RackStatus      constants.RackStatus `json:"rack_status" binding:"required"`
}

type UpdateRackRequest struct {
	ZoneID          uint                 `json:"zone_id" binding:"required"`
	RowIndex        int                  `json:"row_index" binding:"gte=0,lte=100"`
	ColumnIndex     int                  `json:"column_index" binding:"gte=0,lte=100"`
	PowerLimitKW    float64              `json:"power_limit_kw" binding:"required,gt=0,lte=500"`
	AirflowLimitCFM float64              `json:"airflow_limit_cfm" binding:"required,gt=0,lte=100000"`
	RackUnits       int                  `json:"rack_units" binding:"required,gte=12,lte=60"`
	RackStatus      constants.RackStatus `json:"rack_status" binding:"required"`
	Version         uint                 `json:"version" binding:"required"`
}

type RackUtilization struct {
	PowerKW        float64 `json:"power_kw"`
	AirflowCFM     float64 `json:"airflow_cfm"`
	RackUnits      int     `json:"rack_units"`
	PowerPercent   float64 `json:"power_percent"`
	AirflowPercent float64 `json:"airflow_percent"`
	UnitsPercent   float64 `json:"units_percent"`
}

type RackResponse struct {
	ID              uint                 `json:"id"`
	ZoneID          uint                 `json:"zone_id"`
	ZoneCode        string               `json:"zone_code"`
	RackCode        string               `json:"rack_code"`
	RowIndex        int                  `json:"row_index"`
	ColumnIndex     int                  `json:"column_index"`
	PowerLimitKW    float64              `json:"power_limit_kw"`
	AirflowLimitCFM float64              `json:"airflow_limit_cfm"`
	RackUnits       int                  `json:"rack_units"`
	RackStatus      constants.RackStatus `json:"rack_status"`
	Version         uint                 `json:"version"`
	Utilization     RackUtilization      `json:"utilization"`
}

func (r CreateRackRequest) ValidateBusiness() error {
	if !constants.ValidRackStatus(r.RackStatus) {
		return fmt.Errorf("unsupported rack status %q", r.RackStatus)
	}
	if strings.TrimSpace(r.RackCode) == "" {
		return errors.New("rack code is required")
	}
	return nil
}

func (r UpdateRackRequest) ValidateBusiness() error {
	if !constants.ValidRackStatus(r.RackStatus) {
		return fmt.Errorf("unsupported rack status %q", r.RackStatus)
	}
	if r.Version == 0 {
		return errors.New("rack version is required")
	}
	return nil
}

func NewRack(req CreateRackRequest) (model.Rack, error) {
	if err := req.ValidateBusiness(); err != nil {
		return model.Rack{}, err
	}
	return model.Rack{
		ZoneID:          req.ZoneID,
		RackCode:        strings.ToUpper(strings.TrimSpace(req.RackCode)),
		RowIndex:        req.RowIndex,
		ColumnIndex:     req.ColumnIndex,
		PowerLimitKW:    req.PowerLimitKW,
		AirflowLimitCFM: req.AirflowLimitCFM,
		RackUnits:       req.RackUnits,
		RackStatus:      req.RackStatus,
		Version:         1,
	}, nil
}
