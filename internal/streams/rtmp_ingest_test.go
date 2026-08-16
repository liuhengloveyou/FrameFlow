package streams

import (
	"testing"
)

func TestHashPushSecret(t *testing.T) {
	h1 := HashPushSecret("secret")
	h2 := HashPushSecret("secret")
	if h1 == "" || h1 != h2 {
		t.Fatalf("hash mismatch: %q %q", h1, h2)
	}
	if HashPushSecret("") != "" {
		t.Fatal("empty secret should hash to empty")
	}
}

func TestEnsureAndVerifyRTMPIngestSlot(t *testing.T) {
	key := "test_ingest_slot_cam"
	DeleteRTMPIngestSlot(key)
	defer DeleteRTMPIngestSlot(key)

	secret := "abc123secret"
	got, err := EnsureRTMPIngestSlot(key, secret, "")
	if err != nil {
		t.Fatal(err)
	}
	if got != key {
		t.Fatalf("key=%s", got)
	}
	if Get(key) == nil {
		t.Fatal("stream missing")
	}
	if err := VerifyRTMPPushToken(key, secret); err != nil {
		t.Fatalf("verify ok secret: %v", err)
	}
	if err := VerifyRTMPPushToken(key, "wrong"); err == nil {
		t.Fatal("expected reject wrong secret")
	}

	hash := HashPushSecret(secret)
	DeleteRTMPIngestSlot(key)
	if _, err := EnsureRTMPIngestSlot(key, "", hash); err != nil {
		t.Fatal(err)
	}
	if err := VerifyRTMPPushToken(key, secret); err != nil {
		t.Fatalf("restore hash verify: %v", err)
	}
}
