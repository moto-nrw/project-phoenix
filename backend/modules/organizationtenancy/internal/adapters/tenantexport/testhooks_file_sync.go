//go:build testhooks

package tenantexport

import "os"

// SkipFileSyncForTests makes the exports of a test binary skip the sync of
// each artifact. A package writes several hundred files, and on a test
// directory on disk their syncs took most of the package's runtime under
// load (#6452); no test reads a file back after a crash. Call it from
// TestMain, before a test runs.
func SkipFileSyncForTests() {
	syncFile = func(*os.File) error { return nil }
}
