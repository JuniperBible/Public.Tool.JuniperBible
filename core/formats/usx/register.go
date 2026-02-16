//go:build !standalone

package usx

func init() {
	// Register for embedded use via the SDK dispatcher
	// Actual registration will be handled by main binary importing this package
}
