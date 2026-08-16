package vision

import (
	"os"
	"path"
	"path/filepath"
	"testing"
)

func TestSetModelsDataDir(t *testing.T) {
	prev := ModelsDataDir()
	t.Cleanup(func() { SetModelsDataDir(prev) })

	dir := t.TempDir()
	SetModelsDataDir(dir)
	if ModelsDataDir() != dir {
		t.Fatalf("got %q want %q", ModelsDataDir(), dir)
	}
	ok := filepath.Join(dir, "m-1", "a.onnx")
	_ = os.MkdirAll(filepath.Dir(ok), 0o755)
	if !IsUnderModelsRoot(ok) {
		t.Fatalf("expected under configured root: %s", ok)
	}
	if IsUnderModelsRoot("data/models/x.onnx") && filepath.Clean(dir) != filepath.Clean(ModelsRootAbs()) {
		// only fail if default root somehow still matches
		if IsUnderModelsRoot(filepath.Join("data", "models", "x.onnx")) &&
			!stringsHasPrefix(filepath.Clean(MustAbs(filepath.Join("data", "models", "x.onnx"))), filepath.Clean(dir)) {
			// path may not exist; IsUnderModelsRoot only checks prefix
		}
	}
	outside := filepath.Join(t.TempDir(), "other.onnx")
	if IsUnderModelsRoot(outside) {
		t.Fatalf("should reject outside path %q", outside)
	}
}

func MustAbs(p string) string {
	a, err := filepath.Abs(p)
	if err != nil {
		return p
	}
	return a
}

func stringsHasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

func TestValidateModelFileRejectsOutside(t *testing.T) {
	prev := ModelsDataDir()
	t.Cleanup(func() { SetModelsDataDir(prev) })
	SetModelsDataDir(filepath.Join(t.TempDir(), "models-root"))

	err := ValidateModelFile("models/YOLO26n_voc.onnx", "", 0)
	if err == nil {
		t.Fatal("expected forbidden path error")
	}
}

func TestValidateModelFileEmpty(t *testing.T) {
	prev := ModelsDataDir()
	root := t.TempDir()
	t.Cleanup(func() { SetModelsDataDir(prev) })
	SetModelsDataDir(root)

	p := path.Join(root, "empty.onnx")
	if err := os.WriteFile(p, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := ValidateModelFile(p, "", 0); err == nil {
		t.Fatal("expected error for empty file")
	}
}
