// Package ipc provides core IPC types used across all mimicry plugins.
//
// This file consolidates type aliases and documentation for the most commonly
// used IPC protocol types. The actual type definitions are organized as follows:
//
//   - protocol.go: Request, Response, basic result types (Detect, Ingest, Enumerate)
//   - results.go: IR conversion result types (ExtractIR, EmitNative, LossReport)
//   - ir.go: IR structure types (Corpus, Document, ContentBlock, etc.)
//   - tool_base.go: Tool plugin types (ToolInfo, ProfileInfo, etc.)
//
// Plugins migrating from local type definitions can use these shared types
// to eliminate code duplication. See protocol.go for helper functions like
// ReadRequest(), Respond(), and RespondError().
package ipc

// Backward Compatibility Aliases
//
// Many plugins define local "IPCRequest" and "IPCResponse" types.
// These aliases allow gradual migration to the shared types without
// breaking existing plugin code. New plugins should use Request and Response directly.

// IPCRequest is a deprecated alias for Request.
// Maintained for backward compatibility with plugins that haven't migrated yet.
//
// Deprecated: Use Request instead (defined in protocol.go).
type IPCRequest = Request

// IPCResponse is a deprecated alias for Response.
// Maintained for backward compatibility with plugins that haven't migrated yet.
//
// Deprecated: Use Response instead (defined in protocol.go).
type IPCResponse = Response

// Core Type Reference
//
// The following comments document all core IPC types available in this package.
// Import "github.com/JuniperBible/Public.Tool.JuniperBible/plugins/ipc" to use these types.

// Request types (defined in protocol.go):
//   - Request: Incoming JSON request envelope (command + args)
//   - Response: Outgoing JSON response envelope (status + result/error)

// Command result types (defined in protocol.go):
//   - DetectResult: Format detection result (detected + format + reason)
//   - IngestResult: Artifact ingestion result (artifact_id + blob_sha256 + size)
//   - EnumerateResult: File listing result (entries slice)
//   - EnumerateEntry: Single file entry (path + size + is_dir + metadata)

// IR conversion result types (defined in results.go):
//   - ExtractIRResult: IR extraction result (ir_path + loss_class + loss_report)
//   - EmitNativeResult: Native format emission result (output_path + format + loss_class)
//   - LossReport: Conversion loss analysis (source/target formats + loss_class + details)
//   - LostElement: Specific lost element (path + type + reason + original_value)
//   - EmitResult: Multi-file emission result (files + loss_report)
//   - EmittedFile: Single emitted file (path + format + size)

// IR structure types (defined in ir.go):
//   - Corpus: Complete text collection (Bible, commentary, etc.)
//   - Document: Single document within corpus (e.g., Bible book)
//   - ContentBlock: Unit of content with stand-off markup
//   - Token: Tokenized word or morpheme
//   - Anchor: Position in text where spans attach
//   - Span: Markup spanning from one anchor to another
//   - Ref: Biblical or textual reference
//   - ParallelCorpus: Multiple aligned corpora
//   - CorpusRef: Reference to corpus in parallel corpus
//   - Alignment: Alignment between corpora
//   - AlignedUnit: Single aligned unit across translations
//   - TokenAlignment: Word-level alignment
//   - InterlinearLine: Line of interlinear text
//   - InterlinearLayer: One layer of interlinear text

// Tool plugin types (defined in tool_base.go):
//   - ToolInfo: Tool plugin metadata (name + version + profiles + requires)
//   - ProfileInfo: Tool profile description (id + description)
//   - ToolRunRequest: Standard tool execution request (profile + args + out_dir)
//   - ToolIPCRequest: Tool IPC request format (command + path + args)
//   - ToolIPCResponse: Tool IPC response format (success + data + error)
//   - TranscriptEvent: Standard transcript event (event + timestamp + data)
//   - ProfileHandler: Profile execution function type
//   - ToolConfig: Tool plugin configuration

// Helper functions (defined in protocol.go):
//   - ReadRequest() (*Request, error): Read and decode IPC request from stdin
//   - Respond(result interface{}) error: Write success response to stdout
//   - RespondError(msg string) error: Write error response to stdout (doesn't exit)
//   - RespondErrorAndExit(msg string): Write error response and exit(1)
//   - RespondErrorf(format string, args ...interface{}) error: Formatted error
//   - RespondErrorfAndExit(format string, args ...interface{}): Formatted error + exit
//   - MustRespond(result interface{}): Respond or exit on error

// Helper functions (defined in args.go):
//   - StringArg(args, name) (string, error): Extract required string argument
//   - StringArgOr(args, name, default) string: Extract optional string argument
//   - BoolArg(args, name, default) bool: Extract optional bool argument
//   - PathAndOutputDir(args) (path, outputDir string, err error): Extract common args
//   - StoreBlob(outputDir, data) (hashHex string, err error): Store content-addressed blob
//   - ArtifactIDFromPath(path) string: Extract artifact ID from file path

// Helper functions (defined in detect_helpers.go):
//   - DetectByExtension(path, exts) *DetectResult: Detect format by file extension
//   - DetectByMagicBytes(path, patterns) *DetectResult: Detect by magic byte patterns
//   - DetectByXMLRoot(path, rootElement, namespace) *DetectResult: Detect by XML root
//   - And more detection helpers...

// Tool plugin helpers (defined in tool_base.go):
//   - PrintToolInfo(info ToolInfo): Output tool metadata as JSON
//   - RunStandardToolIPC(config *ToolConfig): Run standard tool IPC loop
//   - ParseToolFlags() (reqPath, outDir string): Parse standard tool flags
//   - LoadToolRequest(reqPath, outDir) *ToolRunRequest: Load tool request from JSON
//   - ExecuteWithTranscript(req, config): Execute tool profile with transcript
