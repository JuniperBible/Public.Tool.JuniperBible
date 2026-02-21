//go:build !standalone

package swordpure

// Note: This package is a stub for standalone builds.
// The real implementation is in internal/formats/swordpure/handler.go
// which registers itself via its own init() function.
//
// We don't call Config.RegisterEmbedded() here because the internal handler
// provides the full implementation with decryption and IR extraction support.
