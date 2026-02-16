// Package main contains shared type definitions for the sword-pure plugin.
// This file has no build tags so types are available for both SDK and non-SDK builds.
package main

// skipIRExtraction can be set to true in tests to speed up capsule creation.
// This skips the expensive IR extraction step.
var skipIRExtraction = false

// PluginInfo contains plugin metadata.
type PluginInfo struct {
	PluginID    string   `json:"plugin_id"`
	Version     string   `json:"version"`
	Kind        string   `json:"kind"`
	Description string   `json:"description"`
	Formats     []string `json:"formats"`
}

// ModuleInfo contains metadata about a SWORD module.
type ModuleInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Type        string `json:"type"`
	Language    string `json:"language"`
	Version     string `json:"version"`
	Encoding    string `json:"encoding"`
	DataPath    string `json:"data_path,omitempty"`
	Compressed  bool   `json:"compressed"`
	Encrypted   bool   `json:"encrypted"`
}

// Verse represents a single verse.
type Verse struct {
	Ref  string `json:"ref"`
	Text string `json:"text"`
}
