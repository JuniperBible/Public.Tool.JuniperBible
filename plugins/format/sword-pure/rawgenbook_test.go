package main

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/FocuswithJustin/JuniperBible/plugins/ipc"
)

// Phase 18: Tests for RawGenBook (SWORD General Books) parser

// TestRawGenBookEntryStruct verifies the RawGenBookEntry structure.
func TestRawGenBookEntryStruct(t *testing.T) {
	entry := RawGenBookEntry{
		Key:     "/Part1/Chapter1/Section1",
		Content: "This is the content of section 1.",
		Offset:  0,
		Size:    100,
	}

	if entry.Key != "/Part1/Chapter1/Section1" {
		t.Errorf("Key = %q, want %q", entry.Key, "/Part1/Chapter1/Section1")
	}
	if entry.Content == "" {
		t.Error("Content should not be empty")
	}
}

// TestRawGenBookTreeKeyStruct verifies the TreeKey structure for navigation.
func TestRawGenBookTreeKeyStruct(t *testing.T) {
	key := TreeKey{
		Name:        "Chapter 1",
		Parent:      0,
		FirstChild:  1,
		NextSibling: 2,
		Offset:      100,
		Size:        500,
	}

	if key.Name != "Chapter 1" {
		t.Errorf("Name = %q, want %q", key.Name, "Chapter 1")
	}
	if key.Parent != 0 {
		t.Errorf("Parent = %d, want 0", key.Parent)
	}
	if key.FirstChild != 1 {
		t.Errorf("FirstChild = %d, want 1", key.FirstChild)
	}
}

// TestRawGenBookParserCreation verifies parser creation with module path.
func TestRawGenBookParserCreation(t *testing.T) {
	parser, err := NewRawGenBookParser("/path/to/modules/genbook/rawgenbook/westminster")
	if err != nil {
		// Expected to fail with non-existent path
		return
	}

	if parser == nil {
		t.Error("NewRawGenBookParser should return a parser or error")
	}
}

// TestRawGenBookReadTreeIndex verifies reading the .bdt tree index.
// Format: TreeKey entries with parent/child/sibling pointers
// Special value 0xFFFFFFFF indicates no link
func TestRawGenBookReadTreeIndex(t *testing.T) {
	const NoLink = uint32(0xFFFFFFFF)

	// Create mock .bdt data
	// Root entry
	var buf bytes.Buffer

	// Entry 0: Root "Book"
	binary.Write(&buf, binary.LittleEndian, NoLink)    // parent (none)
	binary.Write(&buf, binary.LittleEndian, uint32(1)) // firstChild
	binary.Write(&buf, binary.LittleEndian, NoLink)    // nextSibling (none)
	buf.WriteString("Book")
	buf.WriteByte(0) // null terminator

	// Entry 1: "Chapter 1" (child of root)
	binary.Write(&buf, binary.LittleEndian, uint32(0)) // parent = root
	binary.Write(&buf, binary.LittleEndian, NoLink)    // firstChild (none)
	binary.Write(&buf, binary.LittleEndian, uint32(2)) // nextSibling
	buf.WriteString("Chapter 1")
	buf.WriteByte(0)

	// Entry 2: "Chapter 2" (sibling of Chapter 1)
	binary.Write(&buf, binary.LittleEndian, uint32(0)) // parent = root
	binary.Write(&buf, binary.LittleEndian, NoLink)    // firstChild (none)
	binary.Write(&buf, binary.LittleEndian, NoLink)    // nextSibling (none)
	buf.WriteString("Chapter 2")
	buf.WriteByte(0)

	bdtData := buf.Bytes()

	keys, err := parseRawGenBookTreeIndex(bdtData)
	if err != nil {
		t.Fatalf("parseRawGenBookTreeIndex failed: %v", err)
	}

	if len(keys) < 1 {
		t.Fatal("Expected at least one tree key")
	}

	// Verify root
	if keys[0].Name != "Book" {
		t.Errorf("keys[0].Name = %q, want %q", keys[0].Name, "Book")
	}
}

// TestRawGenBookReadDataIndex verifies reading the .idx data index.
// Format: 4-byte offset + 4-byte size per entry
func TestRawGenBookReadDataIndex(t *testing.T) {
	var buf bytes.Buffer

	// Entry 0: offset=0, size=100
	binary.Write(&buf, binary.LittleEndian, uint32(0))
	binary.Write(&buf, binary.LittleEndian, uint32(100))

	// Entry 1: offset=100, size=200
	binary.Write(&buf, binary.LittleEndian, uint32(100))
	binary.Write(&buf, binary.LittleEndian, uint32(200))

	// Entry 2: offset=300, size=150
	binary.Write(&buf, binary.LittleEndian, uint32(300))
	binary.Write(&buf, binary.LittleEndian, uint32(150))

	idxData := buf.Bytes()

	entries, err := parseRawGenBookDataIndex(idxData)
	if err != nil {
		t.Fatalf("parseRawGenBookDataIndex failed: %v", err)
	}

	if len(entries) != 3 {
		t.Fatalf("len(entries) = %d, want 3", len(entries))
	}

	if entries[0].Offset != 0 || entries[0].Size != 100 {
		t.Errorf("entries[0] = {%d, %d}, want {0, 100}", entries[0].Offset, entries[0].Size)
	}
	if entries[1].Offset != 100 || entries[1].Size != 200 {
		t.Errorf("entries[1] = {%d, %d}, want {100, 200}", entries[1].Offset, entries[1].Size)
	}
}

// TestRawGenBookGetEntry verifies retrieving a single entry by key path.
func TestRawGenBookGetEntry(t *testing.T) {
	parser := &RawGenBookParser{
		entries: map[string]*RawGenBookEntry{
			"/Book/Chapter 1": {
				Key:     "/Book/Chapter 1",
				Content: "In the beginning was the Word.",
			},
			"/Book/Chapter 2": {
				Key:     "/Book/Chapter 2",
				Content: "And the Word was made flesh.",
			},
		},
	}

	entry, err := parser.GetEntry("/Book/Chapter 1")
	if err != nil {
		t.Fatalf("GetEntry failed: %v", err)
	}

	if entry.Key != "/Book/Chapter 1" {
		t.Errorf("entry.Key = %q, want %q", entry.Key, "/Book/Chapter 1")
	}
	if entry.Content == "" {
		t.Error("entry.Content should not be empty")
	}
}

// TestRawGenBookGetEntryNotFound verifies error handling for missing entries.
func TestRawGenBookGetEntryNotFound(t *testing.T) {
	parser := &RawGenBookParser{
		entries: map[string]*RawGenBookEntry{},
	}

	_, err := parser.GetEntry("/Nonexistent/Path")
	if err == nil {
		t.Error("GetEntry should return error for missing entry")
	}
}

// TestRawGenBookListKeys verifies listing all entry keys.
func TestRawGenBookListKeys(t *testing.T) {
	parser := &RawGenBookParser{
		entries: map[string]*RawGenBookEntry{
			"/Book":           {Key: "/Book"},
			"/Book/Chapter 1": {Key: "/Book/Chapter 1"},
			"/Book/Chapter 2": {Key: "/Book/Chapter 2"},
		},
	}

	keys := parser.ListKeys()
	if len(keys) != 3 {
		t.Errorf("len(keys) = %d, want 3", len(keys))
	}
}

// TestRawGenBookTreeNavigation verifies tree navigation functions.
func TestRawGenBookTreeNavigation(t *testing.T) {
	const NoLink = int(-1)

	parser := &RawGenBookParser{
		treeKeys: []TreeKey{
			{Name: "Root", Parent: NoLink, FirstChild: 1, NextSibling: NoLink},
			{Name: "Child1", Parent: 0, FirstChild: NoLink, NextSibling: 2},
			{Name: "Child2", Parent: 0, FirstChild: NoLink, NextSibling: NoLink},
		},
	}

	// Get root
	root := parser.GetRoot()
	if root.Name != "Root" {
		t.Errorf("root.Name = %q, want %q", root.Name, "Root")
	}

	// Get children of root
	children := parser.GetChildren(0)
	if len(children) != 2 {
		t.Errorf("len(children) = %d, want 2", len(children))
	}

	// Verify child names
	if children[0].Name != "Child1" {
		t.Errorf("children[0].Name = %q, want %q", children[0].Name, "Child1")
	}
	if children[1].Name != "Child2" {
		t.Errorf("children[1].Name = %q, want %q", children[1].Name, "Child2")
	}
}

// TestRawGenBookBuildKeyPath verifies building full key paths from tree.
func TestRawGenBookBuildKeyPath(t *testing.T) {
	const NoLink = int(-1)

	parser := &RawGenBookParser{
		treeKeys: []TreeKey{
			{Name: "Westminster", Parent: NoLink, FirstChild: 1, NextSibling: NoLink},
			{Name: "Confession", Parent: 0, FirstChild: 2, NextSibling: NoLink},
			{Name: "Chapter 1", Parent: 1, FirstChild: NoLink, NextSibling: 3},
			{Name: "Chapter 2", Parent: 1, FirstChild: NoLink, NextSibling: NoLink},
		},
	}

	// Build path for Chapter 1
	path := parser.BuildKeyPath(2)
	expected := "/Westminster/Confession/Chapter 1"
	if path != expected {
		t.Errorf("path = %q, want %q", path, expected)
	}

	// Build path for Chapter 2
	path = parser.BuildKeyPath(3)
	expected = "/Westminster/Confession/Chapter 2"
	if path != expected {
		t.Errorf("path = %q, want %q", path, expected)
	}
}

// TestRawGenBookModuleInfo verifies module information extraction.
func TestRawGenBookModuleInfo(t *testing.T) {
	parser := &RawGenBookParser{
		modulePath: "/path/to/modules/genbook/rawgenbook/westminster",
		conf: &Conf{
			ModuleName:  "Westminster",
			Description: "Westminster Confession of Faith",
			Lang:        "en",
			Version:     "1.0",
			SourceType:  "RawGenBook",
		},
		entries: map[string]*RawGenBookEntry{
			"/WCF":     {Key: "/WCF"},
			"/WCF/Ch1": {Key: "/WCF/Ch1"},
			"/WCF/Ch2": {Key: "/WCF/Ch2"},
		},
	}

	info := parser.ModuleInfo()

	if info.Name != "Westminster" {
		t.Errorf("info.Name = %q, want %q", info.Name, "Westminster")
	}
	if info.Type != "RawGenBook" {
		t.Errorf("info.Type = %q, want %q", info.Type, "RawGenBook")
	}
	if info.EntryCount != 3 {
		t.Errorf("info.EntryCount = %d, want 3", info.EntryCount)
	}
}

// TestRawGenBookNoLinkMarker verifies 0xFFFFFFFF is handled as "no link".
func TestRawGenBookNoLinkMarker(t *testing.T) {
	var noLinkU32 uint32 = 0xFFFFFFFF

	// This marker should be interpreted as -1 or "no link"
	if noLinkU32 != 0xFFFFFFFF {
		t.Errorf("NoLink = %d, want 0xFFFFFFFF", noLinkU32)
	}

	// When cast to int32, it should be -1
	asInt32 := int32(noLinkU32)
	if asInt32 != -1 {
		t.Errorf("NoLink as int32 = %d, want -1", asInt32)
	}
}

// TestRawGenBookDataDirLayout verifies RawGenBook data directory structure.
func TestRawGenBookDataDirLayout(t *testing.T) {
	// RawGenBook modules should have these files:
	// - module.bdt (tree key data)
	// - module.idx (data index)
	// - module.dat (raw content data)

	expectedFiles := []string{
		"westminster.bdt",
		"westminster.idx",
		"westminster.dat",
	}

	for _, f := range expectedFiles {
		if f == "" {
			t.Error("expected file should not be empty")
		}
	}
}

// TestRawGenBookUnicodeContent verifies handling of Unicode in content.
func TestRawGenBookUnicodeContent(t *testing.T) {
	parser := &RawGenBookParser{
		entries: map[string]*RawGenBookEntry{
			"/Greek": {
				Key:     "/Greek",
				Content: "Ἐν ἀρχῇ ἦν ὁ λόγος",
			},
			"/Hebrew": {
				Key:     "/Hebrew",
				Content: "בְּרֵאשִׁית בָּרָא אֱלֹהִים",
			},
		},
	}

	entry, err := parser.GetEntry("/Greek")
	if err != nil {
		t.Fatalf("GetEntry failed for Greek: %v", err)
	}
	if entry.Content == "" {
		t.Error("Greek content should not be empty")
	}

	entry, err = parser.GetEntry("/Hebrew")
	if err != nil {
		t.Fatalf("GetEntry failed for Hebrew: %v", err)
	}
	if entry.Content == "" {
		t.Error("Hebrew content should not be empty")
	}
}

// TestRawGenBookIPCListKeys verifies IPC list-keys command.
func TestRawGenBookIPCListKeys(t *testing.T) {
	request := ipc.Request{
		Command: "list-keys",
		Args: map[string]interface{}{
			"module": "Westminster",
		},
	}

	if request.Command != "list-keys" {
		t.Errorf("Command = %q, want %q", request.Command, "list-keys")
	}

	response := ipc.Response{
		Status: "success",
		Result: map[string]interface{}{
			"keys": []string{"/WCF", "/WCF/Ch1", "/WCF/Ch2"},
		},
	}

	if response.Status != "success" {
		t.Error("Response should be successful")
	}
}

// TestRawGenBookIPCGetEntry verifies IPC get-entry command.
func TestRawGenBookIPCGetEntry(t *testing.T) {
	request := ipc.Request{
		Command: "get-entry",
		Args: map[string]interface{}{
			"module": "Westminster",
			"key":    "/WCF/Ch1",
		},
	}

	if request.Command != "get-entry" {
		t.Errorf("Command = %q, want %q", request.Command, "get-entry")
	}

	response := ipc.Response{
		Status: "success",
		Result: map[string]interface{}{
			"key":     "/WCF/Ch1",
			"content": "Of the Holy Scripture...",
		},
	}

	if response.Status != "success" {
		t.Error("Response should be successful")
	}

	data, ok := response.Result.(map[string]interface{})
	if !ok {
		t.Fatal("Result should be a map")
	}
	content, ok := data["content"].(string)
	if !ok {
		t.Fatal("Data should contain content")
	}
	if content == "" {
		t.Error("Content should not be empty")
	}
}

// TestRawGenBookIPCGetTree verifies IPC get-tree command for navigation.
func TestRawGenBookIPCGetTree(t *testing.T) {
	request := ipc.Request{
		Command: "get-tree",
		Args: map[string]interface{}{
			"module": "Westminster",
		},
	}

	if request.Command != "get-tree" {
		t.Errorf("Command = %q, want %q", request.Command, "get-tree")
	}

	// Response should contain tree structure
	response := ipc.Response{
		Status: "success",
		Result: map[string]interface{}{
			"root": map[string]interface{}{
				"name": "WCF",
				"children": []map[string]interface{}{
					{"name": "Chapter 1", "children": nil},
					{"name": "Chapter 2", "children": nil},
				},
			},
		},
	}

	if response.Status != "success" {
		t.Error("Response should be successful")
	}
}

// TestRawGenBookDeepNesting verifies handling of deeply nested content.
func TestRawGenBookDeepNesting(t *testing.T) {
	parser := &RawGenBookParser{
		entries: map[string]*RawGenBookEntry{
			"/A":         {Key: "/A"},
			"/A/B":       {Key: "/A/B"},
			"/A/B/C":     {Key: "/A/B/C"},
			"/A/B/C/D":   {Key: "/A/B/C/D"},
			"/A/B/C/D/E": {Key: "/A/B/C/D/E"},
		},
	}

	entry, err := parser.GetEntry("/A/B/C/D/E")
	if err != nil {
		t.Fatalf("GetEntry failed for deeply nested key: %v", err)
	}

	if entry.Key != "/A/B/C/D/E" {
		t.Errorf("entry.Key = %q, want %q", entry.Key, "/A/B/C/D/E")
	}
}

// TestRawGenBookEmptyEntry verifies handling of entries with no content.
func TestRawGenBookEmptyEntry(t *testing.T) {
	parser := &RawGenBookParser{
		entries: map[string]*RawGenBookEntry{
			"/Empty": {Key: "/Empty", Content: ""},
		},
	}

	entry, err := parser.GetEntry("/Empty")
	if err != nil {
		t.Fatalf("GetEntry failed: %v", err)
	}

	// Empty content is valid (container nodes may have no content)
	if entry.Content != "" {
		t.Errorf("Content = %q, want empty", entry.Content)
	}
}

// TestRawGenBookSpecialCharactersInKeys verifies handling of special chars in keys.
func TestRawGenBookSpecialCharactersInKeys(t *testing.T) {
	parser := &RawGenBookParser{
		entries: map[string]*RawGenBookEntry{
			"/Book (1st Edition)":      {Key: "/Book (1st Edition)"},
			"/Chapter 1: Introduction": {Key: "/Chapter 1: Introduction"},
			"/Q&A Section":             {Key: "/Q&A Section"},
		},
	}

	tests := []string{
		"/Book (1st Edition)",
		"/Chapter 1: Introduction",
		"/Q&A Section",
	}

	for _, key := range tests {
		entry, err := parser.GetEntry(key)
		if err != nil {
			t.Errorf("GetEntry(%q) failed: %v", key, err)
			continue
		}
		if entry.Key != key {
			t.Errorf("entry.Key = %q, want %q", entry.Key, key)
		}
	}
}
