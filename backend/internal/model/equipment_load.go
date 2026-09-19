package model

import "time"

type EquipmentLoad struct {
	ID              uint         `gorm:"primaryKey" json:"id"`
	Name            string       `gorm:"size:120;not null;uniqueIndex" json:"name"`
	PowerKW         float64      `gorm:"not null" json:"power_kw"`
	HeatKW          float64      `gorm:"not null" json:"heat_kw"`
	AirflowCFM      float64      `gorm:"not null" json:"airflow_cfm"`
	RackUnits       int          `gorm:"not null" json:"rack_units"`
	RedundancyGroup string       `gorm:"size:64;not null;index" json:"redundancy_group"`
	PreferredZoneID *uint        `gorm:"index" json:"preferred_zone_id"`
	LoadStatus      string       `gorm:"size:24;not null;index" json:"load_status"`
	CreatedAt       time.Time    `json:"created_at"`
	UpdatedAt       time.Time    `json:"updated_at"`
	PreferredZone   *ThermalZone `gorm:"foreignKey:PreferredZoneID" json:"preferred_zone,omitempty"`
}

func (EquipmentLoad) TableName() string { return "equipment_loads" }

func (e EquipmentLoad) IsPlannable() bool {
	return e.LoadStatus == "ready"
}

func (e EquipmentLoad) HeatRatio() float64 {
	if e.PowerKW == 0 {
		return 0
	}
	return e.HeatKW / e.PowerKW
}
