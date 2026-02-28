// Package markup provides converters for Bible markup formats to Markdown.
package markup

import (
	"regexp"
	"strings"
)

// OSISConverter converts OSIS XML markup to Markdown.
//
// OSIS (Open Scripture Information Standard) is an XML format for encoding
// Biblical texts with semantic markup for:
//   - Strong's numbers (<w lemma="strong:H430">)
//   - Morphology (<w morph="robinson:V-AAI-3S">)
//   - Notes (<note>)
//   - Cross-references (<reference>)
//   - Divine names (<divineName>)
//   - Poetry (<l>, <lg>)
//   - Titles (<title>)
//   - Red letters (<q who="Jesus">)
type OSISConverter struct {
	// PreserveStrongs keeps Strong's numbers as inline annotations
	PreserveStrongs bool

	// PreserveMorphology keeps morphology codes
	PreserveMorphology bool

	// RedLetterClass is the CSS class for words of Jesus
	RedLetterClass string

	// StrongsFormat controls how Strong's numbers are rendered
	// Options: "inline", "superscript", "tooltip"
	StrongsFormat string
}

// NewOSISConverter creates a converter with default settings.
func NewOSISConverter() *OSISConverter {
	return &OSISConverter{
		PreserveStrongs:    true,
		PreserveMorphology: false,
		RedLetterClass:     "red-letter",
		StrongsFormat:      "superscript",
	}
}

// ConvertResult contains the converted text and extracted annotations.
type ConvertResult struct {
	Text       string
	Strongs    []string
	Morphology []string
	Notes      []string
	HasRedText bool
}

// Convert transforms OSIS XML to Markdown.
func (c *OSISConverter) Convert(osis string) *ConvertResult {
	result := &ConvertResult{
		Strongs:    make([]string, 0),
		Morphology: make([]string, 0),
		Notes:      make([]string, 0),
	}

	text := osis

	// Extract Strong's numbers before removing tags
	if c.PreserveStrongs {
		result.Strongs = c.extractStrongs(text)
	}

	// Extract morphology codes
	if c.PreserveMorphology {
		result.Morphology = c.extractMorphology(text)
	}

	// Extract notes
	result.Notes = c.extractNotes(text)

	// Check for red letter text
	result.HasRedText = strings.Contains(text, `who="Jesus"`) ||
		strings.Contains(text, `marker="Jesus"`)

	// Convert OSIS to Markdown
	text = c.convertToMarkdown(text)

	result.Text = strings.TrimSpace(text)
	return result
}

// extractStrongs extracts Strong's numbers from OSIS word elements.
func (c *OSISConverter) extractStrongs(text string) []string {
	// Pattern: lemma="strong:H430" or lemma="strong:G2316"
	re := regexp.MustCompile(`lemma="strong:([HG]\d+)"`)
	matches := re.FindAllStringSubmatch(text, -1)

	strongs := make([]string, 0, len(matches))
	seen := make(map[string]bool)

	for _, match := range matches {
		if len(match) > 1 && !seen[match[1]] {
			strongs = append(strongs, match[1])
			seen[match[1]] = true
		}
	}

	return strongs
}

// extractMorphology extracts morphology codes from OSIS word elements.
func (c *OSISConverter) extractMorphology(text string) []string {
	// Pattern: morph="robinson:V-AAI-3S" or morph="strongMorph:TH8799"
	re := regexp.MustCompile(`morph="(?:robinson:|strongMorph:)?([^"]+)"`)
	matches := re.FindAllStringSubmatch(text, -1)

	morphs := make([]string, 0, len(matches))
	seen := make(map[string]bool)

	for _, match := range matches {
		if len(match) > 1 && !seen[match[1]] {
			morphs = append(morphs, match[1])
			seen[match[1]] = true
		}
	}

	return morphs
}

// extractNotes extracts note content from OSIS note elements.
func (c *OSISConverter) extractNotes(text string) []string {
	// Pattern: <note>content</note> or <note type="x">content</note>
	re := regexp.MustCompile(`<note[^>]*>([^<]*)</note>`)
	matches := re.FindAllStringSubmatch(text, -1)

	notes := make([]string, 0, len(matches))
	for _, match := range matches {
		if len(match) > 1 && strings.TrimSpace(match[1]) != "" {
			notes = append(notes, strings.TrimSpace(match[1]))
		}
	}

	return notes
}

// convertToMarkdown converts OSIS markup to Markdown.
func (c *OSISConverter) convertToMarkdown(text string) string {
	// Use placeholders for spans we want to preserve
	const divineNamePlaceholder = "\x00DIVINE_NAME_START\x00"
	const divineNameEndPlaceholder = "\x00DIVINE_NAME_END\x00"
	const redLetterPlaceholder = "\x00RED_LETTER_START\x00"
	const redLetterEndPlaceholder = "\x00RED_LETTER_END\x00"

	// Process word elements with Strong's numbers
	if c.PreserveStrongs && c.StrongsFormat == "superscript" {
		// Convert <w lemma="strong:H430">God</w> to God^H430^
		re := regexp.MustCompile(`<w[^>]*lemma="strong:([HG]\d+)"[^>]*>([^<]*)</w>`)
		text = re.ReplaceAllString(text, "$2^$1^")
	} else {
		// Just extract the word content
		re := regexp.MustCompile(`<w[^>]*>([^<]*)</w>`)
		text = re.ReplaceAllString(text, "$1")
	}

	// Convert divine names to placeholders
	re := regexp.MustCompile(`<divineName>([^<]*)</divineName>`)
	text = re.ReplaceAllString(text, divineNamePlaceholder+"$1"+divineNameEndPlaceholder)

	// Convert titles to headings
	re = regexp.MustCompile(`<title[^>]*>([^<]*)</title>`)
	text = re.ReplaceAllString(text, "\n### $1\n")

	// Convert poetry line groups
	re = regexp.MustCompile(`<lg>`)
	text = re.ReplaceAllString(text, "\n")
	re = regexp.MustCompile(`</lg>`)
	text = re.ReplaceAllString(text, "\n")

	// Convert poetry lines with indentation
	re = regexp.MustCompile(`<l level="(\d+)"[^>]*>([^<]*)</l>`)
	text = re.ReplaceAllStringFunc(text, func(match string) string {
		submatches := regexp.MustCompile(`<l level="(\d+)"[^>]*>([^<]*)</l>`).FindStringSubmatch(match)
		if len(submatches) > 2 {
			level := submatches[1]
			content := submatches[2]
			indent := ""
			if level == "2" {
				indent = "    "
			} else if level == "3" {
				indent = "        "
			}
			return indent + content + "\n"
		}
		return match
	})

	// Convert simple poetry lines
	re = regexp.MustCompile(`<l[^>]*>([^<]*)</l>`)
	text = re.ReplaceAllString(text, "$1\n")

	// Convert red letter text to placeholder
	re = regexp.MustCompile(`<q[^>]*who="Jesus"[^>]*>([^<]*)</q>`)
	text = re.ReplaceAllString(text, redLetterPlaceholder+"$1"+redLetterEndPlaceholder)

	// Convert other quotes
	re = regexp.MustCompile(`<q[^>]*>([^<]*)</q>`)
	text = re.ReplaceAllString(text, "\"$1\"")

	// Convert cross-references to links (placeholder)
	re = regexp.MustCompile(`<reference[^>]*osisRef="([^"]*)"[^>]*>([^<]*)</reference>`)
	text = re.ReplaceAllString(text, "[$2]($1)")

	// Remove notes (already extracted)
	re = regexp.MustCompile(`<note[^>]*>[^<]*</note>`)
	text = re.ReplaceAllString(text, "")

	// Remove transChange markers (translator additions)
	re = regexp.MustCompile(`<transChange[^>]*>([^<]*)</transChange>`)
	text = re.ReplaceAllString(text, "*$1*")

	// Remove milestone markers
	re = regexp.MustCompile(`<milestone[^>]*/>`)
	text = re.ReplaceAllString(text, "")

	// Remove verse markers (we handle these separately)
	re = regexp.MustCompile(`<verse[^>]*>`)
	text = re.ReplaceAllString(text, "")
	re = regexp.MustCompile(`</verse>`)
	text = re.ReplaceAllString(text, "")

	// Remove chapter markers
	re = regexp.MustCompile(`<chapter[^>]*>`)
	text = re.ReplaceAllString(text, "")
	re = regexp.MustCompile(`</chapter>`)
	text = re.ReplaceAllString(text, "")

	// Remove any remaining XML tags
	re = regexp.MustCompile(`<[^>]+>`)
	text = re.ReplaceAllString(text, "")

	// Restore placeholders to HTML spans
	text = strings.ReplaceAll(text, divineNamePlaceholder, `<span class="divine-name">`)
	text = strings.ReplaceAll(text, divineNameEndPlaceholder, `</span>`)
	text = strings.ReplaceAll(text, redLetterPlaceholder, `<span class="red-letter">`)
	text = strings.ReplaceAll(text, redLetterEndPlaceholder, `</span>`)

	// Clean up whitespace
	re = regexp.MustCompile(`\s+`)
	text = re.ReplaceAllString(text, " ")

	// Remove leading/trailing whitespace from lines
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimSpace(line)
	}
	text = strings.Join(lines, "\n")

	// Remove multiple blank lines
	re = regexp.MustCompile(`\n{3,}`)
	text = re.ReplaceAllString(text, "\n\n")

	return text
}

// ConvertVerse converts a verse's OSIS text to plain Markdown.
func (c *OSISConverter) ConvertVerse(osis string) string {
	result := c.Convert(osis)
	return result.Text
}

// ConvertWithAnnotations converts OSIS and returns structured data.
func (c *OSISConverter) ConvertWithAnnotations(osis string) *ConvertResult {
	return c.Convert(osis)
}
