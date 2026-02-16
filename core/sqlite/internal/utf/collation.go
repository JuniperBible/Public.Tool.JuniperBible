package utf

import "bytes"

// CollationType represents the collation sequence type.
type CollationType int

const (
	// BINARY performs byte-by-byte comparison
	BINARY CollationType = iota

	// NOCASE performs case-insensitive comparison for ASCII characters (A-Z = a-z)
	NOCASE

	// RTRIM ignores trailing spaces during comparison
	RTRIM
)

// Collation represents a collation function.
type Collation struct {
	Type CollationType
	Name string
}

// BuiltinCollations are the standard SQLite collations.
var BuiltinCollations = map[string]Collation{
	"BINARY": {Type: BINARY, Name: "BINARY"},
	"NOCASE": {Type: NOCASE, Name: "NOCASE"},
	"RTRIM":  {Type: RTRIM, Name: "RTRIM"},
}

// Compare compares two strings using the specified collation.
// Returns:
//
//	-1 if a < b
//	 0 if a == b
//	+1 if a > b
func (c Collation) Compare(a, b string) int {
	switch c.Type {
	case BINARY:
		return CompareBinary(a, b)
	case NOCASE:
		return CompareNoCase(a, b)
	case RTRIM:
		return CompareRTrim(a, b)
	default:
		return CompareBinary(a, b)
	}
}

// CompareBytes compares two byte slices using the specified collation.
func (c Collation) CompareBytes(a, b []byte) int {
	switch c.Type {
	case BINARY:
		return bytes.Compare(a, b)
	case NOCASE:
		return CompareNoCaseBytes(a, b)
	case RTRIM:
		return CompareRTrimBytes(a, b)
	default:
		return bytes.Compare(a, b)
	}
}

// CompareBinary performs byte-by-byte comparison.
func CompareBinary(a, b string) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}

// CompareNoCase performs case-insensitive comparison for ASCII characters.
// This matches SQLite's NOCASE collation which only folds ASCII A-Z to a-z.
func CompareNoCase(a, b string) int {
	aBytes := []byte(a)
	bBytes := []byte(b)

	minLen := len(aBytes)
	if len(bBytes) < minLen {
		minLen = len(bBytes)
	}

	for i := 0; i < minLen; i++ {
		ca := UpperToLower[aBytes[i]]
		cb := UpperToLower[bBytes[i]]

		if ca != cb {
			if ca < cb {
				return -1
			}
			return 1
		}
	}

	// If all compared bytes are equal, compare lengths
	if len(aBytes) < len(bBytes) {
		return -1
	}
	if len(aBytes) > len(bBytes) {
		return 1
	}
	return 0
}

// CompareNoCaseBytes performs case-insensitive comparison on byte slices.
func CompareNoCaseBytes(a, b []byte) int {
	minLen := len(a)
	if len(b) < minLen {
		minLen = len(b)
	}

	for i := 0; i < minLen; i++ {
		ca := UpperToLower[a[i]]
		cb := UpperToLower[b[i]]

		if ca != cb {
			if ca < cb {
				return -1
			}
			return 1
		}
	}

	// If all compared bytes are equal, compare lengths
	if len(a) < len(b) {
		return -1
	}
	if len(a) > len(b) {
		return 1
	}
	return 0
}

// CompareRTrim compares strings while ignoring trailing spaces.
func CompareRTrim(a, b string) int {
	// Remove trailing spaces
	a = rtrimSpaces(a)
	b = rtrimSpaces(b)
	return CompareBinary(a, b)
}

// CompareRTrimBytes compares byte slices while ignoring trailing spaces.
func CompareRTrimBytes(a, b []byte) int {
	// Remove trailing spaces
	a = rtrimSpacesBytes(a)
	b = rtrimSpacesBytes(b)
	return bytes.Compare(a, b)
}

// rtrimSpaces removes trailing spaces from a string.
func rtrimSpaces(s string) string {
	i := len(s)
	for i > 0 && s[i-1] == ' ' {
		i--
	}
	return s[:i]
}

// rtrimSpacesBytes removes trailing spaces from a byte slice.
func rtrimSpacesBytes(s []byte) []byte {
	i := len(s)
	for i > 0 && s[i-1] == ' ' {
		i--
	}
	return s[:i]
}

// StrICmp performs case-insensitive string comparison (SQLite's sqlite3StrICmp).
// This is the same as CompareNoCase but follows SQLite's exact implementation.
func StrICmp(a, b string) int {
	if a == "" {
		if b == "" {
			return 0
		}
		return -1
	}
	if b == "" {
		return 1
	}

	aBytes := []byte(a)
	bBytes := []byte(b)

	for i := 0; i < len(aBytes) && i < len(bBytes); i++ {
		ca := aBytes[i]
		cb := bBytes[i]

		if ca == cb {
			if ca == 0 {
				break
			}
			continue
		}

		diff := int(UpperToLower[ca]) - int(UpperToLower[cb])
		if diff != 0 {
			return diff
		}
	}

	// Check if one string is longer
	if len(aBytes) < len(bBytes) {
		return -1
	}
	if len(aBytes) > len(bBytes) {
		return 1
	}
	return 0
}

// StrNICmp performs case-insensitive string comparison up to n bytes.
func StrNICmp(a, b string, n int) int {
	if a == "" {
		if b == "" {
			return 0
		}
		return -1
	}
	if b == "" {
		return 1
	}

	aBytes := []byte(a)
	bBytes := []byte(b)

	for n > 0 && len(aBytes) > 0 && len(bBytes) > 0 {
		if UpperToLower[aBytes[0]] != UpperToLower[bBytes[0]] {
			return int(UpperToLower[aBytes[0]]) - int(UpperToLower[bBytes[0]])
		}
		aBytes = aBytes[1:]
		bBytes = bBytes[1:]
		n--
	}

	if n <= 0 || len(aBytes) == 0 && len(bBytes) == 0 {
		return 0
	}
	if len(aBytes) == 0 {
		return -1
	}
	if len(bBytes) == 0 {
		return 1
	}
	return int(UpperToLower[aBytes[0]]) - int(UpperToLower[bBytes[0]])
}

// StrIHash computes an 8-bit hash of a string that is insensitive to case differences.
func StrIHash(s string) byte {
	var h byte
	for i := 0; i < len(s); i++ {
		h += UpperToLower[s[i]]
	}
	return h
}

// Like implements the SQL LIKE operator.
// pattern is the pattern to match against (may contain % and _ wildcards)
// str is the string to test
// escape is the escape character (0 if none)
func Like(pattern, str string, escape rune) bool {
	return likeImpl([]byte(pattern), []byte(str), escape, true)
}

// LikeCase implements the SQL LIKE operator with case-sensitivity.
func LikeCase(pattern, str string, escape rune) bool {
	return likeImpl([]byte(pattern), []byte(str), escape, false)
}

// likeImpl implements the LIKE matching algorithm.
// If noCase is true, performs case-insensitive matching for ASCII characters.
func likeImpl(pattern, str []byte, escape rune, noCase bool) bool {
	pi := 0 // pattern index
	si := 0 // string index

	for pi < len(pattern) {
		pc, psize := DecodeRune(pattern[pi:])
		if psize == 0 {
			break
		}

		// Handle escape sequences
		if escape != 0 && pc == escape {
			pi += psize
			if pi >= len(pattern) {
				// Escape at end of pattern
				return false
			}
			pc, psize = DecodeRune(pattern[pi:])
		} else if pc == '%' {
			// Match zero or more characters
			pi += psize

			// % at end matches everything
			if pi >= len(pattern) {
				return true
			}

			// Try matching at each position
			for si <= len(str) {
				if likeImpl(pattern[pi:], str[si:], escape, noCase) {
					return true
				}
				if si >= len(str) {
					break
				}
				_, ssize := DecodeRune(str[si:])
				if ssize == 0 {
					break
				}
				si += ssize
			}
			return false
		} else if pc == '_' {
			// Match exactly one character
			if si >= len(str) {
				return false
			}
			_, ssize := DecodeRune(str[si:])
			if ssize == 0 {
				return false
			}
			si += ssize
			pi += psize
			continue
		}

		// Regular character matching
		if si >= len(str) {
			return false
		}

		sc, ssize := DecodeRune(str[si:])
		if ssize == 0 {
			return false
		}

		// Compare characters
		if noCase {
			pc = ToLower(pc)
			sc = ToLower(sc)
		}

		if pc != sc {
			return false
		}

		pi += psize
		si += ssize
	}

	// Pattern exhausted, string should also be exhausted
	return si >= len(str)
}

// Glob implements the SQL GLOB operator.
// pattern is the pattern to match against (may contain * and ? wildcards)
// str is the string to test
func Glob(pattern, str string) bool {
	return globImpl([]byte(pattern), []byte(str))
}

// globState holds the state for glob pattern matching.
type globState struct {
	pattern []byte
	str     []byte
	pi      int // pattern index
	si      int // string index
}

// globImpl implements the GLOB matching algorithm (case-sensitive).
func globImpl(pattern, str []byte) bool {
	state := &globState{pattern: pattern, str: str}
	return state.match()
}

// match performs the main glob matching loop.
func (g *globState) match() bool {
	for g.pi < len(g.pattern) {
		pc, psize := DecodeRune(g.pattern[g.pi:])
		if psize == 0 {
			break
		}

		switch pc {
		case '*':
			return g.matchStar(psize)
		case '?':
			if !g.matchQuestion(psize) {
				return false
			}
		case '[':
			if !g.matchCharClass(psize) {
				return false
			}
		default:
			if !g.matchLiteral(pc, psize) {
				return false
			}
		}
	}
	return g.si >= len(g.str)
}

// matchStar handles the '*' wildcard (zero or more characters).
func (g *globState) matchStar(psize int) bool {
	g.pi += psize
	if g.pi >= len(g.pattern) {
		return true
	}
	for g.si <= len(g.str) {
		if globImpl(g.pattern[g.pi:], g.str[g.si:]) {
			return true
		}
		if g.si >= len(g.str) {
			break
		}
		_, ssize := DecodeRune(g.str[g.si:])
		if ssize == 0 {
			break
		}
		g.si += ssize
	}
	return false
}

// matchQuestion handles the '?' wildcard (exactly one character).
func (g *globState) matchQuestion(psize int) bool {
	if g.si >= len(g.str) {
		return false
	}
	_, ssize := DecodeRune(g.str[g.si:])
	if ssize == 0 {
		return false
	}
	g.si += ssize
	g.pi += psize
	return true
}

// matchCharClass handles '[...]' character classes.
func (g *globState) matchCharClass(psize int) bool {
	g.pi += psize
	if g.pi >= len(g.pattern) || g.si >= len(g.str) {
		return false
	}

	sc, ssize := DecodeRune(g.str[g.si:])
	if ssize == 0 {
		return false
	}

	invert, matched := g.parseCharClass(sc)
	if invert {
		matched = !matched
	}
	if !matched {
		return false
	}
	g.si += ssize
	return true
}

// parseCharClass parses a character class and checks if sc matches.
func (g *globState) parseCharClass(sc rune) (invert, matched bool) {
	if g.pi < len(g.pattern) && g.pattern[g.pi] == '^' {
		invert = true
		g.pi++
	}

	for g.pi < len(g.pattern) {
		cc, csize := DecodeRune(g.pattern[g.pi:])
		if csize == 0 {
			break
		}
		g.pi += csize

		if cc == ']' {
			break
		}

		if g.isCharRange() {
			g.pi++ // skip '-'
			cc2, csize2 := DecodeRune(g.pattern[g.pi:])
			if csize2 == 0 {
				break
			}
			g.pi += csize2
			if sc >= cc && sc <= cc2 {
				matched = true
			}
		} else if sc == cc {
			matched = true
		}
	}
	return invert, matched
}

// isCharRange checks if the current position is a character range (a-z).
func (g *globState) isCharRange() bool {
	return g.pi < len(g.pattern) && g.pattern[g.pi] == '-' && g.pi+1 < len(g.pattern)
}

// matchLiteral handles literal character matching.
func (g *globState) matchLiteral(pc rune, psize int) bool {
	if g.si >= len(g.str) {
		return false
	}
	sc, ssize := DecodeRune(g.str[g.si:])
	if ssize == 0 || pc != sc {
		return false
	}
	g.pi += psize
	g.si += ssize
	return true
}
