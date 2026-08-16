package app

import (
	"fmt"

	"github.com/liuhengloveyou/FrameFlow/pkg/database"
)

// DBCfg 对应 YAML 根键 database:（由 Init 加载）。
var DBCfg database.Config
var systemDB *database.DB

// InitDatabase 打开系统级数据库（sqlite3 / postgres）并迁移 sys_config。须在依赖持久化的模块 Init 之前调用。
func InitDatabase() error {
	var conf struct {
		Database database.Config `yaml:"database"`
	}
	conf.Database = database.Config{
		Driver: database.DriverSQLite3,
		DSN:    "data/frameflow.db",
	}
	LoadConfig(&conf)
	DBCfg = conf.Database

	db, err := database.Open(DBCfg)
	if err != nil {
		return err
	}
	systemDB = db
	if err := systemDB.AutoMigrate(&SysConfigRow{}); err != nil {
		return fmt.Errorf("automigrate sys_config: %w", err)
	}
	Logger.Info().
		Str("driver", db.Driver).
		Str("dsn", db.DSN).
		Msg("database ready")
	return nil
}

// DB 返回系统级数据库；未初始化时为 nil。
func DB() *database.DB {
	return systemDB
}
