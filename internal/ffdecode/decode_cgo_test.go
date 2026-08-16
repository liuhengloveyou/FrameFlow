//go:build cgo

package ffdecode

import (
	"os"
	"testing"
)

func TestNewClose(t *testing.T) {
	d, err := New(CodecH264)
	if err != nil {
		t.Fatal(err)
	}
	d.Close()
	_, err = d.Decode(nil)
	if err != ErrClosed {
		t.Fatalf("after Close, Decode: got %v want ErrClosed", err)
	}
}

func TestDecodeEmpty(t *testing.T) {
	d, err := New(CodecH265)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	_, err = d.Decode(nil)
	if err != ErrNoFrame {
		t.Fatalf("Decode(nil): got %v want ErrNoFrame", err)
	}
}

func TestDecodeH264IDR(t *testing.T) {
	for _, path := range []string{
		"/tmp/ffdecode-smoke/one.h264",
		"/tmp/ffdecode-smoke/baseline.h264",
		"/tmp/ffdecode-smoke/half.h264",
	} {
		path := path
		t.Run(path, func(t *testing.T) {
			b, err := os.ReadFile(path)
			if err != nil {
				t.Skip(err)
			}
			d, err := New(CodecH264)
			if err != nil {
				t.Fatal(err)
			}
			defer d.Close()
			fr, err := d.Decode(b)
			if err != nil {
				t.Fatalf("Decode: %v", err)
			}
			if fr.Width <= 0 || fr.Height <= 0 || len(fr.RGB) != fr.Width*fr.Height*3 {
				t.Fatalf("bad frame: %dx%d rgb=%d", fr.Width, fr.Height, len(fr.RGB))
			}
			fr2, err := d.Decode(b)
			if err != nil {
				t.Fatalf("Decode again: %v", err)
			}
			if fr2.Width != fr.Width || fr2.Height != fr.Height {
				t.Fatalf("size changed: %dx%d -> %dx%d", fr.Width, fr.Height, fr2.Width, fr2.Height)
			}
		})
	}
}
