package model

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"
)

// Time 是 SQLite 兼容的时间类型：存储为 RFC3339 字符串，
// 实现 sql.Scanner / driver.Valuer / json 序列化，支持重启恢复。
type Time struct {
	time.Time
}

// NewTime 构造 Time。
func NewTime(t time.Time) Time { return Time{t} }

// ParseTime 从 RFC3339 字符串解析 Time。
func ParseTime(s string) (Time, error) {
	tt, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return Time{}, err
	}
	return NewTime(tt), nil
}

// NowUTC 返回当前 UTC 时间。
func NowUTC() Time { return NewTime(time.Now().UTC()) }

// Value 实现 driver.Valuer：输出 RFC3339 字符串。
func (t Time) Value() (driver.Value, error) {
	if t.IsZero() {
		return "", nil
	}
	return t.Time.UTC().Format(time.RFC3339), nil
}

// Scan 实现 sql.Scanner：接受 string / time.Time / []byte。
func (t *Time) Scan(v any) error {
	switch x := v.(type) {
	case nil:
		t.Time = time.Time{}
		return nil
	case string:
		tt, err := time.Parse(time.RFC3339, x)
		if err != nil {
			return fmt.Errorf("解析时间 %q 失败: %w", x, err)
		}
		t.Time = tt
		return nil
	case []byte:
		return t.Scan(string(x))
	case time.Time:
		t.Time = x
		return nil
	default:
		return fmt.Errorf("无法把 %T 扫描为 model.Time", v)
	}
}

// MarshalJSON 输出 RFC3339 字符串。
func (t Time) MarshalJSON() ([]byte, error) {
	if t.IsZero() {
		return []byte("null"), nil
	}
	return json.Marshal(t.Time.UTC().Format(time.RFC3339))
}

// UnmarshalJSON 解析 RFC3339 字符串。
func (t *Time) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	tt, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return fmt.Errorf("解析 JSON 时间 %q 失败: %w", s, err)
	}
	t.Time = tt
	return nil
}
