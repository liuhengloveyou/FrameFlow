package api

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/liuhengloveyou/FrameFlow/pkg/fferrors"
)

func handleReviewEventsList(w http.ResponseWriter, r *http.Request) {
	q := parseReviewQuery(r)
	res, err := aiBizBackend.ListReviewEvents(q)
	if err != nil {
		log.Warn().Err(err).Msg("[review] list events failed")
		fferrors.RespondBizError(w, fferrors.ErrServer.SetMsg(err.Error()))
		return
	}
	ResponseData(w, res)
}

func handleReviewKindCounts(w http.ResponseWriter, r *http.Request) {
	q := parseReviewQuery(r)
	res, err := aiBizBackend.ReviewKindCounts(q)
	if err != nil {
		log.Warn().Err(err).Msg("[review] counts failed")
		fferrors.RespondBizError(w, fferrors.ErrServer.SetMsg(err.Error()))
		return
	}
	ResponseData(w, res)
}

func handleReviewEventGet(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	item, err := aiBizBackend.GetReviewEvent(id)
	if err != nil {
		log.Warn().Err(err).Str("id", id).Msg("[review] get failed")
		fferrors.RespondErr(w, err)
		return
	}
	ResponseData(w, item)
}

func handleReviewEventContext(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	q := r.URL.Query()
	window, _ := strconv.Atoi(q.Get("windowMinutes"))
	page, _ := strconv.Atoi(q.Get("page"))
	pageSize, _ := strconv.Atoi(q.Get("pageSize"))
	res, err := aiBizBackend.ListReviewContext(id, window, page, pageSize)
	if err != nil {
		log.Warn().Err(err).Str("id", id).Msg("[review] context failed")
		fferrors.RespondErr(w, err)
		return
	}
	ResponseData(w, res)
}

func handleReviewEventsMark(w http.ResponseWriter, r *http.Request) {
	var in ReviewMarkInput
	if err := DecodeJSON(r, &in); err != nil {
		fferrors.RespondBizError(w, fferrors.ErrBadRequest.SetMsg(err.Error()))
		return
	}
	n, err := aiBizBackend.MarkReviewEvents(in.IDs, in.Status)
	if err != nil {
		log.Warn().Err(err).Msg("[review] mark failed")
		fferrors.RespondErr(w, err)
		return
	}
	ResponseData(w, n)
}

func handleReviewEventPromote(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	item, err := aiBizBackend.PromoteReviewDetection(id)
	if err != nil {
		log.Warn().Err(err).Str("id", id).Msg("[review] promote failed")
		fferrors.RespondErr(w, err)
		return
	}
	ResponseData(w, item)
}

func parseReviewQuery(r *http.Request) ReviewQuery {
	q := r.URL.Query()
	page, _ := strconv.Atoi(q.Get("page"))
	pageSize, _ := strconv.Atoi(q.Get("pageSize"))
	return ReviewQuery{
		Page:      page,
		PageSize:  pageSize,
		Kind:      strings.TrimSpace(q.Get("kind")),
		Status:    strings.TrimSpace(q.Get("status")),
		Q:         strings.TrimSpace(q.Get("q")),
		CameraIDs: parseCameraIDs(q),
		From:      strings.TrimSpace(q.Get("from")),
		To:        strings.TrimSpace(q.Get("to")),
	}
}

// parseCameraIDs 支持 cameraIds 重复键、逗号分隔，以及 axios 的 cameraIds[]。
// 未传 → nil（不限）；显式空（cameraIds=）→ 空切片（无匹配）。
func parseCameraIDs(q url.Values) []string {
	vals, ok := q["cameraIds"]
	if !ok {
		vals, ok = q["cameraIds[]"]
	}
	if !ok {
		if single := strings.TrimSpace(q.Get("cameraId")); single != "" {
			return []string{single}
		}
		return nil
	}
	out := make([]string, 0, len(vals))
	for _, v := range vals {
		for _, part := range strings.Split(v, ",") {
			part = strings.TrimSpace(part)
			if part != "" {
				out = append(out, part)
			}
		}
	}
	return out
}
