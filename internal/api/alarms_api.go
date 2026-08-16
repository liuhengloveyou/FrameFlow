package api

import (
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/liuhengloveyou/FrameFlow/pkg/fferrors"
)

func handleAlarmsList(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page, _ := strconv.Atoi(q.Get("page"))
	pageSize, _ := strconv.Atoi(q.Get("pageSize"))
	res, err := aiBizBackend.ListAlarms(AlarmQuery{
		Page:     page,
		PageSize: pageSize,
		Level:    q.Get("level"),
		Status:   q.Get("status"),
		Q:        q.Get("q"),
	})
	if err != nil {
		log.Warn().Err(err).Msg("[alarms] list failed")
		fferrors.RespondBizError(w, fferrors.ErrServer.SetMsg(err.Error()))
		return
	}
	ResponseData(w, res)
}

func handleAlarmEvidence(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	rel, err := aiBizBackend.AlarmEvidencePath(id)
	if err != nil {
		log.Warn().Err(err).Str("id", id).Msg("[alarms] evidence lookup failed")
		fferrors.RespondErr(w, err)
		return
	}
	if rel == "" {
		fferrors.RespondBizError(w, fferrors.ErrNotFound.SetMsg("无抓拍证据"))
		return
	}
	abs, err := filepath.Abs(filepath.FromSlash(rel))
	if err != nil {
		fferrors.RespondBizError(w, fferrors.ErrServer.SetMsg(err.Error()))
		return
	}
	root, _ := filepath.Abs(filepath.Join("data", "alarm-evidence"))
	if root == "" || !strings.HasPrefix(abs, root+string(os.PathSeparator)) {
		fferrors.RespondBizError(w, fferrors.ErrBadRequest.SetMsg("非法证据路径"))
		return
	}
	if _, err := os.Stat(abs); err != nil {
		fferrors.RespondBizError(w, fferrors.ErrNotFound.SetMsg("证据文件不存在"))
		return
	}
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Cache-Control", "private, max-age=3600")
	http.ServeFile(w, r, abs)
}

func handleAlarmPatchStatus(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var in AlarmStatusPatch
	if err := DecodeJSON(r, &in); err != nil {
		fferrors.RespondBizError(w, fferrors.ErrBadRequest.SetMsg(err.Error()))
		return
	}
	item, err := aiBizBackend.UpdateAlarmStatus(id, in.Status)
	if err != nil {
		log.Warn().Err(err).Str("id", id).Msg("[alarms] patch status failed")
		fferrors.RespondErr(w, err)
		return
	}
	ResponseData(w, item)
}

func handleAlarmPatchWorkflow(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var in AlarmWorkflowPatch
	if err := DecodeJSON(r, &in); err != nil {
		fferrors.RespondBizError(w, fferrors.ErrBadRequest.SetMsg(err.Error()))
		return
	}
	item, err := aiBizBackend.LinkAlarmWorkflow(id, in.WorkflowID, in.Name)
	if err != nil {
		log.Warn().Err(err).Str("id", id).Msg("[alarms] patch workflow failed")
		fferrors.RespondErr(w, err)
		return
	}
	ResponseData(w, item)
}

func handleLinkageRulesList(w http.ResponseWriter, r *http.Request) {
	items, err := aiBizBackend.ListLinkageRules()
	if err != nil {
		log.Warn().Err(err).Msg("[alarms] list linkage rules failed")
		fferrors.RespondBizError(w, fferrors.ErrServer.SetMsg(err.Error()))
		return
	}
	ResponseData(w, items)
}

func handleLinkageRuleCreate(w http.ResponseWriter, r *http.Request) {
	var in AlarmLinkageRuleInput
	if err := DecodeJSON(r, &in); err != nil {
		fferrors.RespondBizError(w, fferrors.ErrBadRequest.SetMsg(err.Error()))
		return
	}
	item, err := aiBizBackend.CreateLinkageRule(in)
	if err != nil {
		log.Warn().Err(err).Msg("[alarms] create linkage rule failed")
		fferrors.RespondErr(w, err)
		return
	}
	ResponseData(w, item)
}

func handleLinkageRuleUpdate(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var in AlarmLinkageRuleInput
	if err := DecodeJSON(r, &in); err != nil {
		fferrors.RespondBizError(w, fferrors.ErrBadRequest.SetMsg(err.Error()))
		return
	}
	item, err := aiBizBackend.UpdateLinkageRule(id, in)
	if err != nil {
		log.Warn().Err(err).Str("id", id).Msg("[alarms] update linkage rule failed")
		fferrors.RespondErr(w, err)
		return
	}
	ResponseData(w, item)
}

func handleLinkageRuleDelete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := aiBizBackend.DeleteLinkageRule(id); err != nil {
		log.Warn().Err(err).Str("id", id).Msg("[alarms] delete linkage rule failed")
		fferrors.RespondErr(w, err)
		return
	}
	ResponseData(w, map[string]any{"ok": true})
}

func handleLinkageRulesApply(w http.ResponseWriter, r *http.Request) {
	onlyUnlinked := true
	var in ApplyLinkageInput
	if err := DecodeJSON(r, &in); err == nil && in.OnlyUnlinked != nil {
		onlyUnlinked = *in.OnlyUnlinked
	}
	res, err := aiBizBackend.ApplyLinkageRules(onlyUnlinked)
	if err != nil {
		log.Warn().Err(err).Msg("[alarms] apply linkage rules failed")
		fferrors.RespondBizError(w, fferrors.ErrServer.SetMsg(err.Error()))
		return
	}
	ResponseData(w, res)
}
