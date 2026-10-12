package test

import (
	"context"

	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
)

// RequestReviewPolicy preserves the original users:update authorization in
// service tests that are not about the group-leader policy itself.
type RequestReviewPolicy struct {
	UserContext securityruntime.StudentAccessUserContext
}

func (p RequestReviewPolicy) Allows(ctx context.Context, permissions []string, student *userModels.Student) (bool, error) {
	allowed, _ := securityruntime.CanUpdateStudent(ctx, permissions, student, p.UserContext)
	return allowed, nil
}
