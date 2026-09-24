package timetableplanning

import "github.com/moto-nrw/project-phoenix/modules/timetable"

// BulkCancelOptionsForTest builds the switches of a bulk cancellation for the
// external integration tests, which may not import the public timetable
// package (module-behavior-test role).
func BulkCancelOptionsForTest(dryRun, includeClosingDaySeries bool) timetable.BulkCancelOptions {
	return timetable.BulkCancelOptions{DryRun: dryRun, IncludeClosingDaySeries: includeClosingDaySeries}
}
