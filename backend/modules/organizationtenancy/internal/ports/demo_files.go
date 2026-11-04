package ports

import "context"

// DemoSchoolFileStore holds the work list and the markers of the demo
// process's file erasure (#5544) on its restricted connection. Each call is
// one statement; the demo lease admits a single runner.
type DemoSchoolFileStore interface {
	// RetiredDemoSchools lists at most limit due schools, oldest first.
	RetiredDemoSchools(ctx context.Context, standingSlug string, limit int) ([]int64, error)
	// RecordDemoSchoolFileErasure writes the marker with the removed counts
	// only while the school is still due; recorded is false otherwise.
	RecordDemoSchoolFileErasure(ctx context.Context, standingSlug string, schoolID int64, counts map[string]int64) (recorded bool, err error)
}
