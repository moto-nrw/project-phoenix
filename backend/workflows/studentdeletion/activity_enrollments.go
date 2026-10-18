package studentdeletion

import "context"

// ActivityEnrollments counts the child's enrollments in the ambient transaction.
type ActivityEnrollments interface {
	CountStudentEnrollments(context.Context, int64) (int, error)
}
