// Package sql provides SQL statement compilation for the pure Go SQLite engine.
package sql

import (
	"encoding/binary"
	"errors"
	"math"
)

// SQLite Record Format Implementation
//
// A record consists of:
// 1. Header: varint header_size, followed by varint type codes for each column
// 2. Body: column values in sequence
//
// Serial type codes:
//   0: NULL
//   1: 8-bit signed integer
//   2: 16-bit big-endian signed integer
//   3: 24-bit big-endian signed integer
//   4: 32-bit big-endian signed integer
//   5: 48-bit big-endian signed integer
//   6: 64-bit big-endian signed integer
//   7: IEEE 754 float64 (big-endian)
//   8: integer constant 0 (no data stored)
//   9: integer constant 1 (no data stored)
//   10,11: Reserved for internal use
//   N>=12 (even): BLOB of (N-12)/2 bytes
//   N>=13 (odd): TEXT of (N-13)/2 bytes

// SerialType represents a SQLite serial type code
type SerialType uint32

const (
	SerialTypeNull    SerialType = 0
	SerialTypeInt8    SerialType = 1
	SerialTypeInt16   SerialType = 2
	SerialTypeInt24   SerialType = 3
	SerialTypeInt32   SerialType = 4
	SerialTypeInt48   SerialType = 5
	SerialTypeInt64   SerialType = 6
	SerialTypeFloat64 SerialType = 7
	SerialTypeZero    SerialType = 8
	SerialTypeOne     SerialType = 9
)

// Value represents a SQLite value
type Value struct {
	Type   ValueType
	Int    int64
	Float  float64
	Blob   []byte
	Text   string
	IsNull bool
}

// ValueType represents the type of a value
type ValueType int

const (
	TypeNull ValueType = iota
	TypeInteger
	TypeFloat
	TypeText
	TypeBlob
)

// Record represents a SQLite record
type Record struct {
	Values []Value
}

// PutVarint encodes a uint64 as a SQLite varint and appends to buf
// Returns the new buffer
// SQLite varints use 7 bits per byte with continuation bit in high bit
func PutVarint(buf []byte, v uint64) []byte {
	if v <= 0x7f {
		return append(buf, byte(v))
	}
	if v <= 0x3fff {
		return append(buf, byte((v>>7)|0x80), byte(v&0x7f))
	}
	if v <= 0x1fffff {
		return append(buf, byte((v>>14)&0x7f|0x80), byte((v>>7)&0x7f|0x80), byte(v&0x7f))
	}
	if v <= 0xfffffff {
		return append(buf, byte((v>>21)&0x7f|0x80), byte((v>>14)&0x7f|0x80), byte((v>>7)&0x7f|0x80), byte(v&0x7f))
	}
	if v <= 0x7ffffffff {
		return append(buf, byte((v>>28)&0x7f|0x80), byte((v>>21)&0x7f|0x80), byte((v>>14)&0x7f|0x80), byte((v>>7)&0x7f|0x80), byte(v&0x7f))
	}
	if v <= 0x3ffffffffff {
		return append(buf, byte((v>>35)&0x7f|0x80), byte((v>>28)&0x7f|0x80), byte((v>>21)&0x7f|0x80), byte((v>>14)&0x7f|0x80), byte((v>>7)&0x7f|0x80), byte(v&0x7f))
	}
	if v <= 0x1ffffffffffff {
		return append(buf, byte((v>>42)&0x7f|0x80), byte((v>>35)&0x7f|0x80), byte((v>>28)&0x7f|0x80), byte((v>>21)&0x7f|0x80), byte((v>>14)&0x7f|0x80), byte((v>>7)&0x7f|0x80), byte(v&0x7f))
	}
	if v <= 0xffffffffffffff {
		return append(buf, byte((v>>49)&0x7f|0x80), byte((v>>42)&0x7f|0x80), byte((v>>35)&0x7f|0x80), byte((v>>28)&0x7f|0x80), byte((v>>21)&0x7f|0x80), byte((v>>14)&0x7f|0x80), byte((v>>7)&0x7f|0x80), byte(v&0x7f))
	}
	// 9 byte case - first 8 bytes hold 7 bits each, last byte holds 8 bits
	// Shifts: 57, 50, 43, 36, 29, 22, 15, 8, 0
	return append(buf, byte((v>>57)&0x7f|0x80), byte((v>>50)&0x7f|0x80), byte((v>>43)&0x7f|0x80), byte((v>>36)&0x7f|0x80), byte((v>>29)&0x7f|0x80), byte((v>>22)&0x7f|0x80), byte((v>>15)&0x7f|0x80), byte((v>>8)&0x7f|0x80), byte(v))
}

// GetVarint reads a SQLite varint from buf starting at offset
// Returns the value and the number of bytes read
func GetVarint(buf []byte, offset int) (uint64, int) {
	if offset >= len(buf) {
		return 0, 0
	}

	// Fast path for 1-byte case
	if buf[offset] < 0x80 {
		return uint64(buf[offset]), 1
	}

	// Fast path for 2-byte case
	if offset+1 < len(buf) && buf[offset+1] < 0x80 {
		return (uint64(buf[offset]&0x7f) << 7) | uint64(buf[offset+1]), 2
	}

	// General case - decode byte by byte
	var v uint64
	for i := 0; i < 9 && offset+i < len(buf); i++ {
		b := buf[offset+i]
		if i < 8 {
			v = (v << 7) | uint64(b&0x7f)
			if b&0x80 == 0 {
				return v, i + 1
			}
		} else {
			// 9th byte uses all 8 bits
			v = (v << 8) | uint64(b)
			return v, 9
		}
	}
	return v, 0
}

var intRanges = [5]struct {
	lo  int64
	hi  int64
	typ SerialType
}{
	{-128, 127, SerialTypeInt8},
	{-32768, 32767, SerialTypeInt16},
	{-8388608, 8388607, SerialTypeInt24},
	{-2147483648, 2147483647, SerialTypeInt32},
	{-140737488355328, 140737488355327, SerialTypeInt48},
}

func serialTypeForInteger(i int64) SerialType {
	if i == 0 {
		return SerialTypeZero
	}
	if i == 1 {
		return SerialTypeOne
	}
	for _, r := range intRanges {
		if i >= r.lo && i <= r.hi {
			return r.typ
		}
	}
	return SerialTypeInt64
}

// serialTypeHandlers maps each ValueType to its SerialType handler.
var serialTypeHandlers = map[ValueType]func(Value) SerialType{
	TypeNull:    func(_ Value) SerialType { return SerialTypeNull },
	TypeInteger: func(v Value) SerialType { return serialTypeForInteger(v.Int) },
	TypeFloat:   func(_ Value) SerialType { return SerialTypeFloat64 },
	TypeText:    func(v Value) SerialType { return SerialType(13 + 2*len(v.Text)) },
	TypeBlob:    func(v Value) SerialType { return SerialType(12 + 2*len(v.Blob)) },
}

// SerialTypeFor determines the serial type for a value.
func SerialTypeFor(val Value) SerialType {
	if handler, ok := serialTypeHandlers[val.Type]; ok {
		return handler(val)
	}
	return SerialTypeNull
}

// SerialTypeLen returns the number of bytes required to store a value with the given serial type
func SerialTypeLen(serialType SerialType) int {
	switch serialType {
	case SerialTypeNull, SerialTypeZero, SerialTypeOne:
		return 0
	case SerialTypeInt8:
		return 1
	case SerialTypeInt16:
		return 2
	case SerialTypeInt24:
		return 3
	case SerialTypeInt32:
		return 4
	case SerialTypeInt48:
		return 6
	case SerialTypeInt64, SerialTypeFloat64:
		return 8
	default:
		if serialType >= 12 {
			return int(serialType-12) / 2
		}
		return 0
	}
}

// MakeRecord creates a SQLite record from values
func MakeRecord(values []Value) ([]byte, error) {
	if len(values) == 0 {
		return nil, errors.New("cannot create empty record")
	}

	// Calculate serial types and their sizes
	serialTypes := make([]SerialType, len(values))
	serialTypesSize := 0
	bodySize := 0

	for i, val := range values {
		st := SerialTypeFor(val)
		serialTypes[i] = st

		// Each serial type in header is a varint
		serialTypesSize += varintSize(uint64(st))
		bodySize += SerialTypeLen(st)
	}

	// Calculate total header size (includes the header size varint itself)
	// SQLite header size = size of header size varint + size of all serial type varints
	// This is self-referential, so we iterate until stable
	headerSize := serialTypesSize + 1 // Start with 1 byte for header size varint
	for {
		headerSizeVarintLen := varintSize(uint64(headerSize))
		newHeaderSize := headerSizeVarintLen + serialTypesSize
		if newHeaderSize == headerSize {
			break
		}
		headerSize = newHeaderSize
	}

	// Build the record
	buf := make([]byte, 0, headerSize+bodySize)

	// Write header size
	buf = PutVarint(buf, uint64(headerSize))

	// Write serial types
	for _, st := range serialTypes {
		buf = PutVarint(buf, uint64(st))
	}

	// Write body values
	for i, val := range values {
		st := serialTypes[i]
		buf = appendValue(buf, val, st)
	}

	return buf, nil
}

// varintSize returns the number of bytes needed to encode v as a varint
func varintSize(v uint64) int {
	if v <= 0x7f {
		return 1
	}
	if v <= 0x3fff {
		return 2
	}
	if v <= 0x1fffff {
		return 3
	}
	if v <= 0xfffffff {
		return 4
	}
	if v <= 0x7ffffffff {
		return 5
	}
	if v <= 0x3ffffffffff {
		return 6
	}
	if v <= 0x1ffffffffffff {
		return 7
	}
	if v <= 0xffffffffffffff {
		return 8
	}
	return 9
}

// appendValue appends a value to the buffer based on its serial type
func appendValue(buf []byte, val Value, st SerialType) []byte {
	switch st {
	case SerialTypeNull, SerialTypeZero, SerialTypeOne:
		// No data stored
		return buf

	case SerialTypeInt8:
		return append(buf, byte(val.Int))

	case SerialTypeInt16:
		var tmp [2]byte
		binary.BigEndian.PutUint16(tmp[:], uint16(val.Int))
		return append(buf, tmp[:]...)

	case SerialTypeInt24:
		v := uint32(val.Int)
		return append(buf, byte(v>>16), byte(v>>8), byte(v))

	case SerialTypeInt32:
		var tmp [4]byte
		binary.BigEndian.PutUint32(tmp[:], uint32(val.Int))
		return append(buf, tmp[:]...)

	case SerialTypeInt48:
		v := uint64(val.Int)
		return append(buf,
			byte(v>>40), byte(v>>32), byte(v>>24),
			byte(v>>16), byte(v>>8), byte(v))

	case SerialTypeInt64:
		var tmp [8]byte
		binary.BigEndian.PutUint64(tmp[:], uint64(val.Int))
		return append(buf, tmp[:]...)

	case SerialTypeFloat64:
		var tmp [8]byte
		binary.BigEndian.PutUint64(tmp[:], math.Float64bits(val.Float))
		return append(buf, tmp[:]...)

	default:
		// Blob or Text
		if st%2 == 0 {
			// Even: BLOB
			return append(buf, val.Blob...)
		} else {
			// Odd: TEXT
			return append(buf, []byte(val.Text)...)
		}
	}
}

// ParseRecord parses a SQLite record from bytes
func ParseRecord(data []byte) (*Record, error) {
	if len(data) == 0 {
		return nil, errors.New("empty record")
	}

	// Read header size
	headerSize, n := GetVarint(data, 0)
	if n == 0 {
		return nil, errors.New("invalid header size")
	}

	offset := n

	// Read serial types from header
	var serialTypes []SerialType
	for offset < int(headerSize) {
		st, n := GetVarint(data, offset)
		if n == 0 {
			return nil, errors.New("invalid serial type")
		}
		serialTypes = append(serialTypes, SerialType(st))
		offset += n
	}

	// Read values from body
	values := make([]Value, len(serialTypes))
	for i, st := range serialTypes {
		val, n, err := parseValue(data, offset, st)
		if err != nil {
			return nil, err
		}
		values[i] = val
		offset += n
	}

	return &Record{Values: values}, nil
}

// parseZeroWidthConst maps zero-width serial types to their pre-built Values.
var parseZeroWidthConst = map[SerialType]Value{
	SerialTypeNull: {Type: TypeNull, IsNull: true},
	SerialTypeZero: {Type: TypeInteger, Int: 0},
	SerialTypeOne:  {Type: TypeInteger, Int: 1},
}

// parseIntWidth maps serial types Int8–Int64 to their byte widths.
// Index corresponds to serial type value (1–6).
var parseIntWidth = [7]int{0, 1, 2, 3, 4, 6, 8} // index 0 unused

// parseFixedInt decodes a fixed-width big-endian signed integer for serial
// types SerialTypeInt8 through SerialTypeInt64.
func parseFixedInt(data []byte, offset int, st SerialType) (Value, int, error) {
	width := parseIntWidth[st]
	if offset+width > len(data) {
		return Value{}, 0, errors.New("truncated int" + intWidthSuffix(st))
	}
	var v int64
	switch st {
	case SerialTypeInt8:
		v = int64(int8(data[offset]))
	case SerialTypeInt16:
		v = int64(int16(binary.BigEndian.Uint16(data[offset:])))
	case SerialTypeInt24:
		raw := int32(data[offset])<<16 | int32(data[offset+1])<<8 | int32(data[offset+2])
		if raw&0x800000 != 0 {
			raw |= ^0xffffff // sign extend
		}
		v = int64(raw)
	case SerialTypeInt32:
		v = int64(int32(binary.BigEndian.Uint32(data[offset:])))
	case SerialTypeInt48:
		v = int64(data[offset])<<40 | int64(data[offset+1])<<32 |
			int64(data[offset+2])<<24 | int64(data[offset+3])<<16 |
			int64(data[offset+4])<<8 | int64(data[offset+5])
		if v&0x800000000000 != 0 {
			v |= ^0xffffffffffff // sign extend
		}
	default: // SerialTypeInt64
		v = int64(binary.BigEndian.Uint64(data[offset:]))
	}
	return Value{Type: TypeInteger, Int: v}, width, nil
}

// intWidthSuffix returns the bit-width suffix string for error messages.
func intWidthSuffix(st SerialType) string {
	switch st {
	case SerialTypeInt8:
		return "8"
	case SerialTypeInt16:
		return "16"
	case SerialTypeInt24:
		return "24"
	case SerialTypeInt32:
		return "32"
	case SerialTypeInt48:
		return "48"
	default:
		return "64"
	}
}

// parseFloat64 decodes an IEEE 754 float64 from data at offset.
func parseFloat64(data []byte, offset int) (Value, int, error) {
	if offset+8 > len(data) {
		return Value{}, 0, errors.New("truncated float64")
	}
	bits := binary.BigEndian.Uint64(data[offset:])
	return Value{Type: TypeFloat, Float: math.Float64frombits(bits)}, 8, nil
}

// parseBlobOrText decodes a blob (even serial type) or text (odd serial type)
// from data at offset.
func parseBlobOrText(data []byte, offset int, st SerialType) (Value, int, error) {
	length := SerialTypeLen(st)
	if offset+length > len(data) {
		return Value{}, 0, errors.New("truncated blob/text")
	}
	b := make([]byte, length)
	copy(b, data[offset:offset+length])
	if st%2 == 0 {
		return Value{Type: TypeBlob, Blob: b}, length, nil // BLOB
	}
	return Value{Type: TypeText, Text: string(b)}, length, nil // TEXT
}

// parseValue parses a single value from the record body.
func parseValue(data []byte, offset int, st SerialType) (Value, int, error) {
	if v, ok := parseZeroWidthConst[st]; ok {
		return v, 0, nil
	}
	if st >= SerialTypeInt8 && st <= SerialTypeInt64 {
		return parseFixedInt(data, offset, st)
	}
	if st == SerialTypeFloat64 {
		return parseFloat64(data, offset)
	}
	return parseBlobOrText(data, offset, st)
}

// IntValue creates an integer value
func IntValue(i int64) Value {
	return Value{Type: TypeInteger, Int: i}
}

// FloatValue creates a float value
func FloatValue(f float64) Value {
	return Value{Type: TypeFloat, Float: f}
}

// TextValue creates a text value
func TextValue(s string) Value {
	return Value{Type: TypeText, Text: s}
}

// BlobValue creates a blob value
func BlobValue(b []byte) Value {
	return Value{Type: TypeBlob, Blob: b}
}

// NullValue creates a null value
func NullValue() Value {
	return Value{Type: TypeNull, IsNull: true}
}
