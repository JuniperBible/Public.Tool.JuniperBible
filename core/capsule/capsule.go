package capsule

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/FocuswithJustin/JuniperBible/core/cas"
	"github.com/FocuswithJustin/JuniperBible/core/errors"
	"github.com/FocuswithJustin/JuniperBible/core/ir"
	"github.com/ulikunitz/xz"
)

// Injectable functions for testing
var (
	casNewStore          = cas.NewStore
	jsonMarshalCapsule   = json.Marshal
	jsonUnmarshalCapsule = json.Unmarshal
	osReadFileCapsule    = os.ReadFile
	osStatCapsule        = os.Stat
	osWriteFileCapsule   = os.WriteFile
	// Store operation wrappers - set these on capsule instances in tests
	storeStoreWithBlake3 func(*cas.Store, []byte) (*cas.HashResult, error)
	storeRetrieve        func(*cas.Store, string) ([]byte, error)

	// PackWithOptions injectable functions
	gzipNewWriterLevel = gzip.NewWriterLevel
	xzNewWriter        = xz.NewWriter
	manifestToJSONPack func(*Manifest) ([]byte, error)
	filepathWalk       = filepath.Walk
	filepathRel        = filepath.Rel
	osReadFileWalk     = os.ReadFile

	// Unpack injectable functions
	osMkdirAllUnpack  = os.MkdirAll
	osOpenUnpack      = os.Open
	gzipNewReader     = gzip.NewReader
	xzNewReader       = xz.NewReader
	ioReadAllUnpack   = io.ReadAll
	osWriteFileUnpack = os.WriteFile
	casNewStoreUnpack = cas.NewStore

	// DetectCompression injectable function
	fileReadDetect func(io.Reader, []byte) (int, error)

	// SaveManifest injectable function
	manifestToJSONSave func(*Manifest) ([]byte, error)

	// writeToTar injectable function for testing
	writeToTarFunc = writeToTarImpl
)

func init() {
	// Default implementations call the actual store methods
	storeStoreWithBlake3 = func(s *cas.Store, data []byte) (*cas.HashResult, error) {
		return s.StoreWithBlake3(data)
	}
	storeRetrieve = func(s *cas.Store, hash string) ([]byte, error) {
		return s.Retrieve(hash)
	}
	manifestToJSONPack = func(m *Manifest) ([]byte, error) {
		return m.ToJSON()
	}
	fileReadDetect = func(r io.Reader, b []byte) (int, error) {
		return r.Read(b)
	}
	manifestToJSONSave = func(m *Manifest) ([]byte, error) {
		return m.ToJSON()
	}
}

// CompressionType specifies the compression algorithm for capsule archives.
type CompressionType string

const (
	// CompressionXZ uses XZ/LZMA2 compression (default, best ratio).
	CompressionXZ CompressionType = "xz"
	// CompressionGzip uses gzip compression (stdlib, faster).
	CompressionGzip CompressionType = "gzip"
)

// PackOptions configures capsule packing behavior.
type PackOptions struct {
	// Compression specifies the compression algorithm. Defaults to XZ.
	Compression CompressionType
}

// DefaultPackOptions returns the default packing options (XZ compression).
func DefaultPackOptions() *PackOptions {
	return &PackOptions{
		Compression: CompressionXZ,
	}
}

// Capsule represents an in-memory capsule with its manifest and blob store.
type Capsule struct {
	root     string
	Manifest *Manifest
	store    *cas.Store
}

// New creates a new empty capsule at the given root directory.
func New(root string) (*Capsule, error) {
	if err := os.MkdirAll(root, 0755); err != nil {
		return nil, errors.NewIO("create directory", root, err)
	}

	store, err := casNewStore(root)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create blob store")
	}

	return &Capsule{
		root:     root,
		Manifest: NewManifest(),
		store:    store,
	}, nil
}

// Create is an alias for New for convenience.
func Create(root string) (*Capsule, error) {
	return New(root)
}

// IngestFile ingests a file into the capsule, storing it in the CAS
// and recording it as an artifact in the manifest.
func (c *Capsule) IngestFile(path string) (*Artifact, error) {
	// Read the file
	data, err := osReadFileCapsule(path)
	if err != nil {
		return nil, errors.NewIO("read", path, err)
	}

	// Store with both SHA-256 and BLAKE3
	result, err := storeStoreWithBlake3(c.store, data)
	if err != nil {
		return nil, errors.Wrap(err, "failed to store blob")
	}

	// Get file info
	info, err := osStatCapsule(path)
	if err != nil {
		return nil, errors.NewIO("stat", path, err)
	}

	// Generate artifact ID from filename
	baseName := filepath.Base(path)
	artifactID := generateArtifactID(baseName)

	// Ensure unique ID
	for {
		if _, exists := c.Manifest.Artifacts[artifactID]; !exists {
			break
		}
		artifactID = artifactID + "_1"
	}

	// Create blob record
	blobPath := fmt.Sprintf("blobs/sha256/%s/%s", result.SHA256[:2], result.SHA256)
	blobRecord := &BlobRecord{
		SHA256:    result.SHA256,
		BLAKE3:    result.BLAKE3,
		SizeBytes: info.Size(),
		Path:      blobPath,
	}
	c.Manifest.Blobs.BySHA256[result.SHA256] = blobRecord

	// Create artifact
	artifact := &Artifact{
		ID:                artifactID,
		Kind:              "file",
		OriginalName:      baseName,
		SourcePath:        path,
		PrimaryBlobSHA256: result.SHA256,
		Hashes: ArtifactHashes{
			SHA256: result.SHA256,
			BLAKE3: result.BLAKE3,
		},
		SizeBytes: info.Size(),
	}

	c.Manifest.Artifacts[artifactID] = artifact
	return artifact, nil
}

// Pack packs the capsule into a tar.xz archive (default compression).
func (c *Capsule) Pack(archivePath string) error {
	return c.PackWithOptions(archivePath, DefaultPackOptions())
}

// PackWithOptions packs the capsule with specified options.
func (c *Capsule) PackWithOptions(archivePath string, opts *PackOptions) error {
	if opts == nil {
		opts = DefaultPackOptions()
	}

	// Create the archive file
	file, err := os.Create(archivePath)
	if err != nil {
		return fmt.Errorf("failed to create archive: %w", err)
	}
	defer file.Close()

	// Create compression writer based on options
	var compressWriter io.WriteCloser
	switch opts.Compression {
	case CompressionGzip:
		compressWriter, err = gzipNewWriterLevel(file, gzip.BestCompression)
		if err != nil {
			return fmt.Errorf("failed to create gzip writer: %w", err)
		}
	case CompressionXZ:
		fallthrough
	default:
		compressWriter, err = xzNewWriter(file)
		if err != nil {
			return fmt.Errorf("failed to create xz writer: %w", err)
		}
	}
	defer compressWriter.Close()

	// Create tar writer
	tarWriter := tar.NewWriter(compressWriter)
	defer tarWriter.Close()

	// Write manifest.json first
	manifestData, err := manifestToJSONPack(c.Manifest)
	if err != nil {
		return fmt.Errorf("failed to serialize manifest: %w", err)
	}

	if err := writeToTarFunc(tarWriter, "manifest.json", manifestData); err != nil {
		return fmt.Errorf("failed to write manifest: %w", err)
	}

	// Write all blobs
	blobsDir := filepath.Join(c.root, "blobs")
	if _, err := os.Stat(blobsDir); err == nil {
		if err := filepathWalk(blobsDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				return nil
			}

			// Get relative path from root
			relPath, err := filepathRel(c.root, path)
			if err != nil {
				return err
			}

			// Read file
			data, err := osReadFileWalk(path)
			if err != nil {
				return err
			}

			return writeToTarFunc(tarWriter, relPath, data)
		}); err != nil {
			return fmt.Errorf("failed to write blobs: %w", err)
		}
	}

	return nil
}

// DetectCompression detects the compression type of a capsule archive.
func DetectCompression(archivePath string) (CompressionType, error) {
	file, err := os.Open(archivePath)
	if err != nil {
		return "", errors.NewIO("open", archivePath, err)
	}
	defer file.Close()

	// Read magic bytes
	magic := make([]byte, 6)
	n, err := fileReadDetect(file, magic)
	if err != nil {
		return "", errors.NewIO("read magic bytes", archivePath, err)
	}
	if n < 2 {
		return "", errors.NewValidation("archive", "file too small to detect compression")
	}

	// Check for gzip magic (1f 8b)
	if magic[0] == 0x1f && magic[1] == 0x8b {
		return CompressionGzip, nil
	}

	// Check for XZ magic (fd 37 7a 58 5a 00)
	if n >= 6 && magic[0] == 0xfd && magic[1] == 0x37 && magic[2] == 0x7a &&
		magic[3] == 0x58 && magic[4] == 0x5a && magic[5] == 0x00 {
		return CompressionXZ, nil
	}

	return "", errors.NewUnsupported("compression format", "unknown magic bytes")
}

// Unpack unpacks a capsule archive to the given directory.
// Auto-detects compression format (XZ or gzip).
func Unpack(archivePath, destDir string) (*Capsule, error) {
	// Create destination directory
	if err := osMkdirAllUnpack(destDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create destination directory: %w", err)
	}

	// Detect compression type
	compression, err := DetectCompression(archivePath)
	if err != nil {
		return nil, fmt.Errorf("failed to detect compression: %w", err)
	}

	// Open the archive
	file, err := osOpenUnpack(archivePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open archive: %w", err)
	}
	defer file.Close()

	// Create decompression reader based on detected format
	var decompressReader io.Reader
	switch compression {
	case CompressionGzip:
		gzReader, err := gzipNewReader(file)
		if err != nil {
			return nil, fmt.Errorf("failed to create gzip reader: %w", err)
		}
		defer gzReader.Close()
		decompressReader = gzReader
	case CompressionXZ:
		xzReader, err := xzNewReader(file)
		if err != nil {
			return nil, fmt.Errorf("failed to create xz reader: %w", err)
		}
		decompressReader = xzReader
	default:
		return nil, fmt.Errorf("unsupported compression: %s", compression)
	}

	// Create tar reader
	tarReader := tar.NewReader(decompressReader)

	var manifest *Manifest

	// Extract all files
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read tar header: %w", err)
		}

		// Sanitize path
		cleanPath := filepath.Clean(header.Name)
		if strings.HasPrefix(cleanPath, "..") {
			continue // Skip potentially malicious paths
		}

		destPath := filepath.Join(destDir, cleanPath)

		switch header.Typeflag {
		case tar.TypeDir:
			if err := osMkdirAllUnpack(destPath, 0755); err != nil {
				return nil, fmt.Errorf("failed to create directory: %w", err)
			}
		case tar.TypeReg:
			// Ensure parent directory exists
			if err := osMkdirAllUnpack(filepath.Dir(destPath), 0755); err != nil {
				return nil, fmt.Errorf("failed to create parent directory: %w", err)
			}

			// Read file data
			data, err := ioReadAllUnpack(tarReader)
			if err != nil {
				return nil, fmt.Errorf("failed to read file data: %w", err)
			}

			// Write file
			if err := osWriteFileUnpack(destPath, data, 0600); err != nil {
				return nil, fmt.Errorf("failed to write file: %w", err)
			}

			// Parse manifest if this is it
			if header.Name == "manifest.json" {
				manifest, err = ParseManifest(data)
				if err != nil {
					return nil, fmt.Errorf("failed to parse manifest: %w", err)
				}
			}
		}
	}

	if manifest == nil {
		return nil, fmt.Errorf("archive does not contain manifest.json")
	}

	// Create store pointing to unpacked directory
	store, err := casNewStoreUnpack(destDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create store: %w", err)
	}

	return &Capsule{
		root:     destDir,
		Manifest: manifest,
		store:    store,
	}, nil
}

// writeToTarImpl writes a file to the tar archive.
func writeToTarImpl(tw *tar.Writer, name string, data []byte) error {
	header := &tar.Header{
		Name: name,
		Mode: 0600,
		Size: int64(len(data)),
	}

	if err := tw.WriteHeader(header); err != nil {
		return err
	}

	_, err := tw.Write(data)
	return err
}

// generateArtifactID generates an artifact ID from a filename.
func generateArtifactID(name string) string {
	// Remove extension and sanitize
	id := strings.TrimSuffix(name, filepath.Ext(name))
	// Replace invalid characters
	var result strings.Builder
	for _, c := range id {
		if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') ||
			(c >= '0' && c <= '9') || c == '.' || c == '_' || c == '-' || c == ':' {
			result.WriteRune(c)
		} else {
			result.WriteRune('_')
		}
	}
	if result.Len() == 0 {
		return "artifact"
	}
	return result.String()
}

// GetStore returns the underlying CAS store.
func (c *Capsule) GetStore() *cas.Store {
	return c.store
}

// AddRun adds a tool run to the capsule, storing the transcript in the CAS.
func (c *Capsule) AddRun(run *Run, transcriptData []byte) error {
	if run.ID == "" {
		return fmt.Errorf("run ID is required")
	}

	// Store transcript in CAS
	result, err := storeStoreWithBlake3(c.store, transcriptData)
	if err != nil {
		return fmt.Errorf("failed to store transcript: %w", err)
	}

	// Create blob record for transcript
	blobPath := fmt.Sprintf("blobs/sha256/%s/%s", result.SHA256[:2], result.SHA256)
	blobRecord := &BlobRecord{
		SHA256:    result.SHA256,
		BLAKE3:    result.BLAKE3,
		SizeBytes: int64(len(transcriptData)),
		Path:      blobPath,
		MIME:      "application/x-ndjson",
	}
	c.Manifest.Blobs.BySHA256[result.SHA256] = blobRecord

	// Set the transcript hash in run outputs
	if run.Outputs == nil {
		run.Outputs = &RunOutputs{}
	}
	run.Outputs.TranscriptBlobSHA256 = result.SHA256

	// Add run to manifest
	if c.Manifest.Runs == nil {
		c.Manifest.Runs = make(map[string]*Run)
	}
	c.Manifest.Runs[run.ID] = run

	return nil
}

// GetTranscript retrieves the transcript data for a run.
func (c *Capsule) GetTranscript(runID string) ([]byte, error) {
	run, ok := c.Manifest.Runs[runID]
	if !ok {
		return nil, errors.NewNotFound("run", runID)
	}

	if run.Outputs == nil || run.Outputs.TranscriptBlobSHA256 == "" {
		return nil, errors.NewValidation("transcript", fmt.Sprintf("run %s has no transcript", runID))
	}

	return c.store.Retrieve(run.Outputs.TranscriptBlobSHA256)
}

// GetRoot returns the root directory of the capsule.
func (c *Capsule) GetRoot() string {
	return c.root
}

// SaveManifest saves the manifest to disk.
func (c *Capsule) SaveManifest() error {
	manifestData, err := manifestToJSONSave(c.Manifest)
	if err != nil {
		return fmt.Errorf("failed to serialize manifest: %w", err)
	}

	manifestPath := filepath.Join(c.root, "manifest.json")
	if err := osWriteFileCapsule(manifestPath, manifestData, 0600); err != nil {
		return fmt.Errorf("failed to write manifest: %w", err)
	}

	return nil
}

// StoreIR serializes an IR Corpus and stores it in the capsule.
// It creates both an artifact and an IR extraction record.
func (c *Capsule) StoreIR(corpus *ir.Corpus, sourceArtifactID string) (*Artifact, error) {
	// Serialize corpus to JSON
	data, err := jsonMarshalCapsule(corpus)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize IR corpus: %w", err)
	}

	// Store in CAS
	result, err := storeStoreWithBlake3(c.store, data)
	if err != nil {
		return nil, fmt.Errorf("failed to store IR blob: %w", err)
	}

	// Generate artifact ID
	artifactID := fmt.Sprintf("ir-%s", corpus.ID)
	if sourceArtifactID != "" {
		artifactID = fmt.Sprintf("ir-%s", sourceArtifactID)
	}

	// Ensure unique ID
	counter := 0
	originalID := artifactID
	for {
		if _, exists := c.Manifest.Artifacts[artifactID]; !exists {
			break
		}
		counter++
		artifactID = fmt.Sprintf("%s-%d", originalID, counter)
	}

	// Create blob record
	blobPath := fmt.Sprintf("blobs/sha256/%s/%s", result.SHA256[:2], result.SHA256)
	blobRecord := &BlobRecord{
		SHA256:    result.SHA256,
		BLAKE3:    result.BLAKE3,
		SizeBytes: int64(len(data)),
		Path:      blobPath,
		MIME:      "application/json",
	}
	c.Manifest.Blobs.BySHA256[result.SHA256] = blobRecord

	// Create artifact
	artifact := &Artifact{
		ID:                artifactID,
		Kind:              ArtifactKindIR,
		OriginalName:      fmt.Sprintf("%s.ir.json", corpus.ID),
		PrimaryBlobSHA256: result.SHA256,
		Hashes: ArtifactHashes{
			SHA256: result.SHA256,
			BLAKE3: result.BLAKE3,
		},
		SizeBytes: int64(len(data)),
	}
	c.Manifest.Artifacts[artifactID] = artifact

	// Create IR extraction record
	irRecord := &IRRecord{
		ID:               artifactID,
		SourceArtifactID: sourceArtifactID,
		IRBlobSHA256:     result.SHA256,
		IRFormat:         "ir-v1",
		IRVersion:        corpus.Version,
		LossClass:        string(corpus.LossClass),
	}

	// Initialize IRExtractions map if needed
	if c.Manifest.IRExtractions == nil {
		c.Manifest.IRExtractions = make(map[string]*IRRecord)
	}
	c.Manifest.IRExtractions[artifactID] = irRecord

	return artifact, nil
}

// LoadIR retrieves and deserializes an IR Corpus from the capsule.
func (c *Capsule) LoadIR(artifactID string) (*ir.Corpus, error) {
	// Find the artifact
	artifact, ok := c.Manifest.Artifacts[artifactID]
	if !ok {
		return nil, errors.NewNotFound("artifact", artifactID)
	}

	// Verify it's an IR artifact
	if artifact.Kind != ArtifactKindIR {
		return nil, errors.NewValidation("artifact", fmt.Sprintf("artifact %s is not an IR (kind=%s)", artifactID, artifact.Kind))
	}

	// Retrieve the blob
	data, err := storeRetrieve(c.store, artifact.PrimaryBlobSHA256)
	if err != nil {
		return nil, errors.Wrap(err, "failed to retrieve IR blob")
	}

	// Deserialize the corpus
	var corpus ir.Corpus
	if err := jsonUnmarshalCapsule(data, &corpus); err != nil {
		return nil, errors.NewParse("IR corpus", "", err.Error())
	}

	return &corpus, nil
}

// GetIRRecord retrieves the IR extraction record for an artifact.
func (c *Capsule) GetIRRecord(artifactID string) (*IRRecord, error) {
	if c.Manifest.IRExtractions == nil {
		return nil, fmt.Errorf("no IR extractions in manifest")
	}

	record, ok := c.Manifest.IRExtractions[artifactID]
	if !ok {
		return nil, fmt.Errorf("IR extraction not found: %s", artifactID)
	}

	return record, nil
}
