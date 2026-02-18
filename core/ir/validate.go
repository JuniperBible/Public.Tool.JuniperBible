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
	errs = validateCorpusFields(errs, c)
	errs = validateCorpusDocuments(errs, c.Documents)
	errs = validateCorpusMappingTables(errs, c.MappingTables)
	return errs
}

func validateCorpusFields(errs []error, c *Corpus) []error {
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
	return errs
}

func validateCorpusDocuments(errs []error, docs []*Document) []error {
	for i, doc := range docs {
		docPath := fmt.Sprintf("corpus.documents[%d]", i)
		errs = appendNestedErrors(errs, docPath, validateDocumentFn(doc))
	}
	return errs
}

func validateCorpusMappingTables(errs []error, tables []*MappingTable) []error {
	for i, mt := range tables {
		mtPath := fmt.Sprintf("corpus.mapping_tables[%d]", i)
		errs = appendNestedErrors(errs, mtPath, validateMappingTableFn(mt))
	}
	return errs
}

func appendNestedErrors(errs []error, parentPath string, childErrs []error) []error {
	for _, err := range childErrs {
		var ve *ValidationError
		if errors.As(err, &ve) {
			errs = append(errs, newValidationError(
				fmt.Sprintf("%s.%s", parentPath, ve.Path), ve.Message))
		} else {
			errs = append(errs, newValidationError(parentPath, err.Error()))
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
	errs = validateDocumentCanonicalRef(errs, d.CanonicalRef)
	errs = validateDocumentContentBlocks(errs, d.ContentBlocks)
	errs = validateDocumentAnnotations(errs, d.Annotations)
	return errs
}

func validateDocumentCanonicalRef(errs []error, ref *Ref) []error {
	if ref == nil {
		return errs
	}
	for _, err := range validateRefFn(ref) {
		var ve *ValidationError
		if errors.As(err, &ve) {
			errs = append(errs, newValidationError("document.canonical_ref", ve.Message))
		} else {
			errs = append(errs, err)
		}
	}
	return errs
}

func validateDocumentContentBlocks(errs []error, blocks []*ContentBlock) []error {
	for i, cb := range blocks {
		cbPath := fmt.Sprintf("content_blocks[%d]", i)
		errs = appendNestedErrors(errs, cbPath, validateContentBlockFn(cb))
	}
	return errs
}

func validateDocumentAnnotations(errs []error, annotations []*Annotation) []error {
	for i, ann := range annotations {
		annPath := fmt.Sprintf("annotations[%d]", i)
		errs = appendNestedErrors(errs, annPath, validateAnnotationFn(ann))
	}
	return errs
}

// ValidateContentBlock validates a ContentBlock and returns all validation errors.
func ValidateContentBlock(cb *ContentBlock) []error {
	var errs []error
	errs = validateContentBlockFields(errs, cb)
	errs = validateContentBlockTokens(errs, cb.Tokens)
	errs = validateContentBlockAnchors(errs, cb.Anchors)
	return errs
}

func validateContentBlockFields(errs []error, cb *ContentBlock) []error {
	if cb.ID == "" {
		errs = append(errs, newValidationError("content_block", "ID is required"))
	}
	if cb.Sequence < 0 {
		errs = append(errs, newValidationError("content_block.sequence",
			"Sequence cannot be negative"))
	}
	if cb.Hash != "" && !cb.VerifyHash() {
		errs = append(errs, newValidationError("content_block.hash",
			"Hash does not match content"))
	}
	return errs
}

func validateContentBlockTokens(errs []error, tokens []*Token) []error {
	for i, tok := range tokens {
		tokPath := fmt.Sprintf("tokens[%d]", i)
		errs = validateTokenBounds(errs, tokPath, tok)
	}
	return errs
}

func validateTokenBounds(errs []error, path string, tok *Token) []error {
	if tok.CharStart < 0 {
		errs = append(errs, newValidationError(path, "CharStart cannot be negative"))
	}
	if tok.CharEnd < tok.CharStart {
		errs = append(errs, newValidationError(path, "CharEnd cannot be before CharStart"))
	}
	return errs
}

func validateContentBlockAnchors(errs []error, anchors []*Anchor) []error {
	for i, anchor := range anchors {
		if anchor.CharOffset < 0 {
			anchorPath := fmt.Sprintf("anchors[%d]", i)
			errs = append(errs, newValidationError(anchorPath, "CharOffset cannot be negative"))
		}
	}
	return errs
}

// ValidateSpan validates a Span and returns all validation errors.
func ValidateSpan(s *Span) []error {
	var errs []error
	errs = validateSpanFields(errs, s)
	errs = validateSpanRef(errs, s.Ref)
	return errs
}

// validateSpanFields validates the required span fields.
func validateSpanFields(errs []error, s *Span) []error {
	if s.ID == "" {
		errs = append(errs, newValidationError("span", "ID is required"))
	}
	if s.Type != "" && !s.Type.IsValid() {
		errs = append(errs, newValidationError("span.type",
			fmt.Sprintf("invalid SpanType: %q", s.Type)))
	}
	if s.StartAnchorID == "" {
		errs = append(errs, newValidationError("span.start_anchor_id", "StartAnchorID is required"))
	}
	if s.EndAnchorID == "" {
		errs = append(errs, newValidationError("span.end_anchor_id", "EndAnchorID is required"))
	}
	return errs
}

// validateSpanRef validates the span's optional ref field.
func validateSpanRef(errs []error, ref *Ref) []error {
	if ref == nil {
		return errs
	}
	for _, err := range validateRefFn(ref) {
		var ve *ValidationError
		if errors.As(err, &ve) {
			errs = append(errs, newValidationError("span.ref", ve.Message))
		} else {
			errs = append(errs, err)
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

func validateMappingRef(ref *Ref, path string) []error {
	var errs []error
	for _, err := range validateRefFn(ref) {
		var ve *ValidationError
		if errors.As(err, &ve) {
			errs = append(errs, newValidationError(path, ve.Message))
		} else {
			errs = append(errs, err)
		}
	}
	return errs
}

func validateMapping(m *RefMapping, mPath string) []error {
	var errs []error
	if m.From != nil {
		errs = append(errs, validateMappingRef(m.From, fmt.Sprintf("%s.from", mPath))...)
	}
	if m.To != nil {
		errs = append(errs, validateMappingRef(m.To, fmt.Sprintf("%s.to", mPath))...)
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

	for i, m := range mt.Mappings {
		errs = append(errs, validateMapping(m, fmt.Sprintf("mappings[%d]", i))...)
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

type markerCheck struct {
	hasMarker bool
	reason    string
}

func buildMarkerChecks(rawMarkup string) []markerCheck {
	hasDiv := containsOSISMarker(rawMarkup, "div")
	return []markerCheck{
		{containsOSISMarker(rawMarkup, "chapter"), "chapter boundary marker (versification difference)"},
		{hasDiv && containsAttr(rawMarkup, "type=\"book\""), "book boundary marker"},
		{hasDiv && containsAttr(rawMarkup, "type=\"section\""), "section boundary marker"},
		{containsOSISMarker(rawMarkup, "milestone"), "milestone marker only"},
	}
}

func analyzeEmptyText(rawMarkup string) (reason string, isPurposeful bool) {
	if rawMarkup == "" {
		return "no raw markup present - possible data loss", false
	}

	if containsActualText(rawMarkup) {
		return "raw markup contains text but stripped result is empty - possible parsing error", false
	}

	for _, mc := range buildMarkerChecks(rawMarkup) {
		if mc.hasMarker {
			return mc.reason, true
		}
	}

	return "markup-only content (no actual text)", true
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
	for i := 0; i < len(markup); i++ {
		c := markup[i]
		inTag = updateTagState(c, inTag)
		if c == '<' || c == '>' {
			continue
		}
		if isActualTextChar(c, inTag) {
			return true
		}
	}
	return false
}

// updateTagState returns the new inTag state based on character.
func updateTagState(c byte, inTag bool) bool {
	if c == '<' {
		return true
	}
	if c == '>' {
		return false
	}
	return inTag
}

// isActualTextChar checks if c is a non-whitespace character outside a tag.
func isActualTextChar(c byte, inTag bool) bool {
	return !inTag && c != ' ' && c != '\t' && c != '\n' && c != '\r'
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
