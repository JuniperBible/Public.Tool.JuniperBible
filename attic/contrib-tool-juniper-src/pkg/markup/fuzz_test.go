package markup

import (
	"testing"
)

// FuzzOSISConverter tests the OSIS converter with random input
func FuzzOSISConverter(f *testing.F) {
	// Seed corpus with known OSIS patterns
	f.Add("<verse osisID='Gen.1.1'>In the beginning</verse>")
	f.Add("<w lemma='strong:H430'>God</w>")
	f.Add("<note type='study'>A footnote</note>")
	f.Add("<q who='Jesus'>Red letter text</q>")
	f.Add("<divineName>LORD</divineName>")
	f.Add("<title type='chapter'>Chapter 1</title>")
	f.Add("<l>Line of poetry</l>")
	f.Add("<milestone type='x-p'/>")
	f.Add("")
	f.Add("<unclosed>")
	f.Add("<w><w><w>nested</w></w></w>")
	f.Add("<w lemma='strong:H430' morph='robinson:N-ASM'>complex</w>")

	f.Fuzz(func(t *testing.T, input string) {
		conv := NewOSISConverter()
		// Should not panic
		_ = conv.Convert(input)
	})
}

// FuzzThMLConverter tests the ThML converter with random input
func FuzzThMLConverter(f *testing.F) {
	// Seed corpus with known ThML patterns
	f.Add("<scripture>Genesis 1:1</scripture>")
	f.Add("<note>A note</note>")
	f.Add("<b>bold</b>")
	f.Add("<i>italic</i>")
	f.Add("<sup>superscript</sup>")
	f.Add("<sub>subscript</sub>")
	f.Add("<font color='red'>red</font>")
	f.Add("<sync type='Strongs' value='H430'/>God")
	f.Add("<pb/>")
	f.Add("")
	f.Add("<unclosed>")
	f.Add("<b><i><b>nested</b></i></b>")

	f.Fuzz(func(t *testing.T, input string) {
		conv := NewThMLConverter()
		// Should not panic
		_ = conv.Convert(input)
	})
}

// FuzzGBFConverter tests the GBF converter with random input
func FuzzGBFConverter(f *testing.F) {
	// Seed corpus with known GBF patterns
	f.Add("<FR>Red letter<Fr>")
	f.Add("<FI>Italic<Fi>")
	f.Add("<FB>Bold<Fb>")
	f.Add("<FU>Underline<Fu>")
	f.Add("<WH430>word")
	f.Add("<WG2316>word")
	f.Add("<RF>Footnote<Rf>")
	f.Add("<RX>Cross-reference<Rx>")
	f.Add("<CM>")
	f.Add("<CL>")
	f.Add("")
	f.Add("<unclosed")
	f.Add("<FR><FI><FR>nested<Fr><Fi><Fr>")

	f.Fuzz(func(t *testing.T, input string) {
		conv := NewGBFConverter()
		// Should not panic
		_ = conv.Convert(input)
	})
}

// FuzzTEIConverter tests the TEI converter with random input
func FuzzTEIConverter(f *testing.F) {
	// Seed corpus with known TEI patterns
	f.Add("<orth>headword</orth>")
	f.Add("<pron>pronunciation</pron>")
	f.Add("<gramGrp><pos>noun</pos></gramGrp>")
	f.Add("<sense>meaning</sense>")
	f.Add("<etym>etymology</etym>")
	f.Add("<ref target='G2316'>reference</ref>")
	f.Add("<entry><form><orth>test</orth></form></entry>")
	f.Add("")
	f.Add("<unclosed>")
	f.Add("<orth>בְּרֵאשִׁית</orth>")
	f.Add("<orth>Ἐν ἀρχῇ</orth>")

	f.Fuzz(func(t *testing.T, input string) {
		conv := NewTEIConverter()
		// Should not panic
		_ = conv.Convert(input)
	})
}
