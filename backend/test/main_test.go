package test

import "testing"

// The helper package's own tests use the same lifecycle every other package
// does: their own tenant per test, and the leftover gate at the end (#2419).
func TestMain(m *testing.M) {
	PerTestTenants()
	Run(m)
}
