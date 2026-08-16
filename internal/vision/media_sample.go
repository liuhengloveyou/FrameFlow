package vision

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// SampleKind 试跑样张类型。
type SampleKind string

const (
	SampleKindImage SampleKind = "image"
	SampleKindVideo SampleKind = "video"
)

// SampleToJPEG 将上传的图片或视频转为可供 DetectJPEGOnce 使用的 JPEG。
// 视频用系统 ffmpeg 抽取靠近开头的一帧（约 0.5s，失败则退回第 1 帧）。
func SampleToJPEG(data []byte, fileName string) (jpegBytes []byte, kind SampleKind, err error) {
	if len(data) == 0 {
		return nil, "", fmt.Errorf("空文件")
	}
	name := strings.ToLower(strings.TrimSpace(fileName))
	ext := filepath.Ext(name)

	if looksLikeImage(data, ext) {
		img, _, derr := decodeImageBytes(data)
		if derr != nil {
			return nil, "", fmt.Errorf("图片解码失败：%w", derr)
		}
		var buf bytes.Buffer
		if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 92}); err != nil {
			return nil, "", fmt.Errorf("编码 JPEG 失败：%w", err)
		}
		return buf.Bytes(), SampleKindImage, nil
	}

	if looksLikeVideo(data, ext) {
		frame, ferr := extractVideoFrameJPEG(data, ext)
		if ferr != nil {
			return nil, "", ferr
		}
		return frame, SampleKindVideo, nil
	}

	// 未知扩展名：先当图片试，再当视频试
	if img, _, derr := decodeImageBytes(data); derr == nil {
		var buf bytes.Buffer
		_ = jpeg.Encode(&buf, img, &jpeg.Options{Quality: 92})
		return buf.Bytes(), SampleKindImage, nil
	}
	if frame, ferr := extractVideoFrameJPEG(data, ext); ferr == nil {
		return frame, SampleKindVideo, nil
	}
	return nil, "", fmt.Errorf("不支持的文件类型（请上传 JPG/PNG 图片或 MP4/WebM/MOV 视频）")
}

func looksLikeImage(b []byte, ext string) bool {
	switch ext {
	case ".jpg", ".jpeg", ".png", ".gif", ".webp", ".bmp":
		return true
	}
	if len(b) >= 3 && b[0] == 0xff && b[1] == 0xd8 && b[2] == 0xff {
		return true
	}
	if len(b) >= 8 && string(b[0:8]) == "\x89PNG\r\n\x1a\n" {
		return true
	}
	return false
}

func looksLikeVideo(b []byte, ext string) bool {
	switch ext {
	case ".mp4", ".m4v", ".mov", ".webm", ".mkv", ".avi", ".ts", ".mpeg", ".mpg":
		return true
	}
	if len(b) >= 12 && string(b[4:8]) == "ftyp" {
		return true
	}
	if len(b) >= 4 && b[0] == 0x1a && b[1] == 0x45 && b[2] == 0xdf && b[3] == 0xa3 {
		return true // EBML / WebM / MKV
	}
	return false
}

func extractVideoFrameJPEG(data []byte, ext string) ([]byte, error) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return nil, fmt.Errorf("未找到 ffmpeg，无法从视频抽帧（请安装 ffmpeg 或改传图片）")
	}
	if ext == "" || !strings.HasPrefix(ext, ".") {
		ext = ".mp4"
	}
	in, err := os.CreateTemp("", "ff-infer-*"+ext)
	if err != nil {
		return nil, fmt.Errorf("创建临时文件失败：%w", err)
	}
	inPath := in.Name()
	defer os.Remove(inPath)
	if _, err := in.Write(data); err != nil {
		_ = in.Close()
		return nil, fmt.Errorf("写入临时视频失败：%w", err)
	}
	_ = in.Close()

	out, err := os.CreateTemp("", "ff-infer-frame-*.jpg")
	if err != nil {
		return nil, fmt.Errorf("创建临时帧文件失败：%w", err)
	}
	outPath := out.Name()
	_ = out.Close()
	defer os.Remove(outPath)

	// 先尝试 0.5s 处一帧（避开黑场）；失败再取首帧。
	if err := runFFmpegFrame(inPath, outPath, "0.5"); err != nil {
		if err2 := runFFmpegFrame(inPath, outPath, "0"); err2 != nil {
			return nil, fmt.Errorf("视频抽帧失败：%w", err2)
		}
	}
	frame, err := os.ReadFile(outPath)
	if err != nil {
		return nil, fmt.Errorf("读取抽帧结果失败：%w", err)
	}
	if len(frame) == 0 {
		return nil, fmt.Errorf("视频抽帧结果为空")
	}
	// 校验可解码
	if _, _, err := image.Decode(bytes.NewReader(frame)); err != nil {
		return nil, fmt.Errorf("抽帧 JPEG 无效：%w", err)
	}
	return frame, nil
}

func runFFmpegFrame(inPath, outPath, ss string) error {
	args := []string{
		"-hide_banner", "-loglevel", "error", "-y",
	}
	if ss != "" && ss != "0" {
		args = append(args, "-ss", ss)
	}
	args = append(args,
		"-i", inPath,
		"-frames:v", "1",
		"-q:v", "2",
		outPath,
	)
	cmd := exec.Command("ffmpeg", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("%s", msg)
	}
	st, err := os.Stat(outPath)
	if err != nil || st.Size() == 0 {
		return fmt.Errorf("未生成帧文件")
	}
	return nil
}
