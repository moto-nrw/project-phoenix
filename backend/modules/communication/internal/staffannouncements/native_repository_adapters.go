package announcement

import (
	"context"
	"time"

	values "github.com/moto-nrw/project-phoenix/modules/communication/internal/application"
)

func NewRepository(source values.StaffAnnouncementSource) ParentAnnouncementRepository {
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
func (r *nativeAnnouncementRepository) ListDeclarationSubmissions(ctx context.Context, tenantID, announcementID int64) ([]*DeclarationSubmission, error) {
	rows, err := r.source.AnnouncementListDeclarationSubmissions(ctx, tenantID, announcementID)
	return mapAnnouncementRows(values.DeclarationSubmissionSliceFromAnnouncementRecords(rows), func(v *values.DeclarationSubmission) *DeclarationSubmission { return (*DeclarationSubmission)(v) }), err
}
func (r *nativeAnnouncementRepository) ListDueReminders(ctx context.Context, notBefore, dueBefore time.Time) ([]*ParentAnnouncement, error) {
	rows, err := r.source.AnnouncementListDueReminders(ctx, notBefore, dueBefore)
	return mapAnnouncementRows(values.ParentAnnouncementSliceFromAnnouncementRecords(rows), func(v *values.ParentAnnouncement) *ParentAnnouncement {
		return announcementParentAnnouncementFromValue(v)
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
func (r *nativeAnnouncementRepository) AudienceRecipients(ctx context.Context, tenantID, announcementID int64) ([]*AnnouncementRecipientStatus, error) {
	rows, err := r.source.AnnouncementAudienceRecipients(ctx, tenantID, announcementID)
	return mapAnnouncementRows(values.AnnouncementRecipientStatusSliceFromAnnouncementRecords(rows), func(v *values.AnnouncementRecipientStatus) *AnnouncementRecipientStatus {
		return (*AnnouncementRecipientStatus)(v)
	}), err
}
func (r *nativeAnnouncementRepository) ReplaceOptions(ctx context.Context, tenantID, announcementID int64, options []*ParentAnnouncementOption) error {
	err := r.source.AnnouncementReplaceOptions(ctx, tenantID, announcementID, values.ParentAnnouncementOptionSliceToAnnouncementRecords(mapAnnouncementRows(options, func(v *ParentAnnouncementOption) *values.ParentAnnouncementOption {
		return (*values.ParentAnnouncementOption)(v)
	})))
	return err
}
func (r *nativeAnnouncementRepository) LatestDeclarationVersion(ctx context.Context, tenantID, announcementID int64) (*DeclarationVersion, error) {
	rows, err := r.source.AnnouncementLatestDeclarationVersion(ctx, tenantID, announcementID)
	return announcementDeclarationVersionFromValue(values.DeclarationVersionFromAnnouncementRecord(rows)), err
}
func (r *nativeAnnouncementRepository) ReplaceTargets(ctx context.Context, tenantID, announcementID int64, targets []*ParentAnnouncementTarget) error {
	err := r.source.AnnouncementReplaceTargets(ctx, tenantID, announcementID, values.ParentAnnouncementTargetSliceToAnnouncementRecords(mapAnnouncementRows(targets, func(v *ParentAnnouncementTarget) *values.ParentAnnouncementTarget {
		return (*values.ParentAnnouncementTarget)(v)
	})))
	return err
}
func (r *nativeAnnouncementRepository) SetPublished(ctx context.Context, id int64, publishedAt *time.Time) error {
	err := r.source.AnnouncementSetPublished(ctx, id, publishedAt)
	return err
}
func (r *nativeAnnouncementRepository) InsertDeclarationVersion(ctx context.Context, version *DeclarationVersion) error {
	recordversion := values.DeclarationVersionToAnnouncementRecord(announcementDeclarationVersionToValue(version))
	err := r.source.AnnouncementInsertDeclarationVersion(ctx, recordversion)
	if version != nil && recordversion != nil {
		*version = *announcementDeclarationVersionFromValue(values.DeclarationVersionFromAnnouncementRecord(recordversion))
	}
	return err
}
func (r *nativeAnnouncementRepository) FindByID(ctx context.Context, id int64) (*ParentAnnouncement, error) {
	rows, err := r.source.PortalFindByID(ctx, id)
	return announcementParentAnnouncementFromValue(values.ParentAnnouncementFromAnnouncementRecord(rows)), err
}
func (r *nativeAnnouncementRepository) FindByIDForUpdate(ctx context.Context, id int64) (*ParentAnnouncement, error) {
	rows, err := r.source.AnnouncementFindByIDForUpdate(ctx, id)
	return announcementParentAnnouncementFromValue(values.ParentAnnouncementFromAnnouncementRecord(rows)), err
}
func (r *nativeAnnouncementRepository) DeclarationSigners(ctx context.Context, tenantID int64, studentIDs []int64) ([]*DeclarationSigner, error) {
	rows, err := r.source.AnnouncementDeclarationSigners(ctx, tenantID, studentIDs)
	return mapAnnouncementRows(values.DeclarationSignerSliceFromAnnouncementRecords(rows), func(v *values.DeclarationSigner) *DeclarationSigner { return (*DeclarationSigner)(v) }), err
}
func (r *nativeAnnouncementRepository) PollResults(ctx context.Context, tenantID, announcementID int64) (*AnnouncementPollResults, error) {
	rows, err := r.source.AnnouncementPollResults(ctx, tenantID, announcementID)
	return announcementAnnouncementPollResultsFromValue(values.AnnouncementPollResultsFromAnnouncementRecord(rows)), err
}
func (r *nativeAnnouncementRepository) LetterChildStatuses(ctx context.Context, tenantID, announcementID int64) ([]*AnnouncementLetterChildStatus, error) {
	rows, err := r.source.AnnouncementLetterChildStatuses(ctx, tenantID, announcementID)
	return mapAnnouncementRows(values.AnnouncementLetterChildStatusSliceFromAnnouncementRecords(rows), func(v *values.AnnouncementLetterChildStatus) *AnnouncementLetterChildStatus {
		return (*AnnouncementLetterChildStatus)(v)
	}), err
}
func (r *nativeAnnouncementRepository) DeclarationChildren(ctx context.Context, tenantID, announcementID int64) ([]*DeclarationChild, error) {
	rows, err := r.source.AnnouncementDeclarationChildren(ctx, tenantID, announcementID)
	return mapAnnouncementRows(values.DeclarationChildSliceFromAnnouncementRecords(rows), func(v *values.DeclarationChild) *DeclarationChild { return (*DeclarationChild)(v) }), err
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
func (r *nativeAnnouncementRepository) ListOptionsForAnnouncements(ctx context.Context, announcementIDs []int64) ([]*ParentAnnouncementOption, error) {
	rows, err := r.source.PortalListOptionsForAnnouncements(ctx, announcementIDs)
	return mapAnnouncementRows(values.ParentAnnouncementOptionSliceFromAnnouncementRecords(rows), func(v *values.ParentAnnouncementOption) *ParentAnnouncementOption {
		return (*ParentAnnouncementOption)(v)
	}), err
}
func (r *nativeAnnouncementRepository) ListDeclarationVersions(ctx context.Context, tenantID, announcementID int64) ([]*DeclarationVersion, error) {
	rows, err := r.source.AnnouncementListDeclarationVersions(ctx, tenantID, announcementID)
	return mapAnnouncementRows(values.DeclarationVersionSliceFromAnnouncementRecords(rows), func(v *values.DeclarationVersion) *DeclarationVersion {
		return announcementDeclarationVersionFromValue(v)
	}), err
}
func (r *nativeAnnouncementRepository) UnansweredReminderRecipients(ctx context.Context, tenantID, announcementID int64) ([]*AnnouncementPollReminderRecipient, error) {
	rows, err := r.source.AnnouncementUnansweredReminderRecipients(ctx, tenantID, announcementID)
	return mapAnnouncementRows(values.AnnouncementPollReminderRecipientSliceFromAnnouncementRecords(rows), func(v *values.AnnouncementPollReminderRecipient) *AnnouncementPollReminderRecipient {
		return (*AnnouncementPollReminderRecipient)(v)
	}), err
}
func (r *nativeAnnouncementRepository) SchoolName(ctx context.Context, tenantID int64) (string, error) {
	rows, err := r.source.AnnouncementSchoolName(ctx, tenantID)
	return rows, err
}
func (r *nativeAnnouncementRepository) ResolveAudienceEmails(ctx context.Context, tenantID, announcementID int64) ([]*AnnouncementRecipient, error) {
	rows, err := r.source.AnnouncementResolveAudienceEmails(ctx, tenantID, announcementID)
	return mapAnnouncementRows(values.AnnouncementRecipientSliceFromAnnouncementRecords(rows), func(v *values.AnnouncementRecipient) *AnnouncementRecipient { return (*AnnouncementRecipient)(v) }), err
}
func (r *nativeAnnouncementRepository) Stats(ctx context.Context, tenantID, announcementID int64) (*AnnouncementStats, error) {
	rows, err := r.source.AnnouncementStats(ctx, tenantID, announcementID)
	return (*AnnouncementStats)(values.AnnouncementStatsFromAnnouncementRecord(rows)), err
}
func (r *nativeAnnouncementRepository) ListTargets(ctx context.Context, announcementID int64) ([]*ParentAnnouncementTarget, error) {
	rows, err := r.source.AnnouncementListTargets(ctx, announcementID)
	return mapAnnouncementRows(values.ParentAnnouncementTargetSliceFromAnnouncementRecords(rows), func(v *values.ParentAnnouncementTarget) *ParentAnnouncementTarget {
		return (*ParentAnnouncementTarget)(v)
	}), err
}
func (r *nativeAnnouncementRepository) ListOptions(ctx context.Context, announcementID int64) ([]*ParentAnnouncementOption, error) {
	rows, err := r.source.PortalListOptions(ctx, announcementID)
	return mapAnnouncementRows(values.ParentAnnouncementOptionSliceFromAnnouncementRecords(rows), func(v *values.ParentAnnouncementOption) *ParentAnnouncementOption {
		return (*ParentAnnouncementOption)(v)
	}), err
}
func (r *nativeAnnouncementRepository) PollChildren(ctx context.Context, tenantID, announcementID int64) ([]*AnnouncementPollChildStatus, error) {
	rows, err := r.source.AnnouncementPollChildren(ctx, tenantID, announcementID)
	return mapAnnouncementRows(values.AnnouncementPollChildStatusSliceFromAnnouncementRecords(rows), func(v *values.AnnouncementPollChildStatus) *AnnouncementPollChildStatus {
		return (*AnnouncementPollChildStatus)(v)
	}), err
}
func (r *nativeAnnouncementRepository) Update(ctx context.Context, a *ParentAnnouncement) error {
	recorda := values.ParentAnnouncementToAnnouncementRecord(announcementParentAnnouncementToValue(a))
	err := r.source.AnnouncementUpdate(ctx, recorda)
	if a != nil && recorda != nil {
		*a = *announcementParentAnnouncementFromValue(values.ParentAnnouncementFromAnnouncementRecord(recorda))
	}
	return err
}
func (r *nativeAnnouncementRepository) ResolveDeliveryRecipients(ctx context.Context, tenantID, announcementID int64) ([]*AnnouncementDeliveryRecipient, error) {
	rows, err := r.source.AnnouncementResolveDeliveryRecipients(ctx, tenantID, announcementID)
	return mapAnnouncementRows(values.AnnouncementDeliveryRecipientSliceFromAnnouncementRecords(rows), func(v *values.AnnouncementDeliveryRecipient) *AnnouncementDeliveryRecipient {
		return (*AnnouncementDeliveryRecipient)(v)
	}), err
}
func (r *nativeAnnouncementRepository) CountReachableGuardiansForStudents(ctx context.Context, tenantID int64, studentIDs []int64) (int, error) {
	rows, err := r.source.AnnouncementCountReachableGuardiansForStudents(ctx, tenantID, studentIDs)
	return rows, err
}
func (r *nativeAnnouncementRepository) UnacknowledgedReminderRecipients(ctx context.Context, tenantID, announcementID int64) ([]*AnnouncementPollReminderRecipient, error) {
	rows, err := r.source.AnnouncementUnacknowledgedReminderRecipients(ctx, tenantID, announcementID)
	return mapAnnouncementRows(values.AnnouncementPollReminderRecipientSliceFromAnnouncementRecords(rows), func(v *values.AnnouncementPollReminderRecipient) *AnnouncementPollReminderRecipient {
		return (*AnnouncementPollReminderRecipient)(v)
	}), err
}
func (r *nativeAnnouncementRepository) ListForTenant(ctx context.Context, includeInactive bool) ([]*ParentAnnouncement, error) {
	rows, err := r.source.AnnouncementListForTenant(ctx, includeInactive)
	return mapAnnouncementRows(values.ParentAnnouncementSliceFromAnnouncementRecords(rows), func(v *values.ParentAnnouncement) *ParentAnnouncement {
		return announcementParentAnnouncementFromValue(v)
	}), err
}
func (r *nativeAnnouncementRepository) Create(ctx context.Context, a *ParentAnnouncement) error {
	recorda := values.ParentAnnouncementToAnnouncementRecord(announcementParentAnnouncementToValue(a))
	err := r.source.AnnouncementCreate(ctx, recorda)
	if a != nil && recorda != nil {
		*a = *announcementParentAnnouncementFromValue(values.ParentAnnouncementFromAnnouncementRecord(recorda))
	}
	return err
}
func (r *nativeAnnouncementRepository) IsPublishedConflict(err error) bool {
	return r.source.IsParentAnnouncementPublishedConflict(err)
}
func mapAnnouncementRows[A, B any](rows []A, mapRow func(A) B) []B {
	if rows == nil {
		return nil
	}
	out := make([]B, len(rows))
	for i, row := range rows {
		out[i] = mapRow(row)
	}
	return out
}
