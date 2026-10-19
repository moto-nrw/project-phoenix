package parentpostgres

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/communication"
)

func (r *announcementRepository) PortalDeclarationChildrenForAccount(ctx context.Context, accountID int64, announcementIDs []int64) ([]*communication.PortalDeclarationChild, error) {
	return r.DeclarationChildrenForAccount(ctx, accountID, announcementIDs)
}
func (r *announcementRepository) PortalDeclarationSettings(ctx context.Context, announcementIDs []int64) (map[int64]communication.PortalAnnouncementDeclarationSettings, error) {
	return r.DeclarationSettings(ctx, announcementIDs)
}
func (r *announcementRepository) PortalDeclarationSignersForStudents(ctx context.Context, studentIDs []int64) ([]*communication.PortalDeclarationSigner, error) {
	return r.DeclarationSignersForStudents(ctx, studentIDs)
}
func (r *announcementRepository) PortalLatestDeclarationVersion(ctx context.Context, tenantID int64, announcementID int64) (*communication.PortalDeclarationVersion, error) {
	return r.LatestDeclarationVersion(ctx, tenantID, announcementID)
}
func (r *announcementRepository) PortalLatestDeclarationVersions(ctx context.Context, announcementIDs []int64) (map[int64]*communication.PortalDeclarationVersion, error) {
	return r.LatestDeclarationVersions(ctx, announcementIDs)
}
func (r *announcementRepository) PortalListDeclarationSubmissionsForStudents(ctx context.Context, announcementIDs []int64, studentIDs []int64) ([]*communication.PortalDeclarationSubmission, error) {
	return r.ListDeclarationSubmissionsForStudents(ctx, announcementIDs, studentIDs)
}
func (r *announcementRepository) PortalListDeclarationVersions(ctx context.Context, tenantID int64, announcementID int64) ([]*communication.PortalDeclarationVersion, error) {
	return r.ListDeclarationVersions(ctx, tenantID, announcementID)
}
func (r *announcementRepository) PortalHoldDeclarationSigner(ctx context.Context, tenantID int64, announcementID int64, accountID int64, studentID int64) (*communication.PortalDeclarationSignerContext, error) {
	return r.HoldDeclarationSigner(ctx, tenantID, announcementID, accountID, studentID)
}
func (r *announcementRepository) PortalHoldHistoricalDeclarationSigner(ctx context.Context, tenantID int64, announcementID int64, accountID int64, studentID int64) (*communication.PortalDeclarationSignerContext, error) {
	return r.HoldHistoricalDeclarationSigner(ctx, tenantID, announcementID, accountID, studentID)
}
func (r *announcementRepository) PortalInsertDeclarationSubmission(ctx context.Context, submission *communication.PortalDeclarationSubmission) error {
	return r.InsertDeclarationSubmission(ctx, submission)
}
