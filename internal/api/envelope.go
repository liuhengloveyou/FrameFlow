package api

import (
	"encoding/json"
	"net/http"

	"github.com/liuhengloveyou/FrameFlow/pkg/fferrors"
)

// ResponseData 业务 API 成功信封：{ "code": 0, "data": T }（前端读 data.data）。
func ResponseData(w http.ResponseWriter, v any) {
	ResponseJSON(w, map[string]any{"code": 0, "data": v})
}

// WriteAPIError 兼容旧调用；新代码请用 fferrors.RespondBizError / RespondErr。
func WriteAPIError(w http.ResponseWriter, status int, msg string) {
	base := fferrors.ErrBadRequest
	switch status {
	case http.StatusUnauthorized:
		base = fferrors.ErrUnauthorized
	case http.StatusForbidden:
		base = fferrors.ErrForbidden
	case http.StatusNotFound:
		base = fferrors.ErrNotFound
	case http.StatusConflict:
		base = fferrors.ErrConflict
	case http.StatusInternalServerError, http.StatusServiceUnavailable:
		base = fferrors.ErrServer
	}
	fferrors.RespondBizError(w, base.SetMsg(msg).SetHTTPStatus(status))
}

// DecodeJSON 解码请求体。
func DecodeJSON(r *http.Request, dst any) error {
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(dst)
}
