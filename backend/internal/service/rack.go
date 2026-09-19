package service

import (
	"context"
	"fmt"

	"datacenter-thermal-capacity-planner/backend/internal/audit"
	"datacenter-thermal-capacity-planner/backend/internal/dto"
	"datacenter-thermal-capacity-planner/backend/internal/model"
	"datacenter-thermal-capacity-planner/backend/internal/repository"
	"datacenter-thermal-capacity-planner/backend/internal/web"
)

type RackService struct {
	racks *repository.RackRepository
	zones *repository.ThermalZoneRepository
}

func NewRackService(racks *repository.RackRepository, zones *repository.ThermalZoneRepository) *RackService {
	return &RackService{racks: racks, zones: zones}
}

func (s *RackService) List(ctx context.Context, search, status string, zoneID uint, page, size int) ([]dto.RackResponse, int64, error) {
	racks, total, err := s.racks.List(ctx, search, status, zoneID, page, size)
	if err != nil {
		return nil, 0, err
	}
	responses := make([]dto.RackResponse, 0, len(racks))
	for _, rack := range racks {
		responses = append(responses, decodeRack(rack))
	}
	return responses, total, nil
}

func (s *RackService) Get(ctx context.Context, id uint) (dto.RackResponse, error) {
	rack, err := s.racks.Get(ctx, id)
	if err != nil {
		return dto.RackResponse{}, err
	}
	return decodeRack(rack), nil
}

func (s *RackService) Create(ctx context.Context, req dto.CreateRackRequest, actor audit.Entry) (dto.RackResponse, error) {
	rack, err := dto.NewRack(req)
	if err != nil {
		return dto.RackResponse{}, web.Unprocessable("INVALID_RACK", err.Error(), err)
	}
	actor.Action = "rack.create"
	actor.EntityType = "rack"
	actor.AfterSummary = fmt.Sprintf("%s zone=%d position=%d,%d power=%.2f airflow=%.2f", rack.RackCode, rack.ZoneID, rack.RowIndex, rack.ColumnIndex, rack.PowerLimitKW, rack.AirflowLimitCFM)
	if err := s.racks.Create(ctx, &rack, actor); err != nil {
		return dto.RackResponse{}, err
	}
	return s.Get(ctx, rack.ID)
}

func (s *RackService) Update(ctx context.Context, id uint, req dto.UpdateRackRequest, actor audit.Entry) (dto.RackResponse, error) {
	if err := req.ValidateBusiness(); err != nil {
		return dto.RackResponse{}, web.Unprocessable("INVALID_RACK", err.Error(), err)
	}
	current, err := s.racks.Get(ctx, id)
	if err != nil {
		return dto.RackResponse{}, err
	}
	if _, err := s.zones.Get(ctx, req.ZoneID); err != nil {
		return dto.RackResponse{}, web.Unprocessable("ZONE_NOT_FOUND", "selected thermal zone does not exist", err)
	}
	updated := model.Rack{
		ID: id, ZoneID: req.ZoneID, RackCode: current.RackCode, RowIndex: req.RowIndex,
		ColumnIndex: req.ColumnIndex, PowerLimitKW: req.PowerLimitKW, AirflowLimitCFM: req.AirflowLimitCFM,
		RackUnits: req.RackUnits, RackStatus: req.RackStatus, Version: req.Version,
	}
	actor.Action = "rack.update"
	actor.EntityType = "rack"
	actor.BeforeSummary = fmt.Sprintf("zone=%d position=%d,%d version=%d", current.ZoneID, current.RowIndex, current.ColumnIndex, current.Version)
	actor.AfterSummary = fmt.Sprintf("zone=%d position=%d,%d version=%d", req.ZoneID, req.RowIndex, req.ColumnIndex, req.Version+1)
	if err := s.racks.Update(ctx, &updated, req.Version, actor); err != nil {
		return dto.RackResponse{}, err
	}
	return s.Get(ctx, id)
}

func decodeRack(rack model.Rack) dto.RackResponse {
	return dto.RackResponse{
		ID: rack.ID, ZoneID: rack.ZoneID, ZoneCode: rack.ThermalZone.ZoneCode, RackCode: rack.RackCode,
		RowIndex: rack.RowIndex, ColumnIndex: rack.ColumnIndex, PowerLimitKW: rack.PowerLimitKW,
		AirflowLimitCFM: rack.AirflowLimitCFM, RackUnits: rack.RackUnits, RackStatus: rack.RackStatus,
		Version: rack.Version, Utilization: dto.RackUtilization{},
	}
}
