package cameras

import (
	"path/filepath"
	"testing"

	"github.com/liuhengloveyou/FrameFlow/internal/api"
	"github.com/liuhengloveyou/FrameFlow/pkg/database"
	"github.com/stretchr/testify/require"
)

func TestStoreSQLite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ff.db")
	db, err := database.Open(database.Config{Driver: "sqlite3", DSN: path})
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	require.NoError(t, db.AutoMigrate(&CameraRow{}))
	st, err := NewStore(db)
	require.NoError(t, err)

	cam := api.Camera{
		Name: "大厅", IP: "10.0.0.1", Port: 554,
		Protocol: api.CameraRTSP, Status: "offline",
		StreamURL: "rtsp://10.0.0.1:554/stream1",
	}
	require.NoError(t, st.Insert(&cam))
	require.NotZero(t, cam.ID)

	got, ok, err := st.Get(cam.ID)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "大厅", got.Name)

	page, err := st.List(api.CameraQuery{Page: 1, PageSize: 10, Name: "大"})
	require.NoError(t, err)
	require.Equal(t, 1, page.Total)

	gb := api.Camera{
		Name: "国标入口", Protocol: api.CameraGB28181,
		Status: "offline", DeviceID: "34020000001310000002",
		Enrollment: "active", Channel: "34020000001310000002",
	}
	require.NoError(t, st.Insert(&gb))
	gotGB, ok, err := st.Get(gb.ID)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "active", gotGB.Enrollment)
	require.Equal(t, api.CameraGB28181, gotGB.Protocol)

	require.NoError(t, st.Delete(cam.ID))
	require.NoError(t, st.Delete(gb.ID))
}
