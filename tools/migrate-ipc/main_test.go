package main

import (
	"bytes"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunNoArgs(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{}, &stdout, &stderr)
	if code != 1 {
		t.Errorf("expected exit code 1, got %d", code)
	}
	if !strings.Contains(stderr.String(), "Usage:") {
		t.Errorf("expected usage message in stderr, got: %s", stderr.String())
	}
}

func TestRunSuccess(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "migrate-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a file that doesn't need migration (no IPCRequest)
	content := `package main

func main() {}
`
	path := filepath.Join(tempDir, "main.go")
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{path}, &stdout, &stderr)
	if code != 0 {
		t.Errorf("expected exit code 0, got %d, stderr: %s", code, stderr.String())
	}
}

func TestRunError(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"/nonexistent/path.go"}, &stdout, &stderr)
	if code != 1 {
		t.Errorf("expected exit code 1, got %d", code)
	}
	if !strings.Contains(stderr.String(), "Error migrating") {
		t.Errorf("expected error message, got: %s", stderr.String())
	}
}

func TestRunMixedResults(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "migrate-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a valid file
	validContent := `package main

func main() {}
`
	validPath := filepath.Join(tempDir, "valid.go")
	if err := os.WriteFile(validPath, []byte(validContent), 0600); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	var stdout, stderr bytes.Buffer
	// One valid file, one nonexistent
	code := run([]string{validPath, "/nonexistent.go"}, &stdout, &stderr)
	if code != 1 {
		t.Errorf("expected exit code 1 due to error, got %d", code)
	}
}

func TestMigrateFileReadError(t *testing.T) {
	err := migrateFile("/nonexistent/path/file.go")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestMigrateFileAlreadyMigrated(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "migrate-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a file that's already migrated
	content := `package main

import (
	"github.com/JuniperBible/Public.Tool.JuniperBible/plugins/ipc"
)

func main() {
	req, _ := ipc.ReadRequest()
	_ = req
}
`
	path := filepath.Join(tempDir, "main.go")
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	err = migrateFile(path)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestMigrateFileNoIPCRequest(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "migrate-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a file without IPCRequest type
	content := `package main

import "fmt"

func main() {
}
`
	path := filepath.Join(tempDir, "main.go")
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	err = migrateFile(path)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestMigrateFileParseError(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "migrate-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a file with invalid Go syntax
	content := `package main

type IPCRequest struct {
	invalid syntax here {{{{
}
`
	path := filepath.Join(tempDir, "main.go")
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	err = migrateFile(path)
	if err == nil {
		t.Error("expected parse error")
	}
	if !strings.Contains(err.Error(), "parse error") {
		t.Errorf("expected parse error, got: %v", err)
	}
}

func TestMigrateFileSuccess(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "migrate-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a file that needs migration
	content := `package main

import (
	"encoding/json"
	"fmt"
	"os"
)

type IPCRequest struct {
	Method string
	Params map[string]interface{}
}

type IPCResponse struct {
	Result interface{}
	Error  string
}

type DetectResult struct {
	Detected bool
	Format   string
}

func respond(resp IPCResponse) {
	json.NewEncoder(os.Stdout).Encode(resp)
}

func respondError(msg string) {
	respond(IPCResponse{Error: msg})
}

func main() {
	var req IPCRequest
	if err := json.NewDecoder(os.Stdin).Decode(&req); err != nil {
		respondError(fmt.Sprintf("decode error: %v", err))
		return
	}
	respond(IPCResponse{Result: &DetectResult{Detected: true}})
}
`
	path := filepath.Join(tempDir, "main.go")
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	err = migrateFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify migration
	migrated, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read migrated file: %v", err)
	}

	migratedStr := string(migrated)

	// Check that ipc import was added
	if !strings.Contains(migratedStr, "github.com/JuniperBible/Public.Tool.JuniperBible/plugins/ipc") {
		t.Error("expected ipc import to be added")
	}

	// Check that respond was replaced with ipc.MustRespond
	if !strings.Contains(migratedStr, "ipc.MustRespond") {
		t.Error("expected respond to be replaced with ipc.MustRespond")
	}

	// Check that respondError with fmt.Sprintf was replaced with ipc.RespondErrorf
	if !strings.Contains(migratedStr, "ipc.RespondErrorf") {
		t.Error("expected respondError(fmt.Sprintf(...)) to be replaced with ipc.RespondErrorf")
	}
}

func TestFilterDecls(t *testing.T) {
	decl1 := &ast.GenDecl{}
	decl2 := &ast.GenDecl{}
	decl3 := &ast.GenDecl{}

	decls := []ast.Decl{decl1, decl2, decl3}
	toRemove := []ast.Decl{decl2}

	result := filterDecls(decls, toRemove)

	if len(result) != 2 {
		t.Errorf("expected 2 decls, got %d", len(result))
	}

	for _, d := range result {
		if d == decl2 {
			t.Error("decl2 should have been removed")
		}
	}
}

func TestAddIPCImportExistingImport(t *testing.T) {
	src := `package main

import "fmt"

func main() {}
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	addIPCImport(file)

	// Check that import was added
	found := false
	for _, decl := range file.Decls {
		if genDecl, ok := decl.(*ast.GenDecl); ok && genDecl.Tok == token.IMPORT {
			for _, spec := range genDecl.Specs {
				if impSpec, ok := spec.(*ast.ImportSpec); ok {
					if strings.Contains(impSpec.Path.Value, "plugins/ipc") {
						found = true
					}
				}
			}
		}
	}
	if !found {
		t.Error("expected ipc import to be added")
	}
}

func TestAddIPCImportNoExistingImport(t *testing.T) {
	src := `package main

func main() {}
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	addIPCImport(file)

	// Check that import declaration was created
	found := false
	for _, decl := range file.Decls {
		if genDecl, ok := decl.(*ast.GenDecl); ok && genDecl.Tok == token.IMPORT {
			for _, spec := range genDecl.Specs {
				if impSpec, ok := spec.(*ast.ImportSpec); ok {
					if strings.Contains(impSpec.Path.Value, "plugins/ipc") {
						found = true
					}
				}
			}
		}
	}
	if !found {
		t.Error("expected ipc import to be added")
	}
}

func TestRewriteCompositeLits(t *testing.T) {
	src := `package main

func main() {
	x := DetectResult{Detected: true}
	y := &IngestResult{}
	_ = x
	_ = y
}
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	rewriteCompositeLits(file)

	// Check that types were rewritten
	var foundSelector bool
	ast.Inspect(file, func(n ast.Node) bool {
		if lit, ok := n.(*ast.CompositeLit); ok {
			if sel, ok := lit.Type.(*ast.SelectorExpr); ok {
				if x, ok := sel.X.(*ast.Ident); ok && x.Name == "ipc" {
					foundSelector = true
				}
			}
		}
		return true
	})

	if !foundSelector {
		t.Error("expected composite literals to be rewritten with ipc selector")
	}
}

func TestRewriteMainFunc(t *testing.T) {
	src := `package main

func main() {
	// This function exists
}
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	// Should not panic
	rewriteMainFunc(file)
}

func TestMigrateWithRespondError(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "migrate-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a file with respondError(string) call
	content := `package main

import (
	"encoding/json"
	"os"
)

type IPCRequest struct {
	Method string
}

type IPCResponse struct {
	Error string
}

func respond(resp IPCResponse) {
	json.NewEncoder(os.Stdout).Encode(resp)
}

func respondError(msg string) {
	respond(IPCResponse{Error: msg})
}

func main() {
	respondError("simple error")
}
`
	path := filepath.Join(tempDir, "main.go")
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	err = migrateFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	migrated, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read migrated file: %v", err)
	}

	if !strings.Contains(string(migrated), "ipc.RespondError") {
		t.Error("expected ipc.RespondError in migrated file")
	}
}

func TestTypesToRemove(t *testing.T) {
	// Verify the typesToRemove map contains expected entries
	expectedTypes := []string{
		"IPCRequest",
		"IPCResponse",
		"DetectResult",
		"IngestResult",
		"EnumerateResult",
		"EnumerateEntry",
	}

	for _, typ := range expectedTypes {
		if _, ok := typesToRemove[typ]; !ok {
			t.Errorf("expected %s in typesToRemove", typ)
		}
	}
}

func TestFuncsToRemove(t *testing.T) {
	// Verify the funcsToRemove map contains expected entries
	expectedFuncs := []string{"respond", "respondError"}

	for _, fn := range expectedFuncs {
		if !funcsToRemove[fn] {
			t.Errorf("expected %s in funcsToRemove", fn)
		}
	}
}

func TestRewriteCompositeLitsWithStarExpr(t *testing.T) {
	// Test the star expression handling path: *Type{} syntax (rare but possible)
	src := `package main

type DetectResult struct {
	Detected bool
}

func main() {
	// This is a pointer type composite literal
	var x *DetectResult = (*DetectResult)(&DetectResult{Detected: true})
	_ = x
}
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	rewriteCompositeLits(file)
	// Should not panic - the function handles these cases
}

func TestMigrateFileWithEnumerateTypes(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "migrate-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a file with EnumerateResult and EnumerateEntry types
	content := `package main

import (
	"encoding/json"
	"os"
)

type IPCRequest struct {
	Method string
}

type IPCResponse struct {
	Result interface{}
}

type EnumerateResult struct {
	Entries []EnumerateEntry
}

type EnumerateEntry struct {
	Name string
}

func respond(resp IPCResponse) {
	json.NewEncoder(os.Stdout).Encode(resp)
}

func main() {
	respond(IPCResponse{Result: &EnumerateResult{
		Entries: []EnumerateEntry{{Name: "test"}},
	}})
}
`
	path := filepath.Join(tempDir, "main.go")
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	err = migrateFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	migrated, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read migrated file: %v", err)
	}

	migratedStr := string(migrated)

	// Check that slice types were replaced
	if !strings.Contains(migratedStr, "[]ipc.EnumerateEntry") {
		t.Error("expected []EnumerateEntry to be replaced with []ipc.EnumerateEntry")
	}
}

func TestMigrateFileFormatError(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "migrate-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a file that needs migration
	content := `package main

import (
	"encoding/json"
	"os"
)

type IPCRequest struct {
	Method string
}

type IPCResponse struct {
	Result interface{}
}

func respond(resp IPCResponse) {
	json.NewEncoder(os.Stdout).Encode(resp)
}

func main() {
	respond(IPCResponse{})
}
`
	path := filepath.Join(tempDir, "main.go")
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	// Inject failing formatter
	orig := formatNode
	defer func() { formatNode = orig }()
	formatNode = func(dst io.Writer, fset *token.FileSet, node any) error {
		return errors.New("format error")
	}

	err = migrateFile(path)
	if err == nil {
		t.Error("expected format error")
	}
	if !strings.Contains(err.Error(), "format error") {
		t.Errorf("expected format error, got: %v", err)
	}
}

func TestMigrateFileWithIngestResult(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "migrate-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	content := `package main

import (
	"encoding/json"
	"os"
)

type IPCRequest struct {
	Method string
}

type IPCResponse struct {
	Result interface{}
}

type IngestResult struct {
	Path string
}

func respond(resp IPCResponse) {
	json.NewEncoder(os.Stdout).Encode(resp)
}

func main() {
	respond(IPCResponse{Result: &IngestResult{Path: "/tmp"}})
}
`
	path := filepath.Join(tempDir, "main.go")
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	err = migrateFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestMainFunction tests the main function via subprocess.
func TestMainFunction(t *testing.T) {
	if os.Getenv("TEST_MAIN") == "1" {
		os.Args = []string{"migrate-ipc"}
		main()
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=TestMainFunction")
	cmd.Env = append(os.Environ(), "TEST_MAIN=1")
	err := cmd.Run()

	// Should exit 1 with no args
	if err == nil {
		t.Error("expected non-zero exit")
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		if exitErr.ExitCode() != 1 {
			t.Errorf("expected exit code 1, got %d", exitErr.ExitCode())
		}
	}
}

// TestMainFunctionSuccess tests main with a valid file.
func TestMainFunctionSuccess(t *testing.T) {
	if os.Getenv("TEST_MAIN_SUCCESS") == "1" {
		// Create a temp file
		tempDir, err := os.MkdirTemp("", "migrate-test-*")
		if err != nil {
			os.Exit(1)
		}
		defer os.RemoveAll(tempDir)

		content := `package main
func main() {}
`
		path := filepath.Join(tempDir, "main.go")
		if err := os.WriteFile(path, []byte(content), 0600); err != nil {
			os.Exit(1)
		}

		os.Args = []string{"migrate-ipc", path}
		main()
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=TestMainFunctionSuccess")
	cmd.Env = append(os.Environ(), "TEST_MAIN_SUCCESS=1")
	err := cmd.Run()

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			t.Errorf("expected exit code 0, got %d", exitErr.ExitCode())
		}
	}
}
