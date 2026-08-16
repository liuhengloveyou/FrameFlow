package api

import (
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/liuhengloveyou/FrameFlow/pkg/fferrors"
)

// ---------------------------------------------------------------------------
// 算法接入：能力目录 CRUD、推理试跑；以及摄像头通道上的算法绑定与启停。
// 路由见 MountAIBizAPIs（/api/v1/algorithms、cameras/.../algorithms）。
// ---------------------------------------------------------------------------

// handleAlgorithmsList 算法能力列表（支持 q / category / status 筛选）。
func handleAlgorithmsList(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	query, category, status := q.Get("q"), q.Get("category"), q.Get("status")
	page, err := aiBizBackend.ListAlgorithms(query, category, status)
	if err != nil {
		log.Warn().Err(err).
			Str("q", query).Str("category", category).Str("status", status).
			Msg("[algorithms] list algorithms failed")
		fferrors.RespondBizError(w, fferrors.ErrServer.SetMsg(err.Error()))
		return
	}
	log.Debug().
		Str("q", query).Str("category", category).Str("status", status).
		Int("total", page.Total).Int("count", len(page.Items)).
		Msg("[algorithms] list algorithms")
	ResponseData(w, page)
}

// handleAlgorithmCreate 新建算法能力（名称、类别、关联模型、默认参数等）。
func handleAlgorithmCreate(w http.ResponseWriter, r *http.Request) {
	var in AlgorithmInput
	if err := DecodeJSON(r, &in); err != nil {
		log.Warn().Err(err).Msg("[algorithms] create algorithm: bad json")
		fferrors.RespondBizError(w, fferrors.ErrBadRequest.SetMsg(err.Error()))
		return
	}
	log.Info().
		Str("name", in.Name).Str("category", in.Category).Str("model", in.ModelID).
		Msg("[algorithms] create algorithm begin")
	item, err := aiBizBackend.CreateAlgorithm(in)
	if err != nil {
		log.Warn().Err(err).Str("name", in.Name).Msg("[algorithms] create algorithm failed")
		fferrors.RespondErr(w, err)
		return
	}
	log.Info().
		Str("id", item.ID).Str("name", item.Name).Str("status", item.Status).
		Msg("[algorithms] create algorithm ok")
	ResponseData(w, item)
}

// handleAlgorithmUpdate 全量更新算法能力配置。
func handleAlgorithmUpdate(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var in AlgorithmInput
	if err := DecodeJSON(r, &in); err != nil {
		log.Warn().Err(err).Str("id", id).Msg("[algorithms] update algorithm: bad json")
		fferrors.RespondBizError(w, fferrors.ErrBadRequest.SetMsg(err.Error()))
		return
	}
	log.Info().
		Str("id", id).Str("name", in.Name).Str("model", in.ModelID).
		Msg("[algorithms] update algorithm begin")
	item, err := aiBizBackend.UpdateAlgorithm(id, in)
	if err != nil {
		log.Warn().Err(err).Str("id", id).Msg("[algorithms] update algorithm failed")
		fferrors.RespondErr(w, err)
		return
	}
	log.Info().Str("id", item.ID).Str("name", item.Name).Msg("[algorithms] update algorithm ok")
	ResponseData(w, item)
}

// handleAlgorithmDelete 删除算法能力（仍有通道绑定时拒绝）。
func handleAlgorithmDelete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		log.Warn().Msg("[algorithms] delete algorithm: missing id")
		fferrors.RespondBizError(w, fferrors.ErrBadRequest.SetMsg("missing id"))
		return
	}
	log.Info().Str("id", id).Msg("[algorithms] delete algorithm begin")
	if err := aiBizBackend.DeleteAlgorithm(id); err != nil {
		log.Warn().Err(err).Str("id", id).Msg("[algorithms] delete algorithm failed")
		fferrors.RespondErr(w, err)
		return
	}
	log.Info().Str("id", id).Msg("[algorithms] delete algorithm ok")
	ResponseData(w, map[string]string{"status": "ok"})
}

// handleAlgorithmPatchStatus 仅切换算法状态（启用 / 停用等），不影响其它配置。
func handleAlgorithmPatchStatus(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var body struct {
		Status string `json:"status"`
	}
	if err := DecodeJSON(r, &body); err != nil {
		log.Warn().Err(err).Str("id", id).Msg("[algorithms] patch algorithm status: bad json")
		fferrors.RespondBizError(w, fferrors.ErrBadRequest.SetMsg(err.Error()))
		return
	}
	log.Info().Str("id", id).Str("status", body.Status).Msg("[algorithms] patch algorithm status begin")
	item, err := aiBizBackend.UpdateAlgorithmStatus(id, body.Status)
	if err != nil {
		log.Warn().Err(err).Str("id", id).Str("status", body.Status).Msg("[algorithms] patch algorithm status failed")
		fferrors.RespondErr(w, err)
		return
	}
	log.Info().Str("id", item.ID).Str("status", item.Status).Msg("[algorithms] patch algorithm status ok")
	ResponseData(w, item)
}

// handleAlgorithmInferTest 算法推理试跑：上传图片或视频样张，返回检测结果供前端预览。
// 支持 multipart（字段 file / conf / imageName）与 JSON（imageName / conf）。
func handleAlgorithmInferTest(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	ct := r.Header.Get("Content-Type")
	var imageName string
	var conf float64
	var media []byte

	if strings.HasPrefix(ct, "multipart/form-data") {
		r.Body = http.MaxBytesReader(w, r.Body, 128<<20)
		if err := r.ParseMultipartForm(64 << 20); err != nil {
			log.Warn().Err(err).Str("id", id).Msg("[algorithms] infer-test: multipart invalid")
			fferrors.RespondBizError(w, fferrors.ErrBadRequest.SetMsg("上传失败：文件过大或表单无效（图片/视频合计建议 < 64MB）"))
			return
		}
		if v := r.FormValue("conf"); v != "" {
			conf, _ = strconv.ParseFloat(v, 64)
		}
		imageName = strings.TrimSpace(r.FormValue("imageName"))
		if f, hdr, err := r.FormFile("file"); err == nil {
			defer f.Close()
			if imageName == "" {
				imageName = hdr.Filename
			}
			media, err = io.ReadAll(io.LimitReader(f, 96<<20))
			if err != nil {
				log.Warn().Err(err).Str("id", id).Msg("[algorithms] infer-test: read file failed")
				fferrors.RespondBizError(w, fferrors.ErrBadRequest.SetMsg("读取上传文件失败"))
				return
			}
		}
	} else {
		var body struct {
			ImageName string  `json:"imageName"`
			Conf      float64 `json:"conf"`
			ImageB64  string  `json:"imageBase64"`
		}
		if err := DecodeJSON(r, &body); err != nil {
			log.Warn().Err(err).Str("id", id).Msg("[algorithms] infer-test: bad json")
			fferrors.RespondBizError(w, fferrors.ErrBadRequest.SetMsg(err.Error()))
			return
		}
		imageName = body.ImageName
		conf = body.Conf
	}

	log.Info().
		Str("id", id).
		Str("image", imageName).
		Float64("conf", conf).
		Int("bytes", len(media)).
		Str("content_type", ct).
		Str("remote", r.RemoteAddr).
		Msg("[algorithms] infer-test begin")
	res, err := aiBizBackend.InferTest(id, imageName, conf, media)
	if err != nil {
		log.Error().Err(err).
			Str("id", id).
			Str("image", imageName).
			Int("bytes", len(media)).
			Msg("[algorithms] infer-test failed")
		fferrors.RespondErr(w, err)
		return
	}
	dets := 0
	if m, ok := res["detections"].([]map[string]any); ok {
		dets = len(m)
	} else if arr, ok := res["detections"].([]any); ok {
		dets = len(arr)
	}
	log.Info().
		Str("id", id).
		Str("image", imageName).
		Int("detections", dets).
		Msg("[algorithms] infer-test ok")
	ResponseData(w, res)
}

// handleCameraAlgorithmsList 查询某摄像头通道已绑定的算法及运行配置。
func handleCameraAlgorithmsList(w http.ResponseWriter, r *http.Request) {
	cameraID, channelID := r.PathValue("id"), r.PathValue("cid")
	items, err := aiBizBackend.ListCameraAlgorithms(cameraID, channelID)
	if err != nil {
		log.Warn().Err(err).
			Str("camera", cameraID).Str("channel", channelID).
			Msg("[algorithms] list camera algorithms failed")
		fferrors.RespondErr(w, err)
		return
	}
	log.Debug().
		Str("camera", cameraID).Str("channel", channelID).Int("count", len(items)).
		Msg("[algorithms] list camera algorithms")
	ResponseData(w, items)
}

// handleCameraAlgorithmBind 将算法能力绑定到指定通道（创建绑定关系，含默认规则/参数）。
func handleCameraAlgorithmBind(w http.ResponseWriter, r *http.Request) {
	cameraID, channelID := r.PathValue("id"), r.PathValue("cid")
	var in CameraAlgorithmBindInput
	if err := DecodeJSON(r, &in); err != nil {
		log.Warn().Err(err).
			Str("camera", cameraID).Str("channel", channelID).
			Msg("[algorithms] bind camera algorithm: bad json")
		fferrors.RespondBizError(w, fferrors.ErrBadRequest.SetMsg(err.Error()))
		return
	}
	log.Info().
		Str("camera", cameraID).Str("channel", channelID).Str("algorithm", in.AlgorithmID).
		Msg("[algorithms] bind camera algorithm begin")
	item, err := aiBizBackend.BindCameraAlgorithm(cameraID, channelID, in.AlgorithmID)
	if err != nil {
		log.Warn().Err(err).
			Str("camera", cameraID).Str("channel", channelID).Str("algorithm", in.AlgorithmID).
			Msg("[algorithms] bind camera algorithm failed")
		fferrors.RespondErr(w, err)
		return
	}
	log.Info().
		Str("camera", cameraID).Str("channel", channelID).
		Str("bind", item.ID).Str("algorithm", item.AlgorithmID).
		Msg("[algorithms] bind camera algorithm ok")
	ResponseData(w, item)
}

// handleCameraAlgorithmSave 保存通道算法配置（规则区域、阈值、告警策略等补丁更新）。
func handleCameraAlgorithmSave(w http.ResponseWriter, r *http.Request) {
	cameraID, channelID, bindID := r.PathValue("id"), r.PathValue("cid"), r.PathValue("bid")
	var patch CameraAlgorithmPatch
	if err := DecodeJSON(r, &patch); err != nil {
		log.Warn().Err(err).
			Str("camera", cameraID).Str("channel", channelID).Str("bind", bindID).
			Msg("[algorithms] save camera algorithm: bad json")
		fferrors.RespondBizError(w, fferrors.ErrBadRequest.SetMsg(err.Error()))
		return
	}
	log.Info().
		Str("camera", cameraID).Str("channel", channelID).Str("bind", bindID).
		Int("regions", len(patch.Regions)).
		Msg("[algorithms] save camera algorithm begin")
	item, err := aiBizBackend.SaveCameraAlgorithm(cameraID, channelID, bindID, patch)
	if err != nil {
		log.Warn().Err(err).
			Str("camera", cameraID).Str("channel", channelID).Str("bind", bindID).
			Msg("[algorithms] save camera algorithm failed")
		fferrors.RespondErr(w, err)
		return
	}
	log.Info().
		Str("camera", cameraID).Str("channel", channelID).Str("bind", item.ID).
		Msg("[algorithms] save camera algorithm ok")
	ResponseData(w, item)
}

// handleCameraAlgorithmUnbind 解除通道与算法的绑定。
func handleCameraAlgorithmUnbind(w http.ResponseWriter, r *http.Request) {
	cameraID, channelID, bindID := r.PathValue("id"), r.PathValue("cid"), r.PathValue("bid")
	log.Info().
		Str("camera", cameraID).Str("channel", channelID).Str("bind", bindID).
		Msg("[algorithms] unbind camera algorithm begin")
	if err := aiBizBackend.UnbindCameraAlgorithm(cameraID, channelID, bindID); err != nil {
		log.Warn().Err(err).
			Str("camera", cameraID).Str("channel", channelID).Str("bind", bindID).
			Msg("[algorithms] unbind camera algorithm failed")
		fferrors.RespondErr(w, err)
		return
	}
	log.Info().
		Str("camera", cameraID).Str("channel", channelID).Str("bind", bindID).
		Msg("[algorithms] unbind camera algorithm ok")
	ResponseData(w, map[string]string{"status": "ok"})
}

// handleCameraAlgorithmAction 通道算法启停等动作（path action，如 start / stop）。
func handleCameraAlgorithmAction(w http.ResponseWriter, r *http.Request) {
	cameraID, channelID, bindID, action :=
		r.PathValue("id"), r.PathValue("cid"), r.PathValue("bid"), r.PathValue("action")
	log.Info().
		Str("camera", cameraID).Str("channel", channelID).
		Str("bind", bindID).Str("action", action).
		Msg("[algorithms] camera algorithm action begin")
	item, err := aiBizBackend.CameraAlgorithmAction(cameraID, channelID, bindID, action)
	if err != nil {
		log.Warn().Err(err).
			Str("camera", cameraID).Str("channel", channelID).
			Str("bind", bindID).Str("action", action).
			Msg("[algorithms] camera algorithm action failed")
		fferrors.RespondErr(w, err)
		return
	}
	log.Info().
		Str("camera", cameraID).Str("channel", channelID).
		Str("bind", item.ID).Str("action", action).Str("status", item.Status).
		Msg("[algorithms] camera algorithm action ok")
	ResponseData(w, item)
}
