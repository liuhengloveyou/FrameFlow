// Package utils 提供跨模块的通用小工具。
package utils

import "time"

// NowISO 返回当前 UTC 时间的 RFC3339Nano 字符串。
func NowISO() string {
	return time.Now().UTC().Format(time.RFC3339Nano)
}

// FormatISO 将时间格式化为 RFC3339Nano（UTC）；零值返回空串。
func FormatISO(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339Nano)
}

// ParseISO 解析 RFC3339Nano / RFC3339；失败返回零值。
func ParseISO(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		t, _ = time.Parse(time.RFC3339, s)
	}
	return t
}
