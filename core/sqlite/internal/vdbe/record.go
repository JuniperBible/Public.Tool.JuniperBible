package vdbe

import (
	"encoding/binary"
	"fmt"
	"math"
)

// decodeRecord decodes a SQLite record back to a slice of values
func decodeRecord(data []byte) ([]interface{}, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("empty record")
	}

	// Read header size
	headerSize, n := getVarint(data, 0)
	if n == 0 {
		return nil, fmt.Errorf("invalid header size")
	}

	offset := n

	// Read serial types from header
	serialTypes := make([]uint64, 0)
	for offset < int(headerSize) {
		st, n := getVarint(data, offset)
		if n == 0 {
			return nil, fmt.Errorf("invalid serial type at offset %d", offset)
		}
		serialTypes = append(serialTypes, st)
		offset += n
	}

	// Read values from body
	values := make([]interface{}, len(serialTypes))
	for i, st := range serialTypes {
		val, n, err := decodeValue(data, offset, st)
		if err != nil {
			return nil, fmt.Errorf("failed to decode value %d: %w", i, err)
		}
		values[i] = val
		offset += n
	}

	return values, nil
}

// decodeValue decodes a single value from the record body
func decodeValue(data []byte, offset int, serialType uint64) (interface{}, int, error) {
	switch serialType {
	case 0: // NULL
		return nil, 0, nil

	case 8: // integer 0
		return int64(0), 0, nil

	case 9: // integer 1
		return int64(1), 0, nil

	case 1: // int8
		if offset >= len(data) {
			return nil, 0, fmt.Errorf("truncated int8")
		}
		return int64(int8(data[offset])), 1, nil

	case 2: // int16
		if offset+2 > len(data) {
			return nil, 0, fmt.Errorf("truncated int16")
		}
		v := int64(int16(binary.BigEndian.Uint16(data[offset:])))
		return v, 2, nil

	case 3: // int24
		if offset+3 > len(data) {
			return nil, 0, fmt.Errorf("truncated int24")
		}
		v := int32(data[offset])<<16 | int32(data[offset+1])<<8 | int32(data[offset+2])
		if v&0x800000 != 0 {
			v |= ^0xffffff // Sign extend
		}
		return int64(v), 3, nil

	case 4: // int32
		if offset+4 > len(data) {
			return nil, 0, fmt.Errorf("truncated int32")
		}
		v := int64(int32(binary.BigEndian.Uint32(data[offset:])))
		return v, 4, nil

	case 5: // int48
		if offset+6 > len(data) {
			return nil, 0, fmt.Errorf("truncated int48")
		}
		v := int64(data[offset])<<40 | int64(data[offset+1])<<32 |
			int64(data[offset+2])<<24 | int64(data[offset+3])<<16 |
			int64(data[offset+4])<<8 | int64(data[offset+5])
		if v&0x800000000000 != 0 {
			v |= ^0xffffffffffff // Sign extend
		}
		return v, 6, nil

	case 6: // int64
		if offset+8 > len(data) {
			return nil, 0, fmt.Errorf("truncated int64")
		}
		v := int64(binary.BigEndian.Uint64(data[offset:]))
		return v, 8, nil

	case 7: // float64
		if offset+8 > len(data) {
			return nil, 0, fmt.Errorf("truncated float64")
		}
		bits := binary.BigEndian.Uint64(data[offset:])
		v := math.Float64frombits(bits)
		return v, 8, nil

	default:
		// Blob or text
		length := serialTypeLen(serialType)
		if offset+length > len(data) {
			return nil, 0, fmt.Errorf("truncated blob/text")
		}

		b := make([]byte, length)
		copy(b, data[offset:offset+length])

		if serialType%2 == 0 {
			// Even: BLOB
			return b, length, nil
		} else {
			// Odd: TEXT
			return string(b), length, nil
		}
	}
}
