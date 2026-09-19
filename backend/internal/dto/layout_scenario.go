package dto

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"datacenter-thermal-capacity-planner/backend/internal/constants"
	"datacenter-thermal-capacity-planner/backend/internal/model"
)

type CreateLayoutScenarioRequest struct {
	Name    string `json:"name" binding:"required,min=3,max=120"`
	LoadIDs []uint `json:"load_ids" binding:"required,min=1"`
}

type EvaluateScenarioRequest struct {
	Version uint `json:"version" binding:"required"`
}

type TransitionScenarioRequest struct {
	TargetStatus constants.ScenarioStatus `json:"target_status" binding:"required"`
	Version      uint                     `json:"version" binding:"required"`
	Reason       string                   `json:"reason" binding:"max=500"`
}

type PinRackRequest struct {
	LoadID  uint `json:"load_id" binding:"required"`
	RackID  uint `json:"rack_id" binding:"required"`
	Version uint `json:"version" binding:"required"`
}

type UnpinRackRequest struct {
	LoadID  uint `json:"load_id" binding:"required"`
	Version uint `json:"version" binding:"required"`
}

type RackPin struct {
	LoadID uint `json:"load_id"`
	RackID uint `json:"rack_id"`
}

func DecodeRackPins(raw string) []RackPin {
	pins := []RackPin{}
	if strings.TrimSpace(raw) == "" {
		return pins
	}
	_ = json.Unmarshal([]byte(raw), &pins)
	return pins
}

func EncodeRackPins(pins []RackPin) (string, error) {
	if pins == nil {
		pins = []RackPin{}
	}
	encoded, err := json.Marshal(pins)
	if err != nil {
		return "", fmt.Errorf("encode rack pins: %w", err)
	}
	return string(encoded), nil
}

func PinIndex(pins []RackPin) map[uint]uint {
	index := make(map[uint]uint, len(pins))
	for _, pin := range pins {
		index[pin.LoadID] = pin.RackID
	}
	return index
}

type ConstraintViolation struct {
	Code       string  `json:"code"`
	Severity   string  `json:"severity"`
	EntityType string  `json:"entity_type"`
	EntityID   uint    `json:"entity_id"`
	Message    string  `json:"message"`
	Actual     float64 `json:"actual"`
	Limit      float64 `json:"limit"`
	RackID     uint    `json:"rack_id,omitempty"`
	ZoneID     uint    `json:"zone_id,omitempty"`
}

type RackAssignment struct {
	LoadID         uint     `json:"load_id"`
	LoadName       string   `json:"load_name"`
	RackID         uint     `json:"rack_id"`
	RackCode       string   `json:"rack_code"`
	ZoneID         uint     `json:"zone_id"`
	ZoneCode       string   `json:"zone_code"`
	PowerKW        float64  `json:"power_kw"`
	HeatKW         float64  `json:"heat_kw"`
	AirflowCFM     float64  `json:"airflow_cfm"`
	RackUnits      int      `json:"rack_units"`
	PlacementScore float64  `json:"placement_score"`
	Pinned         bool     `json:"pinned"`
	Explanation    []string `json:"explanation"`
}

type ZoneThermalResult struct {
	ZoneID             uint    `json:"zone_id"`
	ZoneCode           string  `json:"zone_code"`
	AssignedHeatKW     float64 `json:"assigned_heat_kw"`
	NeighborHeatKW     float64 `json:"neighbor_heat_kw"`
	EstimatedReturnC   float64 `json:"estimated_return_c"`
	TemperatureMarginC float64 `json:"temperature_margin_c"`
	CoolingMarginKW    float64 `json:"cooling_margin_kw"`
}

type ScenarioResponse struct {
	ID                   uint                     `json:"id"`
	Name                 string                   `json:"name"`
	ScenarioStatus       constants.ScenarioStatus `json:"scenario_status"`
	LoadIDs              []uint                   `json:"load_ids"`
	Pins                 []RackPin                `json:"pins"`
	Assignments          []RackAssignment         `json:"assignments"`
	ZoneResults          []ZoneThermalResult      `json:"zone_results"`
	Violations           []ConstraintViolation    `json:"violations"`
	TotalPowerKW         float64                  `json:"total_power_kw"`
	PeakTempC            float64                  `json:"peak_temp_c"`
	Score                float64                  `json:"score"`
	Version              uint                     `json:"version"`
	AlgorithmVersion     string                   `json:"algorithm_version"`
	CreatedBy            uint                     `json:"created_by"`
	ApprovedBy           *uint                    `json:"approved_by"`
	HasCriticalViolation bool                     `json:"has_critical_violation"`
}

type ScenarioComparison struct {
	Left          ScenarioResponse `json:"left"`
	Right         ScenarioResponse `json:"right"`
	ScoreDelta    float64          `json:"score_delta"`
	PowerDeltaKW  float64          `json:"power_delta_kw"`
	PeakTempDelta float64          `json:"peak_temp_delta_c"`
	Summary       []string         `json:"summary"`
}

func (r CreateLayoutScenarioRequest) ValidateBusiness() error {
	if strings.TrimSpace(r.Name) == "" {
		return errors.New("scenario name is required")
	}
	seen := map[uint]bool{}
	for _, id := range r.LoadIDs {
		if id == 0 {
			return errors.New("load ids must be positive")
		}
		if seen[id] {
			return errors.New("load ids must not contain duplicates")
		}
		seen[id] = true
	}
	return nil
}

func DecodeScenario(value model.LayoutScenario) ScenarioResponse {
	response := ScenarioResponse{
		ID: value.ID, Name: value.Name, ScenarioStatus: value.ScenarioStatus,
		TotalPowerKW: value.TotalPowerKW, PeakTempC: value.PeakTempC,
		Score: value.Score, Version: value.Version, AlgorithmVersion: value.AlgorithmVersion,
		CreatedBy: value.CreatedBy, ApprovedBy: value.ApprovedBy,
		Pins:        DecodeRackPins(value.PinnedRackJSON),
		LoadIDs:     []uint{},
		Assignments: []RackAssignment{}, ZoneResults: []ZoneThermalResult{}, Violations: []ConstraintViolation{},
	}
	_ = json.Unmarshal([]byte(value.InputSnapshotJSON), &struct {
		LoadIDs *[]uint `json:"load_ids"`
	}{LoadIDs: &response.LoadIDs})
	if response.LoadIDs == nil {
		response.LoadIDs = []uint{}
	}
	if response.Pins == nil {
		response.Pins = []RackPin{}
	}
	_ = json.Unmarshal([]byte(value.RackAssignmentsJSON), &response.Assignments)
	_ = json.Unmarshal([]byte(value.ZoneResultsJSON), &response.ZoneResults)
	_ = json.Unmarshal([]byte(value.ConstraintViolationsJSON), &response.Violations)
	for _, violation := range response.Violations {
		if violation.Severity == "critical" {
			response.HasCriticalViolation = true
			break
		}
	}
	return response
}
