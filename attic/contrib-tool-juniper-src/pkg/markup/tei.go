// Package markup provides converters for Bible markup formats to Markdown.
package markup

import (
	"regexp"
	"strings"
)

// TEIConverter converts TEI (Text Encoding Initiative) to Markdown.
//
// TEI is a scholarly XML format sometimes used for SWORD dictionaries and
// lexicons. It includes rich semantic markup:
//   - <entry> - Dictionary entry
//   - <orth> - Orthographic form (headword)
//   - <pron> - Pronunciation
//   - <sense> - Word sense/meaning
//   - <def> - Definition
//   - <etym> - Etymology
//   - <gramGrp> - Grammatical group
//   - <pos> - Part of speech
//   - <gen> - Gender
//   - <num> - Number
//   - <ref> - Reference
//   - <quote> - Quotation
//   - <bibl> - Bibliographic reference
type TEIConverter struct {
	// IncludeEtymology includes etymology in output
	IncludeEtymology bool

	// IncludeGrammar includes grammatical information
	IncludeGrammar bool
}

// NewTEIConverter creates a converter with default settings.
func NewTEIConverter() *TEIConverter {
	return &TEIConverter{
		IncludeEtymology: true,
		IncludeGrammar:   true,
	}
}

// TEIResult contains the converted text and extracted metadata.
type TEIResult struct {
	Text         string
	Headword     string
	PartOfSpeech string
	Etymology    string
	Senses       []string
	References   []string
}

// Convert transforms TEI markup to Markdown.
func (c *TEIConverter) Convert(tei string) *TEIResult {
	result := &TEIResult{
		Senses:     make([]string, 0),
		References: make([]string, 0),
	}

	text := tei

	// Extract headword
	result.Headword = c.extractHeadword(text)

	// Extract part of speech
	result.PartOfSpeech = c.extractPartOfSpeech(text)

	// Extract etymology
	if c.IncludeEtymology {
		result.Etymology = c.extractEtymology(text)
	}

	// Extract senses
	result.Senses = c.extractSenses(text)

	// Extract references
	result.References = c.extractReferences(text)

	// Convert to Markdown
	text = c.convertToMarkdown(text)

	result.Text = strings.TrimSpace(text)
	return result
}

// extractHeadword extracts the main headword from TEI.
func (c *TEIConverter) extractHeadword(text string) string {
	re := regexp.MustCompile(`<orth[^>]*>([^<]*)</orth>`)
	match := re.FindStringSubmatch(text)
	if len(match) > 1 {
		return strings.TrimSpace(match[1])
	}
	return ""
}

// extractPartOfSpeech extracts part of speech information.
func (c *TEIConverter) extractPartOfSpeech(text string) string {
	re := regexp.MustCompile(`<pos[^>]*>([^<]*)</pos>`)
	match := re.FindStringSubmatch(text)
	if len(match) > 1 {
		return strings.TrimSpace(match[1])
	}
	return ""
}

// extractEtymology extracts etymology information.
func (c *TEIConverter) extractEtymology(text string) string {
	re := regexp.MustCompile(`<etym[^>]*>([^<]*)</etym>`)
	match := re.FindStringSubmatch(text)
	if len(match) > 1 {
		return strings.TrimSpace(match[1])
	}
	return ""
}

// extractSenses extracts word senses/definitions.
func (c *TEIConverter) extractSenses(text string) []string {
	// Try <def> first
	re := regexp.MustCompile(`<def[^>]*>([^<]*)</def>`)
	matches := re.FindAllStringSubmatch(text, -1)

	senses := make([]string, 0)
	for _, match := range matches {
		if len(match) > 1 && strings.TrimSpace(match[1]) != "" {
			senses = append(senses, strings.TrimSpace(match[1]))
		}
	}

	// If no <def>, try <sense>
	if len(senses) == 0 {
		re = regexp.MustCompile(`<sense[^>]*>([^<]*)</sense>`)
		matches = re.FindAllStringSubmatch(text, -1)
		for _, match := range matches {
			if len(match) > 1 && strings.TrimSpace(match[1]) != "" {
				senses = append(senses, strings.TrimSpace(match[1]))
			}
		}
	}

	return senses
}

// extractReferences extracts bibliographic references.
func (c *TEIConverter) extractReferences(text string) []string {
	re := regexp.MustCompile(`<ref[^>]*>([^<]*)</ref>`)
	matches := re.FindAllStringSubmatch(text, -1)

	refs := make([]string, 0, len(matches))
	for _, match := range matches {
		if len(match) > 1 && strings.TrimSpace(match[1]) != "" {
			refs = append(refs, strings.TrimSpace(match[1]))
		}
	}

	return refs
}

// convertToMarkdown converts TEI markup to Markdown.
func (c *TEIConverter) convertToMarkdown(text string) string {
	// Convert headword to heading
	re := regexp.MustCompile(`<orth[^>]*>([^<]*)</orth>`)
	text = re.ReplaceAllString(text, "## $1\n")

	// Convert pronunciation to italics
	re = regexp.MustCompile(`<pron[^>]*>([^<]*)</pron>`)
	text = re.ReplaceAllString(text, "/*$1*/")

	// Convert grammatical information
	if c.IncludeGrammar {
		re = regexp.MustCompile(`<pos[^>]*>([^<]*)</pos>`)
		text = re.ReplaceAllString(text, "**$1**")

		re = regexp.MustCompile(`<gen[^>]*>([^<]*)</gen>`)
		text = re.ReplaceAllString(text, "($1)")

		re = regexp.MustCompile(`<num[^>]*>([^<]*)</num>`)
		text = re.ReplaceAllString(text, "($1)")

		re = regexp.MustCompile(`<gramGrp[^>]*>([^<]*)</gramGrp>`)
		text = re.ReplaceAllString(text, " [$1]")
	} else {
		// Remove grammar tags
		re = regexp.MustCompile(`<pos[^>]*>[^<]*</pos>`)
		text = re.ReplaceAllString(text, "")
		re = regexp.MustCompile(`<gen[^>]*>[^<]*</gen>`)
		text = re.ReplaceAllString(text, "")
		re = regexp.MustCompile(`<num[^>]*>[^<]*</num>`)
		text = re.ReplaceAllString(text, "")
		re = regexp.MustCompile(`<gramGrp[^>]*>[^<]*</gramGrp>`)
		text = re.ReplaceAllString(text, "")
	}

	// Convert etymology
	if c.IncludeEtymology {
		re = regexp.MustCompile(`<etym[^>]*>([^<]*)</etym>`)
		text = re.ReplaceAllString(text, "\n**Etymology:** $1\n")
	} else {
		re = regexp.MustCompile(`<etym[^>]*>[^<]*</etym>`)
		text = re.ReplaceAllString(text, "")
	}

	// Convert senses to numbered list
	senseNum := 0
	re = regexp.MustCompile(`<sense[^>]*n="(\d+)"[^>]*>`)
	text = re.ReplaceAllStringFunc(text, func(match string) string {
		submatches := regexp.MustCompile(`<sense[^>]*n="(\d+)"[^>]*>`).FindStringSubmatch(match)
		if len(submatches) > 1 {
			return "\n" + submatches[1] + ". "
		}
		senseNum++
		return "\n" + string(rune('0'+senseNum)) + ". "
	})
	re = regexp.MustCompile(`<sense[^>]*>`)
	text = re.ReplaceAllStringFunc(text, func(match string) string {
		senseNum++
		return "\n" + string(rune('0'+senseNum)) + ". "
	})
	re = regexp.MustCompile(`</sense>`)
	text = re.ReplaceAllString(text, "")

	// Convert definitions
	re = regexp.MustCompile(`<def[^>]*>([^<]*)</def>`)
	text = re.ReplaceAllString(text, "$1")

	// Convert quotes
	re = regexp.MustCompile(`<quote[^>]*>([^<]*)</quote>`)
	text = re.ReplaceAllString(text, "\"$1\"")

	// Convert references to links
	re = regexp.MustCompile(`<ref[^>]*target="([^"]*)"[^>]*>([^<]*)</ref>`)
	text = re.ReplaceAllString(text, "[$2]($1)")
	re = regexp.MustCompile(`<ref[^>]*>([^<]*)</ref>`)
	text = re.ReplaceAllString(text, "*$1*")

	// Convert bibliographic references
	re = regexp.MustCompile(`<bibl[^>]*>([^<]*)</bibl>`)
	text = re.ReplaceAllString(text, "($1)")

	// Remove entry wrapper
	re = regexp.MustCompile(`<entry[^>]*>`)
	text = re.ReplaceAllString(text, "")
	re = regexp.MustCompile(`</entry>`)
	text = re.ReplaceAllString(text, "")

	// Remove remaining XML tags
	re = regexp.MustCompile(`<[^>]+>`)
	text = re.ReplaceAllString(text, "")

	// Clean up whitespace
	re = regexp.MustCompile(`[ \t]+`)
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

// ConvertText converts TEI to plain Markdown text.
func (c *TEIConverter) ConvertText(tei string) string {
	result := c.Convert(tei)
	return result.Text
}
