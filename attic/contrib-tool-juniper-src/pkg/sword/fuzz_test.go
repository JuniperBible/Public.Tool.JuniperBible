package sword

import (
	"testing"
)

// FuzzParseAboutText tests the RTF escape parser with random input
func FuzzParseAboutText(f *testing.F) {
	// Seed corpus with known RTF patterns
	f.Add("Plain text")
	f.Add("Text with \\par paragraph")
	f.Add("\\qc Centered \\qc")
	f.Add("\\pard Normal paragraph")
	f.Add("Multiple \\par breaks \\par here")
	f.Add("\\par\\par\\par consecutive")
	f.Add("")
	f.Add("\\unknown escape")
	f.Add("\\\\backslash")
	f.Add("Very long text " + string(make([]byte, 1000)))

	f.Fuzz(func(t *testing.T, input string) {
		// Should not panic
		_ = parseAboutText(input)
	})
}

// FuzzNormalizeBookID tests book ID normalization with random input
func FuzzNormalizeBookID(f *testing.F) {
	// Seed corpus with known book patterns
	f.Add("Gen")
	f.Add("Genesis")
	f.Add("gen")
	f.Add("GENESIS")
	f.Add("Matt")
	f.Add("Matthew")
	f.Add("1Cor")
	f.Add("1 Corinthians")
	f.Add("Ps")
	f.Add("Psalms")
	f.Add("")
	f.Add("NotABook")
	f.Add("123")
	f.Add("בְּרֵאשִׁית")

	f.Fuzz(func(t *testing.T, input string) {
		// Should not panic
		_, _ = GetBookInfo(input)
	})
}

// FuzzTruncateDescription tests description truncation with random input
func FuzzTruncateDescription(f *testing.F) {
	// Seed corpus with various string lengths
	f.Add("Short")
	f.Add("A medium length description that should not be truncated.")
	f.Add("A very long description that contains many words and should definitely be truncated at some point because it exceeds the maximum length that we want to display in the user interface and needs to be shortened appropriately with ellipsis.")
	f.Add("")
	f.Add("No spaces")
	f.Add("     leading spaces")
	f.Add("trailing spaces     ")
	f.Add("multiple   spaces   between   words")
	f.Add(string(make([]byte, 500)))

	f.Fuzz(func(t *testing.T, input string) {
		// Should not panic
		_ = truncateDescription(input, 100)
	})
}
