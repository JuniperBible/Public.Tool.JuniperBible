// Package types re-exports IPC types for SDK consumers.
// This provides a stable API surface while keeping IPC as the canonical source.
package types

import "github.com/JuniperBible/Public.Tool.JuniperBible/plugins/ipc"

// Request/Response types
type (
	// Request is the IPC request envelope.
	Request = ipc.Request

	// Response is the IPC response envelope.
	Response = ipc.Response
)

// Detection types
type (
	// DetectResult contains format detection results.
	DetectResult = ipc.DetectResult
)

// Ingest types
type (
	// IngestResult contains blob ingestion results.
	IngestResult = ipc.IngestResult
)

// Enumeration types
type (
	// EnumerateResult contains file/directory listing results.
	EnumerateResult = ipc.EnumerateResult

	// EnumerateEntry represents a single entry in an enumeration.
	EnumerateEntry = ipc.EnumerateEntry
)

// IR types
type (
	// Corpus is the root container for a Bible or text collection.
	Corpus = ipc.Corpus

	// Document represents a single book or document within a corpus.
	Document = ipc.Document

	// ContentBlock is a unit of content with stand-off markup.
	ContentBlock = ipc.ContentBlock

	// Token represents a word or punctuation unit.
	Token = ipc.Token

	// Anchor represents a named reference point in text.
	Anchor = ipc.Anchor

	// Span represents a range annotation over text.
	Span = ipc.Span

	// Ref represents a reference to another location.
	Ref = ipc.Ref
)

// Extraction/Emission types
type (
	// ExtractIRResult contains IR extraction results.
	ExtractIRResult = ipc.ExtractIRResult

	// EmitNativeResult contains native format emission results.
	EmitNativeResult = ipc.EmitNativeResult

	// LossReport describes conversion fidelity.
	LossReport = ipc.LossReport
)

// Tool types
type (
	// ToolInfo contains tool plugin metadata.
	ToolInfo = ipc.ToolInfo

	// ProfileInfo describes a tool execution profile.
	ProfileInfo = ipc.ProfileInfo

	// ToolRunRequest contains tool execution parameters.
	ToolRunRequest = ipc.ToolRunRequest
)

// Loss class constants
const (
	LossL0 = "L0" // Byte-for-byte round-trip (lossless)
	LossL1 = "L1" // Semantically lossless (formatting may differ)
	LossL2 = "L2" // Minor loss (some metadata/structure)
	LossL3 = "L3" // Significant loss (text preserved, markup lost)
	LossL4 = "L4" // Text-only (minimal preservation)
)

// Module type constants
const (
	ModuleBible      = "bible"
	ModuleCommentary = "commentary"
	ModuleDictionary = "dictionary"
	ModuleGenBook    = "genbook"
)
