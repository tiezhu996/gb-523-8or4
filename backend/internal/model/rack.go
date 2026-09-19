package model

import (
	"time"

	"datacenter-thermal-capacity-planner/backend/internal/constants"
)

type Rack struct {
	ID              uint                 `gorm:"primaryKey" json:"id"`
	ZoneID          uint                 `gorm:"not null;index;uniqueIndex:idx_rack_position" json:"zone_id"`
	RackCode        string               `gorm:"size:32;not null;uniqueIndex" json:"rack_code"`
	RowIndex        int                  `gorm:"not null;uniqueIndex:idx_rack_position" json:"row_index"`
	ColumnIndex     int                  `gorm:"not null;uniqueIndex:idx_rack_position" json:"column_index"`
	PowerLimitKW    float64              `gorm:"not null" json:"power_limit_kw"`
	AirflowLimitCFM float64              `gorm:"not null" json:"airflow_limit_cfm"`
	RackUnits       int                  `gorm:"not null;default:42" json:"rack_units"`
	RackStatus      constants.RackStatus `gorm:"size:24;not null;index" json:"rack_status"`
	Version         uint                 `gorm:"not null;default:1" json:"version"`
	CreatedAt       time.Time            `json:"created_at"`
	UpdatedAt       time.Time            `json:"updated_at"`
	ThermalZone     ThermalZone          `gorm:"foreignKey:ZoneID" json:"thermal_zone,omitempty"`
}

func (Rack) TableName() string { return "racks" }

func (r Rack) IsUsable() bool {
	return constants.RackCanReceiveLoad(r.RackStatus)
}

func (r Rack) Coordinate() string {
	return r.RackCode
}
