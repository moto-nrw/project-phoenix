package ports

import "context"

// DemoSchoolDatabaseStore is the demo process's side of the database erasure
// of retired demo schools (#5559) on its restricted connection. Each call is
// one statement and so one transaction.
type DemoSchoolDatabaseStore interface {
	// DueDemoSchoolDatabases lists at most limit due schools, oldest first.
	DueDemoSchoolDatabases(ctx context.Context, limit int) ([]int64, error)
	// EraseDemoSchoolDatabase erases one school when it is still due. status
	// is "erased", "not_due", "busy" or "failed"; a failure carries only its
	// SQLSTATE, never a message that could hold a personal value.
	EraseDemoSchoolDatabase(ctx context.Context, schoolID int64) (status, errorCode string, err error)
}
