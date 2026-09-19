package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"datacenter-thermal-capacity-planner/backend/internal/audit"
	"datacenter-thermal-capacity-planner/backend/internal/constants"
	"datacenter-thermal-capacity-planner/backend/internal/dto"
	"datacenter-thermal-capacity-planner/backend/internal/model"
	"datacenter-thermal-capacity-planner/backend/internal/planner"
	"datacenter-thermal-capacity-planner/backend/internal/repository"
	"datacenter-thermal-capacity-planner/backend/internal/web"
)

type LayoutScenarioService struct {
	scenarios *repository.LayoutScenarioRepository
	zones     *repository.ThermalZoneRepository
	racks     *repository.RackRepository
	loads     *repository.EquipmentLoadRepository
	engine    *planner.Engine
}

type scenarioInputSnapshot struct {
	LoadIDs          []uint        `json:"load_ids"`
	Pins             []dto.RackPin `json:"pins"`
	AlgorithmVersion string        `json:"algorithm_version"`
}

func NewLayoutScenarioService(scenarios *repository.LayoutScenarioRepository, zones *repository.ThermalZoneRepository, racks *repository.RackRepository, loads *repository.EquipmentLoadRepository, engine *planner.Engine) *LayoutScenarioService {
	return &LayoutScenarioService{scenarios: scenarios, zones: zones, racks: racks, loads: loads, engine: engine}
}

func (s *LayoutScenarioService) List(ctx context.Context, search, status string, page, size int) ([]dto.ScenarioResponse, int64, error) {
	items, total, err := s.scenarios.List(ctx, search, status, page, size)
	if err != nil {
		return nil, 0, err
	}
	responses := make([]dto.ScenarioResponse, 0, len(items))
	for _, item := range items {
		responses = append(responses, dto.DecodeScenario(item))
	}
	return responses, total, nil
}

func (s *LayoutScenarioService) Get(ctx context.Context, id uint) (dto.ScenarioResponse, error) {
	item, err := s.scenarios.Get(ctx, id)
	if err != nil {
		return dto.ScenarioResponse{}, err
	}
	return dto.DecodeScenario(item), nil
}

func (s *LayoutScenarioService) Create(ctx context.Context, req dto.CreateLayoutScenarioRequest, actor audit.Entry) (dto.ScenarioResponse, error) {
	if err := req.ValidateBusiness(); err != nil {
		return dto.ScenarioResponse{}, web.Unprocessable("INVALID_SCENARIO", err.Error(), err)
	}
	loads, err := s.loads.FindByIDs(ctx, req.LoadIDs)
	if err != nil {
		return dto.ScenarioResponse{}, err
	}
	for _, load := range loads {
		if !load.IsPlannable() {
			return dto.ScenarioResponse{}, web.Unprocessable("LOAD_NOT_READY", fmt.Sprintf("load %d is not ready for planning", load.ID), nil)
		}
	}
	snapshot, err := json.Marshal(scenarioInputSnapshot{LoadIDs: req.LoadIDs, Pins: []dto.RackPin{}, AlgorithmVersion: planner.AlgorithmVersion})
	if err != nil {
		return dto.ScenarioResponse{}, web.Internal(fmt.Errorf("encode scenario input: %w", err))
	}
	item := model.LayoutScenario{
		Name: strings.TrimSpace(req.Name), ScenarioStatus: constants.ScenarioDraft,
		RackAssignmentsJSON: "[]", PinnedRackJSON: "[]", InputSnapshotJSON: string(snapshot), ZoneResultsJSON: "[]",
		ConstraintViolationsJSON: "[]", AlgorithmVersion: planner.AlgorithmVersion,
		Version: 1, CreatedBy: actor.ActorID,
	}
	actor.Action = "layout_scenario.create"
	actor.EntityType = "layout_scenario"
	actor.AfterSummary = fmt.Sprintf("draft loads=%v algorithm=%s", req.LoadIDs, planner.AlgorithmVersion)
	if err := s.scenarios.Create(ctx, &item, actor); err != nil {
		return dto.ScenarioResponse{}, err
	}
	return dto.DecodeScenario(item), nil
}

func (s *LayoutScenarioService) Evaluate(ctx context.Context, id, version uint, actor audit.Entry) (dto.ScenarioResponse, error) {
	current, err := s.scenarios.Get(ctx, id)
	if err != nil {
		return dto.ScenarioResponse{}, err
	}
	var input scenarioInputSnapshot
	if err := json.Unmarshal([]byte(current.InputSnapshotJSON), &input); err != nil || len(input.LoadIDs) == 0 {
		return dto.ScenarioResponse{}, web.Unprocessable("INVALID_SCENARIO_SNAPSHOT", "scenario input snapshot cannot be evaluated", err)
	}
	pins := dto.DecodeRackPins(current.PinnedRackJSON)
	zones, err := s.zones.All(ctx)
	if err != nil {
		return dto.ScenarioResponse{}, err
	}
	racks, err := s.racks.All(ctx)
	if err != nil {
		return dto.ScenarioResponse{}, err
	}
	loads, err := s.loads.FindByIDs(ctx, input.LoadIDs)
	if err != nil {
		return dto.ScenarioResponse{}, err
	}
	result := s.engine.Evaluate(zones, racks, loads, pins)
	if len(result.PinConflicts) > 0 {
		conflicts := enrichPinConflicts(result.PinConflicts, racks, zones)
		actor.Action = "layout_scenario.evaluate.blocked"
		actor.EntityType = "layout_scenario"
		actor.BeforeSummary = fmt.Sprintf("draft pins=%d", len(pins))
		actor.AfterSummary = fmt.Sprintf("pin_conflicts=%d", len(conflicts))
		_ = s.scenarios.RecordPinBlock(ctx, id, actor)
		return dto.ScenarioResponse{}, web.UnprocessableDetails("PINNED_PLACEMENT_CONFLICT",
			"one or more fixed rack assignments violate rack, thermal zone or redundancy constraints; the draft was kept",
			map[string]any{"scenario_id": id, "version": current.Version, "conflicts": conflicts})
	}
	actor.Action = "layout_scenario.evaluate.start"
	actor.EntityType = "layout_scenario"
	evaluating, err := s.scenarios.BeginEvaluation(ctx, id, version, actor)
	if err != nil {
		return dto.ScenarioResponse{}, err
	}
	assignments, err := json.Marshal(result.Assignments)
	if err != nil {
		return dto.ScenarioResponse{}, web.Internal(fmt.Errorf("encode assignments: %w", err))
	}
	zoneResults, err := json.Marshal(result.ZoneResults)
	if err != nil {
		return dto.ScenarioResponse{}, web.Internal(fmt.Errorf("encode zone results: %w", err))
	}
	violations, err := json.Marshal(result.Violations)
	if err != nil {
		return dto.ScenarioResponse{}, web.Internal(fmt.Errorf("encode violations: %w", err))
	}
	fullSnapshot, err := json.Marshal(struct {
		LoadIDs          []uint                `json:"load_ids"`
		Pins             []dto.RackPin         `json:"pins"`
		AlgorithmVersion string                `json:"algorithm_version"`
		Zones            []model.ThermalZone   `json:"zones"`
		Racks            []model.Rack          `json:"racks"`
		Loads            []model.EquipmentLoad `json:"loads"`
	}{LoadIDs: input.LoadIDs, Pins: pins, AlgorithmVersion: planner.AlgorithmVersion, Zones: zones, Racks: racks, Loads: loads})
	if err != nil {
		return dto.ScenarioResponse{}, web.Internal(fmt.Errorf("encode evaluation snapshot: %w", err))
	}
	update := repository.EvaluationUpdate{
		AssignmentsJSON: string(assignments), SnapshotJSON: string(fullSnapshot),
		ZoneResultsJSON: string(zoneResults), ViolationsJSON: string(violations),
		TotalPowerKW: result.TotalPower, PeakTempC: result.PeakTemp, Score: result.Score,
	}
	actor.Action = "layout_scenario.evaluate.finish"
	actor.BeforeSummary = fmt.Sprintf("algorithm=%s input_loads=%d", planner.AlgorithmVersion, len(loads))
	actor.AfterSummary = fmt.Sprintf("score=%.2f assignments=%d violations=%d", result.Score, len(result.Assignments), len(result.Violations))
	if err := s.scenarios.FinishEvaluation(ctx, evaluating, update, actor); err != nil {
		return dto.ScenarioResponse{}, err
	}
	return s.Get(ctx, id)
}

func (s *LayoutScenarioService) Transition(ctx context.Context, id uint, req dto.TransitionScenarioRequest, actor audit.Entry) (dto.ScenarioResponse, error) {
	current, err := s.scenarios.Get(ctx, id)
	if err != nil {
		return dto.ScenarioResponse{}, err
	}
	if current.Version != req.Version {
		return dto.ScenarioResponse{}, web.Conflict("SCENARIO_VERSION_CONFLICT", "scenario was changed by another user", nil)
	}
	if !constants.ValidScenarioStatus(req.TargetStatus) || !constants.CanTransitionScenario(current.ScenarioStatus, req.TargetStatus) {
		return dto.ScenarioResponse{}, web.Unprocessable("INVALID_SCENARIO_TRANSITION", fmt.Sprintf("cannot transition scenario from %s to %s", current.ScenarioStatus, req.TargetStatus), nil)
	}
	decoded := dto.DecodeScenario(current)
	if req.TargetStatus == constants.ScenarioApproved && decoded.HasCriticalViolation {
		return dto.ScenarioResponse{}, web.Unprocessable("CRITICAL_VIOLATIONS", "scenario cannot be approved while critical violations remain", nil)
	}
	actor.Action = "layout_scenario.transition"
	if req.TargetStatus == constants.ScenarioApproved {
		actor.Action = "layout_scenario.approve"
	}
	actor.EntityType = "layout_scenario"
	actor.BeforeSummary = string(current.ScenarioStatus)
	actor.AfterSummary = fmt.Sprintf("%s reason=%s", req.TargetStatus, strings.TrimSpace(req.Reason))
	if err := s.scenarios.Transition(ctx, current, req.TargetStatus, actor.ActorID, actor); err != nil {
		return dto.ScenarioResponse{}, err
	}
	return s.Get(ctx, id)
}

func (s *LayoutScenarioService) Pin(ctx context.Context, id uint, req dto.PinRackRequest, actor audit.Entry) (dto.ScenarioResponse, error) {
	current, err := s.scenarios.Get(ctx, id)
	if err != nil {
		return dto.ScenarioResponse{}, err
	}
	if current.ScenarioStatus != constants.ScenarioDraft {
		return dto.ScenarioResponse{}, web.Unprocessable("SCENARIO_NOT_DRAFT", "rack pins can only be changed while the scenario is a draft", nil)
	}
	var input scenarioInputSnapshot
	if err := json.Unmarshal([]byte(current.InputSnapshotJSON), &input); err != nil {
		return dto.ScenarioResponse{}, web.Unprocessable("INVALID_SCENARIO_SNAPSHOT", "scenario input snapshot cannot be edited", err)
	}
	if !containsID(input.LoadIDs, req.LoadID) {
		return dto.ScenarioResponse{}, web.Unprocessable("PINNED_LOAD_MISSING", "load is not part of this scenario", nil)
	}
	rack, err := s.racks.Get(ctx, req.RackID)
	if err != nil {
		return dto.ScenarioResponse{}, web.Unprocessable("PINNED_RACK_MISSING", "rack does not exist", nil)
	}
	pins := dto.DecodeRackPins(current.PinnedRackJSON)
	replaced := false
	for index := range pins {
		if pins[index].LoadID == req.LoadID {
			pins[index].RackID = req.RackID
			replaced = true
		}
	}
	if !replaced {
		pins = append(pins, dto.RackPin{LoadID: req.LoadID, RackID: req.RackID})
	}
	encoded, err := dto.EncodeRackPins(pins)
	if err != nil {
		return dto.ScenarioResponse{}, web.Internal(err)
	}
	actor.Action = "layout_scenario.pin"
	actor.EntityType = "layout_scenario"
	actor.BeforeSummary = fmt.Sprintf("pins=%d", len(dto.DecodeRackPins(current.PinnedRackJSON)))
	actor.AfterSummary = fmt.Sprintf("load=%d rack=%s pins=%d", req.LoadID, rack.RackCode, len(pins))
	updated, err := s.scenarios.UpdatePins(ctx, id, req.Version, encoded, actor)
	if err != nil {
		return dto.ScenarioResponse{}, err
	}
	return dto.DecodeScenario(updated), nil
}

func (s *LayoutScenarioService) Unpin(ctx context.Context, id uint, req dto.UnpinRackRequest, actor audit.Entry) (dto.ScenarioResponse, error) {
	current, err := s.scenarios.Get(ctx, id)
	if err != nil {
		return dto.ScenarioResponse{}, err
	}
	if current.ScenarioStatus != constants.ScenarioDraft {
		return dto.ScenarioResponse{}, web.Unprocessable("SCENARIO_NOT_DRAFT", "rack pins can only be changed while the scenario is a draft", nil)
	}
	pins := dto.DecodeRackPins(current.PinnedRackJSON)
	next := make([]dto.RackPin, 0, len(pins))
	removed := false
	for _, pin := range pins {
		if pin.LoadID == req.LoadID {
			removed = true
			continue
		}
		next = append(next, pin)
	}
	if !removed {
		return dto.ScenarioResponse{}, web.Unprocessable("PIN_NOT_FOUND", "load has no fixed rack in this draft", nil)
	}
	encoded, err := dto.EncodeRackPins(next)
	if err != nil {
		return dto.ScenarioResponse{}, web.Internal(err)
	}
	actor.Action = "layout_scenario.unpin"
	actor.EntityType = "layout_scenario"
	actor.BeforeSummary = fmt.Sprintf("pins=%d", len(pins))
	actor.AfterSummary = fmt.Sprintf("load=%d pins=%d", req.LoadID, len(next))
	updated, err := s.scenarios.UpdatePins(ctx, id, req.Version, encoded, actor)
	if err != nil {
		return dto.ScenarioResponse{}, err
	}
	return dto.DecodeScenario(updated), nil
}

func containsID(ids []uint, target uint) bool {
	for _, id := range ids {
		if id == target {
			return true
		}
	}
	return false
}

type pinConflictDetail struct {
	dto.ConstraintViolation
	LoadName string `json:"load_name,omitempty"`
	RackCode string `json:"rack_code,omitempty"`
	ZoneCode string `json:"zone_code,omitempty"`
}

func enrichPinConflicts(conflicts []dto.ConstraintViolation, racks []model.Rack, zones []model.ThermalZone) []pinConflictDetail {
	rackByID := map[uint]model.Rack{}
	for _, rack := range racks {
		rackByID[rack.ID] = rack
	}
	zoneByID := map[uint]model.ThermalZone{}
	for _, zone := range zones {
		zoneByID[zone.ID] = zone
	}
	details := make([]pinConflictDetail, 0, len(conflicts))
	for _, conflict := range conflicts {
		detail := pinConflictDetail{ConstraintViolation: conflict}
		if rack, ok := rackByID[conflict.RackID]; ok {
			detail.RackCode = rack.RackCode
			if zone, ok := zoneByID[rack.ZoneID]; ok {
				detail.ZoneCode = zone.ZoneCode
			}
		}
		if zone, ok := zoneByID[conflict.ZoneID]; ok && detail.ZoneCode == "" {
			detail.ZoneCode = zone.ZoneCode
		}
		details = append(details, detail)
	}
	return details
}

func (s *LayoutScenarioService) Compare(ctx context.Context, leftID, rightID uint) (dto.ScenarioComparison, error) {
	left, err := s.Get(ctx, leftID)
	if err != nil {
		return dto.ScenarioComparison{}, err
	}
	right, err := s.Get(ctx, rightID)
	if err != nil {
		return dto.ScenarioComparison{}, err
	}
	summary := []string{
		fmt.Sprintf("Score changed by %.2f points", right.Score-left.Score),
		fmt.Sprintf("Peak return temperature changed by %.2f C", right.PeakTempC-left.PeakTempC),
		fmt.Sprintf("Critical flag changed from %t to %t", left.HasCriticalViolation, right.HasCriticalViolation),
	}
	return dto.ScenarioComparison{
		Left: left, Right: right, ScoreDelta: right.Score - left.Score,
		PowerDeltaKW:  right.TotalPowerKW - left.TotalPowerKW,
		PeakTempDelta: right.PeakTempC - left.PeakTempC, Summary: summary,
	}, nil
}
