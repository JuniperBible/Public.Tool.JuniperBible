// Package format provides helpers for building format plugins.
package format

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/FocuswithJustin/JuniperBible/plugins/ipc"
	"github.com/FocuswithJustin/JuniperBible/plugins/sdk/blob"
	"github.com/FocuswithJustin/JuniperBible/plugins/sdk/errors"
	"github.com/FocuswithJustin/JuniperBible/plugins/sdk/ir"
	"github.com/FocuswithJustin/JuniperBible/plugins/sdk/runtime"
)

// Config defines a format plugin's capabilities and handlers.
type Config struct {
	// Format name (e.g., "JSON", "OSIS", "USFM")
	Name string

	// File extensions this format handles (e.g., []string{".json", ".JSON"})
	Extensions []string

	// MagicBytes to check at file start (optional)
	MagicBytes []byte

	// Detect performs custom format detection.
	// If nil, uses extension and magic byte checking.
	Detect func(path string) (*ipc.DetectResult, error)

	// Parse parses a file and returns an IR Corpus.
	// Required for extract-ir support.
	Parse func(path string) (*ir.Corpus, error)

	// Emit converts an IR Corpus to native format.
	// Required for emit-native support.
	Emit func(corpus *ir.Corpus, outputDir string) (string, error)

	// Enumerate lists contents (for archive formats).
	// If nil, returns empty list (single-file format).
	Enumerate func(path string) (*ipc.EnumerateResult, error)

	// IngestTransform optionally transforms content before blob storage.
	// If nil, stores file content as-is.
	IngestTransform func(path string) ([]byte, map[string]string, error)

	// PluginID for embedded registration (e.g., "format.json")
	// Required for RegisterEmbedded() to work.
	PluginID string

	// Version for manifest (e.g., "1.0.0")
	Version string

	// LossClass default (L0, L1, L2, L3, L4)
	// Indicates expected data loss during conversions.
	LossClass string

	// CanExtractIR indicates if this format supports extract-ir.
	// Automatically set to true if Parse is non-nil.
	CanExtractIR bool

	// CanEmitNative indicates if this format supports emit-native.
	// Automatically set to true if Emit is non-nil.
	CanEmitNative bool
}

// Run starts a format plugin with the given configuration.
func Run(cfg *Config) error {
	return runtime.RunDispatcher(func(d *runtime.Dispatcher) {
		d.Register("detect", makeDetectHandler(cfg))
		d.Register("ingest", makeIngestHandler(cfg))
		d.Register("enumerate", makeEnumerateHandler(cfg))
		d.Register("extract-ir", makeExtractIRHandler(cfg))
		d.Register("emit-native", makeEmitNativeHandler(cfg))
	})
}

// makeDetectHandler creates a detect command handler.
func makeDetectHandler(cfg *Config) func(map[string]interface{}) (interface{}, error) {
	return func(args map[string]interface{}) (interface{}, error) {
		path, ok := args["path"].(string)
		if !ok || path == "" {
			return nil, errors.MissingArg("path")
		}

		// Custom detection
		if cfg.Detect != nil {
			return cfg.Detect(path)
		}

		// Standard detection
		return standardDetect(cfg, path)
	}
}

// standardDetect performs extension and magic byte detection.
func standardDetect(cfg *Config, path string) (*ipc.DetectResult, error) {
	ext := strings.ToLower(filepath.Ext(path))
	for _, e := range cfg.Extensions {
		if strings.ToLower(e) == ext {
			return &ipc.DetectResult{
				Detected: true,
				Format:   cfg.Name,
				Reason:   cfg.Name + " format detected via extension",
			}, nil
		}
	}

	return &ipc.DetectResult{
		Detected: false,
		Reason:   "Extension does not match " + cfg.Name + " format",
	}, nil
}

// makeIngestHandler creates an ingest command handler.
func makeIngestHandler(cfg *Config) func(map[string]interface{}) (interface{}, error) {
	return func(args map[string]interface{}) (interface{}, error) {
		path, ok := args["path"].(string)
		if !ok || path == "" {
			return nil, errors.MissingArg("path")
		}
		outputDir, ok := args["output_dir"].(string)
		if !ok || outputDir == "" {
			return nil, errors.MissingArg("output_dir")
		}

		var data []byte
		var metadata map[string]string
		var err error

		if cfg.IngestTransform != nil {
			data, metadata, err = cfg.IngestTransform(path)
			if err != nil {
				return nil, errors.FileReadError(path, err)
			}
		} else {
			data, err = os.ReadFile(path)
			if err != nil {
				return nil, errors.FileReadError(path, err)
			}
			metadata = map[string]string{}
		}

		// Ensure format is in metadata
		if metadata == nil {
			metadata = map[string]string{}
		}
		if _, ok := metadata["format"]; !ok {
			metadata["format"] = cfg.Name
		}

		// Store blob
		hash, size, err := blob.Store(outputDir, data)
		if err != nil {
			return nil, errors.StorageError(err)
		}

		return &ipc.IngestResult{
			ArtifactID: blob.ArtifactIDFromPath(path),
			BlobSHA256: hash,
			SizeBytes:  size,
			Metadata:   metadata,
		}, nil
	}
}

// makeEnumerateHandler creates an enumerate command handler.
func makeEnumerateHandler(cfg *Config) func(map[string]interface{}) (interface{}, error) {
	return func(args map[string]interface{}) (interface{}, error) {
		path, ok := args["path"].(string)
		if !ok || path == "" {
			return nil, errors.MissingArg("path")
		}

		// Custom enumeration
		if cfg.Enumerate != nil {
			return cfg.Enumerate(path)
		}

		// Single-file formats return empty list
		return &ipc.EnumerateResult{
			Entries: []ipc.EnumerateEntry{},
		}, nil
	}
}

// makeExtractIRHandler creates an extract-ir command handler.
func makeExtractIRHandler(cfg *Config) func(map[string]interface{}) (interface{}, error) {
	return func(args map[string]interface{}) (interface{}, error) {
		path, ok := args["path"].(string)
		if !ok || path == "" {
			return nil, errors.MissingArg("path")
		}
		outputDir, ok := args["output_dir"].(string)
		if !ok || outputDir == "" {
			return nil, errors.MissingArg("output_dir")
		}

		if cfg.Parse == nil {
			return nil, errors.New(errors.CodeInternal, "format does not support IR extraction")
		}

		// Parse to IR
		corpus, err := cfg.Parse(path)
		if err != nil {
			return nil, errors.ParseError(cfg.Name, err)
		}

		// Set source format if not set
		if corpus.SourceFormat == "" {
			corpus.SourceFormat = cfg.Name
		}

		// Write IR
		irPath, err := ir.Write(corpus, outputDir)
		if err != nil {
			return nil, errors.IRWriteError(outputDir, err)
		}

		return &ipc.ExtractIRResult{
			IRPath:    irPath,
			LossClass: determineLossClass(corpus),
			LossReport: &ipc.LossReport{
				SourceFormat: cfg.Name,
				TargetFormat: "IR",
				LossClass:    determineLossClass(corpus),
			},
		}, nil
	}
}

// makeEmitNativeHandler creates an emit-native command handler.
func makeEmitNativeHandler(cfg *Config) func(map[string]interface{}) (interface{}, error) {
	return func(args map[string]interface{}) (interface{}, error) {
		irPath, ok := args["ir_path"].(string)
		if !ok || irPath == "" {
			return nil, errors.MissingArg("ir_path")
		}
		outputDir, ok := args["output_dir"].(string)
		if !ok || outputDir == "" {
			return nil, errors.MissingArg("output_dir")
		}

		if cfg.Emit == nil {
			return nil, errors.New(errors.CodeInternal, "format does not support native emission")
		}

		// Read IR
		corpus, err := ir.Read(irPath)
		if err != nil {
			return nil, errors.IRReadError(irPath, err)
		}

		// Emit native format
		outputPath, err := cfg.Emit(corpus, outputDir)
		if err != nil {
			return nil, errors.FileWriteError(outputDir, err)
		}

		return &ipc.EmitNativeResult{
			OutputPath: outputPath,
			Format:     cfg.Name,
			LossClass:  determineLossClass(corpus),
			LossReport: &ipc.LossReport{
				SourceFormat: "IR",
				TargetFormat: cfg.Name,
				LossClass:    determineLossClass(corpus),
			},
		}, nil
	}
}

// determineLossClass determines the loss class from a corpus.
func determineLossClass(corpus *ir.Corpus) string {
	if corpus == nil {
		return "L4"
	}
	if corpus.LossClass != "" {
		return corpus.LossClass
	}
	return "L1" // Default to semantically lossless
}

// RegisterEmbedded registers this format as an embedded plugin.
// This allows the format to be used without a separate plugin process.
// The Config must have PluginID, Name, and Version set.
func (c *Config) RegisterEmbedded() {
	// This implementation is in embedded.go (for !standalone builds)
	// or embedded_stub.go (for standalone builds)
	registerEmbedded(c)
}
