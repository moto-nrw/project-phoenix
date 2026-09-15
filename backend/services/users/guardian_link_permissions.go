package users

import (
	"context"
	"sort"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/users"
)

// ListGuardiansPage lists guardian profiles with phone numbers for one page
// of the staff directory; page and pageSize follow the request pagination.
func (s *GuardianService) ListGuardiansPage(ctx context.Context, page, pageSize int) ([]*users.GuardianProfile, error) {
	options := base.NewQueryOptions()
	options.WithPagination(page, pageSize)
	return s.ListGuardians(ctx, options)
}

// IsGuardianLinkConstraintViolation reports whether a guardian delete was
// refused by the database because a child link still references the
// profile (or a required column was left empty).
func IsGuardianLinkConstraintViolation(err error) bool {
	return base.IsConstraintViolation(err)
}

// GrantedGuardianPermissions lists the parents-portal permission names a
// student-guardian link grants, in stable order. It applies the same
// per-name rule as authorize.StudentGuardianHasPermission, so a projection
// built from it answers exactly what the security runtime would answer.
func GrantedGuardianPermissions(link *users.StudentGuardian) []string {
	if link == nil {
		return nil
	}
	_, _, _, _, _, permissions := link.GuardianAuthorizationData()
	names := make([]string, 0, len(permissions))
	for name := range permissions {
		if authorize.StudentGuardianHasPermission(link, name) {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}
