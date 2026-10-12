package compose

import (
	"context"

	"github.com/moto-nrw/project-phoenix/models/users"
)

// ListGuardiansPage lists guardian profiles with phone numbers for one page
// of the staff directory; page and pageSize follow the request pagination.
func (s *GuardianService) ListGuardiansPage(ctx context.Context, page, pageSize int) ([]*users.GuardianProfile, error) {
	options := users.NewQueryOptions()
	options.WithPagination(page, pageSize)
	return s.ListGuardians(ctx, options)
}

// IsGuardianLinkConstraintViolation reports whether a guardian delete was
// refused by the database because a child link still references the
// profile (or a required column was left empty).
func IsGuardianLinkConstraintViolation(err error) bool {
	return users.IsConstraintViolation(err)
}
