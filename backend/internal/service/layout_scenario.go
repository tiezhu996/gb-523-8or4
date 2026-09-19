package service

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
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
	LoadIDs          []uint `json:"load_ids"`
	AlgorithmVersion string `json:"algorithm_version"`
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
	snapshot, err := json.Marshal(scenarioInputSnapshot{LoadIDs: req.LoadIDs, AlgorithmVersion: planner.AlgorithmVersion})
	if err != nil {
		return dto.ScenarioResponse{}, web.Internal(fmt.Errorf("encode scenario input: %w", err))
	}
	item := model.LayoutScenario{
		Name: strings.TrimSpace(req.Name), ScenarioStatus: constants.ScenarioDraft,
		RackAssignmentsJSON: "[]", InputSnapshotJSON: string(snapshot), ZoneResultsJSON: "[]",
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

// SetPins replaces the draft pin set. An empty list clears every pin. Pinning
// only records the instruction; feasibility is enforced during evaluation so
// that an infeasible pin is retained on the draft and reported with evidence.
func (s *LayoutScenarioService) SetPins(ctx context.Context, id uint, req dto.UpdatePinsRequest, actor audit.Entry) (dto.ScenarioResponse, error) {
	if err := req.ValidateBusiness(); err != nil {
		return dto.ScenarioResponse{}, web.Unprocessable("INVALID_SCENARIO_PINS", err.Error(), err)
	}
	current, err := s.scenarios.Get(ctx, id)
	if err != nil {
		return dto.ScenarioResponse{}, err
	}
	if current.Version != req.Version {
		return dto.ScenarioResponse{}, web.Conflict("SCENARIO_VERSION_CONFLICT", "scenario was changed by another user", nil)
	}
	if !current.IsEditable() {
		return dto.ScenarioResponse{}, web.Unprocessable("SCENARIO_NOT_DRAFT", "racks can only be pinned while the scenario is a draft", nil)
	}
	var input scenarioInputSnapshot
	if err := json.Unmarshal([]byte(current.InputSnapshotJSON), &input); err != nil || len(input.LoadIDs) == 0 {
		return dto.ScenarioResponse{}, web.Unprocessable("INVALID_SCENARIO_SNAPSHOT", "scenario input snapshot cannot accept pins", err)
	}
	loadByID := make(map[uint]model.EquipmentLoad, len(input.LoadIDs))
	loads, err := s.loads.FindByIDs(ctx, input.LoadIDs)
	if err != nil {
		return dto.ScenarioResponse{}, err
	}
	for _, load := range loads {
		loadByID[load.ID] = load
	}
	racks, err := s.racks.All(ctx)
	if err != nil {
		return dto.ScenarioResponse{}, err
	}
	rackByID := make(map[uint]model.Rack, len(racks))
	for _, rack := range racks {
		rackByID[rack.ID] = rack
	}
	pins := make([]dto.PinnedRack, 0, len(req.Pins))
	for _, requested := range req.Pins {
		load, ok := loadByID[requested.LoadID]
		if !ok {
			return dto.ScenarioResponse{}, web.Unprocessable("PIN_LOAD_NOT_IN_SCENARIO", fmt.Sprintf("load %d is not part of this scenario draft", requested.LoadID), nil)
		}
		if !load.IsPlannable() {
			return dto.ScenarioResponse{}, web.Unprocessable("LOAD_NOT_READY", fmt.Sprintf("load %d is not ready for planning", load.ID), nil)
		}
		rack, ok := rackByID[requested.RackID]
		if !ok {
			return dto.ScenarioResponse{}, web.Unprocessable("PIN_RACK_NOT_FOUND", fmt.Sprintf("rack %d does not exist", requested.RackID), nil)
		}
		pins = append(pins, dto.PinnedRack{
			LoadID: load.ID, RackID: rack.ID, RackCode: rack.RackCode,
			ZoneID: rack.ZoneID, ZoneCode: rack.ThermalZone.ZoneCode,
		})
	}
	// Keep the persisted representation deterministic.
	sortPins(pins)
	encoded, err := json.Marshal(pins)
	if err != nil {
		return dto.ScenarioResponse{}, web.Internal(fmt.Errorf("encode rack pins: %w", err))
	}
	actor.Action = "layout_scenario.set_pins"
	actor.EntityType = "layout_scenario"
	actor.BeforeSummary = fmt.Sprintf("pins=%d", len(dto.DecodePins(current.RackPinsJSON)))
	actor.AfterSummary = fmt.Sprintf("pins=%d", len(pins))
	if err := s.scenarios.UpdatePins(ctx, current, string(encoded), actor); err != nil {
		return dto.ScenarioResponse{}, err
	}
	return s.Get(ctx, id)
}

// sortPins orders pins by load id so the same pin set always serializes identically.
func sortPins(pins []dto.PinnedRack) {
	sort.SliceStable(pins, func(i, j int) bool {
		if pins[i].LoadID == pins[j].LoadID {
			return pins[i].RackID < pins[j].RackID
		}
		return pins[i].LoadID < pins[j].LoadID
	})
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
	pins := dto.DecodePins(current.RackPinsJSON)
	// A pinned placement that fails any constraint rejects the evaluation while
	// preserving the draft; no state transition is attempted on failure.
	if conflicts := s.engine.CheckPins(zones, racks, loads, pins); len(conflicts) > 0 {
		return dto.ScenarioResponse{}, web.Conflict("PIN_CONFLICT", "one or more pinned loads violate rack, thermal zone or redundancy constraints", nil).WithDetails(conflicts)
	}
	actor.Action = "layout_scenario.evaluate.start"
	actor.EntityType = "layout_scenario"
	evaluating, err := s.scenarios.BeginEvaluation(ctx, id, version, actor)
	if err != nil {
		return dto.ScenarioResponse{}, err
	}
	result := s.engine.Evaluate(zones, racks, loads, pins...)
	if result.PinnedFailure {
		return dto.ScenarioResponse{}, web.Conflict("PIN_CONFLICT", "one or more pinned loads violate rack, thermal zone or redundancy constraints", nil).WithDetails(result.Violations)
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
		AlgorithmVersion string                `json:"algorithm_version"`
		Pins             []dto.PinnedRack      `json:"pins"`
		Zones            []model.ThermalZone   `json:"zones"`
		Racks            []model.Rack          `json:"racks"`
		Loads            []model.EquipmentLoad `json:"loads"`
	}{LoadIDs: input.LoadIDs, AlgorithmVersion: planner.AlgorithmVersion, Pins: pins, Zones: zones, Racks: racks, Loads: loads})
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
