package ir

import (
	"errors"
	"fmt"
)

// validateDocumentFn is injectable for testing error type handling.
var validateDocumentFn = ValidateDocument

// validateMappingTableFn is injectable for testing error type handling.
var validateMappingTableFn = ValidateMappingTable

// validateRefFn is injectable for testing error type handling.
var validateRefFn = ValidateRef

// validateContentBlockFn is injectable for testing error type handling.
var validateContentBlockFn = ValidateContentBlock

// validateAnnotationFn is injectable for testing error type handling.
var validateAnnotationFn = ValidateAnnotation

// ValidationError represents a validation error with context.
type ValidationError struct {
	Path    string
	Message string
}

func (e *ValidationError) Error() string {
	if e.Path != "" {
		return fmt.Sprintf("%s: %s", e.Path, e.Message)
	}
	return e.Message
}

// newValidationError creates a new ValidationError.
func newValidationError(path, message string) error {
	return &ValidationError{Path: path, Message: message}
}

// ValidateCorpus validates a Corpus and returns all validation errors.
func ValidateCorpus(c *Corpus) []error {
	var errs []error

	if c.ID == "" {
		errs = append(errs, newValidationError("corpus", "ID is required"))
	}

	if c.Version == "" {
		errs = append(errs, newValidationError("corpus", "Version is required"))
	}

	if c.ModuleType != "" && !c.ModuleType.IsValid() {
		errs = append(errs, newValidationError("corpus.module_type",
			fmt.Sprintf("invalid ModuleType: %q", c.ModuleType)))
	}

	// Validate documents
	for i, doc := range c.Documents {
		docPath := fmt.Sprintf("corpus.documents[%d]", i)
		docErrs := validateDocumentFn(doc)
		for _, err := range docErrs {
			var ve *ValidationError
			if errors.As(err, &ve) {
				errs = append(errs, newValidationError(
					fmt.Sprintf("%s.%s", docPath, ve.Path), ve.Message))
			} else {
				errs = append(errs, newValidationError(docPath, err.Error()))
			}
		}
	}

	// Validate mapping tables
	for i, mt := range c.MappingTables {
		mtPath := fmt.Sprintf("corpus.mapping_tables[%d]", i)
		mtErrs := validateMappingTableFn(mt)
		for _, err := range mtErrs {
			var ve *ValidationError
			if errors.As(err, &ve) {
				errs = append(errs, newValidationError(
					fmt.Sprintf("%s.%s", mtPath, ve.Path), ve.Message))
			} else {
				errs = append(errs, newValidationError(mtPath, err.Error()))
			}
		}
	}

	return errs
}

// ValidateDocument validates a Document and returns all validation errors.
func ValidateDocument(d *Document) []error {
	var errs []error

	if d.ID == "" {
		errs = append(errs, newValidationError("document", "ID is required"))
	}

	// Validate canonical ref if present
	if d.CanonicalRef != nil {
		refErrs := validateRefFn(d.CanonicalRef)
		for _, err := range refErrs {
			var ve *ValidationError
			if errors.As(err, &ve) {
				errs = append(errs, newValidationError("document.canonical_ref", ve.Message))
			} else {
				errs = append(errs, err)
			}
		}
	}

	// Validate content blocks
	for i, cb := range d.ContentBlocks {
		cbPath := fmt.Sprintf("content_blocks[%d]", i)
		cbErrs := validateContentBlockFn(cb)
		for _, err := range cbErrs {
			var ve *ValidationError
			if errors.As(err, &ve) {
				errs = append(errs, newValidationError(
					fmt.Sprintf("%s.%s", cbPath, ve.Path), ve.Message))
			} else {
				errs = append(errs, newValidationError(cbPath, err.Error()))
			}
		}
	}

	// Validate annotations
	for i, ann := range d.Annotations {
		annPath := fmt.Sprintf("annotations[%d]", i)
		annErrs := validateAnnotationFn(ann)
		for _, err := range annErrs {
			var ve *ValidationError
			if errors.As(err, &ve) {
				errs = append(errs, newValidationError(
					fmt.Sprintf("%s.%s", annPath, ve.Path), ve.Message))
			} else {
				errs = append(errs, newValidationError(annPath, err.Error()))
			}
		}
	}

	return errs
}

// ValidateContentBlock validates a ContentBlock and returns all validation errors.
func ValidateContentBlock(cb *ContentBlock) []error {
	var errs []error

	if cb.ID == "" {
		errs = append(errs, newValidationError("content_block", "ID is required"))
	}

	if cb.Sequence < 0 {
		errs = append(errs, newValidationError("content_block.sequence",
			"Sequence cannot be negative"))
	}

	// Validate hash if present
	if cb.Hash != "" && !cb.VerifyHash() {
		errs = append(errs, newValidationError("content_block.hash",
			"Hash does not match content"))
	}

	// Validate tokens
	for i, tok := range cb.Tokens {
		tokPath := fmt.Sprintf("tokens[%d]", i)
		if tok.CharStart < 0 {
			errs = append(errs, newValidationError(tokPath,
				"CharStart cannot be negative"))
		}
		if tok.CharEnd < tok.CharStart {
			errs = append(errs, newValidationError(tokPath,
				"CharEnd cannot be before CharStart"))
		}
	}

	// Validate anchors
	for i, anchor := range cb.Anchors {
		anchorPath := fmt.Sprintf("anchors[%d]", i)
		if anchor.CharOffset < 0 {
			errs = append(errs, newValidationError(anchorPath,
				"CharOffset cannot be negative"))
		}
	}

	return errs
}

// ValidateSpan validates a Span and returns all validation errors.
func ValidateSpan(s *Span) []error {
	var errs []error

	if s.ID == "" {
		errs = append(errs, newValidationError("span", "ID is required"))
	}

	if s.Type != "" && !s.Type.IsValid() {
		errs = append(errs, newValidationError("span.type",
			fmt.Sprintf("invalid SpanType: %q", s.Type)))
	}

	if s.StartAnchorID == "" {
		errs = append(errs, newValidationError("span.start_anchor_id",
			"StartAnchorID is required"))
	}

	if s.EndAnchorID == "" {
		errs = append(errs, newValidationError("span.end_anchor_id",
			"EndAnchorID is required"))
	}

	// Validate ref if present
	if s.Ref != nil {
		refErrs := validateRefFn(s.Ref)
		for _, err := range refErrs {
			var ve *ValidationError
			if errors.As(err, &ve) {
				errs = append(errs, newValidationError("span.ref", ve.Message))
			} else {
				errs = append(errs, err)
			}
		}
	}

	return errs
}

// ValidateRef validates a Ref and returns all validation errors.
func ValidateRef(r *Ref) []error {
	var errs []error

	if r.Book == "" {
		errs = append(errs, newValidationError("ref", "Book is required"))
	}

	if r.Chapter < 0 {
		errs = append(errs, newValidationError("ref.chapter",
			"Chapter cannot be negative"))
	}

	if r.Verse < 0 {
		errs = append(errs, newValidationError("ref.verse",
			"Verse cannot be negative"))
	}

	if r.VerseEnd > 0 && r.VerseEnd < r.Verse {
		errs = append(errs, newValidationError("ref.verse_end",
			"VerseEnd cannot be before Verse"))
	}

	return errs
}

// ValidateAnnotation validates an Annotation and returns all validation errors.
func ValidateAnnotation(a *Annotation) []error {
	var errs []error

	if a.ID == "" {
		errs = append(errs, newValidationError("annotation", "ID is required"))
	}

	if a.SpanID == "" {
		errs = append(errs, newValidationError("annotation.span_id",
			"SpanID is required"))
	}

	if a.Type != "" && !a.Type.IsValid() {
		errs = append(errs, newValidationError("annotation.type",
			fmt.Sprintf("invalid AnnotationType: %q", a.Type)))
	}

	if a.Confidence < 0 || a.Confidence > 1 {
		errs = append(errs, newValidationError("annotation.confidence",
			"Confidence must be between 0 and 1"))
	}

	return errs
}

// ValidateLossReport validates a LossReport and returns all validation errors.
func ValidateLossReport(lr *LossReport) []error {
	var errs []error

	if lr.SourceFormat == "" {
		errs = append(errs, newValidationError("loss_report",
			"SourceFormat is required"))
	}

	if lr.TargetFormat == "" {
		errs = append(errs, newValidationError("loss_report",
			"TargetFormat is required"))
	}

	if lr.LossClass != "" && !lr.LossClass.IsValid() {
		errs = append(errs, newValidationError("loss_report.loss_class",
			fmt.Sprintf("invalid LossClass: %q", lr.LossClass)))
	}

	return errs
}

// ValidateMappingTable validates a MappingTable and returns all validation errors.
func ValidateMappingTable(mt *MappingTable) []error {
	var errs []error

	if mt.ID == "" {
		errs = append(errs, newValidationError("mapping_table", "ID is required"))
	}

	if mt.FromSystem != "" && !mt.FromSystem.IsValid() {
		errs = append(errs, newValidationError("mapping_table.from_system",
			fmt.Sprintf("invalid VersificationID: %q", mt.FromSystem)))
	}

	if mt.ToSystem != "" && !mt.ToSystem.IsValid() {
		errs = append(errs, newValidationError("mapping_table.to_system",
			fmt.Sprintf("invalid VersificationID: %q", mt.ToSystem)))
	}

	// Validate mappings
	for i, m := range mt.Mappings {
		mPath := fmt.Sprintf("mappings[%d]", i)
		if m.From != nil {
			refErrs := validateRefFn(m.From)
			for _, err := range refErrs {
				var ve *ValidationError
				if errors.As(err, &ve) {
					errs = append(errs, newValidationError(
						fmt.Sprintf("%s.from", mPath), ve.Message))
				} else {
					errs = append(errs, err)
				}
			}
		}
		if m.To != nil {
			refErrs := validateRefFn(m.To)
			for _, err := range refErrs {
				var ve *ValidationError
				if errors.As(err, &ve) {
					errs = append(errs, newValidationError(
						fmt.Sprintf("%s.to", mPath), ve.Message))
				} else {
					errs = append(errs, err)
				}
			}
		}
	}

	return errs
}

// Validate validates the entire corpus and returns all validation errors.
// This is a convenience function that calls ValidateCorpus.
func Validate(c *Corpus) []error {
	return ValidateCorpus(c)
}

// IsValid returns true if the corpus has no validation errors.
func IsValid(c *Corpus) bool {
	return len(Validate(c)) == 0
}

// EmptyTextResult describes a content block with empty text.
type EmptyTextResult struct {
	DocumentID     string
	ContentBlockID string
	RawMarkup      string // From Attributes["raw_markup"] if present
	Reason         string
	IsPurposeful   bool
}

// ValidateEmptyTextFields checks all content blocks for empty text fields and
// determines if they are purposeful (structural markup only) or potential data corruption.
// Returns a slice of results describing each empty text field found.
func ValidateEmptyTextFields(c *Corpus) []EmptyTextResult {
	var results []EmptyTextResult

	for _, doc := range c.Documents {
		for _, cb := range doc.ContentBlocks {
			if cb.Text == "" {
				result := EmptyTextResult{
					DocumentID:     doc.ID,
					ContentBlockID: cb.ID,
				}

				// Get raw markup from attributes if present
				if cb.Attributes != nil {
					if rawMarkup, ok := cb.Attributes["raw_markup"].(string); ok {
						result.RawMarkup = rawMarkup
					}
				}

				// Check if raw markup contains structural markers only
				reason, isPurposeful := analyzeEmptyText(result.RawMarkup)
				result.Reason = reason
				result.IsPurposeful = isPurposeful

				results = append(results, result)
			}
		}
	}

	return results
}

// analyzeEmptyText determines why a text field is empty and whether it's purposeful.
// Returns a reason description and true if the empty text is expected/valid.
func analyzeEmptyText(rawMarkup string) (reason string, isPurposeful bool) {
	if rawMarkup == "" {
		return "no raw markup present - possible data loss", false
	}

	// Check for common structural OSIS/ThML markers
	hasChapterMarker := containsOSISMarker(rawMarkup, "chapter")
	hasBookMarker := containsOSISMarker(rawMarkup, "div") && containsAttr(rawMarkup, "type=\"book\"")
	hasSectionMarker := containsOSISMarker(rawMarkup, "div") && containsAttr(rawMarkup, "type=\"section\"")
	hasMilestone := containsOSISMarker(rawMarkup, "milestone")

	// If it's only structural markup, it's purposeful
	if hasChapterMarker && !containsActualText(rawMarkup) {
		return "chapter boundary marker (versification difference)", true
	}
	if hasBookMarker && !containsActualText(rawMarkup) {
		return "book boundary marker", true
	}
	if hasSectionMarker && !containsActualText(rawMarkup) {
		return "section boundary marker", true
	}
	if hasMilestone && !containsActualText(rawMarkup) {
		return "milestone marker only", true
	}

	// If raw markup exists but produces empty text, check if it's just whitespace/markup
	if !containsActualText(rawMarkup) {
		return "markup-only content (no actual text)", true
	}

	// Raw markup has content but text is empty - possible stripping error
	return "raw markup contains text but stripped result is empty - possible parsing error", false
}

// containsOSISMarker checks if markup contains an OSIS element.
func containsOSISMarker(markup, element string) bool {
	return contains(markup, "<"+element) || contains(markup, "</"+element)
}

// containsAttr checks if markup contains an attribute pattern.
func containsAttr(markup, attr string) bool {
	return contains(markup, attr)
}

// contains is a simple string contains helper.
func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// containsActualText checks if markup contains text outside of tags.
func containsActualText(markup string) bool {
	inTag := false
	hasText := false

	for i := 0; i < len(markup); i++ {
		c := markup[i]
		if c == '<' {
			inTag = true
			continue
		}
		if c == '>' {
			inTag = false
			continue
		}
		if !inTag && c != ' ' && c != '\t' && c != '\n' && c != '\r' {
			hasText = true
			break
		}
	}

	return hasText
}

// ValidateNoUnexpectedEmptyText returns errors for any content blocks with empty text
// that cannot be explained by structural markup. This is useful for catching data corruption.
func ValidateNoUnexpectedEmptyText(c *Corpus) []error {
	var errs []error

	results := ValidateEmptyTextFields(c)
	for _, r := range results {
		if !r.IsPurposeful {
			errs = append(errs, newValidationError(
				fmt.Sprintf("document[%s].content_block[%s]", r.DocumentID, r.ContentBlockID),
				fmt.Sprintf("unexpected empty text: %s", r.Reason)))
		}
	}

	return errs
}
