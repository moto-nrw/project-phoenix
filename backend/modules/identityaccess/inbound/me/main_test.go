package me_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// TestMain gives every test in this binary its own tenant (#2419), so parallel
// tests in the shared package clone cannot see or overwrite each other's rows.
//
// It also creates the package-local public/uploads/avatars/global before any
// test runs. The old-avatar cleanup removes a working-directory relative
// public/ path, so TestDeleteAvatar_WithAvatar has to write there. Created
// mid-run, that directory would make publicDir find it while the upload
// storage keeps the public directory it resolved and cached first.
func TestMain(m *testing.M) {
	testpkg.PerTestTenants()
	if err := os.MkdirAll(filepath.Join("public", "uploads", "avatars", "global"), 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "create package public dir: %v\n", err)
		os.Exit(1)
	}
	testpkg.Run(m)
}
