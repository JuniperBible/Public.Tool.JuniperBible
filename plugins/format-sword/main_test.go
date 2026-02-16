//go:build !sdk

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestParseConfFile(t *testing.T) {
	// Create temp directory with test conf file
	tempDir, err := os.MkdirTemp("", "sword-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create mods.d directory
	modsD := filepath.Join(tempDir, "mods.d")
	if err := os.MkdirAll(modsD, 0755); err != nil {
		t.Fatalf("failed to create mods.d: %v", err)
	}

	// Write test conf file
	confContent := `[TestModule]
DataPath=./modules/texts/ztext/testmodule/
ModDrv=zText
Lang=en
Description=Test Module Description
Version=1.0.0
`
	confPath := filepath.Join(modsD, "testmodule.conf")
	if err := os.WriteFile(confPath, []byte(confContent), 0644); err != nil {
		t.Fatalf("failed to write conf file: %v", err)
	}

	// Parse the conf file
	module, err := parseConfFile(confPath)
	if err != nil {
		t.Fatalf("parseConfFile failed: %v", err)
	}

	// Verify parsed values
	if module.Name != "TestModule" {
		t.Errorf("expected Name 'TestModule', got '%s'", module.Name)
	}
	if module.Description != "Test Module Description" {
		t.Errorf("expected Description 'Test Module Description', got '%s'", module.Description)
	}
	if module.Version != "1.0.0" {
		t.Errorf("expected Version '1.0.0', got '%s'", module.Version)
	}
	if module.DataPath != "./modules/texts/ztext/testmodule/" {
		t.Errorf("expected DataPath './modules/texts/ztext/testmodule/', got '%s'", module.DataPath)
	}
}

func TestParseSwordModules(t *testing.T) {
	// Create temp directory with SWORD structure
	tempDir, err := os.MkdirTemp("", "sword-modules-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create mods.d directory
	modsD := filepath.Join(tempDir, "mods.d")
	if err := os.MkdirAll(modsD, 0755); err != nil {
		t.Fatalf("failed to create mods.d: %v", err)
	}

	// Create modules directory
	modulesDir := filepath.Join(tempDir, "modules")
	if err := os.MkdirAll(modulesDir, 0755); err != nil {
		t.Fatalf("failed to create modules: %v", err)
	}

	// Write two test conf files
	conf1 := `[ModuleA]
Description=Module A Description
Version=1.0.0
`
	conf2 := `[ModuleB]
Description=Module B Description
Version=2.0.0
`
	if err := os.WriteFile(filepath.Join(modsD, "modulea.conf"), []byte(conf1), 0644); err != nil {
		t.Fatalf("failed to write conf1: %v", err)
	}
	if err := os.WriteFile(filepath.Join(modsD, "moduleb.conf"), []byte(conf2), 0644); err != nil {
		t.Fatalf("failed to write conf2: %v", err)
	}

	// Parse modules
	modules, err := parseSwordModules(tempDir)
	if err != nil {
		t.Fatalf("parseSwordModules failed: %v", err)
	}

	if len(modules) != 2 {
		t.Errorf("expected 2 modules, got %d", len(modules))
	}
}

func TestDetectResult(t *testing.T) {
	result := DetectResult{
		Detected: true,
		Format:   "sword",
		Reason:   "SWORD module detected",
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var parsed DetectResult
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if !parsed.Detected {
		t.Error("expected Detected to be true")
	}
	if parsed.Format != "sword" {
		t.Errorf("expected Format 'sword', got '%s'", parsed.Format)
	}
}

func TestSwordModuleStruct(t *testing.T) {
	module := SwordModule{
		Name:        "KJV",
		Description: "King James Version",
		Version:     "1.0.0",
		DataPath:    "./modules/texts/ztext/kjv/",
		ConfPath:    "/path/to/kjv.conf",
		ModDrv:      "zText",
		Lang:        "en",
		Encoding:    "UTF-8",
	}

	if module.Name != "KJV" {
		t.Errorf("expected Name 'KJV', got '%s'", module.Name)
	}
	if module.Lang != "en" {
		t.Errorf("expected Lang 'en', got '%s'", module.Lang)
	}
}

func TestExtractIRResult(t *testing.T) {
	result := ExtractIRResult{
		IRPath:    "/tmp/test.ir.json",
		LossClass: "L2",
		LossReport: &LossReport{
			SourceFormat: "SWORD",
			TargetFormat: "IR",
			LossClass:    "L2",
			Warnings:     []string{"Full text extraction requires libsword"},
		},
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var parsed ExtractIRResult
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if parsed.LossClass != "L2" {
		t.Errorf("expected LossClass 'L2', got '%s'", parsed.LossClass)
	}
	if parsed.LossReport == nil {
		t.Error("expected LossReport to be non-nil")
	}
}

func TestEmitNativeResult(t *testing.T) {
	result := EmitNativeResult{
		OutputPath: "/tmp/output/",
		Format:     "SWORD",
		LossClass:  "L2",
	}

	if result.Format != "SWORD" {
		t.Errorf("expected Format 'SWORD', got '%s'", result.Format)
	}
}

func TestCorpusStruct(t *testing.T) {
	corpus := Corpus{
		ID:           "TestBible",
		Version:      "1.0.0",
		ModuleType:   "BIBLE",
		Language:     "en",
		Title:        "Test Bible",
		SourceFormat: "SWORD",
		LossClass:    "L2",
		Attributes: map[string]string{
			"_sword_module_name": "TestBible",
		},
		Documents: []*Document{
			{
				ID:    "Gen",
				Title: "Genesis",
				Order: 1,
			},
		},
	}

	if corpus.ID != "TestBible" {
		t.Errorf("expected ID 'TestBible', got '%s'", corpus.ID)
	}
	if corpus.LossClass != "L2" {
		t.Errorf("expected LossClass 'L2', got '%s'", corpus.LossClass)
	}
	if len(corpus.Documents) != 1 {
		t.Errorf("expected 1 document, got %d", len(corpus.Documents))
	}
	if corpus.Attributes["_sword_module_name"] != "TestBible" {
		t.Error("expected _sword_module_name attribute")
	}
}

func TestIPCStructs(t *testing.T) {
	// Test IPCRequest
	req := IPCRequest{
		Command: "detect",
		Args: map[string]interface{}{
			"path": "/test/path",
		},
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	var parsedReq IPCRequest
	if err := json.Unmarshal(data, &parsedReq); err != nil {
		t.Fatalf("failed to unmarshal request: %v", err)
	}

	if parsedReq.Command != "detect" {
		t.Errorf("expected Command 'detect', got '%s'", parsedReq.Command)
	}

	// Test IPCResponse
	resp := IPCResponse{
		Status: "ok",
		Result: map[string]bool{"detected": true},
	}

	respData, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("failed to marshal response: %v", err)
	}

	var parsedResp IPCResponse
	if err := json.Unmarshal(respData, &parsedResp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if parsedResp.Status != "ok" {
		t.Errorf("expected Status 'ok', got '%s'", parsedResp.Status)
	}
}

func TestLossReportStruct(t *testing.T) {
	report := LossReport{
		SourceFormat: "SWORD",
		TargetFormat: "IR",
		LossClass:    "L2",
		LostElements: []LostElement{
			{
				Path:        "Gen.1.1",
				ElementType: "verse_content",
				Reason:      "binary format cannot be parsed",
			},
		},
		Warnings: []string{
			"Full text extraction requires libsword",
		},
	}

	if report.LossClass != "L2" {
		t.Errorf("expected LossClass 'L2', got '%s'", report.LossClass)
	}
	if len(report.LostElements) != 1 {
		t.Errorf("expected 1 lost element, got %d", len(report.LostElements))
	}
	if len(report.Warnings) != 1 {
		t.Errorf("expected 1 warning, got %d", len(report.Warnings))
	}
}

func TestEnumerateResult(t *testing.T) {
	result := EnumerateResult{
		Entries: []EnumerateEntry{
			{
				Path:      "mods.d/kjv.conf",
				SizeBytes: 256,
				IsDir:     false,
				Metadata: map[string]string{
					"module_name": "KJV",
				},
			},
			{
				Path:      "modules",
				SizeBytes: 0,
				IsDir:     true,
			},
		},
	}

	if len(result.Entries) != 2 {
		t.Errorf("expected 2 entries, got %d", len(result.Entries))
	}

	confEntry := result.Entries[0]
	if confEntry.IsDir {
		t.Error("conf entry should not be a directory")
	}
	if confEntry.Metadata["module_name"] != "KJV" {
		t.Error("expected module_name metadata")
	}

	dirEntry := result.Entries[1]
	if !dirEntry.IsDir {
		t.Error("modules entry should be a directory")
	}
}
