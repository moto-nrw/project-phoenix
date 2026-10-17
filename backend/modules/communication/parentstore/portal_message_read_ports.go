package parentstore

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/communication"
)

func (r *messageRepository) PortalListByThread(ctx context.Context, threadID int64, limit int) ([]*communication.PortalParentMessage, error) {
	rows, err := r.ListByThread(ctx, threadID, limit)
	out, mapErr := portalFromRetainedParentMessageSlice(rows)
	if err != nil {
		return out, err
	}
	return out, mapErr
}
func (r parentMessageStaffRepository) PortalListThreadsForGuardianStudent(ctx context.Context, accountID int64, studentID int64) ([]*communication.PortalInboxThread, error) {
	rows, err := r.ListThreadsForGuardianStudent(ctx, accountID, studentID)
	out, mapErr := portalFromRetainedInboxThreadSlice(rows)
	if err != nil {
		return out, err
	}
	return out, mapErr
}
func (r parentMessageStaffRepository) PortalListThreadsForGuardianTenants(ctx context.Context, accountID int64, tenantIDs []int64) ([]*communication.PortalInboxThread, error) {
	rows, err := r.ListThreadsForGuardianTenants(ctx, accountID, tenantIDs)
	out, mapErr := portalFromRetainedInboxThreadSlice(rows)
	if err != nil {
		return out, err
	}
	return out, mapErr
}
func (r schoolParentMessageReadRepository) PortalListThreadsForGuardianStudent(ctx context.Context, accountID int64, studentID int64) ([]*communication.PortalInboxThread, error) {
	rows, err := r.ListThreadsForGuardianStudent(ctx, accountID, studentID)
	out, mapErr := portalFromRetainedInboxThreadSlice(rows)
	if err != nil {
		return out, err
	}
	return out, mapErr
}
func (r schoolParentMessageReadRepository) PortalListThreadsForGuardianTenants(ctx context.Context, accountID int64, tenantIDs []int64) ([]*communication.PortalInboxThread, error) {
	rows, err := r.ListThreadsForGuardianTenants(ctx, accountID, tenantIDs)
	out, mapErr := portalFromRetainedInboxThreadSlice(rows)
	if err != nil {
		return out, err
	}
	return out, mapErr
}
func (r groupParentMessageReadRepository) PortalListThreadsForGuardianStudent(ctx context.Context, accountID int64, studentID int64) ([]*communication.PortalInboxThread, error) {
	rows, err := r.ListThreadsForGuardianStudent(ctx, accountID, studentID)
	out, mapErr := portalFromRetainedInboxThreadSlice(rows)
	if err != nil {
		return out, err
	}
	return out, mapErr
}
func (r groupParentMessageReadRepository) PortalListThreadsForGuardianTenants(ctx context.Context, accountID int64, tenantIDs []int64) ([]*communication.PortalInboxThread, error) {
	rows, err := r.ListThreadsForGuardianTenants(ctx, accountID, tenantIDs)
	out, mapErr := portalFromRetainedInboxThreadSlice(rows)
	if err != nil {
		return out, err
	}
	return out, mapErr
}
