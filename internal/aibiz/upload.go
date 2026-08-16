package aibiz

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/liuhengloveyou/FrameFlow/internal/api"
	"github.com/liuhengloveyou/FrameFlow/internal/vision"
	"github.com/liuhengloveyou/FrameFlow/pkg/utils"
)

// UploadModel 将上传的权重（及可选标签文件）写入 {models_dir}/{id}/ 并登记入库。
func (s *Service) UploadModel(in api.ModelRegisterInput, modelFileName string, modelReader io.Reader, labelsFileName string, labelsReader io.Reader) (api.ModelItem, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return api.ModelItem{}, fmt.Errorf("请填写模型名称")
	}
	if modelReader == nil || strings.TrimSpace(modelFileName) == "" {
		return api.ModelItem{}, fmt.Errorf("请上传模型文件")
	}
	formats := in.Formats
	if len(formats) == 0 {
		formats = []string{formatFromExt(modelFileName)}
	}
	ver := strings.TrimSpace(in.Version)
	if ver == "" {
		ver = "1.0.0"
	}

	id := newID("m-")
	dir := filepath.Join(vision.ModelsDataDir(), id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return api.ModelItem{}, fmt.Errorf("创建模型目录失败: %w", err)
	}

	safeName := sanitizeFileName(modelFileName)
	modelPath := filepath.Join(dir, safeName)
	size, err := writeUploadFile(modelPath, modelReader)
	if err != nil {
		_ = os.RemoveAll(dir)
		return api.ModelItem{}, fmt.Errorf("保存模型文件失败: %w", err)
	}
	if in.FileSizeBytes > 0 && size != in.FileSizeBytes {
		_ = os.RemoveAll(dir)
		return api.ModelItem{}, fmt.Errorf("模型文件不完整：期望 %d 字节，实际写入 %d 字节（请重新上传）", in.FileSizeBytes, size)
	}

	var labelsPath string
	if labelsReader != nil && strings.TrimSpace(labelsFileName) != "" {
		lp := filepath.Join(dir, sanitizeFileName(labelsFileName))
		if _, err := writeUploadFile(lp, labelsReader); err != nil {
			_ = os.RemoveAll(dir)
			return api.ModelItem{}, fmt.Errorf("保存标签文件失败: %w", err)
		}
		labelsPath = resolveModelPath(lp)
	}

	absModel := resolveModelPath(modelPath)
	if err := vision.ValidateModelFile(absModel, labelsPath, in.FileSizeBytes); err != nil {
		_ = os.RemoveAll(dir)
		return api.ModelItem{}, fmt.Errorf("模型校验失败：%w", err)
	}

	now := utils.NowISO()
	localVer := ver
	row := &ModelRow{
		ID:             id,
		Name:           name,
		Description:    strings.TrimSpace(in.Description),
		TagsJSON:       mustJSON(in.Tags),
		FormatsJSON:    mustJSON(formats),
		LicenseStatus:  "authorized",
		ModelStatus:    "ready",
		Version:        ver,
		PublishedAt:    now,
		LocalVersion:   &localVer,
		LocalUpdatedAt: &now,
		RuntimeScale:   1,
		FileName:       safeName,
		FileSizeBytes:  size,
		LocalPath:      resolveModelPath(modelPath),
		LabelsPath:     labelsPath,
	}
	if row.Description == "" {
		row.Description = "上传接入模型"
	}
	if err := s.store.insertModel(row); err != nil {
		_ = os.RemoveAll(dir)
		return api.ModelItem{}, err
	}
	msg := fmt.Sprintf("已上传模型 · %s（%.1f MB）", safeName, float64(size)/1024/1024)
	if labelsPath != "" {
		msg += " · 含类别标签"
	}
	_ = s.store.insertUpdate(&ModelUpdateRow{
		ID:             newID("u-"),
		ModelName:      row.Name,
		CurrentVersion: "-",
		TargetVersion:  ver,
		OperatedAt:     now,
		Status:         "success",
		Message:        msg,
	})
	s.ensureORTReady()
	return s.modelToDTO(*row), nil
}

func writeUploadFile(path string, r io.Reader) (int64, error) {
	f, err := os.Create(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	n, err := io.Copy(f, r)
	if err != nil {
		return n, err
	}
	return n, f.Sync()
}

func sanitizeFileName(name string) string {
	name = filepath.Base(strings.TrimSpace(name))
	name = strings.ReplaceAll(name, "..", "_")
	if name == "" || name == "." || name == "/" {
		return "model.bin"
	}
	return name
}

func formatFromExt(name string) string {
	ext := strings.ToLower(filepath.Ext(name))
	switch ext {
	case ".onnx":
		return "ONNX"
	case ".rknn":
		return "RKNN"
	case ".engine", ".plan", ".trt":
		return "TensorRT"
	default:
		return "ONNX"
	}
}

func isManagedModelDir(localPath string) bool {
	return vision.IsUnderModelsRoot(localPath)
}

func removeManagedModelFiles(localPath string) {
	if !isManagedModelDir(localPath) {
		return
	}
	dir := filepath.Dir(resolveModelPath(localPath))
	root := filepath.Clean(vision.ModelsRootAbs())
	dir = filepath.Clean(dir)
	// 只删除根下的一级子目录（通常为 data/models/{id}），避免误删整个 models_dir
	if dir == root || !strings.HasPrefix(dir, root+string(os.PathSeparator)) {
		return
	}
	_ = os.RemoveAll(dir)
}
