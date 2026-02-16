//go:build !standalone

package format

import (
	"github.com/FocuswithJustin/JuniperBible/core/plugins"
	"github.com/FocuswithJustin/JuniperBible/plugins/ipc"
)

// registerEmbedded registers a format Config as an embedded plugin.
// This is only compiled when NOT building standalone plugins.
func registerEmbedded(cfg *Config) {
	if cfg.PluginID == "" {
		panic("format.Config.PluginID is required for embedded registration")
	}
	if cfg.Name == "" {
		panic("format.Config.Name is required for embedded registration")
	}
	if cfg.Version == "" {
		cfg.Version = "1.0.0" // Default version
	}

	// Auto-detect IR capabilities
	canExtractIR := cfg.Parse != nil
	canEmitNative := cfg.Emit != nil
	if cfg.CanExtractIR {
		canExtractIR = true
	}
	if cfg.CanEmitNative {
		canEmitNative = true
	}

	// Default loss class
	lossClass := cfg.LossClass
	if lossClass == "" {
		lossClass = "L1"
	}

	// Create manifest
	manifest := &plugins.PluginManifest{
		PluginID: cfg.PluginID,
		Version:  cfg.Version,
		Kind:     "format",
		Capabilities: plugins.Capabilities{
			Inputs:  cfg.Extensions,
			Outputs: cfg.Extensions,
		},
		IRSupport: &plugins.IRCapabilities{
			CanExtract: canExtractIR,
			CanEmit:    canEmitNative,
			LossClass:  lossClass,
			Formats:    []string{cfg.Name},
		},
	}

	// Create embedded plugin
	ep := &plugins.EmbeddedPlugin{
		Manifest: manifest,
		Format:   &embeddedFormatAdapter{cfg: cfg},
	}

	// Register
	plugins.RegisterEmbeddedPlugin(ep)
}

// embeddedFormatAdapter adapts a format.Config to the plugins.EmbeddedFormatHandler interface.
type embeddedFormatAdapter struct {
	cfg *Config
}

// Detect implements plugins.EmbeddedFormatHandler.
func (a *embeddedFormatAdapter) Detect(path string) (*plugins.DetectResult, error) {
	handler := makeDetectHandler(a.cfg)
	result, err := handler(map[string]interface{}{"path": path})
	if err != nil {
		return nil, err
	}

	// Convert ipc.DetectResult to plugins.DetectResult
	ipcResult := result.(*ipc.DetectResult)
	return &plugins.DetectResult{
		Detected: ipcResult.Detected,
		Format:   ipcResult.Format,
		Reason:   ipcResult.Reason,
	}, nil
}

// Ingest implements plugins.EmbeddedFormatHandler.
func (a *embeddedFormatAdapter) Ingest(path, outputDir string) (*plugins.IngestResult, error) {
	handler := makeIngestHandler(a.cfg)
	result, err := handler(map[string]interface{}{
		"path":       path,
		"output_dir": outputDir,
	})
	if err != nil {
		return nil, err
	}

	// Convert ipc.IngestResult to plugins.IngestResult
	ipcResult := result.(*ipc.IngestResult)
	return &plugins.IngestResult{
		ArtifactID: ipcResult.ArtifactID,
		BlobSHA256: ipcResult.BlobSHA256,
		SizeBytes:  ipcResult.SizeBytes,
		Metadata:   ipcResult.Metadata,
	}, nil
}

// Enumerate implements plugins.EmbeddedFormatHandler.
func (a *embeddedFormatAdapter) Enumerate(path string) (*plugins.EnumerateResult, error) {
	handler := makeEnumerateHandler(a.cfg)
	result, err := handler(map[string]interface{}{"path": path})
	if err != nil {
		return nil, err
	}

	// Convert ipc.EnumerateResult to plugins.EnumerateResult
	ipcResult := result.(*ipc.EnumerateResult)
	entries := make([]plugins.EnumerateEntry, len(ipcResult.Entries))
	for i, e := range ipcResult.Entries {
		entries[i] = plugins.EnumerateEntry{
			Path:      e.Path,
			SizeBytes: e.SizeBytes,
			IsDir:     e.IsDir,
			Metadata:  e.Metadata,
		}
	}
	return &plugins.EnumerateResult{Entries: entries}, nil
}

// ExtractIR implements plugins.EmbeddedFormatHandler.
func (a *embeddedFormatAdapter) ExtractIR(path, outputDir string) (*plugins.ExtractIRResult, error) {
	handler := makeExtractIRHandler(a.cfg)
	result, err := handler(map[string]interface{}{
		"path":       path,
		"output_dir": outputDir,
	})
	if err != nil {
		return nil, err
	}

	// Convert ipc.ExtractIRResult to plugins.ExtractIRResult
	ipcResult := result.(*ipc.ExtractIRResult)
	pluginResult := &plugins.ExtractIRResult{
		IRPath:    ipcResult.IRPath,
		LossClass: ipcResult.LossClass,
	}

	if ipcResult.LossReport != nil {
		pluginResult.LossReport = convertLossReport(ipcResult.LossReport)
	}

	return pluginResult, nil
}

// EmitNative implements plugins.EmbeddedFormatHandler.
func (a *embeddedFormatAdapter) EmitNative(irPath, outputDir string) (*plugins.EmitNativeResult, error) {
	handler := makeEmitNativeHandler(a.cfg)
	result, err := handler(map[string]interface{}{
		"ir_path":    irPath,
		"output_dir": outputDir,
	})
	if err != nil {
		return nil, err
	}

	// Convert ipc.EmitNativeResult to plugins.EmitNativeResult
	ipcResult := result.(*ipc.EmitNativeResult)
	pluginResult := &plugins.EmitNativeResult{
		OutputPath: ipcResult.OutputPath,
		Format:     ipcResult.Format,
		LossClass:  ipcResult.LossClass,
	}

	if ipcResult.LossReport != nil {
		pluginResult.LossReport = convertLossReport(ipcResult.LossReport)
	}

	return pluginResult, nil
}

// convertLossReport converts ipc.LossReport to plugins.LossReportIPC.
func convertLossReport(ipcReport *ipc.LossReport) *plugins.LossReportIPC {
	if ipcReport == nil {
		return nil
	}

	lostElements := make([]plugins.LostElementIPC, len(ipcReport.LostElements))
	for i, e := range ipcReport.LostElements {
		lostElements[i] = plugins.LostElementIPC{
			Path:          e.Path,
			ElementType:   e.ElementType,
			Reason:        e.Reason,
			OriginalValue: e.OriginalValue,
		}
	}

	return &plugins.LossReportIPC{
		SourceFormat: ipcReport.SourceFormat,
		TargetFormat: ipcReport.TargetFormat,
		LossClass:    ipcReport.LossClass,
		LostElements: lostElements,
		Warnings:     ipcReport.Warnings,
	}
}
