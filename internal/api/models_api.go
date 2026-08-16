package api

import (
	"net/http"

	"github.com/liuhengloveyou/FrameFlow/pkg/fferrors"
)

// ---------------------------------------------------------------------------
// 模型仓库：元数据登记、本地权重生命周期、健康检查与更新记录。
// 上传并登记见 model_upload.go；路由见 MountAIBizAPIs（/api/v1/models）。
// ---------------------------------------------------------------------------

// handleModelsList 模型仓库列表（含本地/远程状态汇总）。
func handleModelsList(w http.ResponseWriter, r *http.Request) {
	items, total, err := aiBizBackend.ListModels()
	if err != nil {
		log.Warn().Err(err).Msg("[models] list models failed")
		fferrors.RespondBizError(w, fferrors.ErrServer.SetMsg(err.Error()))
		return
	}
	log.Debug().Int("total", total).Int("count", len(items)).Msg("[models] list models")
	ResponseData(w, map[string]any{"items": items, "total": total})
}

// handleModelRegister 仅登记模型元数据（不上传权重文件；有文件时走 upload）。
func handleModelRegister(w http.ResponseWriter, r *http.Request) {
	var in ModelRegisterInput
	if err := DecodeJSON(r, &in); err != nil {
		log.Warn().Err(err).Msg("[models] register model: bad json")
		fferrors.RespondBizError(w, fferrors.ErrBadRequest.SetMsg(err.Error()))
		return
	}
	log.Info().
		Str("name", in.Name).Str("version", in.Version).Str("file", in.FileName).
		Msg("[models] register model begin")
	item, err := aiBizBackend.RegisterModel(in)
	if err != nil {
		log.Warn().Err(err).Str("name", in.Name).Msg("[models] register model failed")
		fferrors.RespondErr(w, err)
		return
	}
	log.Info().Str("id", item.ID).Str("name", item.Name).Msg("[models] register model ok")
	ResponseData(w, item)
}

// handleModelUpdates 模型版本/更新记录列表。
func handleModelUpdates(w http.ResponseWriter, r *http.Request) {
	items, err := aiBizBackend.ListModelUpdates()
	if err != nil {
		log.Warn().Err(err).Msg("[models] list model updates failed")
		fferrors.RespondBizError(w, fferrors.ErrServer.SetMsg(err.Error()))
		return
	}
	log.Debug().Int("count", len(items)).Msg("[models] list model updates")
	ResponseData(w, items)
}

// handleModelDelete 从仓库删除模型（含元数据与本地文件，若存在）。
func handleModelDelete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		log.Warn().Msg("[models] delete model: missing id")
		fferrors.RespondBizError(w, fferrors.ErrBadRequest.SetMsg("missing id"))
		return
	}
	log.Info().Str("id", id).Msg("[models] delete model begin")
	if err := aiBizBackend.DeleteModel(id); err != nil {
		log.Warn().Err(err).Str("id", id).Msg("[models] delete model failed")
		fferrors.RespondErr(w, err)
		return
	}
	log.Info().Str("id", id).Msg("[models] delete model ok")
	ResponseData(w, map[string]string{"status": "ok"})
}

// handleModelDownload 将模型标记为已下载 / 拉取到本机可用。
func handleModelDownload(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	log.Info().Str("id", id).Msg("[models] download model begin")
	item, err := aiBizBackend.DownloadModel(id)
	if err != nil {
		log.Warn().Err(err).Str("id", id).Msg("[models] download model failed")
		fferrors.RespondErr(w, err)
		return
	}
	log.Info().
		Str("id", item.ID).Str("name", item.Name).Str("status", item.ModelStatus).
		Msg("[models] download model ok")
	ResponseData(w, item)
}

// handleModelRemoveLocal 仅移除本机权重文件，保留仓库中的元数据登记。
func handleModelRemoveLocal(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	log.Info().Str("id", id).Msg("[models] remove local model begin")
	item, err := aiBizBackend.RemoveLocalModel(id)
	if err != nil {
		log.Warn().Err(err).Str("id", id).Msg("[models] remove local model failed")
		fferrors.RespondErr(w, err)
		return
	}
	log.Info().
		Str("id", item.ID).Str("name", item.Name).Str("status", item.ModelStatus).
		Msg("[models] remove local model ok")
	ResponseData(w, item)
}

// handleModelHealth 模型健康检查（文件是否存在、格式/可读性等探测结果）。
func handleModelHealth(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	log.Info().Str("id", id).Msg("[models] model health begin")
	res, err := aiBizBackend.CheckModelHealth(id)
	if err != nil {
		log.Warn().Err(err).Str("id", id).Msg("[models] model health failed")
		fferrors.RespondErr(w, err)
		return
	}
	log.Info().
		Str("id", id).Bool("ok", res.OK).Int("latency_ms", res.LatencyMs).
		Msg("[models] model health ok")
	ResponseData(w, res)
}

// handleModelScale 调整模型 scale 参数（推理输入缩放等）。
func handleModelScale(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var body struct {
		Scale float64 `json:"scale"`
	}
	if err := DecodeJSON(r, &body); err != nil {
		log.Warn().Err(err).Str("id", id).Msg("[models] scale model: bad json")
		fferrors.RespondBizError(w, fferrors.ErrBadRequest.SetMsg(err.Error()))
		return
	}
	log.Info().Str("id", id).Float64("scale", body.Scale).Msg("[models] scale model begin")
	item, err := aiBizBackend.ScaleModel(id, body.Scale)
	if err != nil {
		log.Warn().Err(err).Str("id", id).Float64("scale", body.Scale).Msg("[models] scale model failed")
		fferrors.RespondErr(w, err)
		return
	}
	log.Info().
		Str("id", item.ID).Float64("scale", item.RuntimeScale).
		Msg("[models] scale model ok")
	ResponseData(w, item)
}
