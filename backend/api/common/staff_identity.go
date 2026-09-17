package common

import (
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/legacy/usercontext"
)

// IsUnlinkedStaffIdentity distinguishes missing staff linkage from lookup failures.
// HTTP adapters use this distinction to deny access without masking server errors.
func IsUnlinkedStaffIdentity(err error) bool {
	return errors.Is(err, usercontext.ErrUserNotLinkedToPerson) ||
		errors.Is(err, usercontext.ErrUserNotLinkedToStaff)
}
