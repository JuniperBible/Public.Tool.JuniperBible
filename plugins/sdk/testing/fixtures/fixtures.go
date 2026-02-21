// Package fixtures provides shared test data for SDK-based plugins.
// This eliminates duplicated test data creation across format plugins.
package fixtures

import (
	"github.com/JuniperBible/Public.Tool.JuniperBible/plugins/sdk/ir"
)

// MinimalBible returns a minimal Bible corpus with only Genesis 1:1.
// Useful for basic functionality testing.
func MinimalBible() *ir.Corpus {
	corpus := ir.NewCorpus("test-minimal", "BIBLE", "en")
	corpus.Title = "Minimal Test Bible"
	corpus.Version = "1.0.0"
	corpus.Publisher = "Test Publisher"
	corpus.LossClass = "L0"

	// Create Genesis with verse 1:1
	doc := ir.NewDocument("Gen", "Genesis", 1)
	cb := ir.NewContentBlock("cb-1", 1, "In the beginning God created the heavens and the earth.")

	// Add verse anchor and span
	cb.Anchors = []*ir.Anchor{
		{
			ID:       "a-1-0",
			Position: 0,
			Spans: []*ir.Span{
				{
					ID:            "s-Gen.1.1",
					Type:          "VERSE",
					StartAnchorID: "a-1-0",
					Ref: &ir.Ref{
						Book:    "Gen",
						Chapter: 1,
						Verse:   1,
						OSISID:  "Gen.1.1",
					},
				},
			},
		},
	}

	ir.AddContentBlock(doc, cb)
	ir.AddDocument(corpus, doc)

	return corpus
}

// SampleBible returns a sample Bible corpus with multiple books and verses.
// Includes Genesis 1:1-3, Psalm 23:1, and John 3:16.
func SampleBible() *ir.Corpus {
	corpus := ir.NewCorpus("test-sample", "BIBLE", "en")
	corpus.Title = "Sample Test Bible"
	corpus.Version = "1.0.0"
	corpus.Publisher = "Test Publisher"
	corpus.Versification = "KJV"
	corpus.LossClass = "L0"

	// Genesis 1:1-3
	gen := ir.NewDocument("Gen", "Genesis", 1)

	cb1 := ir.NewContentBlock("cb-1", 1, "In the beginning God created the heavens and the earth.")
	cb1.Anchors = []*ir.Anchor{
		{
			ID:       "a-1-0",
			Position: 0,
			Spans: []*ir.Span{
				{
					ID:            "s-Gen.1.1",
					Type:          "VERSE",
					StartAnchorID: "a-1-0",
					Ref: &ir.Ref{
						Book:    "Gen",
						Chapter: 1,
						Verse:   1,
						OSISID:  "Gen.1.1",
					},
				},
			},
		},
	}
	ir.AddContentBlock(gen, cb1)

	cb2 := ir.NewContentBlock("cb-2", 2, "And the earth was without form and void; and darkness was upon the face of the deep.")
	cb2.Anchors = []*ir.Anchor{
		{
			ID:       "a-2-0",
			Position: 0,
			Spans: []*ir.Span{
				{
					ID:            "s-Gen.1.2",
					Type:          "VERSE",
					StartAnchorID: "a-2-0",
					Ref: &ir.Ref{
						Book:    "Gen",
						Chapter: 1,
						Verse:   2,
						OSISID:  "Gen.1.2",
					},
				},
			},
		},
	}
	ir.AddContentBlock(gen, cb2)

	cb3 := ir.NewContentBlock("cb-3", 3, "And God said, Let there be light: and there was light.")
	cb3.Anchors = []*ir.Anchor{
		{
			ID:       "a-3-0",
			Position: 0,
			Spans: []*ir.Span{
				{
					ID:            "s-Gen.1.3",
					Type:          "VERSE",
					StartAnchorID: "a-3-0",
					Ref: &ir.Ref{
						Book:    "Gen",
						Chapter: 1,
						Verse:   3,
						OSISID:  "Gen.1.3",
					},
				},
			},
		},
	}
	ir.AddContentBlock(gen, cb3)
	ir.AddDocument(corpus, gen)

	// Psalm 23:1
	ps := ir.NewDocument("Ps", "Psalms", 19)
	cb4 := ir.NewContentBlock("cb-4", 1, "The LORD is my shepherd; I shall not want.")
	cb4.Anchors = []*ir.Anchor{
		{
			ID:       "a-4-0",
			Position: 0,
			Spans: []*ir.Span{
				{
					ID:            "s-Ps.23.1",
					Type:          "VERSE",
					StartAnchorID: "a-4-0",
					Ref: &ir.Ref{
						Book:    "Ps",
						Chapter: 23,
						Verse:   1,
						OSISID:  "Ps.23.1",
					},
				},
			},
		},
	}
	ir.AddContentBlock(ps, cb4)
	ir.AddDocument(corpus, ps)

	// John 3:16
	john := ir.NewDocument("John", "John", 43)
	cb5 := ir.NewContentBlock("cb-5", 1, "For God so loved the world, that he gave his only begotten Son, that whosoever believeth in him should not perish, but have everlasting life.")
	cb5.Anchors = []*ir.Anchor{
		{
			ID:       "a-5-0",
			Position: 0,
			Spans: []*ir.Span{
				{
					ID:            "s-John.3.16",
					Type:          "VERSE",
					StartAnchorID: "a-5-0",
					Ref: &ir.Ref{
						Book:    "John",
						Chapter: 3,
						Verse:   16,
						OSISID:  "John.3.16",
					},
				},
			},
		},
	}
	ir.AddContentBlock(john, cb5)
	ir.AddDocument(corpus, john)

	return corpus
}

// BibleJSON returns a sample JSON Bible format string.
// Includes Genesis 1:1-2 in the custom JSON Bible schema.
func BibleJSON() string {
	return `{
  "meta": {
    "id": "test",
    "title": "Test Bible",
    "version": "1.0.0",
    "language": "en",
    "publisher": "Test Publisher"
  },
  "books": [
    {
      "id": "Gen",
      "name": "Genesis",
      "order": 1,
      "chapters": [
        {
          "number": 1,
          "verses": [
            {
              "book": "Gen",
              "chapter": 1,
              "verse": 1,
              "text": "In the beginning God created the heavens and the earth.",
              "id": "Gen.1.1"
            },
            {
              "book": "Gen",
              "chapter": 1,
              "verse": 2,
              "text": "And the earth was without form and void.",
              "id": "Gen.1.2"
            }
          ]
        }
      ]
    }
  ]
}`
}

// BibleOSIS returns a sample OSIS XML Bible format string.
// Includes Genesis 1:1-2 in OSIS format.
func BibleOSIS() string {
	return `<?xml version="1.0" encoding="UTF-8"?>
<osis xmlns="http://www.bibletechnologies.net/2003/OSIS/namespace">
  <osisText osisIDWork="TEST" osisRefWork="Bible" xml:lang="en">
    <header>
      <work osisWork="TEST">
        <title>Test Bible</title>
        <identifier type="OSIS">TEST</identifier>
        <refSystem>Bible</refSystem>
      </work>
    </header>
    <div type="book" osisID="Gen">
      <title type="main">Genesis</title>
      <chapter osisID="Gen.1">
        <verse osisID="Gen.1.1">In the beginning God created the heavens and the earth.</verse>
        <verse osisID="Gen.1.2">And the earth was without form and void.</verse>
      </chapter>
    </div>
  </osisText>
</osis>`
}

// BibleUSFM returns a sample USFM Bible format string.
// Includes Genesis 1:1-2 in USFM format.
func BibleUSFM() string {
	return `\id GEN
\h Genesis
\mt Genesis
\c 1
\p
\v 1 In the beginning God created the heavens and the earth.
\v 2 And the earth was without form and void.
`
}

// BiblePlainText returns a sample plain text Bible format.
// Simple reference:text format.
func BiblePlainText() string {
	return `Gen 1:1 In the beginning God created the heavens and the earth.
Gen 1:2 And the earth was without form and void.
`
}

// BibleMarkdown returns a sample Markdown Bible format.
func BibleMarkdown() string {
	return `# Genesis

## Chapter 1

**1** In the beginning God created the heavens and the earth.

**2** And the earth was without form and void.
`
}

// EmptyCorpus returns an empty corpus with minimal metadata.
// Useful for testing error cases or emit-native with no content.
func EmptyCorpus() *ir.Corpus {
	corpus := ir.NewCorpus("test-empty", "BIBLE", "en")
	corpus.Title = "Empty Test Bible"
	corpus.Version = "1.0.0"
	return corpus
}

// CorpusWithMetadata returns a corpus with rich metadata but no content.
// Useful for testing metadata preservation.
func CorpusWithMetadata() *ir.Corpus {
	corpus := ir.NewCorpus("test-metadata", "BIBLE", "en")
	corpus.Title = "Metadata Test Bible"
	corpus.Version = "2.1.0"
	corpus.Publisher = "Test Publisher Inc."
	corpus.Rights = "Public Domain"
	corpus.Description = "A test Bible with rich metadata"
	corpus.Versification = "NRSV"
	corpus.LossClass = "L1"
	corpus.Attributes = map[string]string{
		"copyright_year": "2024",
		"edition":        "First Edition",
		"translator":     "Test Translator",
	}
	return corpus
}
