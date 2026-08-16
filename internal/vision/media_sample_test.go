package vision

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestSampleToJPEGImage(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 32, 24))
	for y := 0; y < 24; y++ {
		for x := 0; x < 32; x++ {
			img.Set(x, y, color.RGBA{R: 200, G: 100, B: 50, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatal(err)
	}
	out, kind, err := SampleToJPEG(buf.Bytes(), "t.jpg")
	if err != nil {
		t.Fatal(err)
	}
	if kind != SampleKindImage {
		t.Fatalf("kind=%s", kind)
	}
	if len(out) < 100 {
		t.Fatalf("jpeg too small: %d", len(out))
	}
}

func TestSampleToJPEGVideo(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not available")
	}
	dir := t.TempDir()
	mp4 := filepath.Join(dir, "tiny.mp4")
	cmd := exec.Command("ffmpeg", "-hide_banner", "-loglevel", "error", "-y",
		"-f", "lavfi", "-i", "color=c=blue:s=64x48:d=1",
		"-c:v", "libx264", "-pix_fmt", "yuv420p", mp4)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("cannot generate tiny mp4: %v %s", err, out)
	}
	data, err := os.ReadFile(mp4)
	if err != nil {
		t.Fatal(err)
	}
	jpegBytes, kind, err := SampleToJPEG(data, "tiny.mp4")
	if err != nil {
		t.Fatal(err)
	}
	if kind != SampleKindVideo {
		t.Fatalf("kind=%s", kind)
	}
	if len(jpegBytes) < 100 {
		t.Fatalf("frame jpeg too small: %d", len(jpegBytes))
	}
}
