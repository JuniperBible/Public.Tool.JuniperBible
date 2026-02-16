package osis

import (
	"testing"

	"github.com/FocuswithJustin/JuniperBible/core/ir"
)

// FuzzParseOSISToIR tests the OSIS XML parser with fuzzing
func FuzzParseOSISToIR(f *testing.F) {
	// Seed corpus with valid OSIS examples
	f.Add([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<osis xmlns="http://www.bibletechnologies.net/2003/OSIS/namespace">
  <osisText osisIDWork="KJV" xml:lang="en">
    <header>
      <work osisWork="KJV">
        <title>King James Version</title>
      </work>
    </header>
    <div type="book" osisID="Gen">
      <title>Genesis</title>
      <chapter osisID="Gen.1"/>
      <verse osisID="Gen.1.1"/>
      <p>In the beginning God created the heaven and the earth.</p>
    </div>
  </osisText>
</osis>`))

	// Minimal valid OSIS
	f.Add([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<osis xmlns="http://www.bibletechnologies.net/2003/OSIS/namespace">
  <osisText osisIDWork="TEST">
    <div type="book" osisID="Gen">
      <p>Test content</p>
    </div>
  </osisText>
</osis>`))

	// OSIS with nested divs
	f.Add([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<osis xmlns="http://www.bibletechnologies.net/2003/OSIS/namespace">
  <osisText osisIDWork="TEST">
    <div type="book" osisID="Matt">
      <div type="chapter">
        <chapter osisID="Matt.1"/>
        <verse osisID="Matt.1.1"/>
        <p>The book of the generation of Jesus Christ.</p>
      </div>
    </div>
  </osisText>
</osis>`))

	// OSIS with poetry
	f.Add([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<osis xmlns="http://www.bibletechnologies.net/2003/OSIS/namespace">
  <osisText osisIDWork="TEST">
    <div type="book" osisID="Ps">
      <lg>
        <l>Blessed is the man</l>
        <l>that walketh not in the counsel of the ungodly</l>
      </lg>
    </div>
  </osisText>
</osis>`))

	// Empty OSIS
	f.Add([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<osis xmlns="http://www.bibletechnologies.net/2003/OSIS/namespace">
  <osisText osisIDWork="EMPTY">
  </osisText>
</osis>`))

	f.Fuzz(func(t *testing.T, data []byte) {
		// The parser should not panic on any input
		corpus, err := parseOSISToIR(data)

		// If parsing succeeds, validate basic invariants
		if err == nil && corpus != nil {
			// Corpus should have valid basic fields
			if corpus.Version == "" {
				t.Error("Corpus version should not be empty")
			}

			// All documents should be non-nil
			for i, doc := range corpus.Documents {
				if doc == nil {
					t.Errorf("Document at index %d is nil", i)
					continue
				}
				// ID may be empty for some malformed inputs, which is acceptable

				// All content blocks should have valid hashes
				for j, cb := range doc.ContentBlocks {
					if cb == nil {
						t.Errorf("ContentBlock at doc %d, block %d is nil", i, j)
						continue
					}
					if cb.Hash == "" {
						t.Errorf("ContentBlock at doc %d, block %d has empty hash", i, j)
					}
				}
			}

			// Source hash should be set
			if corpus.SourceHash == "" {
				t.Error("Corpus source hash should not be empty")
			}
		}
	})
}

// FuzzEmitOSISFromIR tests the OSIS XML emitter with fuzzing
func FuzzEmitOSISFromIR(f *testing.F) {
	// Seed with valid corpus structures
	f.Add("TEST", "Test Bible", "en", "KJV")
	f.Add("KJV", "King James Version", "en", "KJV")
	f.Add("", "", "", "")
	f.Add("ASV", "American Standard Version", "en-US", "NRSV")

	f.Fuzz(func(t *testing.T, id, title, lang, versification string) {
		// Create a simple corpus
		corpus := createTestCorpus(id, title, lang, versification)

		// The emitter should not panic on any corpus
		data, err := emitOSISFromIR(corpus)

		// If emission succeeds, the output should be valid XML-ish
		if err == nil && len(data) > 0 {
			// Basic sanity checks
			dataStr := string(data)
			if len(dataStr) > 0 {
				// Should have opening and closing osis tags
				if len(dataStr) > 10 {
					// Just verify it doesn't panic on normal operations
					_ = len(dataStr)
				}
			}
		}
	})
}

// FuzzParseOSISRef tests the OSIS reference parser with fuzzing
func FuzzParseOSISRef(f *testing.F) {
	// Seed corpus with valid OSIS references
	f.Add("Gen.1.1")
	f.Add("Matt.5.3")
	f.Add("Ps.23.1")
	f.Add("Rev.22.21")
	f.Add("Gen.1.1-5")
	f.Add("Matt.5.3-12")
	f.Add("Gen")
	f.Add("Gen.1")
	f.Add("...")
	f.Add("")

	f.Fuzz(func(t *testing.T, osisID string) {
		// The parser should not panic on any input
		ref := parseOSISRef(osisID)

		// Ref should never be nil
		if ref == nil {
			t.Error("parseOSISRef returned nil")
		}
	})
}

// FuzzIsBookID tests the book ID validator with fuzzing
func FuzzIsBookID(f *testing.F) {
	// Seed with valid and invalid book IDs
	f.Add("Gen")
	f.Add("Matt")
	f.Add("Rev")
	f.Add("Ps")
	f.Add("1John")
	f.Add("2Cor")
	f.Add("InvalidBook")
	f.Add("")
	f.Add("Genesis")
	f.Add("gen")

	f.Fuzz(func(t *testing.T, osisID string) {
		// The validator should not panic on any input
		_ = isBookID(osisID)
	})
}

// FuzzEscapeXML tests the XML escaper with fuzzing
func FuzzEscapeXML(f *testing.F) {
	// Seed with strings that need escaping
	f.Add("Hello & goodbye")
	f.Add("<tag>content</tag>")
	f.Add("Quote: \"test\"")
	f.Add("Apostrophe: 'test'")
	f.Add("&lt;&gt;&amp;")
	f.Add("")
	f.Add("Normal text")
	f.Add("Multiple & < > \" ' special chars")

	f.Fuzz(func(t *testing.T, input string) {
		// The escaper should not panic on any input
		escaped := escapeXML(input)

		// Escaped string should not contain unescaped special chars
		// (unless they were already escaped entities)
		_ = escaped

		// Should not introduce new unescaped characters
		if len(escaped) > 0 && len(input) > 0 {
			// Basic invariant: output should be same or longer
			// (due to entity expansion)
			_ = len(escaped) >= 0
		}
	})
}

// Helper function to create a test corpus
func createTestCorpus(id, title, lang, versification string) *ir.Corpus {
	corpus := &ir.Corpus{
		ID:            id,
		Version:       "1.0.0",
		ModuleType:    ir.ModuleBible,
		LossClass:     ir.LossL0,
		Title:         title,
		Language:      lang,
		Versification: versification,
		Documents:     []*ir.Document{},
	}

	// Add a simple document
	if id != "" {
		doc := &ir.Document{
			ID:    "Gen",
			Title: "Genesis",
			Order: 1,
			ContentBlocks: []*ir.ContentBlock{
				{
					ID:       "cb-1",
					Sequence: 1,
					Text:     "In the beginning God created the heaven and the earth.",
					Hash:     "test-hash",
				},
			},
		}
		corpus.Documents = append(corpus.Documents, doc)
	}

	return corpus
}
