//go:build testhooks

package tenantexport

import "os"

// Test binaries skip the sync of each artifact. A package writes several
// hundred files, and on a test directory on disk their syncs took most of
// the runtime of the export tests under load (#6452); no test reads a file
// back after a crash. Production builds carry no testhooks tag and sync.
func init() {
	syncFile = func(*os.File) error { return nil }
}
