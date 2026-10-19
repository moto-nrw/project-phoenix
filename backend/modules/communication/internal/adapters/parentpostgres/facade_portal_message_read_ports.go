package parentpostgres

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/communication"
)

func (r parentMessageStaffRepository) PortalListThreadsForGuardianStudent(ctx context.Context, accountID int64, studentID int64) ([]*communication.PortalInboxThread, error) {
	return r.ListThreadsForGuardianStudent(ctx, accountID, studentID)
}
func (r parentMessageStaffRepository) PortalListThreadsForGuardianTenants(ctx context.Context, accountID int64, tenantIDs []int64) ([]*communication.PortalInboxThread, error) {
	return r.ListThreadsForGuardianTenants(ctx, accountID, tenantIDs)
}
func (r schoolParentMessageReadRepository) PortalListThreadsForGuardianStudent(ctx context.Context, accountID int64, studentID int64) ([]*communication.PortalInboxThread, error) {
	return r.ListThreadsForGuardianStudent(ctx, accountID, studentID)
}
func (r schoolParentMessageReadRepository) PortalListThreadsForGuardianTenants(ctx context.Context, accountID int64, tenantIDs []int64) ([]*communication.PortalInboxThread, error) {
	return r.ListThreadsForGuardianTenants(ctx, accountID, tenantIDs)
}
func (r groupParentMessageReadRepository) PortalListThreadsForGuardianStudent(ctx context.Context, accountID int64, studentID int64) ([]*communication.PortalInboxThread, error) {
	return r.ListThreadsForGuardianStudent(ctx, accountID, studentID)
}
func (r groupParentMessageReadRepository) PortalListThreadsForGuardianTenants(ctx context.Context, accountID int64, tenantIDs []int64) ([]*communication.PortalInboxThread, error) {
	return r.ListThreadsForGuardianTenants(ctx, accountID, tenantIDs)
}

func (r *messageRepository) PortalListByThread(ctx context.Context, threadID int64, limit int) ([]*communication.PortalParentMessage, error) {
	return r.ListByThread(ctx, threadID, limit)
}
