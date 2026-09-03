package absence

import (
	"context"

	"github.com/moto-nrw/project-phoenix/internal/careplanning"
	"github.com/uptrace/bun"
)

// BindCareStudentLockForDB installs the People Directory's student row lock
// behind the excused-request writers (#2662) for test graphs. Production binds it
// through the timetable services; hand-wired graphs bind their own database.
func BindCareStudentLockForDB(db *bun.DB, lock func(ctx context.Context, studentID int64) error, notFound error) {
	careplanning.BindStudentLockForDB(db, lock, notFound)
}
