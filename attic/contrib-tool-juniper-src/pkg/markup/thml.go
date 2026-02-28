// Package markup provides converters for Bible markup formats to Markdown.
package markup

import (
	"regexp"
	"strings"
)

// ThMLConverter converts ThML (Theological Markup Language) to Markdown.
//
// ThML is an older XML-based format used by some SWORD modules, particularly
// commentaries. It includes:
//   - Scripture references (<scripRef>)
//   - Notes and footnotes (<note>)
//   - Foreign language text (<foreign>)
//   - Emphasis (<em>, <b>, <i>)
//   - Paragraph breaks (<p>, <br>)
//   - Headings (<h1>-<h6>)
type ThMLConverter struct {
	// PreserveScripRefs keeps scripture references as links
	PreserveScripRefs bool
}

// NewThMLConverter creates a converter with default settings.
func NewThMLConverter() *ThMLConverter {
	return &ThMLConverter{
		PreserveScripRefs: true,
	}
}

// ThMLResult contains the converted text and extracted metadata.
type ThMLResult struct {
	Text       string
	ScripRefs  []string
	Notes      []string
	HasForeign bool
}

// Convert transforms ThML markup to Markdown.
func (c *ThMLConverter) Convert(thml string) *ThMLResult {
	result := &ThMLResult{
		ScripRefs: make([]string, 0),
		Notes:     make([]string, 0),
	}

	text := thml

	// Extract scripture references
	if c.PreserveScripRefs {
		result.ScripRefs = c.extractScripRefs(text)
	}

	// Extract notes
	result.Notes = c.extractNotes(text)

	// Check for foreign text
	result.HasForeign = strings.Contains(text, "<foreign")

	// Convert to Markdown
	text = c.convertToMarkdown(text)

	result.Text = strings.TrimSpace(text)
	return result
}

// extractScripRefs extracts scripture references from ThML.
func (c *ThMLConverter) extractScripRefs(text string) []string {
	// Pattern: <scripRef passage="Gen.1.1">Genesis 1:1</scripRef>
	re := regexp.MustCompile(`<scripRef[^>]*passage="([^"]*)"[^>]*>`)
	matches := re.FindAllStringSubmatch(text, -1)

	refs := make([]string, 0, len(matches))
	seen := make(map[string]bool)

	for _, match := range matches {
		if len(match) > 1 && !seen[match[1]] {
			refs = append(refs, match[1])
			seen[match[1]] = true
		}
	}

	return refs
}

// extractNotes extracts note content from ThML.
func (c *ThMLConverter) extractNotes(text string) []string {
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

// convertToMarkdown converts ThML markup to Markdown.
func (c *ThMLConverter) convertToMarkdown(text string) string {
	// Use placeholders for content we want to preserve
	const scripRefStart = "\x00SCRIPREF_START\x00"
	const scripRefEnd = "\x00SCRIPREF_END\x00"
	const foreignStart = "\x00FOREIGN_START\x00"
	const foreignEnd = "\x00FOREIGN_END\x00"

	// Convert scripture references to placeholder links
	re := regexp.MustCompile(`<scripRef[^>]*passage="([^"]*)"[^>]*>([^<]*)</scripRef>`)
	text = re.ReplaceAllString(text, scripRefStart+"$2]($1)"+scripRefEnd)

	// Convert foreign text to emphasis
	re = regexp.MustCompile(`<foreign[^>]*>([^<]*)</foreign>`)
	text = re.ReplaceAllString(text, foreignStart+"$1"+foreignEnd)

	// Convert headings
	re = regexp.MustCompile(`<h1[^>]*>([^<]*)</h1>`)
	text = re.ReplaceAllString(text, "\n# $1\n")
	re = regexp.MustCompile(`<h2[^>]*>([^<]*)</h2>`)
	text = re.ReplaceAllString(text, "\n## $1\n")
	re = regexp.MustCompile(`<h3[^>]*>([^<]*)</h3>`)
	text = re.ReplaceAllString(text, "\n### $1\n")
	re = regexp.MustCompile(`<h4[^>]*>([^<]*)</h4>`)
	text = re.ReplaceAllString(text, "\n#### $1\n")
	re = regexp.MustCompile(`<h5[^>]*>([^<]*)</h5>`)
	text = re.ReplaceAllString(text, "\n##### $1\n")
	re = regexp.MustCompile(`<h6[^>]*>([^<]*)</h6>`)
	text = re.ReplaceAllString(text, "\n###### $1\n")

	// Convert emphasis
	re = regexp.MustCompile(`<em>([^<]*)</em>`)
	text = re.ReplaceAllString(text, "*$1*")
	re = regexp.MustCompile(`<i>([^<]*)</i>`)
	text = re.ReplaceAllString(text, "*$1*")
	re = regexp.MustCompile(`<b>([^<]*)</b>`)
	text = re.ReplaceAllString(text, "**$1**")
	re = regexp.MustCompile(`<strong>([^<]*)</strong>`)
	text = re.ReplaceAllString(text, "**$1**")

	// Convert paragraphs and breaks
	re = regexp.MustCompile(`<p[^>]*>`)
	text = re.ReplaceAllString(text, "\n\n")
	re = regexp.MustCompile(`</p>`)
	text = re.ReplaceAllString(text, "")
	re = regexp.MustCompile(`<br[^>]*>`)
	text = re.ReplaceAllString(text, "\n")
	re = regexp.MustCompile(`<br/>`)
	text = re.ReplaceAllString(text, "\n")

	// Convert blockquotes
	re = regexp.MustCompile(`<blockquote[^>]*>([^<]*)</blockquote>`)
	text = re.ReplaceAllStringFunc(text, func(match string) string {
		submatches := regexp.MustCompile(`<blockquote[^>]*>([^<]*)</blockquote>`).FindStringSubmatch(match)
		if len(submatches) > 1 {
			lines := strings.Split(submatches[1], "\n")
			for i, line := range lines {
				lines[i] = "> " + line
			}
			return strings.Join(lines, "\n")
		}
		return match
	})

	// Remove notes (already extracted)
	re = regexp.MustCompile(`<note[^>]*>[^<]*</note>`)
	text = re.ReplaceAllString(text, "")

	// Remove sync markers
	re = regexp.MustCompile(`<sync[^>]*/>`)
	text = re.ReplaceAllString(text, "")
	re = regexp.MustCompile(`<sync[^>]*>`)
	text = re.ReplaceAllString(text, "")

	// Remove div markers but keep content
	re = regexp.MustCompile(`<div[^>]*>`)
	text = re.ReplaceAllString(text, "")
	re = regexp.MustCompile(`</div>`)
	text = re.ReplaceAllString(text, "")

	// Remove remaining XML tags
	re = regexp.MustCompile(`<[^>]+>`)
	text = re.ReplaceAllString(text, "")

	// Restore placeholders
	text = strings.ReplaceAll(text, scripRefStart, "[")
	text = strings.ReplaceAll(text, scripRefEnd, "")
	text = strings.ReplaceAll(text, foreignStart, "*")
	text = strings.ReplaceAll(text, foreignEnd, "*")

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

// ConvertText converts ThML to plain Markdown text.
func (c *ThMLConverter) ConvertText(thml string) string {
	result := c.Convert(thml)
	return result.Text
}
