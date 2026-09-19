package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"datacenter-thermal-capacity-planner/backend/internal/audit"
	"datacenter-thermal-capacity-planner/backend/internal/model"
	"datacenter-thermal-capacity-planner/backend/internal/web"
	"gorm.io/gorm"
)

type ThermalZoneRepository struct {
	db    *gorm.DB
	audit *audit.Repository
}

func NewThermalZoneRepository(db *gorm.DB, auditRepo *audit.Repository) *ThermalZoneRepository {
	return &ThermalZoneRepository{db: db, audit: auditRepo}
}

func (r *ThermalZoneRepository) List(ctx context.Context, search, status string, page, size int) ([]model.ThermalZone, int64, error) {
	query := r.db.WithContext(ctx).Model(&model.ThermalZone{})
	if search != "" {
		like := "%" + strings.ToLower(search) + "%"
		query = query.Where("LOWER(zone_code) LIKE ? OR LOWER(name) LIKE ?", like, like)
	}
	if status != "" {
		query = query.Where("zone_status = ?", status)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count thermal zones: %w", err)
	}
	var zones []model.ThermalZone
	if err := query.Order("zone_code ASC").Offset((page - 1) * size).Limit(size).Find(&zones).Error; err != nil {
		return nil, 0, fmt.Errorf("list thermal zones: %w", err)
	}
	return zones, total, nil
}

func (r *ThermalZoneRepository) Get(ctx context.Context, id uint) (model.ThermalZone, error) {
	var zone model.ThermalZone
	if err := r.db.WithContext(ctx).First(&zone, id).Error; err != nil {
		return model.ThermalZone{}, fmt.Errorf("get thermal zone %d: %w", id, err)
	}
	return zone, nil
}

func (r *ThermalZoneRepository) GetByCode(ctx context.Context, code string) (model.ThermalZone, error) {
	var zone model.ThermalZone
	if err := r.db.WithContext(ctx).Where("zone_code = ?", strings.ToUpper(code)).First(&zone).Error; err != nil {
		return model.ThermalZone{}, fmt.Errorf("get thermal zone by code: %w", err)
	}
	return zone, nil
}

func (r *ThermalZoneRepository) All(ctx context.Context) ([]model.ThermalZone, error) {
	var zones []model.ThermalZone
	if err := r.db.WithContext(ctx).Order("zone_code ASC").Find(&zones).Error; err != nil {
		return nil, fmt.Errorf("list all thermal zones: %w", err)
	}
	return zones, nil
}

func (r *ThermalZoneRepository) RackCount(ctx context.Context, zoneID uint) (int64, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&model.Rack{}).Where("zone_id = ?", zoneID).Count(&count).Error; err != nil {
		return 0, fmt.Errorf("count zone racks: %w", err)
	}
	return count, nil
}

func (r *ThermalZoneRepository) Create(ctx context.Context, zone *model.ThermalZone, entry audit.Entry) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(zone).Error; err != nil {
			if errors.Is(err, gorm.ErrDuplicatedKey) {
				return web.Conflict("ZONE_CODE_EXISTS", "zone code already exists", err)
			}
			return fmt.Errorf("create thermal zone: %w", err)
		}
		entry.EntityID = zone.ID
		if err := r.audit.RecordWithDB(ctx, tx, entry); err != nil {
			return err
		}
		return nil
	})
}

func (r *ThermalZoneRepository) Update(ctx context.Context, zone *model.ThermalZone, entry audit.Entry) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&model.ThermalZone{}).Where("id = ?", zone.ID).Updates(map[string]any{
			"name": zone.Name, "cooling_capacity_kw": zone.CoolingCapacityKW,
			"supply_temp_c": zone.SupplyTempC, "max_return_temp_c": zone.MaxReturnTempC,
			"adjacency_json": zone.AdjacencyJSON, "zone_status": zone.ZoneStatus,
		})
		if result.Error != nil {
			return fmt.Errorf("update thermal zone: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			return web.NotFound("thermal zone")
		}
		if err := r.audit.RecordWithDB(ctx, tx, entry); err != nil {
			return err
		}
		return nil
	})
}
