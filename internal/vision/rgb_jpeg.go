package vision

import (
	"bytes"
	"image"
	"image/jpeg"
)

// EncodeJPEG 把 packed RGB888 编码为 JPEG。仅告警证据落库这类低频路径调用
// （1080p 约 50ms），不要放进逐帧检测热路径。
func EncodeJPEG(rgb []byte, w, h int) ([]byte, error) {
	return rgbToJPEG(rgb, w, h)
}

// rgbToJPEG 将 packed RGB 转为 JPEG（告警抓拍用）。
func rgbToJPEG(rgb []byte, w, h int) ([]byte, error) {
	if w <= 0 || h <= 0 || len(rgb) < w*h*3 {
		return nil, errBadRGB
	}
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			i := (y*w + x) * 3
			o := img.PixOffset(x, y)
			img.Pix[o] = rgb[i]
			img.Pix[o+1] = rgb[i+1]
			img.Pix[o+2] = rgb[i+2]
			img.Pix[o+3] = 255
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 85}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

type badRGBError struct{}

func (badRGBError) Error() string { return "vision: bad rgb buffer" }

var errBadRGB = badRGBError{}
