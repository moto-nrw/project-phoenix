package announcement

import (
	"context"
	"time"

	values "github.com/moto-nrw/project-phoenix/modules/communication/internal/application"
)

func NewRepository(source values.StaffAnnouncementSource) values.ParentAnnouncementRepository {
	if source == nil {
		return nil
	}
	return &nativeAnnouncementRepository{source: source}
}

type nativeAnnouncementRepository struct {
	source values.StaffAnnouncementSource
}

func (r *nativeAnnouncementRepository) ClearEngagement(ctx context.Context, announcementID int64) error {
	err := r.source.AnnouncementClearEngagement(ctx, announcementID)
	return err
}
func (r *nativeAnnouncementRepository) SetReminder(ctx context.Context, id int64, reminderAt *time.Time, reminderText *string) (bool, error) {
	rows, err := r.source.AnnouncementSetReminder(ctx, id, reminderAt, reminderText)
	return rows, err
}
func (r *nativeAnnouncementRepository) ListDeclarationSubmissions(ctx context.Context, tenantID, announcementID int64) ([]*values.StaffDeclarationSubmission, error) {
	rows, err := r.source.AnnouncementListDeclarationSubmissions(ctx, tenantID, announcementID)
	return values.MapAnnouncementRows(values.DeclarationSubmissionSliceFromAnnouncementRecords(rows), func(v *values.DeclarationSubmission) *values.StaffDeclarationSubmission {
		return (*values.StaffDeclarationSubmission)(v)
	}), err
}
func (r *nativeAnnouncementRepository) ListDueReminders(ctx context.Context, notBefore, dueBefore time.Time) ([]*values.StaffParentAnnouncement, error) {
	rows, err := r.source.AnnouncementListDueReminders(ctx, notBefore, dueBefore)
	return values.MapAnnouncementRows(values.ParentAnnouncementSliceFromAnnouncementRecords(rows), func(v *values.ParentAnnouncement) *values.StaffParentAnnouncement {
		return values.StaffAnnouncementParentAnnouncementFromValue(v)
	}), err
}
func (r *nativeAnnouncementRepository) ClaimReminder(ctx context.Context, id int64, sentAt time.Time) (bool, error) {
	rows, err := r.source.AnnouncementClaimReminder(ctx, id, sentAt)
	return rows, err
}
func (r *nativeAnnouncementRepository) CountDeclarationSubmissions(ctx context.Context, tenantID, announcementID int64) (int, error) {
	rows, err := r.source.AnnouncementCountDeclarationSubmissions(ctx, tenantID, announcementID)
	return rows, err
}
func (r *nativeAnnouncementRepository) AudienceRecipients(ctx context.Context, tenantID, announcementID int64) ([]*values.StaffAnnouncementRecipientStatus, error) {
	rows, err := r.source.AnnouncementAudienceRecipients(ctx, tenantID, announcementID)
	return values.MapAnnouncementRows(values.AnnouncementRecipientStatusSliceFromAnnouncementRecords(rows), func(v *values.AnnouncementRecipientStatus) *values.StaffAnnouncementRecipientStatus {
		return (*values.StaffAnnouncementRecipientStatus)(v)
	}), err
}
func (r *nativeAnnouncementRepository) ReplaceOptions(ctx context.Context, tenantID, announcementID int64, options []*values.StaffParentAnnouncementOption) error {
	err := r.source.AnnouncementReplaceOptions(ctx, tenantID, announcementID, values.ParentAnnouncementOptionSliceToAnnouncementRecords(values.MapAnnouncementRows(options, func(v *values.StaffParentAnnouncementOption) *values.ParentAnnouncementOption {
		return (*values.ParentAnnouncementOption)(v)
	})))
	return err
}
func (r *nativeAnnouncementRepository) LatestDeclarationVersion(ctx context.Context, tenantID, announcementID int64) (*values.StaffDeclarationVersion, error) {
	rows, err := r.source.AnnouncementLatestDeclarationVersion(ctx, tenantID, announcementID)
	return values.StaffAnnouncementDeclarationVersionFromValue(values.DeclarationVersionFromAnnouncementRecord(rows)), err
}
func (r *nativeAnnouncementRepository) ReplaceTargets(ctx context.Context, tenantID, announcementID int64, targets []*values.StaffParentAnnouncementTarget) error {
	err := r.source.AnnouncementReplaceTargets(ctx, tenantID, announcementID, values.ParentAnnouncementTargetSliceToAnnouncementRecords(values.MapAnnouncementRows(targets, func(v *values.StaffParentAnnouncementTarget) *values.ParentAnnouncementTarget {
		return (*values.ParentAnnouncementTarget)(v)
	})))
	return err
}
func (r *nativeAnnouncementRepository) SetPublished(ctx context.Context, id int64, publishedAt *time.Time) error {
	err := r.source.AnnouncementSetPublished(ctx, id, publishedAt)
	return err
}
func (r *nativeAnnouncementRepository) InsertDeclarationVersion(ctx context.Context, version *values.StaffDeclarationVersion) error {
	recordversion := values.DeclarationVersionToAnnouncementRecord(values.StaffAnnouncementDeclarationVersionToValue(version))
	err := r.source.AnnouncementInsertDeclarationVersion(ctx, recordversion)
	if version != nil && recordversion != nil {
		*version = *values.StaffAnnouncementDeclarationVersionFromValue(values.DeclarationVersionFromAnnouncementRecord(recordversion))
	}
	return err
}
func (r *nativeAnnouncementRepository) FindByID(ctx context.Context, id int64) (*values.StaffParentAnnouncement, error) {
	rows, err := r.source.PortalFindByID(ctx, id)
	return values.StaffAnnouncementParentAnnouncementFromValue(values.ParentAnnouncementFromAnnouncementRecord(rows)), err
}
func (r *nativeAnnouncementRepository) FindByIDForUpdate(ctx context.Context, id int64) (*values.StaffParentAnnouncement, error) {
	rows, err := r.source.AnnouncementFindByIDForUpdate(ctx, id)
	return values.StaffAnnouncementParentAnnouncementFromValue(values.ParentAnnouncementFromAnnouncementRecord(rows)), err
}
func (r *nativeAnnouncementRepository) DeclarationSigners(ctx context.Context, tenantID int64, studentIDs []int64) ([]*values.StaffDeclarationSigner, error) {
	rows, err := r.source.AnnouncementDeclarationSigners(ctx, tenantID, studentIDs)
	return values.MapAnnouncementRows(values.DeclarationSignerSliceFromAnnouncementRecords(rows), func(v *values.DeclarationSigner) *values.StaffDeclarationSigner {
		return (*values.StaffDeclarationSigner)(v)
	}), err
}
func (r *nativeAnnouncementRepository) PollResults(ctx context.Context, tenantID, announcementID int64) (*values.StaffAnnouncementPollResults, error) {
	rows, err := r.source.AnnouncementPollResults(ctx, tenantID, announcementID)
	return values.StaffAnnouncementAnnouncementPollResultsFromValue(values.AnnouncementPollResultsFromAnnouncementRecord(rows)), err
}
func (r *nativeAnnouncementRepository) LetterChildStatuses(ctx context.Context, tenantID, announcementID int64) ([]*values.StaffAnnouncementLetterChildStatus, error) {
	rows, err := r.source.AnnouncementLetterChildStatuses(ctx, tenantID, announcementID)
	return values.MapAnnouncementRows(values.AnnouncementLetterChildStatusSliceFromAnnouncementRecords(rows), func(v *values.AnnouncementLetterChildStatus) *values.StaffAnnouncementLetterChildStatus {
		return (*values.StaffAnnouncementLetterChildStatus)(v)
	}), err
}
func (r *nativeAnnouncementRepository) DeclarationChildren(ctx context.Context, tenantID, announcementID int64) ([]*values.StaffDeclarationChild, error) {
	rows, err := r.source.AnnouncementDeclarationChildren(ctx, tenantID, announcementID)
	return values.MapAnnouncementRows(values.DeclarationChildSliceFromAnnouncementRecords(rows), func(v *values.DeclarationChild) *values.StaffDeclarationChild {
		return (*values.StaffDeclarationChild)(v)
	}), err
}
func (r *nativeAnnouncementRepository) ReleaseReminderClaim(ctx context.Context, id int64, sentAt time.Time) (bool, error) {
	rows, err := r.source.AnnouncementReleaseReminderClaim(ctx, id, sentAt)
	return rows, err
}
func (r *nativeAnnouncementRepository) Delete(ctx context.Context, id int64) error {
	err := r.source.AnnouncementDelete(ctx, id)
	return err
}
func (r *nativeAnnouncementRepository) PublishIfDraft(ctx context.Context, id int64, publishedAt time.Time) (bool, error) {
	rows, err := r.source.AnnouncementPublishIfDraft(ctx, id, publishedAt)
	return rows, err
}
func (r *nativeAnnouncementRepository) ListOptionsForAnnouncements(ctx context.Context, announcementIDs []int64) ([]*values.StaffParentAnnouncementOption, error) {
	rows, err := r.source.PortalListOptionsForAnnouncements(ctx, announcementIDs)
	return values.MapAnnouncementRows(values.ParentAnnouncementOptionSliceFromAnnouncementRecords(rows), func(v *values.ParentAnnouncementOption) *values.StaffParentAnnouncementOption {
		return (*values.StaffParentAnnouncementOption)(v)
	}), err
}
func (r *nativeAnnouncementRepository) ListDeclarationVersions(ctx context.Context, tenantID, announcementID int64) ([]*values.StaffDeclarationVersion, error) {
	rows, err := r.source.AnnouncementListDeclarationVersions(ctx, tenantID, announcementID)
	return values.MapAnnouncementRows(values.DeclarationVersionSliceFromAnnouncementRecords(rows), func(v *values.DeclarationVersion) *values.StaffDeclarationVersion {
		return values.StaffAnnouncementDeclarationVersionFromValue(v)
	}), err
}
func (r *nativeAnnouncementRepository) UnansweredReminderRecipients(ctx context.Context, tenantID, announcementID int64) ([]*values.StaffAnnouncementPollReminderRecipient, error) {
	rows, err := r.source.AnnouncementUnansweredReminderRecipients(ctx, tenantID, announcementID)
	return values.MapAnnouncementRows(values.AnnouncementPollReminderRecipientSliceFromAnnouncementRecords(rows), func(v *values.AnnouncementPollReminderRecipient) *values.StaffAnnouncementPollReminderRecipient {
		return (*values.StaffAnnouncementPollReminderRecipient)(v)
	}), err
}
func (r *nativeAnnouncementRepository) SchoolName(ctx context.Context, tenantID int64) (string, error) {
	rows, err := r.source.AnnouncementSchoolName(ctx, tenantID)
	return rows, err
}
func (r *nativeAnnouncementRepository) ResolveAudienceEmails(ctx context.Context, tenantID, announcementID int64) ([]*values.StaffAnnouncementRecipient, error) {
	rows, err := r.source.AnnouncementResolveAudienceEmails(ctx, tenantID, announcementID)
	return values.MapAnnouncementRows(values.AnnouncementRecipientSliceFromAnnouncementRecords(rows), func(v *values.AnnouncementRecipient) *values.StaffAnnouncementRecipient {
		return (*values.StaffAnnouncementRecipient)(v)
	}), err
}
func (r *nativeAnnouncementRepository) Stats(ctx context.Context, tenantID, announcementID int64) (*values.StaffAnnouncementStats, error) {
	rows, err := r.source.AnnouncementStats(ctx, tenantID, announcementID)
	return (*values.StaffAnnouncementStats)(values.AnnouncementStatsFromAnnouncementRecord(rows)), err
}
func (r *nativeAnnouncementRepository) ListTargets(ctx context.Context, announcementID int64) ([]*values.StaffParentAnnouncementTarget, error) {
	rows, err := r.source.AnnouncementListTargets(ctx, announcementID)
	return values.MapAnnouncementRows(values.ParentAnnouncementTargetSliceFromAnnouncementRecords(rows), func(v *values.ParentAnnouncementTarget) *values.StaffParentAnnouncementTarget {
		return (*values.StaffParentAnnouncementTarget)(v)
	}), err
}
func (r *nativeAnnouncementRepository) ListOptions(ctx context.Context, announcementID int64) ([]*values.StaffParentAnnouncementOption, error) {
	rows, err := r.source.PortalListOptions(ctx, announcementID)
	return values.MapAnnouncementRows(values.ParentAnnouncementOptionSliceFromAnnouncementRecords(rows), func(v *values.ParentAnnouncementOption) *values.StaffParentAnnouncementOption {
		return (*values.StaffParentAnnouncementOption)(v)
	}), err
}
func (r *nativeAnnouncementRepository) PollChildren(ctx context.Context, tenantID, announcementID int64) ([]*values.StaffAnnouncementPollChildStatus, error) {
	rows, err := r.source.AnnouncementPollChildren(ctx, tenantID, announcementID)
	return values.MapAnnouncementRows(values.AnnouncementPollChildStatusSliceFromAnnouncementRecords(rows), func(v *values.AnnouncementPollChildStatus) *values.StaffAnnouncementPollChildStatus {
		return (*values.StaffAnnouncementPollChildStatus)(v)
	}), err
}
func (r *nativeAnnouncementRepository) Update(ctx context.Context, a *values.StaffParentAnnouncement) error {
	recorda := values.ParentAnnouncementToAnnouncementRecord(values.StaffAnnouncementParentAnnouncementToValue(a))
	err := r.source.AnnouncementUpdate(ctx, recorda)
	if a != nil && recorda != nil {
		*a = *values.StaffAnnouncementParentAnnouncementFromValue(values.ParentAnnouncementFromAnnouncementRecord(recorda))
	}
	return err
}
func (r *nativeAnnouncementRepository) ResolveDeliveryRecipients(ctx context.Context, tenantID, announcementID int64) ([]*values.StaffAnnouncementDeliveryRecipient, error) {
	rows, err := r.source.AnnouncementResolveDeliveryRecipients(ctx, tenantID, announcementID)
	return values.MapAnnouncementRows(values.AnnouncementDeliveryRecipientSliceFromAnnouncementRecords(rows), func(v *values.AnnouncementDeliveryRecipient) *values.StaffAnnouncementDeliveryRecipient {
		return (*values.StaffAnnouncementDeliveryRecipient)(v)
	}), err
}
func (r *nativeAnnouncementRepository) CountReachableGuardiansForStudents(ctx context.Context, tenantID int64, studentIDs []int64) (int, error) {
	rows, err := r.source.AnnouncementCountReachableGuardiansForStudents(ctx, tenantID, studentIDs)
	return rows, err
}
func (r *nativeAnnouncementRepository) UnacknowledgedReminderRecipients(ctx context.Context, tenantID, announcementID int64) ([]*values.StaffAnnouncementPollReminderRecipient, error) {
	rows, err := r.source.AnnouncementUnacknowledgedReminderRecipients(ctx, tenantID, announcementID)
	return values.MapAnnouncementRows(values.AnnouncementPollReminderRecipientSliceFromAnnouncementRecords(rows), func(v *values.AnnouncementPollReminderRecipient) *values.StaffAnnouncementPollReminderRecipient {
		return (*values.StaffAnnouncementPollReminderRecipient)(v)
	}), err
}
func (r *nativeAnnouncementRepository) ListForTenant(ctx context.Context, includeInactive bool) ([]*values.StaffParentAnnouncement, error) {
	rows, err := r.source.AnnouncementListForTenant(ctx, includeInactive)
	return values.MapAnnouncementRows(values.ParentAnnouncementSliceFromAnnouncementRecords(rows), func(v *values.ParentAnnouncement) *values.StaffParentAnnouncement {
		return values.StaffAnnouncementParentAnnouncementFromValue(v)
	}), err
}
func (r *nativeAnnouncementRepository) Create(ctx context.Context, a *values.StaffParentAnnouncement) error {
	recorda := values.ParentAnnouncementToAnnouncementRecord(values.StaffAnnouncementParentAnnouncementToValue(a))
	err := r.source.AnnouncementCreate(ctx, recorda)
	if a != nil && recorda != nil {
		*a = *values.StaffAnnouncementParentAnnouncementFromValue(values.ParentAnnouncementFromAnnouncementRecord(recorda))
	}
	return err
}
func (r *nativeAnnouncementRepository) IsPublishedConflict(err error) bool {
	return r.source.IsParentAnnouncementPublishedConflict(err)
}
