package auth

import "os"

// Test hooks (export_test.go pattern).
var (
	BrowserCommand    = browserCommand
	BrowserCommandFor = browserCommandFor
	NewFileStore      = newFileStore
)

// SetUserHomeDir overrides os.UserHomeDir for tests. Call the returned
// restore function to reset.
func SetUserHomeDir(fn func() (string, error)) func() {
	prev := userHomeDir
	userHomeDir = fn
	return func() { userHomeDir = prev }
}

func ResetUserHomeDir() {
	userHomeDir = os.UserHomeDir
}
