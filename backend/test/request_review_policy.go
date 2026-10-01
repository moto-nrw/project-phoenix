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

func (p RequestReviewPolicy) Allows(ctx context.Context, permissions []string, student *userModels.Student) (bool, error) {
	allowed, _ := authorize.CanUpdateStudent(ctx, permissions, student, p.UserContext)
	return allowed, nil
}
