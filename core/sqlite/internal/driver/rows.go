package driver

import (
	"context"
	"database/sql/driver"
	"io"

	"github.com/FocuswithJustin/JuniperBible/core/sqlite/internal/vdbe"
)

// Rows implements database/sql/driver.Rows for SQLite.
// It provides iteration over query results by executing the compiled VDBE program.
// The VDBE manages btree cursors internally through OpOpenRead/OpOpenWrite opcodes,
// and iterates using OpRewind/OpNext opcodes that interact with the btree layer.
type Rows struct {
	stmt    *Stmt
	vdbe    *vdbe.VDBE
	columns []string
	ctx     context.Context
	closed  bool
}

// Columns returns the column names.
func (r *Rows) Columns() []string {
	return r.columns
}

// Close closes the rows iterator.
func (r *Rows) Close() error {
	if r.closed {
		return nil
	}

	r.closed = true

	if r.vdbe != nil {
		r.vdbe.Finalize()
		r.vdbe = nil
	}

	return nil
}

// Next advances to the next row and populates dest with the row values.
//
// Integration with btree cursors:
// The VDBE program compiled from SELECT statements contains opcodes that:
// 1. OpOpenRead - Opens a btree cursor on the table's root page
// 2. OpRewind - Moves the btree cursor to the first entry
// 3. OpColumn - Reads column data from the current btree cursor position
// 4. OpResultRow - Packages the columns into a result row
// 5. OpNext - Advances the btree cursor to the next entry and loops
//
// Each call to Step() executes VDBE opcodes until either:
// - OpResultRow is hit (returns true with row data)
// - OpHalt is hit (returns false, signaling end of results)
// - An error occurs
func (r *Rows) Next(dest []driver.Value) error {
	if r.closed {
		return io.EOF
	}

	// Check context cancellation
	select {
	case <-r.ctx.Done():
		return r.ctx.Err()
	default:
	}

	// Step the VDBE to get the next row
	// This executes opcodes that interact with btree cursors
	hasMore, err := r.vdbe.Step()
	if err != nil {
		return err
	}

	if !hasMore {
		return io.EOF
	}

	// Check if we have a result row
	// ResultRow is populated by the OpResultRow opcode after OpColumn
	// opcodes have read data from the btree cursor
	if r.vdbe.ResultRow == nil {
		return io.EOF
	}

	// Copy values from result row to dest
	if len(dest) < len(r.vdbe.ResultRow) {
		return driver.ErrSkip
	}

	for i, mem := range r.vdbe.ResultRow {
		dest[i] = memToValue(mem)
	}

	return nil
}

// ColumnTypeScanType returns the scan type for a column.
func (r *Rows) ColumnTypeScanType(index int) interface{} {
	// SQLite is dynamically typed, so we return interface{}
	return nil
}

// ColumnTypeDatabaseTypeName returns the database type name for a column.
// SQLite uses dynamic typing, so the type is determined by the actual value.
// The column metadata could also be retrieved from the schema, but the
// actual type affinity comes from the data stored in btree cells.
func (r *Rows) ColumnTypeDatabaseTypeName(index int) string {
	// SQLite doesn't have static column types, but we can inspect the result
	if r.vdbe.ResultRow != nil && index < len(r.vdbe.ResultRow) {
		mem := r.vdbe.ResultRow[index]
		if mem.IsNull() {
			return "NULL"
		} else if mem.IsInt() {
			return "INTEGER"
		} else if mem.IsReal() {
			return "REAL"
		} else if mem.IsStr() {
			return "TEXT"
		} else if mem.IsBlob() {
			return "BLOB"
		}
	}
	return ""
}

// memToValue converts a VDBE memory cell to a driver.Value.
func memToValue(mem *vdbe.Mem) driver.Value {
	if mem.IsNull() {
		return nil
	} else if mem.IsInt() {
		return mem.IntValue()
	} else if mem.IsReal() {
		return mem.RealValue()
	} else if mem.IsStr() {
		return mem.StrValue()
	} else if mem.IsBlob() {
		return mem.BlobValue()
	}
	return nil
}
