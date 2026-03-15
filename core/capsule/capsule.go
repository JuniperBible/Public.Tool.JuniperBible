package capsule

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/JuniperBible/Public.Tool.JuniperBible/core/cas"
	"github.com/JuniperBible/Public.Tool.JuniperBible/core/errors"
	"github.com/JuniperBible/Public.Tool.JuniperBible/core/ir"
	"github.com/ulikunitz/xz"
)

// Injectable functions for testing
var (
	jsonMarshalCapsule   = json.Marshal
	jsonUnmarshalCapsule = json.Unmarshal
	osReadFileCapsule    = os.ReadFile
	osStatCapsule        = os.Stat
	osWriteFileCapsule   = os.WriteFile
	// Store operation wrappers - set these on capsule instances in tests
	storeStoreWithBlake3 func(context.Context, cas.BlobStore, []byte) (*cas.HashResult, error)
	storeRetrieve        func(context.Context, cas.BlobStore, string) ([]byte, error)

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

	// DetectCompression injectable function
	fileReadDetect func(io.Reader, []byte) (int, error)

	// SaveManifest injectable function
	manifestToJSONSave func(*Manifest) ([]byte, error)

	// writeToTar injectable function for testing
	writeToTarFunc = writeToTarImpl
)

func init() {
	// Default implementations call the actual store methods
	storeStoreWithBlake3 = func(ctx context.Context, s cas.BlobStore, data []byte) (*cas.HashResult, error) {
		return s.StoreWithBlake3(ctx, data)
	}
	storeRetrieve = func(ctx context.Context, s cas.BlobStore, hash string) ([]byte, error) {
		return s.Retrieve(ctx, hash)
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
	store    cas.BlobStore
}

// CapsuleOption configures capsule construction.
type CapsuleOption func(*capsuleConfig)

type capsuleConfig struct {
	veronicaCAS cas.VeronicaCAS
}

// WithVeronicaClient configures the capsule to use a Veronica CAS backend.
// The root directory is still used for BLAKE3 pointer files and local metadata.
func WithVeronicaClient(client cas.VeronicaCAS) CapsuleOption {
	return func(cfg *capsuleConfig) {
		cfg.veronicaCAS = client
	}
}

// New creates a new empty capsule at the given root directory.
func New(root string, opts ...CapsuleOption) (*Capsule, error) {
	if err := os.MkdirAll(root, 0700); err != nil {
		return nil, errors.NewIO("create directory", root, err)
	}

	var cfg capsuleConfig
	for _, opt := range opts {
		opt(&cfg)
	}

	if cfg.veronicaCAS == nil {
		return nil, errors.NewValidation("capsule", "Veronica CAS client is required")
	}

	store, err := cas.NewVeronicaStore(cfg.veronicaCAS, root)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create Veronica store")
	}

	return &Capsule{
		root:     root,
		Manifest: NewManifest(),
		store:    store,
	}, nil
}

// Create is an alias for New for convenience.
func Create(root string, opts ...CapsuleOption) (*Capsule, error) {
	return New(root, opts...)
}

// IngestFile ingests a file into the capsule, storing it in the CAS
// and recording it as an artifact in the manifest.
func (c *Capsule) IngestFile(ctx context.Context, path string) (*Artifact, error) {
	// Read the file
	data, err := osReadFileCapsule(path)
	if err != nil {
		return nil, errors.NewIO("read", path, err)
	}

	// Store with both SHA-256 and BLAKE3
	result, err := storeStoreWithBlake3(ctx, c.store, data)
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

	file, err := os.Create(archivePath)
	if err != nil {
		return fmt.Errorf("failed to create archive: %w", err)
	}
	defer file.Close()

	compressWriter, err := newCompressWriter(file, opts.Compression)
	if err != nil {
		return err
	}
	defer compressWriter.Close()

	tarWriter := tar.NewWriter(compressWriter)
	defer tarWriter.Close()

	if err := writeManifestToTar(tarWriter, c.Manifest); err != nil {
		return err
	}

	if err := writeBlobsToTar(tarWriter, c.root); err != nil {
		return fmt.Errorf("failed to write blobs: %w", err)
	}

	return nil
}

// writeManifestToTar serializes the manifest and writes it as manifest.json
// into the tar archive.
func writeManifestToTar(tw *tar.Writer, m *Manifest) error {
	data, err := manifestToJSONPack(m)
	if err != nil {
		return fmt.Errorf("failed to serialize manifest: %w", err)
	}
	if err := writeToTarFunc(tw, "manifest.json", data); err != nil {
		return fmt.Errorf("failed to write manifest: %w", err)
	}
	return nil
}

// newCompressWriter creates a compression writer for the given type, defaulting to XZ.
func newCompressWriter(w io.Writer, compression CompressionType) (io.WriteCloser, error) {
	if compression == CompressionGzip {
		cw, err := gzipNewWriterLevel(w, gzip.BestCompression)
		if err != nil {
			return nil, fmt.Errorf("failed to create gzip writer: %w", err)
		}
		return cw, nil
	}
	cw, err := xzNewWriter(w)
	if err != nil {
		return nil, fmt.Errorf("failed to create xz writer: %w", err)
	}
	return cw, nil
}

// writeBlobsToTar walks the blobs directory and writes every file into tw.
// It is a no-op when the blobs directory does not exist.
func writeBlobsToTar(tw *tar.Writer, root string) error {
	blobsDir := filepath.Join(root, "blobs")
	if _, err := os.Stat(blobsDir); err != nil {
		return nil // blobs directory absent – nothing to write
	}
	return filepathWalk(blobsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		relPath, err := filepathRel(root, path)
		if err != nil {
			return err
		}
		data, err := osReadFileWalk(path)
		if err != nil {
			return err
		}
		return writeToTarFunc(tw, relPath, data)
	})
}

type magicEntry struct {
	magic       []byte
	compression CompressionType
}

var compressionMagicTable = []magicEntry{
	{[]byte{0x1f, 0x8b}, CompressionGzip},
	{[]byte{0xfd, 0x37, 0x7a, 0x58, 0x5a, 0x00}, CompressionXZ},
}

func readMagicBytes(archivePath string) ([]byte, error) {
	file, err := os.Open(archivePath)
	if err != nil {
		return nil, errors.NewIO("open", archivePath, err)
	}
	defer file.Close()

	buf := make([]byte, 6)
	n, err := fileReadDetect(file, buf)
	if err != nil {
		return nil, errors.NewIO("read magic bytes", archivePath, err)
	}
	if n < 2 {
		return nil, errors.NewValidation("archive", "file too small to detect compression")
	}
	return buf[:n], nil
}

func matchesMagic(buf, magic []byte) bool {
	if len(buf) < len(magic) {
		return false
	}
	for i, b := range magic {
		if buf[i] != b {
			return false
		}
	}
	return true
}

func DetectCompression(archivePath string) (CompressionType, error) {
	buf, err := readMagicBytes(archivePath)
	if err != nil {
		return "", err
	}

	for _, entry := range compressionMagicTable {
		if matchesMagic(buf, entry.magic) {
			return entry.compression, nil
		}
	}

	return "", errors.NewUnsupported("compression format", "unknown magic bytes")
}

// Unpack unpacks a capsule archive to the given directory.
// Auto-detects compression format (XZ or gzip).
func Unpack(archivePath, destDir string, opts ...CapsuleOption) (*Capsule, error) {
	if err := osMkdirAllUnpack(destDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create destination directory: %w", err)
	}

	compression, err := DetectCompression(archivePath)
	if err != nil {
		return nil, fmt.Errorf("failed to detect compression: %w", err)
	}

	file, err := osOpenUnpack(archivePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open archive: %w", err)
	}
	defer file.Close()

	decompressReader, err := newDecompressReader(compression, file)
	if err != nil {
		return nil, err
	}

	manifest, err := extractTarEntries(tar.NewReader(decompressReader), destDir)
	if err != nil {
		return nil, err
	}

	if manifest == nil {
		return nil, fmt.Errorf("archive does not contain manifest.json")
	}

	var cfg capsuleConfig
	for _, opt := range opts {
		opt(&cfg)
	}

	if cfg.veronicaCAS == nil {
		return nil, fmt.Errorf("Veronica CAS client is required")
	}

	store, err := cas.NewVeronicaStore(cfg.veronicaCAS, destDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create Veronica store: %w", err)
	}

	return &Capsule{root: destDir, Manifest: manifest, store: store}, nil
}

// newDecompressReader creates a decompression reader for the given compression type.
func newDecompressReader(compression CompressionType, file *os.File) (io.Reader, error) {
	switch compression {
	case CompressionGzip:
		gzReader, err := gzipNewReader(file)
		if err != nil {
			return nil, fmt.Errorf("failed to create gzip reader: %w", err)
		}
		return gzReader, nil
	case CompressionXZ:
		xzReader, err := xzNewReader(file)
		if err != nil {
			return nil, fmt.Errorf("failed to create xz reader: %w", err)
		}
		return xzReader, nil
	default:
		return nil, fmt.Errorf("unsupported compression: %s", compression)
	}
}

// extractTarEntries reads all entries from tarReader into destDir and returns
// the parsed Manifest once the manifest.json entry is encountered.
func extractTarEntries(tarReader *tar.Reader, destDir string) (*Manifest, error) {
	var manifest *Manifest
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			return manifest, nil
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read tar header: %w", err)
		}

		parsed, err := processTarEntry(tarReader, header, destDir)
		if err != nil {
			return nil, err
		}
		if parsed != nil {
			manifest = parsed
		}
	}
}

// processTarEntry processes a single tar entry.
func processTarEntry(tarReader *tar.Reader, header *tar.Header, destDir string) (*Manifest, error) {
	cleanPath := filepath.Clean(header.Name)
	if strings.HasPrefix(cleanPath, "..") {
		return nil, nil // skip potentially malicious paths
	}

	destPath := filepath.Join(destDir, cleanPath)

	switch header.Typeflag {
	case tar.TypeDir:
		return nil, handleTarDir(destPath)
	case tar.TypeReg:
		return handleTarRegFile(tarReader, destPath, header.Name)
	default:
		return nil, nil
	}
}

// handleTarDir creates the directory at destPath.
func handleTarDir(destPath string) error {
	if err := osMkdirAllUnpack(destPath, 0700); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}
	return nil
}

// handleTarRegFile writes a regular tar entry to destPath.
// When headerName is "manifest.json" the parsed Manifest is also returned.
func handleTarRegFile(tarReader *tar.Reader, destPath, headerName string) (*Manifest, error) {
	if err := osMkdirAllUnpack(filepath.Dir(destPath), 0700); err != nil {
		return nil, fmt.Errorf("failed to create parent directory: %w", err)
	}

	data, err := ioReadAllUnpack(tarReader)
	if err != nil {
		return nil, fmt.Errorf("failed to read file data: %w", err)
	}

	if err := osWriteFileUnpack(destPath, data, 0600); err != nil {
		return nil, fmt.Errorf("failed to write file: %w", err)
	}

	if headerName != "manifest.json" {
		return nil, nil
	}

	manifest, err := ParseManifest(data)
	if err != nil {
		return nil, fmt.Errorf("failed to parse manifest: %w", err)
	}
	return manifest, nil
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

var artifactCharRanges = &unicode.RangeTable{
	R16: []unicode.Range16{
		{Lo: '-', Hi: '-', Stride: 1},
		{Lo: '.', Hi: '.', Stride: 1},
		{Lo: '0', Hi: '9', Stride: 1},
		{Lo: ':', Hi: ':', Stride: 1},
		{Lo: 'A', Hi: 'Z', Stride: 1},
		{Lo: '_', Hi: '_', Stride: 1},
		{Lo: 'a', Hi: 'z', Stride: 1},
	},
}

func isValidArtifactChar(c rune) bool {
	return unicode.Is(artifactCharRanges, c)
}

func sanitizeArtifactChar(c rune) rune {
	if isValidArtifactChar(c) {
		return c
	}
	return '_'
}

func generateArtifactID(name string) string {
	id := strings.TrimSuffix(name, filepath.Ext(name))
	result := strings.Map(sanitizeArtifactChar, id)
	if result == "" {
		return "artifact"
	}
	return result
}

// GetStore returns the underlying CAS store.
func (c *Capsule) GetStore() cas.BlobStore {
	return c.store
}

// AddRun adds a tool run to the capsule, storing the transcript in the CAS.
func (c *Capsule) AddRun(ctx context.Context, run *Run, transcriptData []byte) error {
	if run.ID == "" {
		return fmt.Errorf("run ID is required")
	}

	// Store transcript in CAS
	result, err := storeStoreWithBlake3(ctx, c.store, transcriptData)
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
func (c *Capsule) GetTranscript(ctx context.Context, runID string) ([]byte, error) {
	run, ok := c.Manifest.Runs[runID]
	if !ok {
		return nil, errors.NewNotFound("run", runID)
	}

	if run.Outputs == nil || run.Outputs.TranscriptBlobSHA256 == "" {
		return nil, errors.NewValidation("transcript", fmt.Sprintf("run %s has no transcript", runID))
	}

	return c.store.Retrieve(ctx, run.Outputs.TranscriptBlobSHA256)
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
func (c *Capsule) StoreIR(ctx context.Context, corpus *ir.Corpus, sourceArtifactID string) (*Artifact, error) {
	// Serialize corpus to JSON
	data, err := jsonMarshalCapsule(corpus)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize IR corpus: %w", err)
	}

	// Store in CAS
	result, err := storeStoreWithBlake3(ctx, c.store, data)
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
func (c *Capsule) LoadIR(ctx context.Context, artifactID string) (*ir.Corpus, error) {
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
	data, err := storeRetrieve(ctx, c.store, artifact.PrimaryBlobSHA256)
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
