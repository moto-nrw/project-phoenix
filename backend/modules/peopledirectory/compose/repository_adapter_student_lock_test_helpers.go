//go:build testhooks

package compose

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
)

// CareStudentLock supplies care-day test graphs (#2662) with the student row
// lock and missing-child sentinel. Care Planning maps that sentinel to
// sql.ErrNoRows; this package does not name SQL errors.
func CareStudentLock(students peopledirectory.StudentCommand) (lock func(context.Context, int64) error, notFound error) {
	return students.LockStudent, peopledirectory.ErrStudentNotFound
}
