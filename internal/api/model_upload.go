package api

import (
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/liuhengloveyou/FrameFlow/pkg/fferrors"
)

const maxModelUploadBytes = 512 << 20 // 512 MiB

// handleModelUpload 上传权重文件并登记到模型仓库（multipart：file 必填，labels 可选）。
func handleModelUpload(w http.ResponseWriter, r *http.Request) {
	if aiBizBackend == nil {
		log.Warn().Msg("[models] upload model: backend nil")
		fferrors.RespondBizError(w, fferrors.ErrServer.SetMsg("AI 业务未就绪").SetHTTPStatus(http.StatusServiceUnavailable))
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxModelUploadBytes+1<<20)
	// 内存只缓冲 32MB，更大文件落到临时目录，避免 YOLO 大权重撑爆进程内存
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		log.Warn().Err(err).Msg("[models] upload model: multipart invalid")
		fferrors.RespondBizError(w, fferrors.ErrBadRequest.SetMsg("上传失败：文件过大或表单无效（上限约 512MB）"))
		return
	}

	file, hdr, err := r.FormFile("file")
	if err != nil {
		log.Warn().Err(err).Msg("[models] upload model: missing file")
		fferrors.RespondBizError(w, fferrors.ErrBadRequest.SetMsg("请选择模型文件（字段名 file）"))
		return
	}
	defer file.Close()

	ext := strings.ToLower(filepathExt(hdr.Filename))
	switch ext {
	case ".onnx", ".rknn", ".engine", ".plan", ".trt", ".pt", ".bin":
	default:
		log.Warn().Str("file", hdr.Filename).Str("ext", ext).Msg("[models] upload model: unsupported format")
		fferrors.RespondBizError(w, fferrors.ErrBadRequest.SetMsg("不支持的模型格式，请上传 .onnx / .rknn / .engine 等"))
		return
	}

	var labelsName string
	var labelsReader io.Reader
	if lf, lh, err := r.FormFile("labels"); err == nil {
		defer lf.Close()
		labelsName = lh.Filename
		labelsReader = lf
	}

	formats := splitCSV(r.FormValue("formats"))
	if len(formats) == 0 {
		formats = []string{"ONNX"}
	}
	in := ModelRegisterInput{
		Name:        r.FormValue("name"),
		Description: r.FormValue("description"),
		Version:     r.FormValue("version"),
		Formats:     formats,
		Tags:        splitCSV(r.FormValue("tags")),
		FileName:    hdr.Filename,
	}
	if in.Name == "" {
		in.Name = strings.TrimSuffix(hdr.Filename, filepathExt(hdr.Filename))
	}
	if sz := r.FormValue("fileSizeBytes"); sz != "" {
		if n, e := strconv.ParseInt(sz, 10, 64); e == nil {
			in.FileSizeBytes = n
		}
	}
	if in.FileSizeBytes <= 0 && hdr.Size > 0 {
		in.FileSizeBytes = hdr.Size
	}

	log.Info().
		Str("name", in.Name).Str("file", hdr.Filename).
		Int64("size", in.FileSizeBytes).Str("labels", labelsName).
		Msg("[models] upload model begin")
	item, err := aiBizBackend.UploadModel(in, hdr.Filename, file, labelsName, labelsReader)
	if err != nil {
		log.Warn().Err(err).Str("name", in.Name).Str("file", hdr.Filename).Msg("[models] upload model failed")
		fferrors.RespondErr(w, err)
		return
	}
	log.Info().
		Str("id", item.ID).Str("name", item.Name).Str("file", item.FileName).
		Msg("[models] upload model ok")
	ResponseData(w, item)
}

func splitCSV(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	parts := strings.FieldsFunc(s, func(r rune) bool {
		return r == ',' || r == '，' || r == ' ' || r == '\t' || r == ';'
	})
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func filepathExt(name string) string {
	i := strings.LastIndex(name, ".")
	if i < 0 {
		return ""
	}
	return name[i:]
}
