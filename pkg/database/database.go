// Package database 提供 FrameFlow 系统级 GORM 数据库抽象，同时支持 sqlite3 与 postgres。
package database

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Driver 名称常量。
const (
	DriverSQLite3  = "sqlite3"
	DriverPostgres = "postgres"
)

// Dialect 数据库方言。
type Dialect int

const (
	DialectSQLite Dialect = iota
	DialectPostgres
)

// Config 系统数据库配置（YAML 根键 database:）。
type Config struct {
	// Driver: sqlite3 | postgres（也接受 sqlite / pg / postgresql）
	Driver string `yaml:"driver"`
	// DSN: sqlite 为文件路径；postgres 为连接串
	DSN string `yaml:"dsn"`
	// Path 兼容别名：等同于 sqlite 的 dsn
	Path string `yaml:"path"`
}

// DB 包装 *gorm.DB。
type DB struct {
	*gorm.DB
	Dialect Dialect
	Driver  string
	DSN     string
}

// Open 按配置打开 GORM 连接（不跑业务 AutoMigrate）。
func Open(cfg Config) (*DB, error) {
	driver, dsn, dialect, err := Normalize(cfg)
	if err != nil {
		return nil, err
	}

	gormCfg := &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
		NowFunc: func() time.Time {
			return time.Now().UTC()
		},
	}

	var gdb *gorm.DB
	switch dialect {
	case DialectSQLite:
		if err := ensureSQLiteDir(dsn); err != nil {
			return nil, err
		}
		openDSN := appendSQLiteParams(dsn)
		gdb, err = gorm.Open(sqlite.Open(openDSN), gormCfg)
	case DialectPostgres:
		gdb, err = gorm.Open(postgres.New(postgres.Config{
			DriverName: "pgx",
			DSN:        dsn,
		}), gormCfg)
	default:
		return nil, fmt.Errorf("database: unsupported dialect")
	}
	if err != nil {
		return nil, err
	}

	sqlDB, err := gdb.DB()
	if err != nil {
		return nil, err
	}
	if dialect == DialectSQLite {
		sqlDB.SetMaxOpenConns(1)
		sqlDB.SetMaxIdleConns(1)
		if _, err := sqlDB.Exec(`PRAGMA busy_timeout=5000; PRAGMA foreign_keys=ON;`); err != nil {
			_ = sqlDB.Close()
			return nil, err
		}
	} else {
		sqlDB.SetMaxOpenConns(20)
		sqlDB.SetMaxIdleConns(5)
	}
	if err := sqlDB.Ping(); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("database ping: %w", err)
	}

	return &DB{DB: gdb, Dialect: dialect, Driver: driver, DSN: dsn}, nil
}

// Normalize 规范化 driver/dsn。
func Normalize(cfg Config) (driver, dsn string, dialect Dialect, err error) {
	driver = strings.ToLower(strings.TrimSpace(cfg.Driver))
	dsn = strings.TrimSpace(cfg.DSN)
	if dsn == "" {
		dsn = strings.TrimSpace(cfg.Path)
	}
	if driver == "" {
		if dsn == "" {
			driver = DriverSQLite3
			dsn = "data/frameflow.db"
		} else if looksLikePostgresDSN(dsn) {
			driver = DriverPostgres
		} else {
			driver = DriverSQLite3
		}
	}
	switch driver {
	case "sqlite", DriverSQLite3:
		driver = DriverSQLite3
		dialect = DialectSQLite
		if dsn == "" {
			dsn = "data/frameflow.db"
		}
	case "pg", "postgresql", DriverPostgres:
		driver = DriverPostgres
		dialect = DialectPostgres
		if dsn == "" {
			return "", "", 0, fmt.Errorf("database: dsn required for postgres")
		}
	default:
		return "", "", 0, fmt.Errorf("database: unsupported driver %q (want sqlite3 or postgres)", driver)
	}
	return driver, dsn, dialect, nil
}

// IsUniqueErr 判断是否唯一约束冲突。
func IsUniqueErr(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "unique") ||
		strings.Contains(msg, "duplicate key") ||
		strings.Contains(msg, "23505")
}

// IsSQLite 是否 sqlite 方言。
func (db *DB) IsSQLite() bool { return db != nil && db.Dialect == DialectSQLite }

// IsPostgres 是否 postgres 方言。
func (db *DB) IsPostgres() bool { return db != nil && db.Dialect == DialectPostgres }

// Close 关闭底层连接池。
func (db *DB) Close() error {
	if db == nil || db.DB == nil {
		return nil
	}
	sqlDB, err := db.DB.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

func looksLikePostgresDSN(dsn string) bool {
	lower := strings.ToLower(dsn)
	return strings.HasPrefix(lower, "postgres://") ||
		strings.HasPrefix(lower, "postgresql://") ||
		strings.Contains(lower, "host=")
}

func ensureSQLiteDir(path string) error {
	dir := filepath.Dir(path)
	if dir == "." || dir == "" {
		return nil
	}
	return os.MkdirAll(dir, 0o755)
}

func appendSQLiteParams(dsn string) string {
	// gorm sqlite 驱动走 mattn/go-sqlite3；补上 busy_timeout / WAL
	sep := "?"
	if strings.Contains(dsn, "?") {
		sep = "&"
	}
	return dsn + sep + "_busy_timeout=5000&_journal_mode=WAL&_fk=1"
}
