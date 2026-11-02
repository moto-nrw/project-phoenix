package communication

import (
	"context"
)

type PortalMessageThreadAccess interface {
	PortalFindByStudentGuardian(ctx context.Context, studentID int64, guardianAccountID int64, audience string) (*PortalParentMessageThread, error)
	PortalGetOrCreate(ctx context.Context, tenantID int64, studentID int64, guardianAccountID int64, audience string) (*PortalParentMessageThread, error)
}
