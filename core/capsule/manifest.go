// Package capsule provides the core capsule data structures and operations.
// A capsule is a portable, immutable container for storing files and their
// behavioral transcripts from reference tools.
package capsule

import (
	"encoding/json"
	"time"

	"github.com/JuniperBible/juniper/core/ir"
)

// Version is the current capsule format version.
const Version = "1.0.0"

// ArtifactKindIR is the artifact kind for IR extractions.
const ArtifactKindIR = "ir"

// Manifest represents the capsule manifest (manifest.json).
type Manifest struct {
	CapsuleVersion string                `json:"capsule_version"`
	CreatedAt      string                `json:"created_at"`
	Tool           ToolInfo              `json:"tool"`
	Blobs          BlobIndex             `json:"blobs"`
	Artifacts      map[string]*Artifact  `json:"artifacts"`
	Runs           map[string]*Run       `json:"runs"`
	IRExtractions  map[string]*IRRecord  `json:"ir_extractions,omitempty"`
	RoundtripPlans map[string]*Plan      `json:"roundtrip_plans,omitempty"`
	SelfChecks     map[string]*SelfCheck `json:"self_checks,omitempty"`
	Exports        map[string]*Export    `json:"exports,omitempty"`
	Attributes     Attributes            `json:"attributes,omitempty"`
}

// ToolInfo describes the tool that created this capsule.
type ToolInfo struct {
	Name       string     `json:"name"`
	Version    string     `json:"version"`
	GitRev     string     `json:"git_rev,omitempty"`
	Attributes Attributes `json:"attributes,omitempty"`
}

// BlobIndex indexes blobs by their hash.
type BlobIndex struct {
	BySHA256 map[string]*BlobRecord `json:"by_sha256"`
	ByBLAKE3 map[string]*BlobRecord `json:"by_blake3,omitempty"`
}

// BlobRecord describes a blob in the capsule.
type BlobRecord struct {
	SHA256     string     `json:"sha256"`
	BLAKE3     string     `json:"blake3,omitempty"`
	SizeBytes  int64      `json:"size_bytes"`
	Path       string     `json:"path"`
	MIME       string     `json:"mime,omitempty"`
	Attributes Attributes `json:"attributes,omitempty"`
}

// Artifact represents a stored artifact.
type Artifact struct {
	ID                string              `json:"id"`
	Kind              string              `json:"kind"`
	OriginalName      string              `json:"original_name,omitempty"`
	SourcePath        string              `json:"source_path,omitempty"`
	PrimaryBlobSHA256 string              `json:"primary_blob_sha256"`
	Hashes            ArtifactHashes      `json:"hashes"`
	SizeBytes         int64               `json:"size_bytes,omitempty"`
	Detected          *DetectionResult    `json:"detected,omitempty"`
	Components        []ArtifactComponent `json:"components,omitempty"`
	Attributes        Attributes          `json:"attributes,omitempty"`
}

// ArtifactHashes contains the hashes for an artifact.
type ArtifactHashes struct {
	SHA256 string `json:"sha256"`
	BLAKE3 string `json:"blake3,omitempty"`
}

// DetectionResult describes format detection results.
type DetectionResult struct {
	FormatID   string     `json:"format_id,omitempty"`
	Confidence float64    `json:"confidence,omitempty"`
	Attributes Attributes `json:"attributes,omitempty"`
}

// ArtifactComponent describes a component of an artifact.
type ArtifactComponent struct {
	Path       string `json:"path"`
	ArtifactID string `json:"artifact_id"`
}

// Run describes a tool run.
type Run struct {
	ID         string      `json:"id"`
	Engine     *Engine     `json:"engine"`
	Plugin     *PluginInfo `json:"plugin"`
	Inputs     []RunInput  `json:"inputs"`
	Command    *Command    `json:"command,omitempty"`
	Outputs    *RunOutputs `json:"outputs"`
	Status     string      `json:"status"`
	Errors     []string    `json:"errors,omitempty"`
	Attributes Attributes  `json:"attributes,omitempty"`
}

// Engine describes the execution environment.
type Engine struct {
	EngineID   string     `json:"engine_id"`
	Type       string     `json:"type"`
	Nix        *NixConfig `json:"nix"`
	Env        *EnvConfig `json:"env"`
	Attributes Attributes `json:"attributes,omitempty"`
}

// NixConfig describes the Nix configuration.
type NixConfig struct {
	FlakeLockSHA256 string   `json:"flake_lock_sha256"`
	System          string   `json:"system"`
	Derivations     []string `json:"derivations"`
}

// EnvConfig describes environment settings.
type EnvConfig struct {
	TZ         string     `json:"TZ"`
	LCALL      string     `json:"LC_ALL"`
	LANG       string     `json:"LANG"`
	Attributes Attributes `json:"attributes,omitempty"`
}

// PluginInfo describes a plugin.
type PluginInfo struct {
	PluginID      string     `json:"plugin_id"`
	PluginVersion string     `json:"plugin_version"`
	Kind          string     `json:"kind"`
	Attributes    Attributes `json:"attributes,omitempty"`
}

// RunInput describes an input to a run.
type RunInput struct {
	ArtifactID string     `json:"artifact_id"`
	Role       string     `json:"role,omitempty"`
	Attributes Attributes `json:"attributes,omitempty"`
}

// Command describes a command invocation.
type Command struct {
	Argv       []string   `json:"argv,omitempty"`
	Profile    string     `json:"profile,omitempty"`
	Attributes Attributes `json:"attributes,omitempty"`
}

// RunOutputs describes the outputs of a run.
type RunOutputs struct {
	TranscriptBlobSHA256 string              `json:"transcript_blob_sha256"`
	Artifacts            []RunOutputArtifact `json:"artifacts,omitempty"`
	StdoutBlobSHA256     string              `json:"stdout_blob_sha256,omitempty"`
	StderrBlobSHA256     string              `json:"stderr_blob_sha256,omitempty"`
	Attributes           Attributes          `json:"attributes,omitempty"`
}

// RunOutputArtifact describes an output artifact from a run.
type RunOutputArtifact struct {
	ArtifactID string     `json:"artifact_id"`
	Label      string     `json:"label,omitempty"`
	Attributes Attributes `json:"attributes,omitempty"`
}

// Plan describes a round-trip plan.
type Plan struct {
	ID          string      `json:"id"`
	Description string      `json:"description"`
	Steps       []PlanStep  `json:"steps"`
	Checks      []PlanCheck `json:"checks"`
	Attributes  Attributes  `json:"attributes,omitempty"`
}

// PlanStep describes a step in a plan.
type PlanStep struct {
	Type       string       `json:"type"`
	RunTool    *RunToolStep `json:"run_tool,omitempty"`
	Export     *ExportStep  `json:"export,omitempty"`
	Label      string       `json:"label,omitempty"`
	Attributes Attributes   `json:"attributes,omitempty"`
}

// RunToolStep describes a RUN_TOOL step.
type RunToolStep struct {
	ToolPluginID string     `json:"tool_plugin_id"`
	Profile      string     `json:"profile"`
	Inputs       []string   `json:"inputs"`
	Args         Attributes `json:"args,omitempty"`
}

// ExportStep describes an EXPORT step.
type ExportStep struct {
	Mode       string `json:"mode"`
	ArtifactID string `json:"artifact_id"`
}

// PlanCheck describes a check in a plan.
type PlanCheck struct {
	Type            string           `json:"type"`
	ByteEqual       *ByteEqualCheck  `json:"byte_equal,omitempty"`
	TranscriptEqual *TranscriptCheck `json:"transcript_equal,omitempty"`
	Label           string           `json:"label,omitempty"`
	Attributes      Attributes       `json:"attributes,omitempty"`
}

// ByteEqualCheck describes a BYTE_EQUAL check.
type ByteEqualCheck struct {
	ArtifactA string `json:"artifact_a"`
	ArtifactB string `json:"artifact_b"`
}

// TranscriptCheck describes a TRANSCRIPT_EQUAL check.
type TranscriptCheck struct {
	RunA string `json:"run_a"`
	RunB string `json:"run_b"`
}

// SelfCheck describes a self-check record.
type SelfCheck struct {
	ID                string     `json:"id"`
	PlanID            string     `json:"plan_id"`
	TargetArtifactIDs []string   `json:"target_artifact_ids"`
	ReportBlobSHA256  string     `json:"report_blob_sha256"`
	Status            string     `json:"status"`
	Attributes        Attributes `json:"attributes,omitempty"`
}

// Export describes an export record.
type Export struct {
	ID               string     `json:"id"`
	Mode             string     `json:"mode"`
	ArtifactID       string     `json:"artifact_id"`
	ResultBlobSHA256 string     `json:"result_blob_sha256"`
	Attributes       Attributes `json:"attributes,omitempty"`
}

// Attributes is a map of arbitrary key-value pairs.
type Attributes map[string]interface{}

// IRRecord describes an IR extraction in the manifest.
type IRRecord struct {
	// ID is the unique identifier for this IR extraction.
	ID string `json:"id"`

	// SourceArtifactID is the artifact from which the IR was extracted.
	SourceArtifactID string `json:"source_artifact_id"`

	// IRBlobSHA256 is the SHA-256 hash of the IR blob.
	IRBlobSHA256 string `json:"ir_blob_sha256"`

	// IRFormat identifies the IR format version (e.g., "ir-v1").
	IRFormat string `json:"ir_format,omitempty"`

	// IRVersion is the IR schema version (e.g., "1.0.0").
	IRVersion string `json:"ir_version,omitempty"`

	// LossClass indicates the fidelity of the extraction (L0-L4).
	LossClass string `json:"loss_class,omitempty"`

	// LossReport contains detailed loss information.
	LossReport *ir.LossReport `json:"loss_report,omitempty"`

	// ExtractorPlugin is the plugin that performed the extraction.
	ExtractorPlugin string `json:"extractor_plugin,omitempty"`

	// Attributes contains additional metadata.
	Attributes Attributes `json:"attributes,omitempty"`
}

// NewManifest creates a new manifest with default values.
func NewManifest() *Manifest {
	return &Manifest{
		CapsuleVersion: Version,
		CreatedAt:      time.Now().UTC().Format(time.RFC3339),
		Tool: ToolInfo{
			Name:    "capsule",
			Version: Version,
		},
		Blobs: BlobIndex{
			BySHA256: make(map[string]*BlobRecord),
		},
		Artifacts: make(map[string]*Artifact),
		Runs:      make(map[string]*Run),
	}
}

// ToJSON serializes the manifest to JSON.
func (m *Manifest) ToJSON() ([]byte, error) {
	return json.MarshalIndent(m, "", "  ")
}

// ParseManifest parses a manifest from JSON.
func ParseManifest(data []byte) (*Manifest, error) {
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return &m, nil
}
