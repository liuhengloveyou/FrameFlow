package vision

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	ort "github.com/yalue/onnxruntime_go"

	"github.com/liuhengloveyou/FrameFlow/pkg/fferrors"
)

const (
	minONNXBytes         = 4 << 10 // 4 KiB
	defaultModelsDataDir = "data/models"
)

var (
	modelsDirMu   sync.RWMutex
	modelsDataDir = defaultModelsDataDir
)

// ModelsDataDir 返回当前模型根目录（默认 data/models）。
func ModelsDataDir() string {
	modelsDirMu.RLock()
	defer modelsDirMu.RUnlock()
	return modelsDataDir
}

// SetModelsDataDir 设置模型上传/存放根目录；空则恢复默认 data/models，并确保目录存在。
func SetModelsDataDir(dir string) {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		dir = defaultModelsDataDir
	}
	modelsDirMu.Lock()
	modelsDataDir = dir
	modelsDirMu.Unlock()
	_ = os.MkdirAll(dir, 0o755)
}

// ModelsRootAbs 返回模型根目录的绝对路径。
func ModelsRootAbs() string {
	root := ModelsDataDir()
	abs, err := filepath.Abs(root)
	if err != nil {
		return root
	}
	return abs
}

// IsUnderModelsRoot 判断路径是否位于已配置的模型根目录之下。
func IsUnderModelsRoot(p string) bool {
	p = strings.TrimSpace(p)
	if p == "" {
		return false
	}
	abs := p
	if !filepath.IsAbs(p) {
		var err error
		abs, err = filepath.Abs(p)
		if err != nil {
			return false
		}
	}
	root := ModelsRootAbs()
	absClean := filepath.Clean(abs)
	rootClean := filepath.Clean(root)
	sep := string(os.PathSeparator)
	return absClean == rootClean || strings.HasPrefix(absClean, rootClean+sep)
}

// IsForbiddenModelPath 非模型根目录下的路径一律禁止。
func IsForbiddenModelPath(p string) bool {
	p = strings.TrimSpace(p)
	if p == "" {
		return false
	}
	return !IsUnderModelsRoot(p)
}

// ResolveModelPath 将相对路径规范为绝对路径。
func ResolveModelPath(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return ""
	}
	if filepath.IsAbs(p) {
		return filepath.Clean(p)
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		return filepath.Clean(p)
	}
	return abs
}

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

// ValidateModelFile 校验本地模型是否完整、可加载，且必须位于配置的模型根目录下。
func ValidateModelFile(modelPath, labelsPath string, expectedSize int64) error {
	modelPath = strings.TrimSpace(modelPath)
	if modelPath == "" {
		return fmt.Errorf("模型路径为空")
	}
	root := ModelsDataDir()
	if IsForbiddenModelPath(modelPath) {
		return fmt.Errorf("模型文件必须位于 %s/ 目录下", root)
	}
	st, err := os.Stat(modelPath)
	if err != nil {
		return fmt.Errorf("模型文件不存在：%w", err)
	}
	if st.IsDir() {
		return fmt.Errorf("模型路径是目录而非文件")
	}
	size := st.Size()
	if size <= 0 {
		return fmt.Errorf("模型文件为空，可能上传未完成")
	}
	if expectedSize > 0 && size != expectedSize {
		return fmt.Errorf("模型文件不完整：期望 %d 字节，实际 %d 字节（可能上传中断）", expectedSize, size)
	}

	ext := strings.ToLower(filepath.Ext(modelPath))
	switch ext {
	case ".onnx":
		if err := validateONNXFile(modelPath, size); err != nil {
			return err
		}
	case ".rknn", ".engine", ".plan", ".trt", ".pt", ".bin":
		if size < 1024 {
			return fmt.Errorf("模型文件过小（%d 字节），可能上传不完整", size)
		}
	default:
		if size < 1024 {
			return fmt.Errorf("模型文件过小（%d 字节），可能上传不完整", size)
		}
	}

	if lp := strings.TrimSpace(labelsPath); lp != "" {
		if IsForbiddenModelPath(lp) {
			return fmt.Errorf("标签文件必须位于 %s/ 目录下", root)
		}
		if _, err := loadLabels(lp); err != nil {
			return fmt.Errorf("标签文件无效：%w", err)
		}
	}
	return nil
}

func validateONNXFile(path string, size int64) error {
	if size < minONNXBytes {
		return fmt.Errorf("ONNX 文件过小（%d 字节），可能上传不完整", size)
	}
	head := make([]byte, 64)
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	n, _ := f.Read(head)
	_ = f.Close()
	if n < 8 {
		return fmt.Errorf("无法读取 ONNX 文件头，文件可能损坏")
	}

	if err := ensureORT(""); err != nil {
		return fmt.Errorf("ONNX Runtime 未就绪，无法校验模型：%w", err)
	}
	inName, outName, err := pickYoloIONames(path)
	if err != nil {
		return fmt.Errorf("ONNX 结构无效或不完整（无法解析输入/输出）：%w", err)
	}
	sess, err := ort.NewDynamicAdvancedSession(path, []string{inName}, []string{outName}, nil)
	if err != nil {
		return fmt.Errorf("ONNX 无法加载（文件可能损坏或不完整）：%w", err)
	}
	_ = sess.Destroy()
	if inName == "" || outName == "" {
		return fferrors.ErrVisionModelNoIO
	}
	return nil
}
