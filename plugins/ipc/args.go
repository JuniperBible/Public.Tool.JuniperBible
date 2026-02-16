package ipc

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
)

// StringArg extracts a required string argument from args.
// Returns an error if the argument is missing or not a string.
func StringArg(args map[string]interface{}, name string) (string, error) {
	v, ok := args[name]
	if !ok {
		return "", fmt.Errorf("%s argument required", name)
	}
	s, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("%s argument required", name)
	}
	return s, nil
}

// StringArgOr extracts an optional string argument with a default value.
func StringArgOr(args map[string]interface{}, name, defaultVal string) string {
	v, ok := args[name]
	if !ok {
		return defaultVal
	}
	s, ok := v.(string)
	if !ok {
		return defaultVal
	}
	return s
}

// BoolArg extracts an optional bool argument with a default value.
func BoolArg(args map[string]interface{}, name string, defaultVal bool) bool {
	v, ok := args[name]
	if !ok {
		return defaultVal
	}
	b, ok := v.(bool)
	if !ok {
		return defaultVal
	}
	return b
}

// PathAndOutputDir extracts the common path and output_dir arguments.
// Returns an error if either is missing.
func PathAndOutputDir(args map[string]interface{}) (path, outputDir string, err error) {
	path, err = StringArg(args, "path")
	if err != nil {
		return "", "", err
	}
	outputDir, err = StringArg(args, "output_dir")
	if err != nil {
		return "", "", err
	}
	return path, outputDir, nil
}

// StoreBlob stores data as a content-addressed blob and returns the hash.
// Creates the directory structure: outputDir/hash[:2]/hash
func StoreBlob(outputDir string, data []byte) (hashHex string, err error) {
	hash := sha256.Sum256(data)
	hashHex = hex.EncodeToString(hash[:])

	blobDir := filepath.Join(outputDir, hashHex[:2])
	if err := os.MkdirAll(blobDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create blob directory: %w", err)
	}

	blobPath := filepath.Join(blobDir, hashHex)
	if err := os.WriteFile(blobPath, data, 0600); err != nil {
		return "", fmt.Errorf("failed to write blob: %w", err)
	}

	return hashHex, nil
}

// ArtifactIDFromPath extracts an artifact ID from a file path.
// Removes the extension if present, but preserves hidden file names.
func ArtifactIDFromPath(path string) string {
	base := filepath.Base(path)
	ext := filepath.Ext(base)
	if ext != "" && ext != base {
		// Only strip extension if it's not the entire filename
		// (handles hidden files like .gitignore)
		return base[:len(base)-len(ext)]
	}
	return base
}
