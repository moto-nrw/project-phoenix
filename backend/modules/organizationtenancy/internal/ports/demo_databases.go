package ports

import "context"

// The outcomes of erasing one retired demo school, as the database function
// platform.erase_retired_demo_school reports them.
const (
	// DemoSchoolErased: the school and every row of it are gone.
	DemoSchoolErased = "erased"
	// DemoSchoolNotDue: the school was no longer due; nothing changed.
	DemoSchoolNotDue = "not_due"
	// DemoSchoolBusy: another transaction held a lock; nothing changed and
	// the next run tries again.
	DemoSchoolBusy = "busy"
	// DemoSchoolFailed: the erasure failed and rolled back.
	DemoSchoolFailed = "failed"
)

// DemoSchoolDatabaseStore is the demo process's side of the database erasure
// of retired demo schools (#5559) on its restricted connection. Each call is
// one statement and so one transaction.
type DemoSchoolDatabaseStore interface {
	// DueDemoSchoolDatabases lists at most limit due schools in random order.
	DueDemoSchoolDatabases(ctx context.Context, limit int) ([]int64, error)
	// EraseDemoSchoolDatabase erases one school when it is still due. A
	// failure carries only its SQLSTATE, never a message that could hold a
	// personal value.
	EraseDemoSchoolDatabase(ctx context.Context, schoolID int64) (status, errorCode string, err error)
}
