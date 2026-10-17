package parentstore

import (
	"context"

	models "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/communication"
)

func (r *announcementRepository) PortalDeclarationChildrenForAccount(ctx context.Context, accountID int64, announcementIDs []int64) ([]*communication.PortalDeclarationChild, error) {
	result0, result1 := r.DeclarationChildrenForAccount(ctx, accountID, announcementIDs)
	return portalRecordSlice(result0, func(row *models.DeclarationChild) *communication.PortalDeclarationChild {
		return portalRecordPointer(row, portalFromRetainedDeclarationChild)
	}), result1
}
func (r *announcementRepository) PortalDeclarationSettings(ctx context.Context, announcementIDs []int64) (map[int64]communication.PortalAnnouncementDeclarationSettings, error) {
	result0, result1 := r.DeclarationSettings(ctx, announcementIDs)
	return portalRecordMap(result0, func(row models.AnnouncementDeclarationSettings) communication.PortalAnnouncementDeclarationSettings {
		return portalFromRetainedAnnouncementDeclarationSettings(row)
	}), result1
}
func (r *announcementRepository) PortalDeclarationSignersForStudents(ctx context.Context, studentIDs []int64) ([]*communication.PortalDeclarationSigner, error) {
	result0, result1 := r.DeclarationSignersForStudents(ctx, studentIDs)
	return portalRecordSlice(result0, func(row *models.DeclarationSigner) *communication.PortalDeclarationSigner {
		return portalRecordPointer(row, portalFromRetainedDeclarationSigner)
	}), result1
}
func (r *announcementRepository) PortalLatestDeclarationVersion(ctx context.Context, tenantID int64, announcementID int64) (*communication.PortalDeclarationVersion, error) {
	result0, result1 := r.LatestDeclarationVersion(ctx, tenantID, announcementID)
	return portalRecordPointer(result0, portalFromRetainedDeclarationVersion), result1
}
func (r *announcementRepository) PortalLatestDeclarationVersions(ctx context.Context, announcementIDs []int64) (map[int64]*communication.PortalDeclarationVersion, error) {
	result0, result1 := r.LatestDeclarationVersions(ctx, announcementIDs)
	return portalRecordMap(result0, func(row *models.DeclarationVersion) *communication.PortalDeclarationVersion {
		return portalRecordPointer(row, portalFromRetainedDeclarationVersion)
	}), result1
}
func (r *announcementRepository) PortalListDeclarationSubmissionsForStudents(ctx context.Context, announcementIDs []int64, studentIDs []int64) ([]*communication.PortalDeclarationSubmission, error) {
	result0, result1 := r.ListDeclarationSubmissionsForStudents(ctx, announcementIDs, studentIDs)
	return portalRecordSlice(result0, func(row *models.DeclarationSubmission) *communication.PortalDeclarationSubmission {
		return portalRecordPointer(row, portalFromRetainedDeclarationSubmission)
	}), result1
}
func (r *announcementRepository) PortalListDeclarationVersions(ctx context.Context, tenantID int64, announcementID int64) ([]*communication.PortalDeclarationVersion, error) {
	result0, result1 := r.ListDeclarationVersions(ctx, tenantID, announcementID)
	return portalRecordSlice(result0, func(row *models.DeclarationVersion) *communication.PortalDeclarationVersion {
		return portalRecordPointer(row, portalFromRetainedDeclarationVersion)
	}), result1
}
func (r *announcementRepository) PortalHoldDeclarationSigner(ctx context.Context, tenantID int64, announcementID int64, accountID int64, studentID int64) (*communication.PortalDeclarationSignerContext, error) {
	result0, result1 := r.HoldDeclarationSigner(ctx, tenantID, announcementID, accountID, studentID)
	return portalRecordPointer(result0, portalFromRetainedDeclarationSignerContext), result1
}
func (r *announcementRepository) PortalHoldHistoricalDeclarationSigner(ctx context.Context, tenantID int64, announcementID int64, accountID int64, studentID int64) (*communication.PortalDeclarationSignerContext, error) {
	result0, result1 := r.HoldHistoricalDeclarationSigner(ctx, tenantID, announcementID, accountID, studentID)
	return portalRecordPointer(result0, portalFromRetainedDeclarationSignerContext), result1
}
func (r *announcementRepository) PortalInsertDeclarationSubmission(ctx context.Context, submission *communication.PortalDeclarationSubmission) error {
	retainedSubmission := portalRecordPointer(submission, portalToRetainedDeclarationSubmission)
	result0 := r.InsertDeclarationSubmission(ctx, retainedSubmission)
	if submission != nil && retainedSubmission != nil {
		*submission = portalFromRetainedDeclarationSubmission(*retainedSubmission)
	}
	return result0
}
