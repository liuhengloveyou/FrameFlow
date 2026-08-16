package database

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestNormalizeDefaults(t *testing.T) {
	d, dsn, dialect, err := Normalize(Config{})
	require.NoError(t, err)
	require.Equal(t, DriverSQLite3, d)
	require.Equal(t, "data/frameflow.db", dsn)
	require.Equal(t, DialectSQLite, dialect)
}

func TestNormalizePostgresDSN(t *testing.T) {
	d, dsn, dialect, err := Normalize(Config{DSN: "postgres://u:p@localhost/db"})
	require.NoError(t, err)
	require.Equal(t, DriverPostgres, d)
	require.Equal(t, "postgres://u:p@localhost/db", dsn)
	require.Equal(t, DialectPostgres, dialect)
}

func TestOpenSQLiteGorm(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ff.db")
	db, err := Open(Config{Driver: "sqlite3", DSN: path})
	require.NoError(t, err)
	t.Cleanup(func() {
		sqlDB, _ := db.DB.DB()
		if sqlDB != nil {
			_ = sqlDB.Close()
		}
	})

	type row struct {
		ID string `gorm:"primaryKey"`
	}
	require.NoError(t, db.AutoMigrate(&row{}))
	require.NoError(t, db.Create(&row{ID: "a"}).Error)

	var got row
	require.NoError(t, db.First(&got, "id = ?", "a").Error)
	require.Equal(t, "a", got.ID)
	require.ErrorIs(t, db.First(&got, "id = ?", "missing").Error, gorm.ErrRecordNotFound)
}
