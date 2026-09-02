package repositories

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
)

// CareStudentLock exposes the People Directory's student row lock to the
// care-day writers (#2662) together with the sentinel it reports for a
// missing child. The writers keep treating that child as sql.ErrNoRows;
// careplanning performs that mapping because this package may not name
// the SQL error class.
func CareStudentLock(students peopledirectory.StudentCommand) (lock func(context.Context, int64) error, notFound error) {
	return students.LockStudent, peopledirectory.ErrStudentNotFound
}
