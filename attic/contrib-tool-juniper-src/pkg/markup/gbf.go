// Package markup provides converters for Bible markup formats to Markdown.
package markup

import (
	"regexp"
	"strings"
)

// GBFConverter converts GBF (General Bible Format) to Markdown.
//
// GBF is an older SWORD markup format that uses angle-bracket codes:
//   - <FR>...<Fr> - Red letter (words of Jesus)
//   - <FI>...<Fi> - Italic
//   - <FB>...<Fb> - Bold
//   - <FO>...<Fo> - Old Testament quote
//   - <FS>...<Fs> - Superscript
//   - <FU>...<Fu> - Underline
//   - <RF>...<Rf> - Footnote
//   - <RX>...<Rx> - Cross-reference
//   - <WH>...<Wh> - Hebrew word with Strong's
//   - <WG>...<Wg> - Greek word with Strong's
//   - <WT>...<Wt> - Morphology tag
//   - <CM> - Paragraph/section mark
//   - <CL> - Line break
//   - <CI> - Indent
type GBFConverter struct {
	// PreserveStrongs keeps Strong's numbers as annotations
	PreserveStrongs bool

	// PreserveMorphology keeps morphology codes
	PreserveMorphology bool
}

// NewGBFConverter creates a converter with default settings.
func NewGBFConverter() *GBFConverter {
	return &GBFConverter{
		PreserveStrongs:    true,
		PreserveMorphology: false,
	}
}

// GBFResult contains the converted text and extracted metadata.
type GBFResult struct {
	Text       string
	Strongs    []string
	Morphology []string
	Footnotes  []string
	CrossRefs  []string
	HasRedText bool
}

// Convert transforms GBF markup to Markdown.
func (c *GBFConverter) Convert(gbf string) *GBFResult {
	result := &GBFResult{
		Strongs:    make([]string, 0),
		Morphology: make([]string, 0),
		Footnotes:  make([]string, 0),
		CrossRefs:  make([]string, 0),
	}

	text := gbf

	// Extract Strong's numbers
	if c.PreserveStrongs {
		result.Strongs = c.extractStrongs(text)
	}

	// Extract morphology
	if c.PreserveMorphology {
		result.Morphology = c.extractMorphology(text)
	}

	// Extract footnotes
	result.Footnotes = c.extractFootnotes(text)

	// Extract cross-references
	result.CrossRefs = c.extractCrossRefs(text)

	// Check for red letter text
	result.HasRedText = strings.Contains(text, "<FR>")

	// Convert to Markdown
	text = c.convertToMarkdown(text)

	result.Text = strings.TrimSpace(text)
	return result
}

// extractStrongs extracts Strong's numbers from GBF word tags.
func (c *GBFConverter) extractStrongs(text string) []string {
	strongs := make([]string, 0)
	seen := make(map[string]bool)

	// Hebrew: <WH1234> or <WHxxxx>
	reH := regexp.MustCompile(`<WH(\d+)>`)
	matchesH := reH.FindAllStringSubmatch(text, -1)
	for _, match := range matchesH {
		if len(match) > 1 {
			num := "H" + match[1]
			if !seen[num] {
				strongs = append(strongs, num)
				seen[num] = true
			}
		}
	}

	// Greek: <WG1234> or <WGxxxx>
	reG := regexp.MustCompile(`<WG(\d+)>`)
	matchesG := reG.FindAllStringSubmatch(text, -1)
	for _, match := range matchesG {
		if len(match) > 1 {
			num := "G" + match[1]
			if !seen[num] {
				strongs = append(strongs, num)
				seen[num] = true
			}
		}
	}

	return strongs
}

// extractMorphology extracts morphology codes from GBF.
func (c *GBFConverter) extractMorphology(text string) []string {
	re := regexp.MustCompile(`<WT([^>]+)>`)
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

// extractFootnotes extracts footnote content from GBF.
func (c *GBFConverter) extractFootnotes(text string) []string {
	re := regexp.MustCompile(`<RF>([^<]*)<Rf>`)
	matches := re.FindAllStringSubmatch(text, -1)

	notes := make([]string, 0, len(matches))
	for _, match := range matches {
		if len(match) > 1 && strings.TrimSpace(match[1]) != "" {
			notes = append(notes, strings.TrimSpace(match[1]))
		}
	}

	return notes
}

// extractCrossRefs extracts cross-reference content from GBF.
func (c *GBFConverter) extractCrossRefs(text string) []string {
	re := regexp.MustCompile(`<RX>([^<]*)<Rx>`)
	matches := re.FindAllStringSubmatch(text, -1)

	refs := make([]string, 0, len(matches))
	for _, match := range matches {
		if len(match) > 1 && strings.TrimSpace(match[1]) != "" {
			refs = append(refs, strings.TrimSpace(match[1]))
		}
	}

	return refs
}

// regexReplacement pairs a compiled pattern with its replacement string.
type regexReplacement struct {
	re          *regexp.Regexp
	replacement string
}

// strongsReplacements returns the regex replacements for Strong's numbers.
// When preserve is true the numbers are annotated; otherwise they are stripped.
func strongsReplacements(preserve bool) []regexReplacement {
	if preserve {
		return []regexReplacement{
			{regexp.MustCompile(`<WH(\d+)>([^<]*)<Wh>`), "$2^H$1^"}, // Hebrew Strong's: annotate
			{regexp.MustCompile(`<WG(\d+)>([^<]*)<Wg>`), "$2^G$1^"}, // Greek Strong's: annotate
		}
	}
	return []regexReplacement{
		{regexp.MustCompile(`<WH\d+>([^<]*)<Wh>`), "$1"}, // Hebrew Strong's: strip tags
		{regexp.MustCompile(`<WG\d+>([^<]*)<Wg>`), "$1"}, // Greek Strong's: strip tags
	}
}

// normalizeWhitespace collapses runs of spaces/tabs, trims each line, and
// removes runs of more than two consecutive newlines.
func normalizeWhitespace(text string) string {
	text = regexp.MustCompile(`[ \t]+`).ReplaceAllString(text, " ")

	lines := strings.Split(text, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimSpace(line)
	}
	text = strings.Join(lines, "\n")

	return regexp.MustCompile(`\n{3,}`).ReplaceAllString(text, "\n\n")
}

// convertToMarkdown converts GBF markup to Markdown.
func (c *GBFConverter) convertToMarkdown(text string) string {
	const redLetterStart = "\x00RED_START\x00"
	const redLetterEnd = "\x00RED_END\x00"

	// Static replacements applied in order before and after the
	// conditional Strong's block.
	staticBefore := []regexReplacement{
		{regexp.MustCompile(`<FR>([^<]*)<Fr>`), redLetterStart + "$1" + redLetterEnd}, // red letter
		{regexp.MustCompile(`<FI>([^<]*)<Fi>`), "*$1*"},                               // italic
		{regexp.MustCompile(`<FB>([^<]*)<Fb>`), "**$1**"},                             // bold
		{regexp.MustCompile(`<FO>([^<]*)<Fo>`), "> $1"},                               // OT quotation → blockquote
		{regexp.MustCompile(`<FS>([^<]*)<Fs>`), "^$1^"},                               // superscript
		{regexp.MustCompile(`<FU>([^<]*)<Fu>`), "_$1_"},                               // underline → emphasis
	}

	staticAfter := []regexReplacement{
		{regexp.MustCompile(`<WT[^>]*>`), ""},                  // morphology open tags
		{regexp.MustCompile(`<Wt>`), ""},                       // morphology close tag
		{regexp.MustCompile(`<RF>[^<]*<Rf>`), ""},              // footnotes (already extracted)
		{regexp.MustCompile(`<RX>[^<]*<Rx>`), ""},              // cross-references (already extracted)
		{regexp.MustCompile(`<CM>`), "\n\n"},                   // paragraph/section mark
		{regexp.MustCompile(`<CL>`), "\n"},                     // line break
		{regexp.MustCompile(`<CI>`), "    "},                   // indent
		{regexp.MustCompile(`<TS>([^<]*)<Ts>`), "\n### $1\n"}, // title → heading
		{regexp.MustCompile(`<[A-Z][A-Za-z0-9]*>`), ""},       // remaining open GBF tags
		{regexp.MustCompile(`<[A-Za-z][a-z]>`), ""},           // remaining close GBF tags
	}

	for _, r := range staticBefore {
		text = r.re.ReplaceAllString(text, r.replacement)
	}

	for _, r := range strongsReplacements(c.PreserveStrongs) {
		text = r.re.ReplaceAllString(text, r.replacement)
	}

	for _, r := range staticAfter {
		text = r.re.ReplaceAllString(text, r.replacement)
	}

	// Restore red-letter placeholders to HTML spans.
	text = strings.ReplaceAll(text, redLetterStart, `<span class="red-letter">`)
	text = strings.ReplaceAll(text, redLetterEnd, `</span>`)

	return normalizeWhitespace(text)
}

// ConvertText converts GBF to plain Markdown text.
func (c *GBFConverter) ConvertText(gbf string) string {
	result := c.Convert(gbf)
	return result.Text
}
