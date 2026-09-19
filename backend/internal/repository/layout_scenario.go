package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"datacenter-thermal-capacity-planner/backend/internal/audit"
	"datacenter-thermal-capacity-planner/backend/internal/constants"
	"datacenter-thermal-capacity-planner/backend/internal/model"
	"datacenter-thermal-capacity-planner/backend/internal/web"
	"gorm.io/gorm"
)

type LayoutScenarioRepository struct {
	db    *gorm.DB
	audit *audit.Repository
}

func NewLayoutScenarioRepository(db *gorm.DB, auditRepo *audit.Repository) *LayoutScenarioRepository {
	return &LayoutScenarioRepository{db: db, audit: auditRepo}
}

func (r *LayoutScenarioRepository) List(ctx context.Context, search, status string, page, size int) ([]model.LayoutScenario, int64, error) {
	query := r.db.WithContext(ctx).Model(&model.LayoutScenario{})
	if search != "" {
		query = query.Where("LOWER(name) LIKE ?", "%"+strings.ToLower(search)+"%")
	}
	if status != "" {
		query = query.Where("scenario_status = ?", status)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count layout scenarios: %w", err)
	}
	var scenarios []model.LayoutScenario
	err := query.Order("updated_at DESC, id DESC").Offset((page - 1) * size).Limit(size).Find(&scenarios).Error
	if err != nil {
		return nil, 0, fmt.Errorf("list layout scenarios: %w", err)
	}
	return scenarios, total, nil
}

func (r *LayoutScenarioRepository) Get(ctx context.Context, id uint) (model.LayoutScenario, error) {
	var scenario model.LayoutScenario
	if err := r.db.WithContext(ctx).First(&scenario, id).Error; err != nil {
		return model.LayoutScenario{}, fmt.Errorf("get layout scenario %d: %w", id, err)
	}
	return scenario, nil
}

func (r *LayoutScenarioRepository) Create(ctx context.Context, scenario *model.LayoutScenario, entry audit.Entry) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(scenario).Error; err != nil {
			if errors.Is(err, gorm.ErrDuplicatedKey) {
				return web.Conflict("SCENARIO_NAME_EXISTS", "scenario name already exists", err)
			}
			return fmt.Errorf("create layout scenario: %w", err)
		}
		entry.EntityID = scenario.ID
		return r.audit.RecordWithDB(ctx, tx, entry)
	})
}

func (r *LayoutScenarioRepository) BeginEvaluation(ctx context.Context, id, expectedVersion uint, entry audit.Entry) (model.LayoutScenario, error) {
	var scenario model.LayoutScenario
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&scenario, id).Error; err != nil {
			return web.NotFound("layout scenario")
		}
		if scenario.Version != expectedVersion || scenario.ScenarioStatus != constants.ScenarioDraft {
			return web.Conflict("SCENARIO_VERSION_CONFLICT", "scenario must be an unchanged draft before evaluation", nil)
		}
		result := tx.Model(&model.LayoutScenario{}).Where("id = ? AND version = ? AND scenario_status = ?", id, expectedVersion, constants.ScenarioDraft).
			Updates(map[string]any{"scenario_status": constants.ScenarioEvaluating, "version": gorm.Expr("version + 1")})
		if result.Error != nil {
			return fmt.Errorf("begin scenario evaluation: %w", result.Error)
		}
		if result.RowsAffected != 1 {
			return web.Conflict("SCENARIO_VERSION_CONFLICT", "scenario changed while evaluation was starting", nil)
		}
		entry.EntityID = id
		entry.BeforeSummary = string(constants.ScenarioDraft)
		entry.AfterSummary = string(constants.ScenarioEvaluating)
		return r.audit.RecordWithDB(ctx, tx, entry)
	})
	if err != nil {
		return model.LayoutScenario{}, err
	}
	scenario.ScenarioStatus = constants.ScenarioEvaluating
	scenario.Version++
	return scenario, nil
}

type EvaluationUpdate struct {
	AssignmentsJSON string
	SnapshotJSON    string
	ZoneResultsJSON string
	ViolationsJSON  string
	TotalPowerKW    float64
	PeakTempC       float64
	Score           float64
}

func (r *LayoutScenarioRepository) FinishEvaluation(ctx context.Context, scenario model.LayoutScenario, update EvaluationUpdate, entry audit.Entry) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&model.LayoutScenario{}).
			Where("id = ? AND version = ? AND scenario_status = ?", scenario.ID, scenario.Version, constants.ScenarioEvaluating).
			Updates(map[string]any{
				"rack_assignments_json":      update.AssignmentsJSON,
				"input_snapshot_json":        update.SnapshotJSON,
				"zone_results_json":          update.ZoneResultsJSON,
				"constraint_violations_json": update.ViolationsJSON,
				"total_power_kw":             update.TotalPowerKW,
				"peak_temp_c":                update.PeakTempC,
				"score":                      update.Score,
				"scenario_status":            constants.ScenarioPendingReview,
				"version":                    gorm.Expr("version + 1"),
			})
		if result.Error != nil {
			return fmt.Errorf("finish scenario evaluation: %w", result.Error)
		}
		if result.RowsAffected != 1 {
			return web.Conflict("SCENARIO_VERSION_CONFLICT", "scenario changed during evaluation", nil)
		}
		entry.EntityID = scenario.ID
		entry.BeforeSummary = string(constants.ScenarioEvaluating)
		entry.AfterSummary = fmt.Sprintf("%s score=%.1f", constants.ScenarioPendingReview, update.Score)
		return r.audit.RecordWithDB(ctx, tx, entry)
	})
}

func (r *LayoutScenarioRepository) Transition(ctx context.Context, scenario model.LayoutScenario, target constants.ScenarioStatus, actorID uint, entry audit.Entry) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		updates := map[string]any{"scenario_status": target, "version": gorm.Expr("version + 1")}
		if target == constants.ScenarioApproved {
			updates["approved_by"] = actorID
		}
		result := tx.Model(&model.LayoutScenario{}).
			Where("id = ? AND version = ? AND scenario_status = ?", scenario.ID, scenario.Version, scenario.ScenarioStatus).
			Updates(updates)
		if result.Error != nil {
			return fmt.Errorf("transition layout scenario: %w", result.Error)
		}
		if result.RowsAffected != 1 {
			return web.Conflict("SCENARIO_VERSION_CONFLICT", "scenario changed before transition", nil)
		}
		entry.EntityID = scenario.ID
		entry.BeforeSummary = string(scenario.ScenarioStatus)
		entry.AfterSummary = string(target)
		return r.audit.RecordWithDB(ctx, tx, entry)
	})
}
