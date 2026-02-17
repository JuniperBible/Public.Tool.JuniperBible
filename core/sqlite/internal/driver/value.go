package driver

import (
	"database/sql/driver"
	"fmt"
	"time"
)

// ValueConverter provides type conversion utilities for SQLite values.
type ValueConverter struct{}

func isNativeDriverValue(v interface{}) bool {
	switch v.(type) {
	case int64, float64, bool, []byte, string, time.Time:
		return true
	}
	return false
}

func convertUint64(val uint64) (driver.Value, error) {
	if val > 1<<63-1 {
		return nil, fmt.Errorf("uint64 value %d overflows int64", val)
	}
	return int64(val), nil
}

func convertToInt64(v interface{}) (int64, bool) {
	switch val := v.(type) {
	case int:
		return int64(val), true
	case int8:
		return int64(val), true
	case int16:
		return int64(val), true
	case int32:
		return int64(val), true
	case uint:
		return int64(val), true
	case uint8:
		return int64(val), true
	case uint16:
		return int64(val), true
	case uint32:
		return int64(val), true
	}
	return 0, false
}

func (vc ValueConverter) ConvertValue(v interface{}) (driver.Value, error) {
	if v == nil {
		return nil, nil
	}
	if isNativeDriverValue(v) {
		return v, nil
	}
	if i64, ok := convertToInt64(v); ok {
		return i64, nil
	}
	if u64, ok := v.(uint64); ok {
		return convertUint64(u64)
	}
	if f32, ok := v.(float32); ok {
		return float64(f32), nil
	}
	return nil, fmt.Errorf("unsupported type: %T", v)
}

// Result implements database/sql/driver.Result.
type Result struct {
	lastInsertID int64
	rowsAffected int64
}

// LastInsertId returns the last inserted ID.
func (r *Result) LastInsertId() (int64, error) {
	return r.lastInsertID, nil
}

// RowsAffected returns the number of rows affected.
func (r *Result) RowsAffected() (int64, error) {
	return r.rowsAffected, nil
}

// sqliteValueConverter is the default value converter for SQLite.
var sqliteValueConverter = ValueConverter{}
