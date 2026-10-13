package test

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
)

// RequestReviewPolicy preserves the original users:update authorization in
// service tests that are not about the group-leader policy itself. S is the
// child row the service under test decides over.
type RequestReviewPolicy[S interface{ IsAuthorizationStudent() bool }] struct {
	UserContext securityruntime.StudentAccessUserContext
}

func (p RequestReviewPolicy[S]) Allows(ctx context.Context, permissions []string, student S) (bool, error) {
	allowed, _ := securityruntime.CanUpdateStudent(ctx, permissions, student, p.UserContext)
	return allowed, nil
}
