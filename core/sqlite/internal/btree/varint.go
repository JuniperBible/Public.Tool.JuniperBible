package btree

// Variable-length integer encoding/decoding (SQLite format)
// Based on SQLite's varint implementation

// PutVarint writes a 64-bit unsigned integer to p and returns the number of bytes written.
// The integer is encoded as a variable-length integer using SQLite's encoding:
// - Lower 7 bits of each byte are used for data
// - High bit (0x80) set on all bytes except the last
// - Most significant byte first (big-endian)
// - Maximum of 9 bytes (last byte uses all 8 bits)
func PutVarint(p []byte, v uint64) int {
	if v <= 0x7f {
		p[0] = byte(v & 0x7f)
		return 1
	}
	if v <= 0x3fff {
		p[0] = byte((v>>7)&0x7f) | 0x80
		p[1] = byte(v & 0x7f)
		return 2
	}
	return putVarint64(p, v)
}

// putVarint64 handles the general case of encoding a 64-bit varint
func putVarint64(p []byte, v uint64) int {
	if v&(uint64(0xff000000)<<32) != 0 {
		// 9-byte case: all 8 bits of the 9th byte are used
		p[8] = byte(v)
		v >>= 8
		for i := 7; i >= 0; i-- {
			p[i] = byte((v & 0x7f) | 0x80)
			v >>= 7
		}
		return 9
	}

	// Build varint in forward order
	// Count how many 7-bit groups we need
	n := 0
	temp := v
	for {
		n++
		temp >>= 7
		if temp == 0 {
			break
		}
	}

	// Encode from most significant to least significant
	for i := n - 1; i >= 0; i-- {
		shift := uint(i * 7)
		b := byte((v >> shift) & 0x7f)
		if i > 0 {
			b |= 0x80 // Set continuation bit for all except last byte
		}
		p[n-1-i] = b
	}
	return n
}

// GetVarint reads a 64-bit variable-length integer from p and returns
// the value and the number of bytes read.
func GetVarint(p []byte) (uint64, int) {
	// Fast path for 1-byte case
	if p[0] < 0x80 {
		return uint64(p[0]), 1
	}

	// Fast path for 2-byte case
	if len(p) > 1 && p[1] < 0x80 {
		return (uint64(p[0]&0x7f) << 7) | uint64(p[1]), 2
	}

	// General case
	if len(p) < 2 {
		return 0, 0
	}

	// Save original slice for 9-byte case
	orig := p

	const SLOT_2_0 = 0x001fc07f       // (0x7f<<14) | 0x7f
	const SLOT_4_2_0 = 0xf01fc07f     // (0xf<<28) | (0x7f<<14) | 0x7f

	a := uint32(p[0]) << 14
	b := uint32(p[1])
	p = p[2:]
	a |= uint32(p[0])
	// a: p0<<14 | p2 (unmasked)

	if a&0x80 == 0 {
		// 3-byte case
		a &= SLOT_2_0
		b &= 0x7f
		b = b << 7
		a |= b
		return uint64(a), 3
	}

	// 4-byte or larger
	if len(p) < 2 {
		return 0, 0
	}
	b = (b & 0x7f) << 14
	b |= uint32(p[1])
	// b: p1<<14 | p3 (unmasked)

	if b&0x80 == 0 {
		// 4-byte case
		b &= SLOT_2_0
		a &= SLOT_2_0
		a = a << 7
		a |= b
		return uint64(a), 4
	}

	// 5-byte or larger
	if len(p) < 3 {
		return 0, 0
	}
	p = p[2:]
	a = a << 14
	a |= uint32(p[0])
	// a: p0<<28 | p2<<14 | p4 (unmasked)

	if a&0x80 == 0 {
		// 5-byte case
		b &= SLOT_2_0
		a &= SLOT_4_2_0
		b = b << 7
		a |= b
		return uint64(a), 5
	}

	// 6-byte or larger
	if len(p) < 2 {
		return 0, 0
	}
	b = b << 14
	b |= uint32(p[1])
	// b: p1<<28 | p3<<14 | p5 (unmasked)

	if b&0x80 == 0 {
		// 6-byte case
		b &= SLOT_4_2_0
		a &= SLOT_2_0
		a = a << 7
		b |= a
		return uint64(b), 6
	}

	// 7-byte or larger
	if len(p) < 3 {
		return 0, 0
	}
	p = p[2:]
	a = a << 14
	a |= uint32(p[0])
	// a: p2<<28 | p4<<14 | p6 (unmasked)

	if a&0x80 == 0 {
		// 7-byte case
		a &= SLOT_4_2_0
		b &= SLOT_2_0
		b = b << 7
		a |= b
		return uint64(a), 7
	}

	// 8-byte or larger
	if len(p) < 2 {
		return 0, 0
	}
	b = b << 15
	b |= uint32(p[1]) << 8
	// b: p1<<29 | p3<<15 | p5<<8 (unmasked)

	if len(p) < 3 {
		return 0, 0
	}
	b |= uint32(p[2])
	// b: p1<<29 | p3<<15 | p5<<8 | p7 (unmasked)

	if b&0x80 == 0 {
		// 8-byte case
		b &= (0x1f << 24) | (0x7f << 16) | (0x7f << 8) | 0x7f
		a &= SLOT_2_0
		a = a << 8
		a |= b
		return uint64(a), 8
	}

	// 9-byte case: last byte uses all 8 bits (no continuation bit)
	// Decode the first 8 bytes (each contributing 7 bits) and the 9th byte (8 bits)
	var v uint64
	v = uint64(orig[0]&0x7f) << 57
	v |= uint64(orig[1]&0x7f) << 50
	v |= uint64(orig[2]&0x7f) << 43
	v |= uint64(orig[3]&0x7f) << 36
	v |= uint64(orig[4]&0x7f) << 29
	v |= uint64(orig[5]&0x7f) << 22
	v |= uint64(orig[6]&0x7f) << 15
	v |= uint64(orig[7]&0x7f) << 8
	v |= uint64(orig[8]) // All 8 bits
	return v, 9
}

// GetVarint32 reads a 32-bit variable-length integer from p and returns
// the value and the number of bytes read. If the varint is larger than
// 32 bits, it returns 0xffffffff.
func GetVarint32(p []byte) (uint32, int) {
	// Fast path for 1-byte case
	if len(p) > 0 && p[0] < 0x80 {
		return uint32(p[0]), 1
	}

	// Fast path for 2-byte case
	if len(p) > 1 && p[1] < 0x80 {
		return (uint32(p[0]&0x7f) << 7) | uint32(p[1]), 2
	}

	// Fast path for 3-byte case
	if len(p) > 2 && p[2] < 0x80 {
		return (uint32(p[0]&0x7f) << 14) | (uint32(p[1]&0x7f) << 7) | uint32(p[2]), 3
	}

	// Use full 64-bit decoder
	v64, n := GetVarint(p)
	if n > 3 && n <= 9 {
		if v64 > 0xffffffff {
			return 0xffffffff, n
		}
		return uint32(v64), n
	}
	return 0, 0
}

// VarintLen returns the number of bytes required to encode v as a varint
func VarintLen(v uint64) int {
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
