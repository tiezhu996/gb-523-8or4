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

type EquipmentLoadRepository struct {
	db    *gorm.DB
	audit *audit.Repository
}

func NewEquipmentLoadRepository(db *gorm.DB, auditRepo *audit.Repository) *EquipmentLoadRepository {
	return &EquipmentLoadRepository{db: db, audit: auditRepo}
}

func (r *EquipmentLoadRepository) List(ctx context.Context, search, status string, page, size int) ([]model.EquipmentLoad, int64, error) {
	query := r.db.WithContext(ctx).Model(&model.EquipmentLoad{})
	if search != "" {
		like := "%" + strings.ToLower(search) + "%"
		query = query.Where("LOWER(name) LIKE ? OR LOWER(redundancy_group) LIKE ?", like, like)
	}
	if status != "" {
		query = query.Where("load_status = ?", status)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count equipment loads: %w", err)
	}
	var loads []model.EquipmentLoad
	err := query.Preload("PreferredZone").Order("load_status ASC, power_kw DESC, id ASC").
		Offset((page - 1) * size).Limit(size).Find(&loads).Error
	if err != nil {
		return nil, 0, fmt.Errorf("list equipment loads: %w", err)
	}
	return loads, total, nil
}

func (r *EquipmentLoadRepository) AllReady(ctx context.Context) ([]model.EquipmentLoad, error) {
	var loads []model.EquipmentLoad
	if err := r.db.WithContext(ctx).Where("load_status = ?", "ready").Order("id ASC").Find(&loads).Error; err != nil {
		return nil, fmt.Errorf("list ready equipment loads: %w", err)
	}
	return loads, nil
}

func (r *EquipmentLoadRepository) FindByIDs(ctx context.Context, ids []uint) ([]model.EquipmentLoad, error) {
	var loads []model.EquipmentLoad
	if err := r.db.WithContext(ctx).Where("id IN ?", ids).Order("id ASC").Find(&loads).Error; err != nil {
		return nil, fmt.Errorf("find equipment loads: %w", err)
	}
	if len(loads) != len(ids) {
		return nil, web.Unprocessable("LOAD_NOT_FOUND", "one or more selected equipment loads do not exist", nil)
	}
	return loads, nil
}

func (r *EquipmentLoadRepository) Get(ctx context.Context, id uint) (model.EquipmentLoad, error) {
	var load model.EquipmentLoad
	if err := r.db.WithContext(ctx).Preload("PreferredZone").First(&load, id).Error; err != nil {
		return model.EquipmentLoad{}, fmt.Errorf("get equipment load %d: %w", id, err)
	}
	return load, nil
}

func (r *EquipmentLoadRepository) Create(ctx context.Context, load *model.EquipmentLoad, entry audit.Entry) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if load.PreferredZoneID != nil {
			var count int64
			if err := tx.Model(&model.ThermalZone{}).Where("id = ?", *load.PreferredZoneID).Count(&count).Error; err != nil {
				return fmt.Errorf("validate preferred zone: %w", err)
			}
			if count == 0 {
				return web.Unprocessable("PREFERRED_ZONE_NOT_FOUND", "preferred thermal zone does not exist", nil)
			}
		}
		if err := tx.Create(load).Error; err != nil {
			if errors.Is(err, gorm.ErrDuplicatedKey) {
				return web.Conflict("LOAD_NAME_EXISTS", "equipment load name already exists", err)
			}
			return fmt.Errorf("create equipment load: %w", err)
		}
		entry.EntityID = load.ID
		return r.audit.RecordWithDB(ctx, tx, entry)
	})
}

func (r *EquipmentLoadRepository) Update(ctx context.Context, load *model.EquipmentLoad, entry audit.Entry) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&model.EquipmentLoad{}).Where("id = ?", load.ID).Updates(map[string]any{
			"name": load.Name, "power_kw": load.PowerKW, "heat_kw": load.HeatKW,
			"airflow_cfm": load.AirflowCFM, "rack_units": load.RackUnits,
			"redundancy_group": load.RedundancyGroup, "preferred_zone_id": load.PreferredZoneID,
			"load_status": load.LoadStatus,
		})
		if result.Error != nil {
			return fmt.Errorf("update equipment load: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			return web.NotFound("equipment load")
		}
		entry.EntityID = load.ID
		return r.audit.RecordWithDB(ctx, tx, entry)
	})
}
