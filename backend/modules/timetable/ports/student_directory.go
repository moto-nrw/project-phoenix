package ports

import "context"

type TargetStudent struct {
	ID               int64
	SchoolClass      string
	EducationGroupID *int64
	EnrolledUntil    string
}

type StudentDirectory interface {
	ListEnrolledStudents(context.Context) ([]TargetStudent, error)
}

type StudentDirectoryFunc func(context.Context) ([]TargetStudent, error)

func (f StudentDirectoryFunc) ListEnrolledStudents(ctx context.Context) ([]TargetStudent, error) {
	return f(ctx)
}
