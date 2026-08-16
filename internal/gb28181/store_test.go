package gb28181

import (
	"path/filepath"
	"testing"

	"github.com/liuhengloveyou/FrameFlow/pkg/database"
	"github.com/liuhengloveyou/FrameFlow/pkg/gb28181"
)

func TestStoreAddListChannels(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ff.db")
	db, err := database.Open(database.Config{Driver: "sqlite3", DSN: path})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	st, err := openStore(db)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&cameraDeviceRow{}, &ChannelRow{}); err != nil {
		t.Fatal(err)
	}

	d, err := st.AddDevice("34020000001320000001", "入口摄像头", "pass1")
	if err != nil {
		t.Fatal(err)
	}
	if d.ID != "34020000001320000001" {
		t.Fatal(d)
	}
	if _, err := st.AddDevice("34020000001320000001", "dup", ""); err == nil {
		t.Fatal("expected duplicate error")
	}

	pass, ok, err := st.PasswordFor("34020000001320000001")
	if err != nil || !ok || pass != "pass1" {
		t.Fatalf("pass=%q ok=%v err=%v", pass, ok, err)
	}

	chs := []*gb28181.Channel{
		{ID: "34020000001320000001", Name: "ch1", Status: "ON"},
	}
	if err := st.SaveChannels(d.ID, chs); err != nil {
		t.Fatal(err)
	}
	list, err := st.ListDevices()
	if err != nil || len(list) != 1 || len(list[0].Channels) != 1 {
		t.Fatalf("%+v err=%v", list, err)
	}

	d.Online = true
	d.RemoteAddr = "1.2.3.4:5060"
	if err := st.MarkOnline(d); err != nil {
		t.Fatal(err)
	}
	got, ok, err := st.GetDevice(d.ID)
	if err != nil || !ok || !got.Online || got.RemoteAddr != "1.2.3.4:5060" {
		t.Fatalf("%+v", got)
	}

	if err := st.MarkOffline(d.ID); err != nil {
		t.Fatal(err)
	}
	got, ok, err = st.GetDevice(d.ID)
	if err != nil || !ok || got.Online {
		t.Fatalf("expected offline: %+v", got)
	}
	if err := st.TouchKeepalive(d.ID); err != nil {
		t.Fatal(err)
	}
	got, ok, err = st.GetDevice(d.ID)
	if err != nil || !ok || !got.Online || got.LastKeepalive.IsZero() {
		t.Fatalf("expected keepalive online: %+v", got)
	}

	if err := st.DeleteDevice(d.ID); err != nil {
		t.Fatal(err)
	}
	list, _ = st.ListDevices()
	if len(list) != 0 {
		t.Fatal(list)
	}
}

func TestStoreDiscoverPendingConfirmReject(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ff.db")
	db, err := database.Open(database.Config{Driver: "sqlite3", DSN: path})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	st, err := openStore(db)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&cameraDeviceRow{}, &ChannelRow{}); err != nil {
		t.Fatal(err)
	}

	auth, err := st.DiscoverDevice("34020000001310000002", false, "192.168.1.24:5060")
	if err != nil || auth.Status != gb28181.DeviceStatusPending {
		t.Fatalf("%+v err=%v", auth, err)
	}
	gotDev, ok, err := st.GetDevice("34020000001310000002")
	if err != nil || !ok || gotDev.Name != "192.168.1.24:5060" {
		t.Fatalf("default name: %+v err=%v", gotDev, err)
	}
	auth2, err := st.DiscoverDevice("34020000001310000002", false, "ignored")
	if err != nil || auth2.Status != gb28181.DeviceStatusPending {
		t.Fatalf("%+v err=%v", auth2, err)
	}
	if err := st.SetStatus("34020000001310000002", gb28181.DeviceStatusActive); err != nil {
		t.Fatal(err)
	}
	got, ok, err := st.LookupAuth("34020000001310000002")
	if err != nil || !ok || got.Status != gb28181.DeviceStatusActive {
		t.Fatalf("%+v", got)
	}
	if err := st.SetStatus("34020000001310000002", gb28181.DeviceStatusRejected); err != nil {
		t.Fatal(err)
	}
	got, _, _ = st.LookupAuth("34020000001310000002")
	if gb28181.AllowsRegister(got.Status) {
		t.Fatal("rejected should not allow register")
	}
}
