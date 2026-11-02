package parentpostgres

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/communication"
)

func (r *threadRepository) PortalFindByStudentGuardian(ctx context.Context, studentID int64, guardianAccountID int64, audience string) (*communication.PortalParentMessageThread, error) {
	return r.FindByStudentGuardian(ctx, studentID, guardianAccountID, audience)
}
func (r *threadRepository) PortalGetOrCreate(ctx context.Context, tenantID int64, studentID int64, guardianAccountID int64, audience string) (*communication.PortalParentMessageThread, error) {
	return r.GetOrCreate(ctx, tenantID, studentID, guardianAccountID, audience)
}
