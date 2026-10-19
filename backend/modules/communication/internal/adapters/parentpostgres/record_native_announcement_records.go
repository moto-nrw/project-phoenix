package parentpostgres

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/communication"
	"github.com/moto-nrw/project-phoenix/modules/communication/internal/domain"
)

func (r *announcementRepository) AnnouncementClearEngagement(ctx context.Context, announcementID int64) error {
	return r.ClearEngagement(ctx, announcementID)
}
func (r *announcementRepository) AnnouncementSetReminder(ctx context.Context, id int64, reminderAt *time.Time, reminderText *string) (bool, error) {
	return r.SetReminder(ctx, id, reminderAt, reminderText)
}
func (r *announcementRepository) AnnouncementListDeclarationSubmissions(ctx context.Context, tenantID, announcementID int64) ([]*communication.PortalDeclarationSubmission, error) {
	return r.ListDeclarationSubmissions(ctx, tenantID, announcementID)
}
func (r *announcementRepository) AnnouncementListDueReminders(ctx context.Context, notBefore, dueBefore time.Time) ([]*communication.PortalParentAnnouncement, error) {
	return r.ListDueReminders(ctx, notBefore, dueBefore)
}
func (r *announcementRepository) AnnouncementClaimReminder(ctx context.Context, id int64, sentAt time.Time) (bool, error) {
	return r.ClaimReminder(ctx, id, sentAt)
}
func (r *announcementRepository) AnnouncementCountDeclarationSubmissions(ctx context.Context, tenantID, announcementID int64) (int, error) {
	return r.CountDeclarationSubmissions(ctx, tenantID, announcementID)
}
func (r *announcementRepository) AnnouncementAudienceRecipients(ctx context.Context, tenantID, announcementID int64) ([]*communication.PortalAnnouncementRecipientStatus, error) {
	return r.AudienceRecipients(ctx, tenantID, announcementID)
}
func (r *announcementRepository) AnnouncementReplaceOptions(ctx context.Context, tenantID, announcementID int64, options []*communication.PortalParentAnnouncementOption) error {
	return r.ReplaceOptions(ctx, tenantID, announcementID, options)
}
func (r *announcementRepository) AnnouncementLatestDeclarationVersion(ctx context.Context, tenantID, announcementID int64) (*communication.PortalDeclarationVersion, error) {
	return r.LatestDeclarationVersion(ctx, tenantID, announcementID)
}
func (r *announcementRepository) AnnouncementReplaceTargets(ctx context.Context, tenantID, announcementID int64, targets []*communication.PortalParentAnnouncementTarget) error {
	return r.ReplaceTargets(ctx, tenantID, announcementID, targets)
}
func (r *announcementRepository) AnnouncementSetPublished(ctx context.Context, id int64, publishedAt *time.Time) error {
	return r.SetPublished(ctx, id, publishedAt)
}
func (r *announcementRepository) AnnouncementInsertDeclarationVersion(ctx context.Context, version *communication.PortalDeclarationVersion) error {
	return r.InsertDeclarationVersion(ctx, version)
}
func (r *announcementRepository) AnnouncementFindByIDForUpdate(ctx context.Context, id int64) (*communication.PortalParentAnnouncement, error) {
	return r.FindByIDForUpdate(ctx, id)
}
func (r *announcementRepository) AnnouncementDeclarationSigners(ctx context.Context, tenantID int64, studentIDs []int64) ([]*communication.PortalDeclarationSigner, error) {
	return r.DeclarationSigners(ctx, tenantID, studentIDs)
}
func (r *announcementRepository) AnnouncementPollResults(ctx context.Context, tenantID, announcementID int64) (*communication.PortalAnnouncementPollResults, error) {
	return r.PollResults(ctx, tenantID, announcementID)
}
func (r *announcementRepository) AnnouncementLetterChildStatuses(ctx context.Context, tenantID, announcementID int64) ([]*communication.PortalAnnouncementLetterChildStatus, error) {
	return r.LetterChildStatuses(ctx, tenantID, announcementID)
}
func (r *announcementRepository) AnnouncementDeclarationChildren(ctx context.Context, tenantID, announcementID int64) ([]*communication.PortalDeclarationChild, error) {
	return r.DeclarationChildren(ctx, tenantID, announcementID)
}
func (r *announcementRepository) AnnouncementReleaseReminderClaim(ctx context.Context, id int64, sentAt time.Time) (bool, error) {
	return r.ReleaseReminderClaim(ctx, id, sentAt)
}
func (r *announcementRepository) AnnouncementDelete(ctx context.Context, id int64) error {
	return r.Delete(ctx, id)
}
func (r *announcementRepository) AnnouncementPublishIfDraft(ctx context.Context, id int64, publishedAt time.Time) (bool, error) {
	return r.PublishIfDraft(ctx, id, publishedAt)
}
func (r *announcementRepository) AnnouncementListDeclarationVersions(ctx context.Context, tenantID, announcementID int64) ([]*communication.PortalDeclarationVersion, error) {
	return r.ListDeclarationVersions(ctx, tenantID, announcementID)
}
func (r *announcementRepository) AnnouncementUnansweredReminderRecipients(ctx context.Context, tenantID, announcementID int64) ([]*communication.PortalAnnouncementPollReminderRecipient, error) {
	return r.UnansweredReminderRecipients(ctx, tenantID, announcementID)
}
func (r *announcementRepository) AnnouncementSchoolName(ctx context.Context, tenantID int64) (string, error) {
	return r.SchoolName(ctx, tenantID)
}
func (r *announcementRepository) AnnouncementResolveAudienceEmails(ctx context.Context, tenantID, announcementID int64) ([]*communication.PortalAnnouncementRecipient, error) {
	return r.ResolveAudienceEmails(ctx, tenantID, announcementID)
}
func (r *announcementRepository) AnnouncementStats(ctx context.Context, tenantID, announcementID int64) (*communication.PortalAnnouncementStats, error) {
	return r.Stats(ctx, tenantID, announcementID)
}
func (r *announcementRepository) AnnouncementListTargets(ctx context.Context, announcementID int64) ([]*communication.PortalParentAnnouncementTarget, error) {
	return r.ListTargets(ctx, announcementID)
}
func (r *announcementRepository) AnnouncementPollChildren(ctx context.Context, tenantID, announcementID int64) ([]*communication.PortalAnnouncementPollChildStatus, error) {
	return r.PollChildren(ctx, tenantID, announcementID)
}
func (r *announcementRepository) AnnouncementUpdate(ctx context.Context, a *communication.PortalParentAnnouncement) error {
	return r.Update(ctx, a)
}
func (r *announcementRepository) AnnouncementResolveDeliveryRecipients(ctx context.Context, tenantID, announcementID int64) ([]*communication.PortalAnnouncementDeliveryRecipient, error) {
	return r.ResolveDeliveryRecipients(ctx, tenantID, announcementID)
}
func (r *announcementRepository) AnnouncementCountReachableGuardiansForStudents(ctx context.Context, tenantID int64, studentIDs []int64) (int, error) {
	return r.CountReachableGuardiansForStudents(ctx, tenantID, studentIDs)
}
func (r *announcementRepository) AnnouncementUnacknowledgedReminderRecipients(ctx context.Context, tenantID, announcementID int64) ([]*communication.PortalAnnouncementPollReminderRecipient, error) {
	return r.UnacknowledgedReminderRecipients(ctx, tenantID, announcementID)
}
func (r *announcementRepository) AnnouncementListForTenant(ctx context.Context, includeInactive bool) ([]*communication.PortalParentAnnouncement, error) {
	return r.ListForTenant(ctx, includeInactive)
}
func (r *announcementRepository) AnnouncementCreate(ctx context.Context, a *communication.PortalParentAnnouncement) error {
	return r.Create(ctx, a)
}
func (r *announcementRepository) IsParentAnnouncementPublishedConflict(err error) bool {
	return errors.Is(err, domain.ErrParentAnnouncementPublished)
}
