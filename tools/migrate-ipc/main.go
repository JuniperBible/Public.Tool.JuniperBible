// Tool migrate-ipc migrates plugins from local IPC types to shared ipc package.
//
// Usage: go run tools/migrate-ipc/main.go plugins/format/*/main.go
package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io"
	"os"
	"strings"
)

// Types to remove (will be replaced with ipc.* versions)
var typesToRemove = map[string]string{
	"IPCRequest":      "ipc.Request",
	"IPCResponse":     "ipc.Response",
	"DetectResult":    "ipc.DetectResult",
	"IngestResult":    "ipc.IngestResult",
	"EnumerateResult": "ipc.EnumerateResult",
	"EnumerateEntry":  "ipc.EnumerateEntry",
}

// Functions to remove (replaced by ipc package functions)
var funcsToRemove = map[string]bool{
	"respond":      true,
	"respondError": true,
}

// formatNode is the formatter function (injectable for testing).
var formatNode = format.Node

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run executes the migration logic and returns an exit code.
func run(args []string, stdout, stderr io.Writer) int {
	if len(args) < 1 {
		fmt.Fprintln(stderr, "Usage: migrate-ipc <file.go> ...")
		return 1
	}

	hasError := false
	for _, path := range args {
		if err := migrateFile(path); err != nil {
			fmt.Fprintf(stderr, "Error migrating %s: %v\n", path, err)
			hasError = true
		} else {
			fmt.Fprintf(stdout, "Migrated: %s\n", path)
		}
	}

	if hasError {
		return 1
	}
	return 0
}

func migrateFile(path string) error {
	// Read and validate the file
	src, err := readAndValidateFile(path)
	if err != nil {
		return err
	}

	// Parse the file
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, src, parser.ParseComments)
	if err != nil {
		return fmt.Errorf("parse error: %w", err)
	}

	// Transform the AST
	declsToRemove, needsIPCImport := identifyDeclsToRemove(file)
	if needsIPCImport {
		addIPCImport(file)
	}
	file.Decls = filterDecls(file.Decls, declsToRemove)

	// Rewrite AST nodes
	rewriteFunctionCalls(file)
	rewriteCompositeLits(file)
	rewriteMainFunc(file)

	// Format and write the result
	return formatAndWriteFile(path, fset, file)
}

// readAndValidateFile reads a file and checks if it needs migration.
// Returns nil, nil if the file should be skipped.
func readAndValidateFile(path string) ([]byte, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	// Skip if already migrated
	if bytes.Contains(src, []byte("github.com/FocuswithJustin/JuniperBible/plugins/ipc")) {
		fmt.Printf("  Already migrated, skipping: %s\n", path)
		return nil, nil
	}

	// Skip if no IPCRequest type (not a plugin that needs migration)
	if !bytes.Contains(src, []byte("type IPCRequest struct")) {
		fmt.Printf("  No IPCRequest type, skipping: %s\n", path)
		return nil, nil
	}

	return src, nil
}

// identifyDeclsToRemove finds declarations that should be removed during migration.
// Returns the list of declarations to remove and whether IPC import is needed.
func identifyDeclsToRemove(file *ast.File) ([]ast.Decl, bool) {
	var declsToRemove []ast.Decl
	needsIPCImport := false

	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.GenDecl:
			if shouldRemoveTypeDecl(d) {
				declsToRemove = append(declsToRemove, decl)
				needsIPCImport = true
			}
		case *ast.FuncDecl:
			if shouldRemoveFuncDecl(d) {
				declsToRemove = append(declsToRemove, decl)
			}
		}
	}

	return declsToRemove, needsIPCImport
}

// shouldRemoveTypeDecl checks if a type declaration should be removed.
func shouldRemoveTypeDecl(d *ast.GenDecl) bool {
	if d.Tok != token.TYPE {
		return false
	}

	for _, spec := range d.Specs {
		if ts, ok := spec.(*ast.TypeSpec); ok {
			if _, shouldRemove := typesToRemove[ts.Name.Name]; shouldRemove {
				return true
			}
		}
	}
	return false
}

// shouldRemoveFuncDecl checks if a function declaration should be removed.
func shouldRemoveFuncDecl(d *ast.FuncDecl) bool {
	// Only remove top-level functions, not methods
	return d.Recv == nil && funcsToRemove[d.Name.Name]
}

// rewriteFunctionCalls rewrites function calls from old IPC functions to new ones.
func rewriteFunctionCalls(file *ast.File) {
	ast.Inspect(file, func(n ast.Node) bool {
		if callExpr, ok := n.(*ast.CallExpr); ok {
			rewriteCallExpr(callExpr)
		}
		return true
	})
}

// rewriteCallExpr rewrites a single call expression if it matches migration patterns.
func rewriteCallExpr(node *ast.CallExpr) {
	ident, ok := node.Fun.(*ast.Ident)
	if !ok {
		return
	}

	switch ident.Name {
	case "respond":
		node.Fun = &ast.SelectorExpr{
			X:   ast.NewIdent("ipc"),
			Sel: ast.NewIdent("MustRespond"),
		}
	case "respondError":
		rewriteRespondError(node)
	}
}

// rewriteRespondError rewrites respondError calls, handling fmt.Sprintf optimization.
func rewriteRespondError(node *ast.CallExpr) {
	// Check if it's respondError(fmt.Sprintf(...))
	if len(node.Args) == 1 {
		if call, ok := node.Args[0].(*ast.CallExpr); ok {
			if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
				if x, ok := sel.X.(*ast.Ident); ok && x.Name == "fmt" && sel.Sel.Name == "Sprintf" {
					// Convert to ipc.RespondErrorf(format, args...)
					node.Fun = &ast.SelectorExpr{
						X:   ast.NewIdent("ipc"),
						Sel: ast.NewIdent("RespondErrorf"),
					}
					node.Args = call.Args
					return
				}
			}
		}
	}
	node.Fun = &ast.SelectorExpr{
		X:   ast.NewIdent("ipc"),
		Sel: ast.NewIdent("RespondError"),
	}
}

// formatAndWriteFile formats the AST and writes it to disk with post-processing.
func formatAndWriteFile(path string, fset *token.FileSet, file *ast.File) error {
	var buf bytes.Buffer
	if err := formatNode(&buf, fset, file); err != nil {
		return fmt.Errorf("format error: %w", err)
	}

	// Post-process: replace remaining type references that AST couldn't handle
	output := postProcessTypeReferences(buf.String())

	return os.WriteFile(path, []byte(output), 0644)
}

// postProcessTypeReferences replaces type references that AST transformation missed.
func postProcessTypeReferences(output string) string {
	for oldType, newType := range typesToRemove {
		// Replace in variable declarations like "var req IPCRequest"
		output = strings.ReplaceAll(output, "var req "+oldType, "var req "+newType)
		output = strings.ReplaceAll(output, "var resp "+oldType, "var resp "+newType)
		// Replace in slice types
		output = strings.ReplaceAll(output, "[]"+oldType, "[]"+newType)
	}
	return output
}

func addIPCImport(file *ast.File) {
	ipcImport := &ast.ImportSpec{
		Path: &ast.BasicLit{
			Kind:  token.STRING,
			Value: `"github.com/FocuswithJustin/JuniperBible/plugins/ipc"`,
		},
	}

	// Find the import declaration and add to it
	for _, decl := range file.Decls {
		if genDecl, ok := decl.(*ast.GenDecl); ok && genDecl.Tok == token.IMPORT {
			genDecl.Specs = append(genDecl.Specs, ipcImport)
			return
		}
	}

	// No import declaration found, create one
	importDecl := &ast.GenDecl{
		Tok:   token.IMPORT,
		Specs: []ast.Spec{ipcImport},
	}
	file.Decls = append([]ast.Decl{importDecl}, file.Decls...)
}

func filterDecls(decls []ast.Decl, toRemove []ast.Decl) []ast.Decl {
	removeSet := make(map[ast.Decl]bool)
	for _, d := range toRemove {
		removeSet[d] = true
	}

	var result []ast.Decl
	for _, d := range decls {
		if !removeSet[d] {
			result = append(result, d)
		}
	}
	return result
}

func rewriteCompositeLits(file *ast.File) {
	ast.Inspect(file, func(n ast.Node) bool {
		if lit, ok := n.(*ast.CompositeLit); ok {
			if ident, ok := lit.Type.(*ast.Ident); ok {
				if replacement, ok := typesToRemove[ident.Name]; ok {
					// Replace with selector expr
					parts := strings.Split(replacement, ".")
					if len(parts) == 2 {
						lit.Type = &ast.SelectorExpr{
							X:   ast.NewIdent(parts[0]),
							Sel: ast.NewIdent(parts[1]),
						}
					}
				}
			}
		}
		// Handle unary expressions like &DetectResult{}
		if unary, ok := n.(*ast.UnaryExpr); ok && unary.Op == token.AND {
			if lit, ok := unary.X.(*ast.CompositeLit); ok {
				if ident, ok := lit.Type.(*ast.Ident); ok {
					if replacement, ok := typesToRemove[ident.Name]; ok {
						parts := strings.Split(replacement, ".")
						if len(parts) == 2 {
							lit.Type = &ast.SelectorExpr{
								X:   ast.NewIdent(parts[0]),
								Sel: ast.NewIdent(parts[1]),
							}
						}
					}
				}
			}
		}
		return true
	})
}

func rewriteMainFunc(file *ast.File) {
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Name.Name == "main" {
			// Look for the pattern:
			//   var req IPCRequest
			//   if err := json.NewDecoder(os.Stdin).Decode(&req); err != nil {
			//     respondError(...)
			//   }
			// And replace with:
			//   req, err := ipc.ReadRequest()
			//   if err != nil {
			//     ipc.RespondErrorf(...)
			//   }

			// This is complex AST manipulation, so we'll do it via string replacement
			// in the post-processing step instead
		}
	}
}
