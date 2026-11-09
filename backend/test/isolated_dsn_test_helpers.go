//go:build testhooks

package test

import "testing"

// IsolatedTestDSN returns the DSN of the test's isolated clone, for a test
// that needs a pool with its own driver or session settings (for example a
// client read timeout) against the same clone. Call SetupIsolatedTestDB
// first. Only tests call it, so it compiles with the testhooks tag only.
func IsolatedTestDSN(tb testing.TB) string {
	tb.Helper()
	dsn, ok := isolatedTestDSN(tb)
	if !ok {
		tb.Fatal("IsolatedTestDSN needs SetupIsolatedTestDB in the same top-level test")
	}
	return dsn
}
