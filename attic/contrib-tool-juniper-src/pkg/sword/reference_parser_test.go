package sword

import (
	"testing"
)

func TestParseScriptureRange(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantBook    string
		wantChStart *int
		wantVsStart *int
		wantChEnd   *int
		wantVsEnd   *int
		wantStr     string
		wantErr     bool
	}{
		{
			name:        "full reference",
			input:       "Genesis 1:1",
			wantBook:    "Genesis",
			wantChStart: intPtr(1),
			wantVsStart: intPtr(1),
			wantStr:     "Genesis 1:1",
		},
		{
			name:        "abbreviated book",
			input:       "Gen 1:1",
			wantBook:    "Genesis",
			wantChStart: intPtr(1),
			wantVsStart: intPtr(1),
			wantStr:     "Genesis 1:1",
		},
		{
			name:        "abbreviated with period",
			input:       "Gen. 1:1",
			wantBook:    "Genesis",
			wantChStart: intPtr(1),
			wantVsStart: intPtr(1),
			wantStr:     "Genesis 1:1",
		},
		{
			name:        "dot separator",
			input:       "Gen.1.1",
			wantBook:    "Genesis",
			wantChStart: intPtr(1),
			wantVsStart: intPtr(1),
			wantStr:     "Genesis 1:1",
		},
		{
			name:        "verse range",
			input:       "Genesis 1:1-5",
			wantBook:    "Genesis",
			wantChStart: intPtr(1),
			wantVsStart: intPtr(1),
			wantVsEnd:   intPtr(5),
			wantStr:     "Genesis 1:1-5",
		},
		{
			name:        "chapter range",
			input:       "Genesis 1-2",
			wantBook:    "Genesis",
			wantChStart: intPtr(1),
			wantChEnd:   intPtr(2),
			wantStr:     "Genesis 1-2",
		},
		{
			name:        "full chapter",
			input:       "Genesis 1",
			wantBook:    "Genesis",
			wantChStart: intPtr(1),
			wantStr:     "Genesis 1",
		},
		{
			name:     "full book",
			input:    "Genesis",
			wantBook: "Genesis",
			wantStr:  "Genesis",
		},
		{
			name:        "cross-chapter range",
			input:       "Matthew 5:3-7:29",
			wantBook:    "Matthew",
			wantChStart: intPtr(5),
			wantVsStart: intPtr(3),
			wantChEnd:   intPtr(7),
			wantVsEnd:   intPtr(29),
			wantStr:     "Matthew 5:3-7:29",
		},
		{
			name:        "numbered book",
			input:       "1 John 3:16",
			wantBook:    "1John",
			wantChStart: intPtr(3),
			wantVsStart: intPtr(16),
			wantStr:     "1John 3:16",
		},
		{
			name:        "psalms abbreviation",
			input:       "Ps 23:1",
			wantBook:    "Psalms",
			wantChStart: intPtr(23),
			wantVsStart: intPtr(1),
			wantStr:     "Psalms 23:1",
		},
		{
			name:        "revelation abbreviation",
			input:       "Rev 22:21",
			wantBook:    "Revelation",
			wantChStart: intPtr(22),
			wantVsStart: intPtr(21),
			wantStr:     "Revelation 22:21",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ref, err := ParseScriptureRange(tt.input)

			if (err != nil) != tt.wantErr {
				t.Errorf("ParseScriptureRange() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				return
			}

			if ref.Book != tt.wantBook {
				t.Errorf("Book = %q, want %q", ref.Book, tt.wantBook)
			}

			if !intPtrEqual(ref.ChapterStart, tt.wantChStart) {
				t.Errorf("ChapterStart = %v, want %v", intPtrStr(ref.ChapterStart), intPtrStr(tt.wantChStart))
			}

			if !intPtrEqual(ref.VerseStart, tt.wantVsStart) {
				t.Errorf("VerseStart = %v, want %v", intPtrStr(ref.VerseStart), intPtrStr(tt.wantVsStart))
			}

			if !intPtrEqual(ref.ChapterEnd, tt.wantChEnd) {
				t.Errorf("ChapterEnd = %v, want %v", intPtrStr(ref.ChapterEnd), intPtrStr(tt.wantChEnd))
			}

			if !intPtrEqual(ref.VerseEnd, tt.wantVsEnd) {
				t.Errorf("VerseEnd = %v, want %v", intPtrStr(ref.VerseEnd), intPtrStr(tt.wantVsEnd))
			}

			if got := ref.String(); got != tt.wantStr {
				t.Errorf("String() = %q, want %q", got, tt.wantStr)
			}
		})
	}
}

func TestScriptureRangeIsRange(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"Genesis 1:1", false},
		{"Genesis 1:1-5", true},
		{"Genesis 1-2", true},
		{"Genesis 1", false},
		{"Genesis", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			ref, err := ParseScriptureRange(tt.input)
			if err != nil {
				t.Fatalf("ParseScriptureRange() error = %v", err)
			}

			if got := ref.IsRange(); got != tt.want {
				t.Errorf("IsRange() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestScriptureRangeIsChapterOnly(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"Genesis 1:1", false},
		{"Genesis 1", true},
		{"Genesis 1-2", true},
		{"Genesis", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			ref, err := ParseScriptureRange(tt.input)
			if err != nil {
				t.Fatalf("ParseScriptureRange() error = %v", err)
			}

			if got := ref.IsChapterOnly(); got != tt.want {
				t.Errorf("IsChapterOnly() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestScriptureRangeIsBookOnly(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"Genesis 1:1", false},
		{"Genesis 1", false},
		{"Genesis", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			ref, err := ParseScriptureRange(tt.input)
			if err != nil {
				t.Fatalf("ParseScriptureRange() error = %v", err)
			}

			if got := ref.IsBookOnly(); got != tt.want {
				t.Errorf("IsBookOnly() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNormalizeBookName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Gen", "Genesis"},
		{"Gen.", "Genesis"},
		{"genesis", "Genesis"},
		{"GENESIS", "Genesis"},
		{"1 John", "1John"},
		{"1john", "1John"},
		{"Ps", "Psalms"},
		{"Psa", "Psalms"},
		{"Psalm", "Psalms"},
		{"Rev", "Revelation"},
		{"Matt", "Matthew"},
		{"Mt", "Matthew"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := NormalizeBookName(tt.input); got != tt.want {
				t.Errorf("NormalizeBookName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// Helper functions
func intPtr(i int) *int {
	return &i
}

func intPtrEqual(a, b *int) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

func intPtrStr(p *int) string {
	if p == nil {
		return "nil"
	}
	return string(rune('0' + *p))
}
