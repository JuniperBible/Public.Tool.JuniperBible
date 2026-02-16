// Package osis provides the embedded handler for OSIS Bible format plugin.
package osis

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/FocuswithJustin/JuniperBible/core/ir"
)

// OSIS XML Types
type OSISDoc struct {
	XMLName   xml.Name `xml:"osis"`
	Namespace string   `xml:"xmlns,attr,omitempty"`
	OsisText  OSISText `xml:"osisText"`
	RawXML    string   `xml:"-"` // Store original for L0 round-trip
}

type OSISText struct {
	OsisIDWork  string      `xml:"osisIDWork,attr"`
	OsisRefWork string      `xml:"osisRefWork,attr,omitempty"`
	Lang        string      `xml:"lang,attr,omitempty"`
	XMLLang     string      `xml:"http://www.w3.org/XML/1998/namespace lang,attr,omitempty"`
	Header      *OSISHeader `xml:"header,omitempty"`
	Divs        []OSISDiv   `xml:"div"`
}

type OSISHeader struct {
	Work []OSISWork `xml:"work"`
}

type OSISWork struct {
	OsisWork    string `xml:"osisWork,attr"`
	Title       string `xml:"title,omitempty"`
	Type        string `xml:"type,omitempty"`
	Identifier  string `xml:"identifier,omitempty"`
	RefSystem   string `xml:"refSystem,omitempty"`
	Language    string `xml:"language,omitempty"`
	Publisher   string `xml:"publisher,omitempty"`
	Rights      string `xml:"rights,omitempty"`
	Description string `xml:"description,omitempty"`
}

type OSISDiv struct {
	Type     string        `xml:"type,attr,omitempty"`
	OsisID   string        `xml:"osisID,attr,omitempty"`
	Title    string        `xml:"title,omitempty"`
	Divs     []OSISDiv     `xml:"div"`
	Chapters []OSISChapter `xml:"chapter"`
	Verses   []OSISVerse   `xml:"verse"`
	Ps       []OSISP       `xml:"p"`
	Lgs      []OSISLg      `xml:"lg"`
	Ls       []OSISL       `xml:"l"`
	Content  string        `xml:",chardata"`
}

type OSISChapter struct {
	OsisID string `xml:"osisID,attr"`
	SID    string `xml:"sID,attr,omitempty"`
	EID    string `xml:"eID,attr,omitempty"`
}

type OSISVerse struct {
	OsisID string `xml:"osisID,attr,omitempty"`
	SID    string `xml:"sID,attr,omitempty"`
	EID    string `xml:"eID,attr,omitempty"`
}

type OSISP struct {
	Verses  []OSISVerse `xml:"verse"`
	Content string      `xml:",chardata"`
}

type OSISLg struct {
	Ls []OSISL `xml:"l"`
}

type OSISL struct {
	Content string `xml:",chardata"`
}

// parseOSISToIR converts OSIS XML to IR Corpus
func parseOSISToIR(data []byte) (*ir.Corpus, error) {
	var doc OSISDoc
	if err := xml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("xml unmarshal failed: %w", err)
	}

	// Store raw XML for potential L0 round-trip
	doc.RawXML = string(data)

	corpus := &ir.Corpus{
		ID:         doc.OsisText.OsisIDWork,
		Version:    "1.0.0",
		ModuleType: ir.ModuleBible,
		LossClass:  ir.LossL0,
		Documents:  []*ir.Document{},
	}

	// Extract language
	if doc.OsisText.Lang != "" {
		corpus.Language = doc.OsisText.Lang
	} else if doc.OsisText.XMLLang != "" {
		corpus.Language = doc.OsisText.XMLLang
	}

	// Extract header information
	if doc.OsisText.Header != nil {
		for _, work := range doc.OsisText.Header.Work {
			if work.OsisWork == doc.OsisText.OsisIDWork || work.Title != "" {
				corpus.Title = work.Title
				if work.RefSystem != "" {
					corpus.Versification = work.RefSystem
				}
				if work.Language != "" {
					corpus.Language = work.Language
				}
			}
		}
	}

	// Parse books (divs)
	docOrder := 0
	for _, div := range doc.OsisText.Divs {
		docs := parseOSISDiv(&div, &docOrder)
		corpus.Documents = append(corpus.Documents, docs...)
	}

	// Compute source hash
	h := sha256.Sum256(data)
	corpus.SourceHash = hex.EncodeToString(h[:])

	return corpus, nil
}

// parseOSISDiv recursively parses OSIS div elements
func parseOSISDiv(div *OSISDiv, order *int) []*ir.Document {
	var docs []*ir.Document

	// If this is a book-level div
	if div.Type == "book" || (div.OsisID != "" && isBookID(div.OsisID)) {
		*order++
		doc := &ir.Document{
			ID:            div.OsisID,
			Title:         div.Title,
			Order:         *order,
			ContentBlocks: []*ir.ContentBlock{},
		}
		if div.Title == "" && div.OsisID != "" {
			doc.Title = div.OsisID
		}

		// Parse content blocks from this div
		cbSeq := 0
		blocks := extractContentBlocks(div, &cbSeq)
		doc.ContentBlocks = blocks

		docs = append(docs, doc)
	}

	// Recursively process child divs
	for _, childDiv := range div.Divs {
		childDocs := parseOSISDiv(&childDiv, order)
		docs = append(docs, childDocs...)
	}

	return docs
}

// extractContentBlocks extracts content blocks from an OSIS div
func extractContentBlocks(div *OSISDiv, seq *int) []*ir.ContentBlock {
	var blocks []*ir.ContentBlock

	// Process paragraphs
	for _, p := range div.Ps {
		text := strings.TrimSpace(p.Content)
		if text == "" {
			continue
		}

		*seq++
		block := &ir.ContentBlock{
			ID:       fmt.Sprintf("cb-%d", *seq),
			Sequence: *seq,
			Text:     text,
			Anchors:  []*ir.Anchor{},
		}

		// Add verse spans if present
		for _, v := range p.Verses {
			if v.OsisID != "" || v.SID != "" {
				osisID := v.OsisID
				if osisID == "" {
					osisID = v.SID
				}
				// Parse OSIS ref for later use (currently simplified)
				_ = parseOSISRef(osisID)
				anchor := &ir.Anchor{
					ID:             fmt.Sprintf("a-%d-0", *seq),
					ContentBlockID: block.ID,
					CharOffset:     0,
				}
				block.Anchors = append(block.Anchors, anchor)
			}
		}

		// Compute hash
		block.ComputeHash()

		blocks = append(blocks, block)
	}

	// Process poetry lines
	for _, lg := range div.Lgs {
		for _, l := range lg.Ls {
			text := strings.TrimSpace(l.Content)
			if text == "" {
				continue
			}

			*seq++
			block := &ir.ContentBlock{
				ID:       fmt.Sprintf("cb-%d", *seq),
				Sequence: *seq,
				Text:     text,
			}

			// Compute hash
			block.ComputeHash()

			blocks = append(blocks, block)
		}
	}

	// Process direct content
	text := strings.TrimSpace(div.Content)
	if text != "" {
		*seq++
		block := &ir.ContentBlock{
			ID:       fmt.Sprintf("cb-%d", *seq),
			Sequence: *seq,
			Text:     text,
		}
		block.ComputeHash()
		blocks = append(blocks, block)
	}

	// Process nested divs (chapters)
	for _, childDiv := range div.Divs {
		childBlocks := extractContentBlocks(&childDiv, seq)
		blocks = append(blocks, childBlocks...)
	}

	return blocks
}

// parseOSISRef parses an OSIS reference like "Gen.1.1" or "Matt.5.3-12"
func parseOSISRef(osisID string) *ir.Ref {
	ref := &ir.Ref{OSISID: osisID}

	// Parse OSIS ID format: Book.Chapter.Verse or Book.Chapter.Verse-VerseEnd
	parts := strings.Split(osisID, ".")
	if len(parts) >= 1 {
		ref.Book = parts[0]
	}
	if len(parts) >= 2 {
		ref.Chapter, _ = strconv.Atoi(parts[1])
	}
	if len(parts) >= 3 {
		// Handle verse ranges
		versePart := parts[2]
		if strings.Contains(versePart, "-") {
			rangeParts := strings.Split(versePart, "-")
			ref.Verse, _ = strconv.Atoi(rangeParts[0])
			if len(rangeParts) > 1 {
				ref.VerseEnd, _ = strconv.Atoi(rangeParts[1])
			}
		} else {
			ref.Verse, _ = strconv.Atoi(versePart)
		}
	}

	return ref
}

// isBookID checks if an OSIS ID is a book identifier
func isBookID(osisID string) bool {
	// Common Bible book abbreviations
	books := regexp.MustCompile(`^(Gen|Exod|Lev|Num|Deut|Josh|Judg|Ruth|1Sam|2Sam|1Kgs|2Kgs|1Chr|2Chr|Ezra|Neh|Esth|Job|Ps|Prov|Eccl|Song|Isa|Jer|Lam|Ezek|Dan|Hos|Joel|Amos|Obad|Jonah|Mic|Nah|Hab|Zeph|Hag|Zech|Mal|Matt|Mark|Luke|John|Acts|Rom|1Cor|2Cor|Gal|Eph|Phil|Col|1Thess|2Thess|1Tim|2Tim|Titus|Phlm|Heb|Jas|1Pet|2Pet|1John|2John|3John|Jude|Rev)$`)
	return books.MatchString(osisID)
}

// emitOSISFromIR converts IR Corpus back to OSIS XML
func emitOSISFromIR(corpus *ir.Corpus) ([]byte, error) {
	// Otherwise, reconstruct OSIS from IR structure
	var buf bytes.Buffer
	buf.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
	buf.WriteString("\n")
	buf.WriteString(`<osis xmlns="http://www.bibletechnologies.net/2003/OSIS/namespace">`)
	buf.WriteString("\n")
	buf.WriteString(fmt.Sprintf(`  <osisText osisIDWork="%s"`, escapeXML(corpus.ID)))
	if corpus.Language != "" {
		buf.WriteString(fmt.Sprintf(` xml:lang="%s"`, escapeXML(corpus.Language)))
	}
	buf.WriteString(">\n")

	// Write header
	buf.WriteString("    <header>\n")
	buf.WriteString(fmt.Sprintf(`      <work osisWork="%s">`, escapeXML(corpus.ID)))
	buf.WriteString("\n")
	if corpus.Title != "" {
		buf.WriteString(fmt.Sprintf("        <title>%s</title>\n", escapeXML(corpus.Title)))
	}
	if corpus.Language != "" {
		buf.WriteString(fmt.Sprintf("        <language>%s</language>\n", escapeXML(corpus.Language)))
	}
	if corpus.Versification != "" {
		buf.WriteString(fmt.Sprintf("        <refSystem>%s</refSystem>\n", escapeXML(corpus.Versification)))
	}
	buf.WriteString("      </work>\n")
	buf.WriteString("    </header>\n")

	// Write documents (books)
	for _, doc := range corpus.Documents {
		buf.WriteString(fmt.Sprintf(`    <div type="book" osisID="%s">`, escapeXML(doc.ID)))
		buf.WriteString("\n")
		if doc.Title != "" {
			buf.WriteString(fmt.Sprintf("      <title>%s</title>\n", escapeXML(doc.Title)))
		}

		// Write content blocks
		for _, block := range doc.ContentBlocks {
			// Write as paragraph
			buf.WriteString("      <p>")
			buf.WriteString(escapeXML(block.Text))
			buf.WriteString("</p>\n")
		}

		buf.WriteString("    </div>\n")
	}

	buf.WriteString("  </osisText>\n")
	buf.WriteString("</osis>\n")

	return buf.Bytes(), nil
}

// escapeXML escapes special characters for XML
func escapeXML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	s = strings.ReplaceAll(s, "'", "&apos;")
	return s
}
