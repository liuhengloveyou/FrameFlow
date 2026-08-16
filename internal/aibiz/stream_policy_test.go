package aibiz

import "testing"

func TestParseStreamCameraChannel(t *testing.T) {
	cam, ch, ok := parseStreamCameraChannel(
		"gb28181://34020000001310000002/34020000001310000111?stream=sub",
	)
	if !ok || cam != "34020000001310000002" || ch != "34020000001310000111" {
		t.Fatalf("got %s %s ok=%v", cam, ch, ok)
	}
	_, _, ok = parseStreamCameraChannel("rtsp://x/y")
	if ok {
		t.Fatal("rtsp should not resolve")
	}
	_, _, ok = parseStreamCameraChannel("ingest_dd09b51bc871fe06")
	if ok {
		t.Fatal("bare ingest key needs DB lookup, not parseStreamCameraChannel")
	}
}
