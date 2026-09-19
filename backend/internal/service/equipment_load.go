package service

import (
	"context"
	"fmt"
	"strings"

	"datacenter-thermal-capacity-planner/backend/internal/audit"
	"datacenter-thermal-capacity-planner/backend/internal/dto"
	"datacenter-thermal-capacity-planner/backend/internal/model"
	"datacenter-thermal-capacity-planner/backend/internal/repository"
	"datacenter-thermal-capacity-planner/backend/internal/web"
)

type EquipmentLoadService struct {
	loads *repository.EquipmentLoadRepository
	zones *repository.ThermalZoneRepository
}

func NewEquipmentLoadService(loads *repository.EquipmentLoadRepository, zones *repository.ThermalZoneRepository) *EquipmentLoadService {
	return &EquipmentLoadService{loads: loads, zones: zones}
}

func (s *EquipmentLoadService) List(ctx context.Context, search, status string, page, size int) ([]dto.EquipmentLoadResponse, int64, error) {
	loads, total, err := s.loads.List(ctx, search, status, page, size)
	if err != nil {
		return nil, 0, err
	}
	responses := make([]dto.EquipmentLoadResponse, 0, len(loads))
	for _, load := range loads {
		responses = append(responses, decodeLoad(load))
	}
	return responses, total, nil
}

func (s *EquipmentLoadService) Get(ctx context.Context, id uint) (dto.EquipmentLoadResponse, error) {
	load, err := s.loads.Get(ctx, id)
	if err != nil {
		return dto.EquipmentLoadResponse{}, err
	}
	return decodeLoad(load), nil
}

func (s *EquipmentLoadService) Create(ctx context.Context, req dto.CreateEquipmentLoadRequest, actor audit.Entry) (dto.EquipmentLoadResponse, error) {
	load, err := dto.NewEquipmentLoad(req)
	if err != nil {
		return dto.EquipmentLoadResponse{}, web.Unprocessable("INVALID_LOAD", err.Error(), err)
	}
	actor.Action = "equipment_load.create"
	actor.EntityType = "equipment_load"
	actor.AfterSummary = fmt.Sprintf("%s power=%.2f heat=%.2f airflow=%.2f group=%s", load.Name, load.PowerKW, load.HeatKW, load.AirflowCFM, load.RedundancyGroup)
	if err := s.loads.Create(ctx, &load, actor); err != nil {
		return dto.EquipmentLoadResponse{}, err
	}
	return s.Get(ctx, load.ID)
}

func (s *EquipmentLoadService) Update(ctx context.Context, id uint, req dto.UpdateEquipmentLoadRequest, actor audit.Entry) (dto.EquipmentLoadResponse, error) {
	createShape := dto.CreateEquipmentLoadRequest{
		Name: req.Name, PowerKW: req.PowerKW, HeatKW: req.HeatKW, AirflowCFM: req.AirflowCFM,
		RackUnits: req.RackUnits, RedundancyGroup: req.RedundancyGroup, PreferredZoneID: req.PreferredZoneID,
		LoadStatus: req.LoadStatus,
	}
	if err := createShape.ValidateBusiness(); err != nil {
		return dto.EquipmentLoadResponse{}, web.Unprocessable("INVALID_LOAD", err.Error(), err)
	}
	current, err := s.loads.Get(ctx, id)
	if err != nil {
		return dto.EquipmentLoadResponse{}, err
	}
	if req.PreferredZoneID != nil {
		if _, err := s.zones.Get(ctx, *req.PreferredZoneID); err != nil {
			return dto.EquipmentLoadResponse{}, web.Unprocessable("PREFERRED_ZONE_NOT_FOUND", "preferred thermal zone does not exist", err)
		}
	}
	updated := model.EquipmentLoad{
		ID: id, Name: strings.TrimSpace(req.Name), PowerKW: req.PowerKW, HeatKW: req.HeatKW,
		AirflowCFM: req.AirflowCFM, RackUnits: req.RackUnits,
		RedundancyGroup: strings.ToUpper(strings.TrimSpace(req.RedundancyGroup)),
		PreferredZoneID: req.PreferredZoneID, LoadStatus: req.LoadStatus,
	}
	actor.Action = "equipment_load.update"
	actor.EntityType = "equipment_load"
	actor.BeforeSummary = fmt.Sprintf("power=%.2f heat=%.2f status=%s", current.PowerKW, current.HeatKW, current.LoadStatus)
	actor.AfterSummary = fmt.Sprintf("power=%.2f heat=%.2f status=%s", updated.PowerKW, updated.HeatKW, updated.LoadStatus)
	if err := s.loads.Update(ctx, &updated, actor); err != nil {
		return dto.EquipmentLoadResponse{}, err
	}
	return s.Get(ctx, id)
}

func (s *EquipmentLoadService) Validate(ctx context.Context) (dto.BatchValidateResponse, error) {
	loads, err := s.loads.AllReady(ctx)
	if err != nil {
		return dto.BatchValidateResponse{}, err
	}
	response := dto.BatchValidateResponse{Total: len(loads), Results: make([]dto.LoadValidationResult, 0, len(loads))}
	for _, load := range loads {
		issues := []string{}
		if load.HeatKW > load.PowerKW*1.15 {
			issues = append(issues, "heat output exceeds supported planning ratio")
		}
		if load.RackUnits > 60 {
			issues = append(issues, "rack unit request exceeds supported rack size")
		}
		if load.PreferredZoneID != nil {
			if _, err := s.zones.Get(ctx, *load.PreferredZoneID); err != nil {
				issues = append(issues, "preferred thermal zone is unavailable")
			}
		}
		valid := len(issues) == 0
		if valid {
			response.ValidCount++
		}
		response.Results = append(response.Results, dto.LoadValidationResult{LoadID: load.ID, LoadName: load.Name, Valid: valid, Issues: issues})
	}
	return response, nil
}

func decodeLoad(load model.EquipmentLoad) dto.EquipmentLoadResponse {
	zoneCode := ""
	if load.PreferredZone != nil {
		zoneCode = load.PreferredZone.ZoneCode
	}
	state := "valid"
	if !load.IsPlannable() {
		state = "inactive"
	} else if load.HeatKW > load.PowerKW*1.15 {
		state = "invalid"
	}
	return dto.EquipmentLoadResponse{
		ID: load.ID, Name: load.Name, PowerKW: load.PowerKW, HeatKW: load.HeatKW,
		AirflowCFM: load.AirflowCFM, RackUnits: load.RackUnits, RedundancyGroup: load.RedundancyGroup,
		PreferredZoneID: load.PreferredZoneID, PreferredZoneCode: zoneCode, LoadStatus: load.LoadStatus,
		HeatRatio: load.HeatRatio(), ValidationState: state,
	}
}
