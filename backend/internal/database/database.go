package database

import (
	"encoding/json"
	"fmt"
	"log/slog"

	"datacenter-thermal-capacity-planner/backend/internal/audit"
	"datacenter-thermal-capacity-planner/backend/internal/auth"
	"datacenter-thermal-capacity-planner/backend/internal/config"
	"datacenter-thermal-capacity-planner/backend/internal/constants"
	"datacenter-thermal-capacity-planner/backend/internal/model"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func Open(cfg config.Config) (*gorm.DB, error) {
	var dialector gorm.Dialector
	switch cfg.DBDriver {
	case "sqlite":
		dialector = sqlite.Open(cfg.DBDSN)
	case "postgres":
		dialector = postgres.Open(cfg.DBDSN)
	default:
		return nil, fmt.Errorf("open database: unsupported driver %q", cfg.DBDriver)
	}
	db, err := gorm.Open(dialector, &gorm.Config{
		Logger:         logger.Default.LogMode(logger.Warn),
		TranslateError: true,
	})
	if err != nil {
		return nil, fmt.Errorf("open %s database: %w", cfg.DBDriver, err)
	}
	if cfg.DBAutoMigrate {
		if err := migrate(db); err != nil {
			return nil, err
		}
		if err := seed(db); err != nil {
			return nil, err
		}
	}
	return db, nil
}

func migrate(db *gorm.DB) error {
	err := db.AutoMigrate(
		&auth.User{},
		&model.ThermalZone{},
		&model.Rack{},
		&model.EquipmentLoad{},
		&model.LayoutScenario{},
		&audit.Event{},
	)
	if err != nil {
		return fmt.Errorf("auto migrate database: %w", err)
	}
	return nil
}

func seed(db *gorm.DB) error {
	if err := auth.SeedUsers(db); err != nil {
		return err
	}
	var zoneCount int64
	if err := db.Model(&model.ThermalZone{}).Count(&zoneCount).Error; err != nil {
		return fmt.Errorf("count thermal zones: %w", err)
	}
	if zoneCount > 0 {
		return nil
	}
	adjA, _ := json.Marshal(map[string]float64{"TZ-B": 0.22, "TZ-C": 0.08})
	adjB, _ := json.Marshal(map[string]float64{"TZ-A": 0.22, "TZ-C": 0.16})
	adjC, _ := json.Marshal(map[string]float64{"TZ-A": 0.08, "TZ-B": 0.16})
	zones := []model.ThermalZone{
		{ZoneCode: "TZ-A", Name: "North cold aisle", CoolingCapacityKW: 96, SupplyTempC: 18.5, MaxReturnTempC: 31, AdjacencyJSON: string(adjA), ZoneStatus: "active"},
		{ZoneCode: "TZ-B", Name: "Core compute aisle", CoolingCapacityKW: 132, SupplyTempC: 19, MaxReturnTempC: 32, AdjacencyJSON: string(adjB), ZoneStatus: "active"},
		{ZoneCode: "TZ-C", Name: "Network edge aisle", CoolingCapacityKW: 74, SupplyTempC: 18, MaxReturnTempC: 30, AdjacencyJSON: string(adjC), ZoneStatus: "constrained"},
	}
	if err := db.Create(&zones).Error; err != nil {
		return fmt.Errorf("seed thermal zones: %w", err)
	}
	racks := []model.Rack{
		{ZoneID: zones[0].ID, RackCode: "A-01", RowIndex: 1, ColumnIndex: 1, PowerLimitKW: 24, AirflowLimitCFM: 6800, RackUnits: 42, RackStatus: constants.RackAvailable, Version: 1},
		{ZoneID: zones[0].ID, RackCode: "A-02", RowIndex: 1, ColumnIndex: 2, PowerLimitKW: 24, AirflowLimitCFM: 6800, RackUnits: 42, RackStatus: constants.RackAvailable, Version: 1},
		{ZoneID: zones[1].ID, RackCode: "B-01", RowIndex: 2, ColumnIndex: 1, PowerLimitKW: 32, AirflowLimitCFM: 9200, RackUnits: 48, RackStatus: constants.RackAvailable, Version: 1},
		{ZoneID: zones[1].ID, RackCode: "B-02", RowIndex: 2, ColumnIndex: 2, PowerLimitKW: 32, AirflowLimitCFM: 9200, RackUnits: 48, RackStatus: constants.RackReserved, Version: 1},
		{ZoneID: zones[2].ID, RackCode: "C-01", RowIndex: 3, ColumnIndex: 1, PowerLimitKW: 18, AirflowLimitCFM: 5400, RackUnits: 42, RackStatus: constants.RackMaintenance, Version: 1},
	}
	if err := db.Create(&racks).Error; err != nil {
		return fmt.Errorf("seed racks: %w", err)
	}
	loads := []model.EquipmentLoad{
		{Name: "AI training node 01", PowerKW: 18.4, HeatKW: 17.8, AirflowCFM: 5200, RackUnits: 8, RedundancyGroup: "AI-A", PreferredZoneID: &zones[1].ID, LoadStatus: "ready"},
		{Name: "AI training node 02", PowerKW: 18.1, HeatKW: 17.5, AirflowCFM: 5100, RackUnits: 8, RedundancyGroup: "AI-A", PreferredZoneID: &zones[0].ID, LoadStatus: "ready"},
		{Name: "Storage fabric", PowerKW: 9.6, HeatKW: 8.9, AirflowCFM: 2800, RackUnits: 12, RedundancyGroup: "STORAGE-B", PreferredZoneID: &zones[2].ID, LoadStatus: "ready"},
		{Name: "Telemetry cluster", PowerKW: 6.2, HeatKW: 5.8, AirflowCFM: 1900, RackUnits: 6, RedundancyGroup: "OPS-C", PreferredZoneID: &zones[0].ID, LoadStatus: "ready"},
	}
	if err := db.Create(&loads).Error; err != nil {
		return fmt.Errorf("seed equipment loads: %w", err)
	}
	slog.Info("database seed completed", "zones", len(zones), "racks", len(racks), "loads", len(loads))
	return nil
}

func Ping(db *gorm.DB) error {
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("get sql database: %w", err)
	}
	if err := sqlDB.Ping(); err != nil {
		return fmt.Errorf("ping database: %w", err)
	}
	return nil
}
