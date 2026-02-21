package capsule

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/JuniperBible/Public.Tool.JuniperBible/core/errors"
	"github.com/JuniperBible/Public.Tool.JuniperBible/core/ir"
	"github.com/JuniperBible/Public.Tool.JuniperBible/core/plugins"
)

// Injectable functions for testing
var (
	pluginsExecutePlugin         = plugins.ExecutePlugin
	pluginsParseExtractIRResult  = plugins.ParseExtractIRResult
	pluginsParseEmitNativeResult = plugins.ParseEmitNativeResult
	osMkdirTemp                  = os.MkdirTemp
	osRemoveAll                  = os.RemoveAll
	osWriteFileExport            = os.WriteFile
	osReadFileExport             = os.ReadFile
	osMkdirAllExport             = os.MkdirAll
	osWriteFileIdentity          = os.WriteFile
)

// ExportMode defines how an artifact should be exported.
type ExportMode string

const (
	// ExportModeIdentity exports the original bytes verbatim.
	ExportModeIdentity ExportMode = "IDENTITY"

	// ExportModeDerived exports a derived artifact (future use).
	ExportModeDerived ExportMode = "DERIVED"
)

// Export exports an artifact to the given path.
// In IDENTITY mode, it writes the original bytes verbatim, ensuring byte-for-byte preservation.
func (c *Capsule) Export(artifactID string, mode ExportMode, destPath string) error {
	// Find the artifact
	artifact, ok := c.Manifest.Artifacts[artifactID]
	if !ok {
		return errors.NewNotFound("artifact", artifactID)
	}

	switch mode {
	case ExportModeIdentity:
		return c.exportIdentity(artifact, destPath)
	case ExportModeDerived:
		return errors.NewUnsupported("export mode", "DERIVED export mode not yet implemented")
	default:
		return errors.NewValidation("export mode", fmt.Sprintf("unknown export mode: %s", mode))
	}
}

// exportIdentity exports an artifact's original bytes verbatim.
func (c *Capsule) exportIdentity(artifact *Artifact, destPath string) error {
	// Retrieve the blob using the primary SHA-256 hash
	data, err := c.store.Retrieve(artifact.PrimaryBlobSHA256)
	if err != nil {
		return fmt.Errorf("failed to retrieve blob: %w", err)
	}

	// Ensure parent directory exists
	if err := osMkdirAllExport(filepath.Dir(destPath), 0700); err != nil {
		return fmt.Errorf("failed to create parent directory: %w", err)
	}

	// Write the exact bytes - no transformation
	if err := osWriteFileIdentity(destPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// ExportToBytes exports an artifact and returns its bytes instead of writing to a file.
// Useful for in-memory operations and testing.
func (c *Capsule) ExportToBytes(artifactID string, mode ExportMode) ([]byte, error) {
	artifact, ok := c.Manifest.Artifacts[artifactID]
	if !ok {
		return nil, errors.NewNotFound("artifact", artifactID)
	}

	switch mode {
	case ExportModeIdentity:
		return c.store.Retrieve(artifact.PrimaryBlobSHA256)
	case ExportModeDerived:
		return nil, errors.NewUnsupported("export mode", "DERIVED export mode not yet implemented")
	default:
		return nil, errors.NewValidation("export mode", fmt.Sprintf("unknown export mode: %s", mode))
	}
}

// DerivedExportOptions configures a derived export operation.
type DerivedExportOptions struct {
	// TargetFormat is the desired output format (e.g., "osis", "usfm", "json").
	TargetFormat string

	// PluginLoader provides access to format plugins.
	PluginLoader *plugins.Loader

	// SourcePlugin overrides automatic source plugin detection.
	SourcePlugin *plugins.Plugin

	// TargetPlugin overrides automatic target plugin detection.
	TargetPlugin *plugins.Plugin
}

// DerivedExportResult contains the results of a derived export.
type DerivedExportResult struct {
	// OutputPath is where the derived file was written.
	OutputPath string

	// LossReports contains the loss reports from each conversion step.
	LossReports []*ir.LossReport

	// CombinedLossClass is the overall loss class (worst of all steps).
	CombinedLossClass ir.LossClass

	// IRBlobSHA256 is the hash of the intermediate IR (if preserved).
	IRBlobSHA256 string
}

// ExportDerived exports an artifact to a different format via the IR.
// The conversion flow is: Source Format -> extract-ir -> IR -> emit-native -> Target Format.
func (c *Capsule) ExportDerived(artifactID string, opts DerivedExportOptions, destPath string) (*DerivedExportResult, error) {
	if err := validateDerivedExportOpts(opts); err != nil {
		return nil, err
	}

	artifact, ok := c.Manifest.Artifacts[artifactID]
	if !ok {
		return nil, errors.NewNotFound("artifact", artifactID)
	}

	return c.runDerivedConversion(artifact, opts, destPath)
}

// runDerivedConversion orchestrates temp-dir setup, plugin execution, and output
// copy for a single derived-export operation.
func (c *Capsule) runDerivedConversion(artifact *Artifact, opts DerivedExportOptions, destPath string) (*DerivedExportResult, error) {
	tempDir, err := osMkdirTemp("", "capsule-derived-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer osRemoveAll(tempDir)

	sourcePath, sourceFormat, err := c.writeSourceToTemp(artifact, tempDir)
	if err != nil {
		return nil, err
	}

	sourcePlugin, targetPlugin, err := resolveDerivedPlugins(opts, sourceFormat)
	if err != nil {
		return nil, err
	}

	lossReports, err := runConversionPipeline(sourcePlugin, targetPlugin, sourcePath, tempDir, destPath)
	if err != nil {
		return nil, err
	}

	return &DerivedExportResult{
		OutputPath:        destPath,
		LossReports:       lossReports,
		CombinedLossClass: combineLossClasses(lossReports),
	}, nil
}

// runConversionPipeline runs extract-ir -> emit-native -> copy-to-dest and
// returns the combined loss reports.
func runConversionPipeline(srcPlugin, tgtPlugin *plugins.Plugin, sourcePath, tempDir, destPath string) ([]*ir.LossReport, error) {
	extractResult, extractLoss, err := runExtractIR(srcPlugin, sourcePath, tempDir)
	if err != nil {
		return nil, err
	}

	emitResult, emitLoss, err := runEmitNative(tgtPlugin, extractResult.IRPath, tempDir)
	if err != nil {
		return nil, err
	}

	if err := copyOutputToDestination(emitResult.OutputPath, destPath); err != nil {
		return nil, err
	}

	return collectLossReports(extractLoss, emitLoss), nil
}

// validateDerivedExportOpts checks that the required options are set.
func validateDerivedExportOpts(opts DerivedExportOptions) error {
	if opts.PluginLoader == nil && (opts.SourcePlugin == nil || opts.TargetPlugin == nil) {
		return errors.NewValidation("DerivedExportOptions", "requires PluginLoader or both SourcePlugin and TargetPlugin")
	}
	if opts.TargetFormat == "" {
		return errors.NewValidation("TargetFormat", "is required")
	}
	return nil
}

// writeSourceToTemp retrieves the artifact blob and writes it to a temp file.
// Returns the temp file path and the detected source format.
func (c *Capsule) writeSourceToTemp(artifact *Artifact, tempDir string) (sourcePath, sourceFormat string, err error) {
	sourceFormat = ""
	if artifact.Detected != nil {
		sourceFormat = artifact.Detected.FormatID
	}

	sourcePath = filepath.Join(tempDir, "source")
	data, err := c.store.Retrieve(artifact.PrimaryBlobSHA256)
	if err != nil {
		return "", "", fmt.Errorf("failed to retrieve source blob: %w", err)
	}
	if err := osWriteFileExport(sourcePath, data, 0600); err != nil {
		return "", "", fmt.Errorf("failed to write source file: %w", err)
	}
	return sourcePath, sourceFormat, nil
}

// resolveDerivedPlugins returns the source and target plugins, looking them up
// via the loader when they are not explicitly provided in opts.
func resolveDerivedPlugins(opts DerivedExportOptions, sourceFormat string) (sourcePlugin, targetPlugin *plugins.Plugin, err error) {
	sourcePlugin = opts.SourcePlugin
	if sourcePlugin == nil {
		sourcePlugin, err = findPluginForFormat(opts.PluginLoader, sourceFormat)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to find source plugin for format %q: %w", sourceFormat, err)
		}
	}

	targetPlugin = opts.TargetPlugin
	if targetPlugin == nil {
		targetPlugin, err = findPluginForFormat(opts.PluginLoader, opts.TargetFormat)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to find target plugin for format %q: %w", opts.TargetFormat, err)
		}
	}
	return sourcePlugin, targetPlugin, nil
}

// runExtractIR creates the IR staging directory and calls the source plugin.
func runExtractIR(plugin *plugins.Plugin, sourcePath, tempDir string) (*plugins.ExtractIRResult, *ir.LossReport, error) {
	irDir := filepath.Join(tempDir, "ir")
	if err := osMkdirAllExport(irDir, 0700); err != nil {
		return nil, nil, fmt.Errorf("failed to create IR dir: %w", err)
	}
	result, loss, err := extractIRFromPlugin(plugin, sourcePath, irDir)
	if err != nil {
		return nil, nil, fmt.Errorf("extract-ir failed: %w", err)
	}
	return result, loss, nil
}

// runEmitNative creates the output staging directory and calls the target plugin.
func runEmitNative(plugin *plugins.Plugin, irPath, tempDir string) (*plugins.EmitNativeResult, *ir.LossReport, error) {
	outputDir := filepath.Join(tempDir, "output")
	if err := osMkdirAllExport(outputDir, 0700); err != nil {
		return nil, nil, fmt.Errorf("failed to create output dir: %w", err)
	}
	result, loss, err := emitNativeFromPlugin(plugin, irPath, outputDir)
	if err != nil {
		return nil, nil, fmt.Errorf("emit-native failed: %w", err)
	}
	return result, loss, nil
}

// copyOutputToDestination reads the plugin output file and writes it to destPath.
func copyOutputToDestination(outputPath, destPath string) error {
	if err := osMkdirAllExport(filepath.Dir(destPath), 0700); err != nil {
		return fmt.Errorf("failed to create destination dir: %w", err)
	}
	data, err := osReadFileExport(outputPath)
	if err != nil {
		return fmt.Errorf("failed to read output: %w", err)
	}
	if err := osWriteFileExport(destPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write destination: %w", err)
	}
	return nil
}

// collectLossReports builds the loss report slice, omitting nil entries.
func collectLossReports(reports ...*ir.LossReport) []*ir.LossReport {
	out := make([]*ir.LossReport, 0, len(reports))
	for _, r := range reports {
		if r != nil {
			out = append(out, r)
		}
	}
	return out
}

// findPluginForFormat finds a plugin that supports the given format.
func findPluginForFormat(loader *plugins.Loader, format string) (*plugins.Plugin, error) {
	if format == "" {
		return nil, errors.NewValidation("format", "cannot be empty")
	}

	// Look for a format plugin matching the format name
	pluginName := "format-" + format
	plugin, err := loader.GetPlugin(pluginName)
	if err == nil && plugin != nil {
		return plugin, nil
	}

	// Try without prefix
	plugin, err = loader.GetPlugin(format)
	if err == nil && plugin != nil {
		return plugin, nil
	}

	return nil, errors.NewNotFound("plugin", format)
}

// extractIRFromPlugin calls extract-ir on a plugin and returns the result.
func extractIRFromPlugin(plugin *plugins.Plugin, sourcePath, outputDir string) (*plugins.ExtractIRResult, *ir.LossReport, error) {
	req := plugins.NewExtractIRRequest(sourcePath, outputDir)
	resp, err := pluginsExecutePlugin(plugin, req)
	if err != nil {
		return nil, nil, err
	}

	result, err := pluginsParseExtractIRResult(resp)
	if err != nil {
		return nil, nil, err
	}

	var lossReport *ir.LossReport
	if result.LossReport != nil {
		lossReport = convertIPCLossReport(result.LossReport)
	}

	return result, lossReport, nil
}

// emitNativeFromPlugin calls emit-native on a plugin and returns the result.
func emitNativeFromPlugin(plugin *plugins.Plugin, irPath, outputDir string) (*plugins.EmitNativeResult, *ir.LossReport, error) {
	req := plugins.NewEmitNativeRequest(irPath, outputDir)
	resp, err := pluginsExecutePlugin(plugin, req)
	if err != nil {
		return nil, nil, err
	}

	result, err := pluginsParseEmitNativeResult(resp)
	if err != nil {
		return nil, nil, err
	}

	var lossReport *ir.LossReport
	if result.LossReport != nil {
		lossReport = convertIPCLossReport(result.LossReport)
	}

	return result, lossReport, nil
}

// convertIPCLossReport converts an IPC loss report to an IR loss report.
func convertIPCLossReport(ipc *plugins.LossReportIPC) *ir.LossReport {
	report := &ir.LossReport{
		SourceFormat: ipc.SourceFormat,
		TargetFormat: ipc.TargetFormat,
		LossClass:    ir.LossClass(ipc.LossClass),
		Warnings:     ipc.Warnings,
	}

	for _, elem := range ipc.LostElements {
		report.LostElements = append(report.LostElements, ir.LostElement{
			Path:          elem.Path,
			ElementType:   elem.ElementType,
			Reason:        elem.Reason,
			OriginalValue: elem.OriginalValue,
		})
	}

	return report
}

// combineLossClasses returns the worst (highest) loss class from a list of reports.
func combineLossClasses(reports []*ir.LossReport) ir.LossClass {
	worstLevel := 0
	for _, report := range reports {
		if report != nil && report.LossClass.Level() > worstLevel {
			worstLevel = report.LossClass.Level()
		}
	}
	return levelToLossClass(worstLevel)
}

// levelToLossClass converts an integer level to a LossClass.
// This is extracted for testability - if new loss levels are added,
// tests will fail to ensure this function is updated.
func levelToLossClass(level int) ir.LossClass {
	switch level {
	case 0:
		return ir.LossL0
	case 1:
		return ir.LossL1
	case 2:
		return ir.LossL2
	case 3:
		return ir.LossL3
	case 4:
		return ir.LossL4
	default:
		// This should be unreachable with current LossClass definitions.
		// If hit, a new loss level was added without updating this function.
		return ir.LossL0
	}
}

// ExportDerivedToBytes exports an artifact to a different format and returns the bytes.
func (c *Capsule) ExportDerivedToBytes(artifactID string, opts DerivedExportOptions) ([]byte, *DerivedExportResult, error) {
	tempDir, err := osMkdirTemp("", "capsule-derived-bytes-*")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer osRemoveAll(tempDir)

	destPath := filepath.Join(tempDir, "output")
	result, err := c.ExportDerived(artifactID, opts, destPath)
	if err != nil {
		return nil, nil, err
	}

	data, err := osReadFileExport(destPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read output: %w", err)
	}

	return data, result, nil
}

// CombinedLossReport creates a summary loss report from multiple conversion steps.
func CombinedLossReport(reports []*ir.LossReport) *ir.LossReport {
	if len(reports) == 0 {
		return &ir.LossReport{
			LossClass: ir.LossL0,
		}
	}

	combined := &ir.LossReport{
		SourceFormat: reports[0].SourceFormat,
		TargetFormat: reports[len(reports)-1].TargetFormat,
		LossClass:    combineLossClasses(reports),
	}

	for _, report := range reports {
		if report != nil {
			combined.LostElements = append(combined.LostElements, report.LostElements...)
			combined.Warnings = append(combined.Warnings, report.Warnings...)
		}
	}

	return combined
}

// Ensure json is used (for future serialization)
var _ = json.Marshal
