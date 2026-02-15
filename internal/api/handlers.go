package api

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/ulikunitz/xz"

	"github.com/FocuswithJustin/JuniperBible/core/errors"
	"github.com/FocuswithJustin/JuniperBible/internal/safefile"
	"github.com/FocuswithJustin/JuniperBible/internal/validation"
)

// APIResponse is the standard API response wrapper.
type APIResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   *APIError   `json:"error,omitempty"`
	Meta    *APIMeta    `json:"meta,omitempty"`
}

// APIError represents an API error.
type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// APIMeta contains response metadata.
type APIMeta struct {
	Total     int    `json:"total,omitempty"`
	Timestamp string `json:"timestamp"`
}

// CapsuleInfo describes a capsule.
type CapsuleInfo struct {
	ID        string           `json:"id"`
	Name      string           `json:"name"`
	Path      string           `json:"path"`
	Size      int64            `json:"size"`
	Format    string           `json:"format"`
	CreatedAt string           `json:"created_at,omitempty"`
	Manifest  *CapsuleManifest `json:"manifest,omitempty"`
	Artifacts []ArtifactInfo   `json:"artifacts,omitempty"`
}

// CapsuleManifest is the manifest.json structure.
type CapsuleManifest struct {
	Version      string            `json:"version"`
	ModuleType   string            `json:"module_type,omitempty"`
	Title        string            `json:"title,omitempty"`
	Language     string            `json:"language,omitempty"`
	Rights       string            `json:"rights,omitempty"`
	SourceFormat string            `json:"source_format,omitempty"`
	CreatedAt    string            `json:"created_at,omitempty"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

// ArtifactInfo describes an artifact.
type ArtifactInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Size int64  `json:"size"`
	Hash string `json:"hash,omitempty"`
}

// PluginInfo describes a plugin.
type PluginInfo struct {
	Name        string   `json:"name"`
	Type        string   `json:"type"`
	Description string   `json:"description"`
	Formats     []string `json:"formats,omitempty"`
}

// FormatInfo describes a supported format.
type FormatInfo struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Extensions  []string `json:"extensions"`
	LossClass   string   `json:"loss_class"`
	Description string   `json:"description"`
	CanExtract  bool     `json:"can_extract"`
	CanEmit     bool     `json:"can_emit"`
}

// ConvertRequest is the request body for conversion.
type ConvertRequest struct {
	Source       string                 `json:"source"`
	TargetFormat string                 `json:"target_format"`
	Options      map[string]interface{} `json:"options,omitempty"`
}

// ConvertResult is the result of a conversion.
type ConvertResult struct {
	OutputPath string `json:"output_path"`
	LossClass  string `json:"loss_class"`
	Duration   string `json:"duration"`
}

// HealthInfo is the health check response.
type HealthInfo struct {
	Status   string `json:"status"`
	Version  string `json:"version"`
	Uptime   string `json:"uptime"`
	Capsules int    `json:"capsules"`
	Plugins  int    `json:"plugins"`
}

var startTime = time.Now()

func handleRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		respondError(w, http.StatusNotFound, "NOT_FOUND", "Endpoint not found")
		return
	}

	respond(w, http.StatusOK, map[string]interface{}{
		"name":    "Juniper Bible API",
		"version": "0.2.0",
		"docs":    "/api/docs",
		"endpoints": []string{
			"GET /health",
			"GET /capsules",
			"POST /capsules",
			"GET /capsules/:id",
			"DELETE /capsules/:id",
			"POST /convert",
			"GET /plugins",
			"GET /formats",
			"WS /ws",
			"POST /jobs",
			"GET /jobs/:id",
			"DELETE /jobs/:id",
		},
	})
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only GET is allowed")
		return
	}

	capsules := listCapsules()
	plugins := listPlugins()

	respond(w, http.StatusOK, HealthInfo{
		Status:   "healthy",
		Version:  "0.2.0",
		Uptime:   time.Since(startTime).String(),
		Capsules: len(capsules),
		Plugins:  len(plugins),
	})
}

func handleCapsules(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		listCapsulesHandler(w, r)
	case http.MethodPost:
		createCapsuleHandler(w, r)
	default:
		respondError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only GET and POST are allowed")
	}
}

func listCapsulesHandler(w http.ResponseWriter, r *http.Request) {
	capsules := listCapsules()

	response := APIResponse{
		Success: true,
		Data:    capsules,
		Meta: &APIMeta{
			Total:     len(capsules),
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func createCapsuleHandler(w http.ResponseWriter, r *http.Request) {
	BroadcastProgress("upload", "parsing", "Parsing upload request", 10)

	// Parse multipart form with size limit
	if err := r.ParseMultipartForm(validation.MaxFileSize); err != nil {
		BroadcastError("upload", "Failed to parse multipart form or file too large")
		respondError(w, http.StatusBadRequest, "INVALID_REQUEST", "Failed to parse multipart form or file too large")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		BroadcastError("upload", "No file uploaded")
		respondError(w, http.StatusBadRequest, "MISSING_FILE", "No file uploaded")
		return
	}
	defer file.Close()

	// Validate filename
	if err := validation.ValidateFilename(header.Filename); err != nil {
		BroadcastError("upload", "Invalid filename provided")
		respondError(w, http.StatusBadRequest, "INVALID_FILENAME", "Invalid filename provided")
		return
	}

	// Validate file content matches claimed type (magic byte validation)
	BroadcastProgress("upload", "validating", "Validating file type", 30)
	fileType, err := validation.ValidateFileType(file, header.Filename)
	if err != nil {
		BroadcastError("upload", fmt.Sprintf("File validation failed: %v", err))
		respondError(w, http.StatusBadRequest, "INVALID_FILE_TYPE", fmt.Sprintf("File validation failed: %v", err))
		return
	}
	// Reset file pointer after reading magic bytes
	if _, err := file.Seek(0, 0); err != nil {
		BroadcastError("upload", "Failed to process file")
		respondError(w, http.StatusInternalServerError, "FILE_PROCESSING_ERROR", "Failed to process file")
		return
	}

	// Sanitize the filename to prevent path traversal
	// Use our API-specific ValidatePath for defense in depth
	safePath, err := ValidatePath(ServerConfig.CapsulesDir, header.Filename)
	if err != nil {
		BroadcastError("upload", fmt.Sprintf("Invalid file path: %v", err))
		respondError(w, http.StatusBadRequest, "INVALID_PATH", fmt.Sprintf("Invalid file path: %v", err))
		return
	}

	BroadcastProgress("upload", "saving", "Saving uploaded file", 50)

	// Save the uploaded file with size limit
	destPath := filepath.Join(ServerConfig.CapsulesDir, safePath)
	destFile, err := os.Create(destPath)
	if err != nil {
		BroadcastError("upload", fmt.Sprintf("Failed to save file: %v", err))
		respondError(w, http.StatusInternalServerError, "SAVE_FAILED", fmt.Sprintf("Failed to save file: %v", err))
		return
	}
	defer destFile.Close()

	// Limit the copy to prevent DoS
	written, err := io.CopyN(destFile, file, validation.MaxFileSize)
	if err != nil && err != io.EOF {
		os.Remove(destPath) // Clean up on error
		BroadcastError("upload", "Failed to write file")
		respondError(w, http.StatusInternalServerError, "SAVE_FAILED", "Failed to write file")
		return
	}

	// Check if there's more data (file too large)
	if _, err := file.Read(make([]byte, 1)); err != io.EOF {
		os.Remove(destPath) // Clean up oversized file
		BroadcastError("upload", "File exceeds maximum size limit")
		respondError(w, http.StatusBadRequest, "FILE_TOO_LARGE", "File exceeds maximum size limit")
		return
	}

	capsule := CapsuleInfo{
		ID:     header.Filename,
		Name:   header.Filename,
		Path:   safePath,
		Size:   written,
		Format: detectFormat(header.Filename),
	}

	BroadcastComplete("upload", "Upload completed successfully", map[string]interface{}{
		"filename":  header.Filename,
		"size":      written,
		"file_type": string(fileType),
	})

	respond(w, http.StatusCreated, capsule)
}

func handleCapsuleByID(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/capsules/")
	if id == "" {
		respondError(w, http.StatusBadRequest, "MISSING_ID", "Capsule ID is required")
		return
	}

	// Validate the capsule ID to prevent path traversal
	// Use our API-specific ValidateID for comprehensive validation
	if err := ValidateID(id); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_ID", fmt.Sprintf("Invalid capsule ID: %v", err))
		return
	}

	// Verify the path is safe using ValidatePath
	if _, err := ValidatePath(ServerConfig.CapsulesDir, id); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_PATH", fmt.Sprintf("Invalid capsule path: %v", err))
		return
	}

	switch r.Method {
	case http.MethodGet:
		getCapsuleHandler(w, r, id)
	case http.MethodDelete:
		deleteCapsuleHandler(w, r, id)
	default:
		respondError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only GET and DELETE are allowed")
	}
}

func getCapsuleHandler(w http.ResponseWriter, r *http.Request, id string) {
	// Validate and sanitize ID to prevent path traversal
	// ValidatePath provides comprehensive protection
	safePath, err := ValidatePath(ServerConfig.CapsulesDir, id)
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_ID", fmt.Sprintf("Invalid capsule ID: %v", err))
		return
	}

	capsulePath := filepath.Join(ServerConfig.CapsulesDir, safePath)
	info, err := os.Stat(capsulePath)
	if err != nil {
		notFoundErr := errors.NewNotFound("capsule", id)
		respondError(w, http.StatusNotFound, "NOT_FOUND", notFoundErr.Error())
		return
	}

	capsule := CapsuleInfo{
		ID:     id,
		Name:   id,
		Path:   id,
		Size:   info.Size(),
		Format: detectFormat(id),
	}

	// Read manifest and artifacts
	manifest, artifacts, err := readCapsule(capsulePath)
	if err == nil {
		capsule.Manifest = manifest
		capsule.Artifacts = artifacts
	}

	respond(w, http.StatusOK, capsule)
}

func deleteCapsuleHandler(w http.ResponseWriter, r *http.Request, id string) {
	// Validate and sanitize ID to prevent path traversal
	// ValidatePath provides comprehensive protection
	safePath, err := ValidatePath(ServerConfig.CapsulesDir, id)
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_ID", fmt.Sprintf("Invalid capsule ID: %v", err))
		return
	}

	capsulePath := filepath.Join(ServerConfig.CapsulesDir, safePath)
	if _, err := os.Stat(capsulePath); err != nil {
		notFoundErr := errors.NewNotFound("capsule", id)
		respondError(w, http.StatusNotFound, "NOT_FOUND", notFoundErr.Error())
		return
	}

	if err := os.Remove(capsulePath); err != nil {
		ioErr := errors.NewIO("delete", capsulePath, err)
		respondError(w, http.StatusInternalServerError, "DELETE_FAILED", ioErr.Error())
		return
	}

	respond(w, http.StatusOK, map[string]string{"message": "Capsule deleted"})
}

func handleConvert(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only POST is allowed")
		return
	}

	var req ConvertRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		parseErr := errors.NewParse("JSON", "request body", err.Error())
		respondError(w, http.StatusBadRequest, "INVALID_JSON", parseErr.Error())
		return
	}

	if req.Source == "" || req.TargetFormat == "" {
		validErr := errors.NewValidation("request", "source and target_format are required")
		respondError(w, http.StatusBadRequest, "MISSING_PARAMS", validErr.Error())
		return
	}

	// Conversion not yet implemented via API
	unsupportedErr := errors.NewUnsupported("API conversion", "not yet implemented via API, use the CLI")
	respondError(w, http.StatusNotImplemented, "NOT_IMPLEMENTED", unsupportedErr.Error())
}

func handlePlugins(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only GET is allowed")
		return
	}

	plugins := listPlugins()

	response := APIResponse{
		Success: true,
		Data:    plugins,
		Meta: &APIMeta{
			Total:     len(plugins),
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func handleFormats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only GET is allowed")
		return
	}

	formats := []FormatInfo{
		{ID: "osis", Name: "OSIS XML", Extensions: []string{".osis", ".xml"}, LossClass: "L0", Description: "Open Scripture Information Standard", CanExtract: true, CanEmit: true},
		{ID: "usfm", Name: "USFM", Extensions: []string{".usfm", ".sfm"}, LossClass: "L0", Description: "Unified Standard Format Markers", CanExtract: true, CanEmit: true},
		{ID: "usx", Name: "USX", Extensions: []string{".usx"}, LossClass: "L0", Description: "Unified Scripture XML", CanExtract: true, CanEmit: true},
		{ID: "zefania", Name: "Zefania XML", Extensions: []string{".xml"}, LossClass: "L0", Description: "Zefania Bible format", CanExtract: true, CanEmit: true},
		{ID: "theword", Name: "TheWord", Extensions: []string{".ont", ".nt", ".twm"}, LossClass: "L0", Description: "TheWord Bible software", CanExtract: true, CanEmit: true},
		{ID: "json", Name: "JSON", Extensions: []string{".json"}, LossClass: "L0", Description: "JSON structure", CanExtract: true, CanEmit: true},
		{ID: "sword", Name: "SWORD", Extensions: []string{""}, LossClass: "L2", Description: "SWORD module format", CanExtract: true, CanEmit: true},
		{ID: "html", Name: "HTML", Extensions: []string{".html", ".htm"}, LossClass: "L1", Description: "Static HTML site", CanExtract: true, CanEmit: true},
		{ID: "epub", Name: "EPUB", Extensions: []string{".epub"}, LossClass: "L1", Description: "EPUB3 ebook", CanExtract: false, CanEmit: true},
		{ID: "markdown", Name: "Markdown", Extensions: []string{".md"}, LossClass: "L1", Description: "Hugo-compatible Markdown", CanExtract: false, CanEmit: true},
		{ID: "sqlite", Name: "SQLite", Extensions: []string{".db", ".sqlite"}, LossClass: "L1", Description: "SQLite database", CanExtract: true, CanEmit: true},
		{ID: "esword", Name: "e-Sword", Extensions: []string{".bblx", ".cmtx"}, LossClass: "L1", Description: "e-Sword format", CanExtract: true, CanEmit: true},
		{ID: "txt", Name: "Plain Text", Extensions: []string{".txt"}, LossClass: "L3", Description: "Plain text (verse per line)", CanExtract: true, CanEmit: true},
	}

	response := APIResponse{
		Success: true,
		Data:    formats,
		Meta: &APIMeta{
			Total:     len(formats),
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// Helper functions

func listCapsules() []CapsuleInfo {
	var capsules []CapsuleInfo

	filepath.Walk(ServerConfig.CapsulesDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			return nil
		}

		ext := filepath.Ext(path)
		if ext == ".xz" || ext == ".gz" || ext == ".tar" {
			rel, _ := filepath.Rel(ServerConfig.CapsulesDir, path)
			capsules = append(capsules, CapsuleInfo{
				ID:        rel,
				Name:      filepath.Base(path),
				Path:      rel,
				Size:      info.Size(),
				Format:    detectFormat(path),
				CreatedAt: info.ModTime().UTC().Format(time.RFC3339),
			})
		}
		return nil
	})

	sort.Slice(capsules, func(i, j int) bool {
		return capsules[i].Name < capsules[j].Name
	})

	return capsules
}

func listPlugins() []PluginInfo {
	var plugins []PluginInfo

	// Format plugins
	formatDir := filepath.Join(ServerConfig.PluginsDir, "format")
	if entries, err := os.ReadDir(formatDir); err == nil {
		for _, entry := range entries {
			if entry.IsDir() {
				plugins = append(plugins, PluginInfo{
					Name:        entry.Name(),
					Type:        "format",
					Description: fmt.Sprintf("Format plugin for %s", entry.Name()),
				})
			}
		}
	}

	// Tool plugins
	toolDir := filepath.Join(ServerConfig.PluginsDir, "tool")
	if entries, err := os.ReadDir(toolDir); err == nil {
		for _, entry := range entries {
			if entry.IsDir() {
				plugins = append(plugins, PluginInfo{
					Name:        entry.Name(),
					Type:        "tool",
					Description: fmt.Sprintf("Tool plugin for %s", entry.Name()),
				})
			}
		}
	}

	return plugins
}

func detectFormat(path string) string {
	if strings.HasSuffix(path, ".tar.xz") {
		return "tar.xz"
	}
	if strings.HasSuffix(path, ".tar.gz") {
		return "tar.gz"
	}
	if strings.HasSuffix(path, ".tar") {
		return "tar"
	}
	return "unknown"
}

func readCapsule(path string) (*CapsuleManifest, []ArtifactInfo, error) {
	f, err := safefile.Open(path)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()

	var reader io.Reader = f

	if strings.HasSuffix(path, ".xz") {
		xzReader, err := xz.NewReader(reader)
		if err != nil {
			return nil, nil, fmt.Errorf("xz decompress: %w", err)
		}
		reader = xzReader
	} else if strings.HasSuffix(path, ".gz") {
		gzReader, err := gzip.NewReader(reader)
		if err != nil {
			return nil, nil, fmt.Errorf("gzip decompress: %w", err)
		}
		defer gzReader.Close()
		reader = gzReader
	}

	tarReader := tar.NewReader(reader)
	var manifest *CapsuleManifest
	var artifacts []ArtifactInfo

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, nil, err
		}

		if header.Name == "manifest.json" {
			data, err := io.ReadAll(tarReader)
			if err != nil {
				return nil, nil, err
			}
			manifest = &CapsuleManifest{}
			if err := json.Unmarshal(data, manifest); err != nil {
				return nil, nil, err
			}
		} else if !header.FileInfo().IsDir() {
			artifacts = append(artifacts, ArtifactInfo{
				ID:   header.Name,
				Name: filepath.Base(header.Name),
				Size: header.Size,
			})
		}
	}

	return manifest, artifacts, nil
}

func respond(w http.ResponseWriter, status int, data interface{}) {
	response := APIResponse{
		Success: true,
		Data:    data,
		Meta: &APIMeta{
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(response)
}

func respondError(w http.ResponseWriter, status int, code, message string) {
	response := APIResponse{
		Success: false,
		Error: &APIError{
			Code:    code,
			Message: message,
		},
		Meta: &APIMeta{
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(response)
}
