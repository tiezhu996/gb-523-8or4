package model

import (
	"time"

	"datacenter-thermal-capacity-planner/backend/internal/constants"
)

type LayoutScenario struct {
	ID                       uint                     `gorm:"primaryKey" json:"id"`
	Name                     string                   `gorm:"size:120;not null;uniqueIndex" json:"name"`
	ScenarioStatus           constants.ScenarioStatus `gorm:"size:32;not null;index" json:"scenario_status"`
	RackAssignmentsJSON      string                   `gorm:"type:text;not null;default:'[]'" json:"rack_assignments_json"`
	InputSnapshotJSON        string                   `gorm:"type:text;not null;default:'{}'" json:"input_snapshot_json"`
	ZoneResultsJSON          string                   `gorm:"type:text;not null;default:'[]'" json:"zone_results_json"`
	TotalPowerKW             float64                  `gorm:"not null;default:0" json:"total_power_kw"`
	PeakTempC                float64                  `gorm:"not null;default:0" json:"peak_temp_c"`
	ConstraintViolationsJSON string                   `gorm:"type:text;not null;default:'[]'" json:"constraint_violations_json"`
	Score                    float64                  `gorm:"not null;default:0" json:"score"`
	AlgorithmVersion         string                   `gorm:"size:32;not null;default:'thermal-v1'" json:"algorithm_version"`
	Version                  uint                     `gorm:"not null;default:1" json:"version"`
	CreatedBy                uint                     `gorm:"not null;index" json:"created_by"`
	ApprovedBy               *uint                    `gorm:"index" json:"approved_by"`
	CreatedAt                time.Time                `json:"created_at"`
	UpdatedAt                time.Time                `json:"updated_at"`
}

func (LayoutScenario) TableName() string { return "layout_scenarios" }

func (s LayoutScenario) IsEditable() bool {
	return s.ScenarioStatus == constants.ScenarioDraft
}

func (s LayoutScenario) IsTerminal() bool {
	return s.ScenarioStatus == constants.ScenarioArchived
}
