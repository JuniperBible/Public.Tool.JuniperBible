package utf

// Varint encoding and decoding functions following SQLite's format.
//
// SQLite uses a variable-length integer encoding where:
// - 1-9 bytes encode a 64-bit integer
// - High bit of each byte indicates continuation (except byte 9)
// - Format:
//     7 bits - A
//    14 bits - BA
//    21 bits - BBA
//    28 bits - BBBA
//    35 bits - BBBBA
//    42 bits - BBBBBA
//    49 bits - BBBBBBA
//    56 bits - BBBBBBBA
//    64 bits - BBBBBBBBC
//
// Where:
//   A = 0xxxxxxx    7 bits of data and one flag bit
//   B = 1xxxxxxx    7 bits of data and one flag bit
//   C = xxxxxxxx    8 bits of data (no flag bit on 9th byte)

// PutVarint encodes a 64-bit unsigned integer into buf and returns the number of bytes written.
// buf must be at least 9 bytes long.
func PutVarint(buf []byte, v uint64) int {
	// Fast path for small values
	if v <= 0x7f {
		buf[0] = byte(v)
		return 1
	}
	if v <= 0x3fff {
		buf[0] = byte((v>>7)&0x7f) | 0x80
		buf[1] = byte(v & 0x7f)
		return 2
	}

	// Slow path for larger values
	return putVarint64(buf, v)
}

// putVarint64 handles encoding of larger varints.
func putVarint64(buf []byte, v uint64) int {
	// Check if we need 9 bytes (highest bit of high byte is set)
	if v&(uint64(0xff000000)<<32) != 0 {
		buf[8] = byte(v)
		v >>= 8
		for i := 7; i >= 0; i-- {
			buf[i] = byte(v&0x7f) | 0x80
			v >>= 7
		}
		return 9
	}

	// Build varint in reverse
	var temp [10]byte
	n := 0
	for {
		temp[n] = byte(v&0x7f) | 0x80
		n++
		v >>= 7
		if v == 0 {
			break
		}
	}

	// Clear high bit of first byte (least significant)
	temp[0] &= 0x7f

	// Copy to output buffer in correct order
	for i := 0; i < n; i++ {
		buf[i] = temp[n-1-i]
	}

	return n
}

// GetVarint decodes a varint from buf and returns the value and number of bytes read.
// Returns (0, 0) if the buffer is empty.
func GetVarint(buf []byte) (uint64, int) {
	if len(buf) == 0 {
		return 0, 0
	}

	// Fast path for 1-byte varints
	if buf[0] < 0x80 {
		return uint64(buf[0]), 1
	}

	// Fast path for 2-byte varints
	if len(buf) >= 2 && buf[1] < 0x80 {
		return uint64(buf[0]&0x7f)<<7 | uint64(buf[1]), 2
	}

	// Slow path for larger varints
	return getVarintSlow(buf)
}

// getVarintSlow handles decoding of larger varints.
func getVarintSlow(buf []byte) (uint64, int) {
	if len(buf) < 3 {
		// Not enough data
		return 0, 0
	}

	// Constants for bit manipulation
	const slot_2_0 = 0x001fc07f   // (0x7f<<14) | 0x7f
	const slot_4_2_0 = 0xf01fc07f // (0xf<<28) | slot_2_0

	var a, b, s uint32

	a = uint32(buf[0]) << 14
	b = uint32(buf[1])
	a |= uint32(buf[2])

	// 3-byte varint
	if a&0x80 == 0 {
		a &= slot_2_0
		b &= 0x7f
		b = b << 7
		a |= b
		return uint64(a), 3
	}

	if len(buf) < 4 {
		return 0, 0
	}

	// Save state
	a &= slot_2_0

	b = b << 14
	b |= uint32(buf[3])

	// 4-byte varint
	if b&0x80 == 0 {
		b &= slot_2_0
		a = a << 7
		a |= b
		return uint64(a), 4
	}

	if len(buf) < 5 {
		return 0, 0
	}

	b &= slot_2_0
	s = a

	a = a << 14
	a |= uint32(buf[4])

	// 5-byte varint
	if a&0x80 == 0 {
		b = b << 7
		a |= b
		s = s >> 18
		return uint64(s)<<32 | uint64(a), 5
	}

	if len(buf) < 6 {
		return 0, 0
	}

	s = s << 7
	s |= b

	b = b << 14
	b |= uint32(buf[5])

	// 6-byte varint
	if b&0x80 == 0 {
		a &= slot_2_0
		a = a << 7
		a |= b
		s = s >> 18
		return uint64(s)<<32 | uint64(a), 6
	}

	if len(buf) < 7 {
		return 0, 0
	}

	a = a << 14
	a |= uint32(buf[6])

	// 7-byte varint
	if a&0x80 == 0 {
		a &= slot_4_2_0
		b &= slot_2_0
		b = b << 7
		a |= b
		s = s >> 11
		return uint64(s)<<32 | uint64(a), 7
	}

	if len(buf) < 8 {
		return 0, 0
	}

	a &= slot_2_0

	b = b << 14
	b |= uint32(buf[7])

	// 8-byte varint
	if b&0x80 == 0 {
		b &= slot_4_2_0
		a = a << 7
		a |= b
		s = s >> 4
		return uint64(s)<<32 | uint64(a), 8
	}

	if len(buf) < 9 {
		return 0, 0
	}

	// 9-byte varint
	a = a << 15
	a |= uint32(buf[8])

	b &= slot_2_0
	b = b << 8
	a |= b

	s = s << 4
	b = uint32(buf[4])
	b &= 0x7f
	b = b >> 3
	s |= b

	return uint64(s)<<32 | uint64(a), 9
}

// GetVarint32 decodes a varint from buf as a 32-bit value.
// If the varint is larger than 32 bits, returns 0xffffffff.
// Returns (value, bytes_read).
func GetVarint32(buf []byte) (uint32, int) {
	if len(buf) == 0 {
		return 0, 0
	}

	// Fast path for 1-byte varints
	if buf[0] < 0x80 {
		return uint32(buf[0]), 1
	}

	// Fast path for 2-byte varints
	if len(buf) >= 2 && buf[1] < 0x80 {
		return uint32(buf[0]&0x7f)<<7 | uint32(buf[1]), 2
	}

	// Fast path for 3-byte varints
	if len(buf) >= 3 && buf[2] < 0x80 {
		return uint32(buf[0]&0x7f)<<14 | uint32(buf[1]&0x7f)<<7 | uint32(buf[2]), 3
	}

	// Four or more bytes
	v64, n := GetVarint(buf)
	if n > 3 && n <= 9 {
		if v64 > 0xffffffff {
			return 0xffffffff, n
		}
		return uint32(v64), n
	}

	return 0, 0
}

// VarintLen returns the number of bytes needed to encode v as a varint.
func VarintLen(v uint64) int {
	if v <= 0x7f {
		return 1
	}
	if v <= 0x3fff {
		return 2
	}
	// For larger values, check if we need the 9-byte encoding
	if v&(uint64(0xff000000)<<32) != 0 {
		return 9
	}
	// Count how many bytes the normal encoding needs
	i := 1
	for v >>= 7; v != 0; v >>= 7 {
		i++
	}
	return i
}

// Put4Byte encodes a 32-bit big-endian integer into buf.
// buf must be at least 4 bytes long.
func Put4Byte(buf []byte, v uint32) {
	buf[0] = byte(v >> 24)
	buf[1] = byte(v >> 16)
	buf[2] = byte(v >> 8)
	buf[3] = byte(v)
}

// Get4Byte decodes a 32-bit big-endian integer from buf.
func Get4Byte(buf []byte) uint32 {
	return uint32(buf[0])<<24 | uint32(buf[1])<<16 | uint32(buf[2])<<8 | uint32(buf[3])
}

// Put8Byte encodes a 64-bit big-endian integer into buf.
// buf must be at least 8 bytes long.
func Put8Byte(buf []byte, v uint64) {
	buf[0] = byte(v >> 56)
	buf[1] = byte(v >> 48)
	buf[2] = byte(v >> 40)
	buf[3] = byte(v >> 32)
	buf[4] = byte(v >> 24)
	buf[5] = byte(v >> 16)
	buf[6] = byte(v >> 8)
	buf[7] = byte(v)
}

// Get8Byte decodes a 64-bit big-endian integer from buf.
func Get8Byte(buf []byte) uint64 {
	return uint64(buf[0])<<56 | uint64(buf[1])<<48 | uint64(buf[2])<<40 | uint64(buf[3])<<32 |
		uint64(buf[4])<<24 | uint64(buf[5])<<16 | uint64(buf[6])<<8 | uint64(buf[7])
}
