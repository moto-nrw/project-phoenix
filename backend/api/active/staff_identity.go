package active

import (
	"context"
)

// CurrentStaff supplies the authenticated staff identity used by active routes.
type CurrentStaff interface {
	GetCurrentStaff(context.Context) (*StaffIdentity, error)
}

type StaffAccess interface {
	CurrentStaff
	HasCurrentStaff(context.Context) (bool, error)
}
