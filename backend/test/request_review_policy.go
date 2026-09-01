package test

import (
	"context"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
)

// RequestReviewPolicy preserves the original users:update authorization in
// service tests that are not about the group-leader policy itself.
type RequestReviewPolicy struct {
	UserContext authorize.StudentAccessUserContext
}

func (p RequestReviewPolicy) StudentFilter(ctx context.Context, permissions []string) (func(*userModels.Student) bool, error) {
	writable := authorize.WritableStudentFilter(ctx, permissions, p.UserContext)
	return func(student *userModels.Student) bool { return writable(student) }, nil
}

func (p RequestReviewPolicy) Allows(ctx context.Context, permissions []string, student *userModels.Student) (bool, error) {
	allowed, _ := authorize.CanUpdateStudent(ctx, permissions, student, p.UserContext)
	return allowed, nil
}

// AbsenceRequestReviewPolicy is the absence-specific counterpart: users with
// only users:absence keep the same access they had before policy injection.
type AbsenceRequestReviewPolicy struct {
	UserContext authorize.StudentAccessUserContext
}

func (p AbsenceRequestReviewPolicy) StudentFilter(ctx context.Context, permissions []string) (func(*userModels.Student) bool, error) {
	writable := authorize.AbsenceWritableStudentFilter(ctx, permissions, p.UserContext)
	return func(student *userModels.Student) bool { return writable(student) }, nil
}

func (p AbsenceRequestReviewPolicy) Allows(ctx context.Context, permissions []string, student *userModels.Student) (bool, error) {
	allowed, _ := authorize.CanManageStudentAbsence(ctx, permissions, student, p.UserContext)
	return allowed, nil
}
