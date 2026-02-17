package ipc

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// HandleDetect provides a common detect handler for simple file-based formats.
// It checks:
// 1. Path exists and is a file
// 2. File extension matches one of the allowed extensions
// 3. File content contains at least one of the markers
//
// Example:
//
//	HandleDetect(args, []string{".json"}, []string{"\"meta\""}, "JSON")
func matchesExtension(path string, extensions []string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	for _, allowed := range extensions {
		if ext == strings.ToLower(allowed) {
			return true
		}
	}
	return false
}

func checkMarkers(path string, markers []string) (bool, error) {
	if len(markers) == 0 {
		return true, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	content := string(data)
	for _, marker := range markers {
		if strings.Contains(content, marker) {
			return true, nil
		}
	}
	return false, nil
}

func validateDetectPath(path string) (bool, string) {
	info, err := os.Stat(path)
	if err != nil {
		return false, fmt.Sprintf("cannot stat: %v", err)
	}
	if info.IsDir() {
		return false, "path is a directory"
	}
	return true, ""
}

func HandleDetect(args map[string]interface{}, extensions []string, markers []string, formatName string) {
	path, err := StringArg(args, "path")
	if err != nil {
		RespondError(err.Error())
		return
	}

	if ok, reason := validateDetectPath(path); !ok {
		MustRespond(&DetectResult{Detected: false, Reason: reason})
		return
	}

	if !matchesExtension(path, extensions) {
		MustRespond(&DetectResult{Detected: false, Reason: fmt.Sprintf("not a %s file", formatName)})
		return
	}

	found, err := checkMarkers(path, markers)
	if err != nil {
		MustRespond(&DetectResult{Detected: false, Reason: fmt.Sprintf("cannot read file: %v", err)})
		return
	}
	if !found {
		MustRespond(&DetectResult{Detected: false, Reason: fmt.Sprintf("no %s markers found", formatName)})
		return
	}

	MustRespond(&DetectResult{
		Detected: true,
		Format:   formatName,
		Reason:   fmt.Sprintf("%s format detected", formatName),
	})
}

// HandleIngest provides a common ingest handler for file-based formats.
// It:
// 1. Reads the file
// 2. Computes SHA-256 hash
// 3. Stores the blob in content-addressed storage
// 4. Returns the ingest result
func HandleIngest(args map[string]interface{}, formatName string) {
	path, outputDir, err := PathAndOutputDir(args)
	if err != nil {
		RespondError(err.Error())
		return
	}

	data, err := os.ReadFile(path)
	if err != nil {
		RespondErrorf("failed to read file: %v", err)
		return
	}

	hashHex, err := StoreBlob(outputDir, data)
	if err != nil {
		RespondErrorf("failed to store blob: %v", err)
		return
	}

	artifactID := ArtifactIDFromPath(path)
	MustRespond(&IngestResult{
		ArtifactID: artifactID,
		BlobSHA256: hashHex,
		SizeBytes:  int64(len(data)),
		Metadata: map[string]string{
			"format": formatName,
		},
	})
}

// StandardIngest provides a flexible ingest handler that supports custom metadata.
// This is the recommended helper for format plugins to avoid code duplication.
//
// It performs the standard ingest workflow:
// 1. Extracts path and output_dir from args
// 2. Reads the file contents
// 3. Stores the blob in content-addressed storage (outputDir/hash[:2]/hash)
// 4. Generates artifact ID from filename
// 5. Responds with IngestResult including custom metadata
//
// The metadataFunc callback allows plugins to provide format-specific metadata.
// If metadataFunc is nil, only the format field is included.
//
// Example usage:
//
//	func handleIngest(args map[string]interface{}) {
//	    ipc.StandardIngest(args, "zip", func(path string, data []byte) map[string]string {
//	        entryCount := countZipEntries(path)
//	        return map[string]string{
//	            "format": "zip",
//	            "entry_count": fmt.Sprintf("%d", entryCount),
//	            "original_name": filepath.Base(path),
//	        }
//	    })
//	}
func StandardIngest(args map[string]interface{}, formatName string, metadataFunc func(path string, data []byte) map[string]string) {
	path, outputDir, err := PathAndOutputDir(args)
	if err != nil {
		RespondError(err.Error())
		return
	}

	data, err := os.ReadFile(path)
	if err != nil {
		RespondErrorf("failed to read file: %v", err)
		return
	}

	hashHex, err := StoreBlob(outputDir, data)
	if err != nil {
		RespondErrorf("failed to store blob: %v", err)
		return
	}

	artifactID := ArtifactIDFromPath(path)

	// Generate metadata
	metadata := map[string]string{
		"format": formatName,
	}
	if metadataFunc != nil {
		customMetadata := metadataFunc(path, data)
		for k, v := range customMetadata {
			metadata[k] = v
		}
	}

	MustRespond(&IngestResult{
		ArtifactID: artifactID,
		BlobSHA256: hashHex,
		SizeBytes:  int64(len(data)),
		Metadata:   metadata,
	})
}

// HandleEnumerateSingleFile provides a common enumerate handler for single-file formats.
func HandleEnumerateSingleFile(args map[string]interface{}, formatName string) {
	path, err := StringArg(args, "path")
	if err != nil {
		RespondError(err.Error())
		return
	}

	info, err := os.Stat(path)
	if err != nil {
		RespondErrorf("failed to stat: %v", err)
		return
	}

	MustRespond(&EnumerateResult{
		Entries: []EnumerateEntry{
			{
				Path:      filepath.Base(path),
				SizeBytes: info.Size(),
				IsDir:     false,
				Metadata: map[string]string{
					"format": formatName,
				},
			},
		},
	})
}

// ComputeHash computes SHA-256 hash of data and returns hex string.
func ComputeHash(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

// ComputeSourceHash computes SHA-256 hash and returns both raw bytes and hex string.
func ComputeSourceHash(data []byte) (raw [32]byte, hexStr string) {
	raw = sha256.Sum256(data)
	hexStr = hex.EncodeToString(raw[:])
	return
}
