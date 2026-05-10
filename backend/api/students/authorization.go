package students

import (
	"context"
	"net/http"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/models/users"
	userContextService "github.com/moto-nrw/project-phoenix/services/usercontext"
)

// HTTP-side wrappers around auth/authorize/student_access.go.

func hasAdminPermissions(permissions []string) bool {
	for _, perm := range permissions {
		if perm == "admin:*" || perm == "*:*" {
			return true
		}
	}
	return false
}

// getPermissionsFromRequest extracts permissions from request context.
func getPermissionsFromRequest(r *http.Request) []string {
	return jwt.PermissionsFromCtx(r.Context())
}

func canUpdateStudent(ctx context.Context, userPermissions []string, student *users.Student, ucs userContextService.UserContextService) (bool, error) {
	return authorize.CanUpdateStudent(ctx, userPermissions, student, ucs)
}

func canDeleteStudent(ctx context.Context, userPermissions []string, student *users.Student, ucs userContextService.UserContextService) (bool, error) {
	return authorize.CanDeleteStudent(ctx, userPermissions, student, ucs)
}

func isGroupSupervisor(ctx context.Context, groupID int64, ucs userContextService.UserContextService) bool {
	return authorize.IsGroupSupervisor(ctx, groupID, ucs)
}
