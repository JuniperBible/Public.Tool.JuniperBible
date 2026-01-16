package driver

import (
	"database/sql/driver"
	"fmt"
	"time"
)

// ValueConverter provides type conversion utilities for SQLite values.
type ValueConverter struct{}

// ConvertValue converts a Go value to a driver.Value.
func (vc ValueConverter) ConvertValue(v interface{}) (driver.Value, error) {
	// Handle nil
	if v == nil {
		return nil, nil
	}

	// Handle types that are already valid driver.Value
	switch v.(type) {
	case int64, float64, bool, []byte, string, time.Time:
		return v, nil
	}

	// Convert common types
	switch val := v.(type) {
	case int:
		return int64(val), nil
	case int8:
		return int64(val), nil
	case int16:
		return int64(val), nil
	case int32:
		return int64(val), nil
	case uint:
		return int64(val), nil
	case uint8:
		return int64(val), nil
	case uint16:
		return int64(val), nil
	case uint32:
		return int64(val), nil
	case uint64:
		// Check for overflow
		if val > 1<<63-1 {
			return nil, fmt.Errorf("uint64 value %d overflows int64", val)
		}
		return int64(val), nil
	case float32:
		return float64(val), nil
	default:
		return nil, fmt.Errorf("unsupported type: %T", v)
	}
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
