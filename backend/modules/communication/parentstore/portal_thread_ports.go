package parentstore

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/communication"
)

func (r *threadRepository) PortalFindByStudentGuardian(ctx context.Context, studentID int64, guardianAccountID int64) (*communication.PortalParentMessageThread, error) {
	result0, result1 := r.FindByStudentGuardian(ctx, studentID, guardianAccountID)
	return portalRecordPointer(result0, portalFromRetainedParentMessageThread), result1
}
func (r *threadRepository) PortalGetOrCreate(ctx context.Context, tenantID int64, studentID int64, guardianAccountID int64) (*communication.PortalParentMessageThread, error) {
	result0, result1 := r.GetOrCreate(ctx, tenantID, studentID, guardianAccountID)
	return portalRecordPointer(result0, portalFromRetainedParentMessageThread), result1
}
