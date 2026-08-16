package fferrors

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	gerr "github.com/liuhengloveyou/go-errors"
)

// Biz HTTP 业务错误（AINVR BizError 风格；码表与本包其它错误统一用 go-errors 注册）。
type Biz struct {
	Code       int
	Reason     string
	Msg        string
	HTTPStatus int
}

func (e *Biz) Error() string {
	if e == nil {
		return ""
	}
	return e.Msg
}

// SetMsg 返回带新文案的副本，不修改预置变量。
func (e *Biz) SetMsg(s string) *Biz {
	if e == nil {
		return &Biz{Code: 43000, Reason: "ErrBadRequest", Msg: s, HTTPStatus: http.StatusBadRequest}
	}
	return &Biz{Code: e.Code, Reason: e.Reason, Msg: s, HTTPStatus: e.HTTPStatus}
}

// SetHTTPStatus 返回带新 HTTP 状态码的副本。
func (e *Biz) SetHTTPStatus(status int) *Biz {
	if e == nil {
		return &Biz{Code: 43004, Reason: "ErrServer", Msg: "服务器发生错误", HTTPStatus: status}
	}
	return &Biz{Code: e.Code, Reason: e.Reason, Msg: e.Msg, HTTPStatus: status}
}

func mustBiz(code int, reason, msg string, httpStatus int) *Biz {
	_ = gerr.NewError(code, msg) // 注册码表，防重复
	return &Biz{Code: code, Reason: reason, Msg: msg, HTTPStatus: httpStatus}
}

// --- HTTP / API 通用（43xxx）---

var (
	ErrBadRequest   = mustBiz(43000, "ErrBadRequest", "请求参数有误", http.StatusBadRequest)
	ErrUnauthorized = mustBiz(43001, "ErrUnauthorized", "未登录或会话已失效", http.StatusUnauthorized)
	ErrForbidden    = mustBiz(43002, "ErrForbidden", "无权限执行此操作", http.StatusForbidden)
	ErrNotFound     = mustBiz(43003, "ErrNotFound", "资源未找到", http.StatusNotFound)
	ErrServer       = mustBiz(43004, "ErrServer", "服务器发生错误", http.StatusInternalServerError)
	ErrConflict     = mustBiz(43005, "ErrConflict", "资源冲突", http.StatusConflict)
)

// RespondBizError 写入 JSON 业务错误：{ code:-1, msg, reason }。
func RespondBizError(w http.ResponseWriter, err *Biz) {
	if err == nil {
		err = ErrServer
	}
	status := err.HTTPStatus
	if status == 0 {
		status = http.StatusBadRequest
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"code":   -1,
		"msg":    err.Error(),
		"reason": err.Reason,
	})
}

// RespondErr 将普通 error 映射为 Biz 并写出。
func RespondErr(w http.ResponseWriter, err error) {
	RespondBizError(w, MapPlain(err))
}

// HTTPStatusFromCode 按业务码映射默认 HTTP 状态（无显式覆盖时）。
func HTTPStatusFromCode(code int) int {
	switch {
	case code == 43001:
		return http.StatusUnauthorized
	case code == 43002:
		return http.StatusForbidden
	case code == 43003:
		return http.StatusNotFound
	case code == 43005:
		return http.StatusConflict
	case code == 43004:
		return http.StatusInternalServerError
	case code/100 == 404: // 40400–40499 streams not found 等
		return http.StatusNotFound
	case code >= 42000 && code < 43000: // gb28181 面向用户
		return http.StatusBadRequest
	case code >= 41000 && code < 42000: // vision
		return http.StatusInternalServerError
	case code >= 40000 && code < 41000:
		return http.StatusBadRequest
	default:
		return http.StatusBadRequest
	}
}

// FromGoError 将 go-errors.Error 转为 Biz（HTTP 层写出）。
func FromGoError(ge *gerr.Error) *Biz {
	if ge == nil {
		return ErrServer
	}
	return &Biz{
		Code:       ge.Code,
		Reason:     fmt.Sprintf("E%d", ge.Code),
		Msg:        ge.Message,
		HTTPStatus: HTTPStatusFromCode(ge.Code),
	}
}

// AsBiz 从任意 error 提取 *Biz；支持 *Biz 与 *gerr.Error。
func AsBiz(err error) (*Biz, bool) {
	if err == nil {
		return nil, false
	}
	var b *Biz
	if errors.As(err, &b) {
		return b, true
	}
	var ge *gerr.Error
	if errors.As(err, &ge) {
		return FromGoError(ge), true
	}
	return nil, false
}

// MapPlain 将普通 error 映射为 Biz（启发式；优先识别本包已注册错误）。
func MapPlain(err error) *Biz {
	if err == nil {
		return ErrServer
	}
	if b, ok := AsBiz(err); ok {
		return b
	}
	msg := err.Error()
	low := strings.ToLower(msg)
	switch {
	case strings.Contains(msg, "不存在") || strings.Contains(low, "not found"):
		return ErrNotFound.SetMsg(msg)
	case strings.Contains(low, "already exists") || strings.Contains(msg, "无法删除") || strings.Contains(msg, "仍有"):
		return ErrConflict.SetMsg(msg)
	case strings.Contains(msg, "请") || strings.Contains(msg, "无效") || strings.Contains(msg, "不能"):
		return ErrBadRequest.SetMsg(msg)
	default:
		return ErrBadRequest.SetMsg(msg)
	}
}
