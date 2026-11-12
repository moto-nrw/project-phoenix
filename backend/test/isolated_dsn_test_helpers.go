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

// PackageTestDSN returns the DSN of the package clone, for a test that needs
// a pool with its own driver against it (for example a fault-injecting
// connector) but nothing database-global: it isolates through its own
// tenant (OwnTenant) and stays off the one isolated-clone slot a package
// has, on which every SetupIsolatedTestDB test waits for the one before it.
func PackageTestDSN(tb testing.TB) string {
	tb.Helper()
	SetupTestDB(tb)
	if perTestDatabases || hasIsolatedTestDB(tb) {
		tb.Fatal("PackageTestDSN is for tests on the package clone")
	}
	return packageClone.DSN
}
