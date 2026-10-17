package parentstore

import (
	"context"
	"errors"
	"time"

	models "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/communication"
)

func portalFromRetainedAnnouncementPollOptionResult(v models.AnnouncementPollOptionResult) communication.PortalAnnouncementPollOptionResult {
	return communication.PortalAnnouncementPollOptionResult{
		OptionID: v.OptionID,
		Label:    v.Label,
		Position: v.Position,
		Count:    v.Count,
	}
}

func portalFromRetainedAnnouncementPollChildStatus(v models.AnnouncementPollChildStatus) communication.PortalAnnouncementPollChildStatus {
	return communication.PortalAnnouncementPollChildStatus{
		StudentID:     v.StudentID,
		FirstName:     v.FirstName,
		LastName:      v.LastName,
		SchoolClass:   v.SchoolClass,
		AnswerLabels:  v.AnswerLabels,
		RespondedAt:   v.RespondedAt,
		CanAnswer:     v.CanAnswer,
		GuardianCount: v.GuardianCount,
	}
}

func portalToRetainedParentAnnouncementTarget(v communication.PortalParentAnnouncementTarget) models.ParentAnnouncementTarget {
	return models.ParentAnnouncementTarget{
		TenantID:       v.TenantID,
		ID:             v.ID,
		AnnouncementID: v.AnnouncementID,
		TargetType:     v.TargetType,
		TargetRefID:    v.TargetRefID,
		TargetRefText:  v.TargetRefText,
		CreatedAt:      v.CreatedAt,
	}
}
func portalFromRetainedAnnouncementPollResults(v models.AnnouncementPollResults) communication.PortalAnnouncementPollResults {
	return communication.PortalAnnouncementPollResults{
		ChildCount:       v.ChildCount,
		TargetChildCount: v.TargetChildCount,
		AnsweredCount:    v.AnsweredCount,
		Options: portalRecordSlice(v.Options, func(row *models.AnnouncementPollOptionResult) *communication.PortalAnnouncementPollOptionResult {
			return portalRecordPointer(row, portalFromRetainedAnnouncementPollOptionResult)
		}),
	}
}

func portalFromRetainedAnnouncementPollReminderRecipient(v models.AnnouncementPollReminderRecipient) communication.PortalAnnouncementPollReminderRecipient {
	return communication.PortalAnnouncementPollReminderRecipient{
		AccountID:    v.AccountID,
		Email:        v.Email,
		FirstName:    v.FirstName,
		LastName:     v.LastName,
		PortalLocale: v.PortalLocale,
	}
}

func portalFromRetainedAnnouncementRecipient(v models.AnnouncementRecipient) communication.PortalAnnouncementRecipient {
	return communication.PortalAnnouncementRecipient{
		AccountID: v.AccountID,
		Email:     v.Email,
		FirstName: v.FirstName,
		LastName:  v.LastName,
	}
}

func portalFromRetainedAnnouncementRecipientStatus(v models.AnnouncementRecipientStatus) communication.PortalAnnouncementRecipientStatus {
	return communication.PortalAnnouncementRecipientStatus{
		AccountID:      v.AccountID,
		FirstName:      v.FirstName,
		LastName:       v.LastName,
		PortalLocale:   v.PortalLocale,
		ReadAt:         v.ReadAt,
		AcknowledgedAt: v.AcknowledgedAt,
	}
}

func portalToRetainedParentAnnouncement(v communication.PortalParentAnnouncement) models.ParentAnnouncement {
	return models.ParentAnnouncement{
		Model: models.Model{ID: v.ID, CreatedAt: v.CreatedAt, UpdatedAt: v.UpdatedAt}, TenantModel: models.TenantModel{TenantID: v.TenantID},
		Title:                   v.Title,
		Body:                    v.Body,
		Priority:                v.Priority,
		LinkURL:                 v.LinkURL,
		RequiresAcknowledgement: v.RequiresAcknowledgement,
		SendEmail:               v.SendEmail,
		PublishedAt:             v.PublishedAt,
		ExpiresAt:               v.ExpiresAt,
		Active:                  v.Active,
		CreatedBy:               v.CreatedBy,
		ResponseType:            v.ResponseType,
		ResponseDeadline:        v.ResponseDeadline,
		DeliveryMode:            v.DeliveryMode,
		EmailAudience:           v.EmailAudience,
		SystemKind:              v.SystemKind,
		ReminderAt:              v.ReminderAt,
		ReminderText:            v.ReminderText,
		ReminderSentAt:          v.ReminderSentAt,
		Targets: portalRecordSlice(v.Targets, func(row *communication.PortalParentAnnouncementTarget) *models.ParentAnnouncementTarget {
			return portalRecordPointer(row, portalToRetainedParentAnnouncementTarget)
		}),
		Options: portalRecordSlice(v.Options, func(row *communication.PortalParentAnnouncementOption) *models.ParentAnnouncementOption {
			return portalRecordPointer(row, portalToRetainedParentAnnouncementOption)
		}),
		Declaration: portalToRetainedAnnouncementDeclarationSettings(v.Declaration),
	}
}
func portalToRetainedAnnouncementDeclarationSettings(v communication.PortalAnnouncementDeclarationSettings) models.AnnouncementDeclarationSettings {
	return models.AnnouncementDeclarationSettings{
		Kind:             v.Kind,
		Signers:          v.Signers,
		Revocable:        v.Revocable,
		RequiresPassword: v.RequiresPassword,
	}
}
func portalFromRetainedAnnouncementStats(v models.AnnouncementStats) communication.PortalAnnouncementStats {
	return communication.PortalAnnouncementStats{
		TargetCount:       v.TargetCount,
		ReadCount:         v.ReadCount,
		AcknowledgedCount: v.AcknowledgedCount,
	}
}

func portalToRetainedParentAnnouncementOption(v communication.PortalParentAnnouncementOption) models.ParentAnnouncementOption {
	return models.ParentAnnouncementOption{
		TenantID:       v.TenantID,
		ID:             v.ID,
		AnnouncementID: v.AnnouncementID,
		Label:          v.Label,
		Position:       v.Position,
		CreatedAt:      v.CreatedAt,
	}
}
func portalFromRetainedAnnouncementDeliveryRecipient(v models.AnnouncementDeliveryRecipient) communication.PortalAnnouncementDeliveryRecipient {
	return communication.PortalAnnouncementDeliveryRecipient{
		GuardianProfileID: v.GuardianProfileID,
		AccountID:         v.AccountID,
		FirstName:         v.FirstName,
		LastName:          v.LastName,
		Email:             v.Email,
		HasPortalAccess:   v.HasPortalAccess,
		PortalLocale:      v.PortalLocale,
	}
}

func portalFromRetainedAnnouncementLetterChildStatus(v models.AnnouncementLetterChildStatus) communication.PortalAnnouncementLetterChildStatus {
	return communication.PortalAnnouncementLetterChildStatus{
		StudentID:      v.StudentID,
		FirstName:      v.FirstName,
		LastName:       v.LastName,
		SchoolClass:    v.SchoolClass,
		CanConfirm:     v.CanConfirm,
		AcknowledgedAt: v.AcknowledgedAt,
		AckFirstName:   v.AckFirstName,
		AckLastName:    v.AckLastName,
	}
}

func (r *announcementRepository) AnnouncementClearEngagement(ctx context.Context, announcementID int64) error {
	err := r.ClearEngagement(ctx, announcementID)
	return err
}
func (r *announcementRepository) AnnouncementSetReminder(ctx context.Context, id int64, reminderAt *time.Time, reminderText *string) (bool, error) {
	rows, err := r.SetReminder(ctx, id, reminderAt, reminderText)
	return rows, err
}
func (r *announcementRepository) AnnouncementListDeclarationSubmissions(ctx context.Context, tenantID, announcementID int64) ([]*communication.PortalDeclarationSubmission, error) {
	rows, err := r.ListDeclarationSubmissions(ctx, tenantID, announcementID)
	return portalRecordSlice(rows, func(row *models.DeclarationSubmission) *communication.PortalDeclarationSubmission {
		return portalRecordPointer(row, portalFromRetainedDeclarationSubmission)
	}), err
}
func (r *announcementRepository) AnnouncementListDueReminders(ctx context.Context, notBefore, dueBefore time.Time) ([]*communication.PortalParentAnnouncement, error) {
	rows, err := r.ListDueReminders(ctx, notBefore, dueBefore)
	return portalRecordSlice(rows, func(row *models.ParentAnnouncement) *communication.PortalParentAnnouncement {
		return portalRecordPointer(row, portalFromRetainedParentAnnouncement)
	}), err
}
func (r *announcementRepository) AnnouncementClaimReminder(ctx context.Context, id int64, sentAt time.Time) (bool, error) {
	rows, err := r.ClaimReminder(ctx, id, sentAt)
	return rows, err
}
func (r *announcementRepository) AnnouncementCountDeclarationSubmissions(ctx context.Context, tenantID, announcementID int64) (int, error) {
	rows, err := r.CountDeclarationSubmissions(ctx, tenantID, announcementID)
	return rows, err
}
func (r *announcementRepository) AnnouncementAudienceRecipients(ctx context.Context, tenantID, announcementID int64) ([]*communication.PortalAnnouncementRecipientStatus, error) {
	rows, err := r.AudienceRecipients(ctx, tenantID, announcementID)
	return portalRecordSlice(rows, func(row *models.AnnouncementRecipientStatus) *communication.PortalAnnouncementRecipientStatus {
		return portalRecordPointer(row, portalFromRetainedAnnouncementRecipientStatus)
	}), err
}
func (r *announcementRepository) AnnouncementReplaceOptions(ctx context.Context, tenantID, announcementID int64, options []*communication.PortalParentAnnouncementOption) error {
	err := r.ReplaceOptions(ctx, tenantID, announcementID, portalRecordSlice(options, func(row *communication.PortalParentAnnouncementOption) *models.ParentAnnouncementOption {
		return portalRecordPointer(row, portalToRetainedParentAnnouncementOption)
	}))
	return err
}
func (r *announcementRepository) AnnouncementLatestDeclarationVersion(ctx context.Context, tenantID, announcementID int64) (*communication.PortalDeclarationVersion, error) {
	rows, err := r.LatestDeclarationVersion(ctx, tenantID, announcementID)
	return portalRecordPointer(rows, portalFromRetainedDeclarationVersion), err
}
func (r *announcementRepository) AnnouncementReplaceTargets(ctx context.Context, tenantID, announcementID int64, targets []*communication.PortalParentAnnouncementTarget) error {
	err := r.ReplaceTargets(ctx, tenantID, announcementID, portalRecordSlice(targets, func(row *communication.PortalParentAnnouncementTarget) *models.ParentAnnouncementTarget {
		return portalRecordPointer(row, portalToRetainedParentAnnouncementTarget)
	}))
	return err
}
func (r *announcementRepository) AnnouncementSetPublished(ctx context.Context, id int64, publishedAt *time.Time) error {
	err := r.SetPublished(ctx, id, publishedAt)
	return err
}
func (r *announcementRepository) AnnouncementInsertDeclarationVersion(ctx context.Context, version *communication.PortalDeclarationVersion) error {
	retainedversion := portalRecordPointer(version, portalToRetainedDeclarationVersion)
	err := r.InsertDeclarationVersion(ctx, retainedversion)
	if version != nil && retainedversion != nil {
		*version = portalFromRetainedDeclarationVersion(*retainedversion)
	}
	return err
}
func (r *announcementRepository) AnnouncementFindByIDForUpdate(ctx context.Context, id int64) (*communication.PortalParentAnnouncement, error) {
	rows, err := r.FindByIDForUpdate(ctx, id)
	return portalRecordPointer(rows, portalFromRetainedParentAnnouncement), err
}
func (r *announcementRepository) AnnouncementDeclarationSigners(ctx context.Context, tenantID int64, studentIDs []int64) ([]*communication.PortalDeclarationSigner, error) {
	rows, err := r.DeclarationSigners(ctx, tenantID, studentIDs)
	return portalRecordSlice(rows, func(row *models.DeclarationSigner) *communication.PortalDeclarationSigner {
		return portalRecordPointer(row, portalFromRetainedDeclarationSigner)
	}), err
}
func (r *announcementRepository) AnnouncementPollResults(ctx context.Context, tenantID, announcementID int64) (*communication.PortalAnnouncementPollResults, error) {
	rows, err := r.PollResults(ctx, tenantID, announcementID)
	return portalRecordPointer(rows, portalFromRetainedAnnouncementPollResults), err
}
func (r *announcementRepository) AnnouncementLetterChildStatuses(ctx context.Context, tenantID, announcementID int64) ([]*communication.PortalAnnouncementLetterChildStatus, error) {
	rows, err := r.LetterChildStatuses(ctx, tenantID, announcementID)
	return portalRecordSlice(rows, func(row *models.AnnouncementLetterChildStatus) *communication.PortalAnnouncementLetterChildStatus {
		return portalRecordPointer(row, portalFromRetainedAnnouncementLetterChildStatus)
	}), err
}
func (r *announcementRepository) AnnouncementDeclarationChildren(ctx context.Context, tenantID, announcementID int64) ([]*communication.PortalDeclarationChild, error) {
	rows, err := r.DeclarationChildren(ctx, tenantID, announcementID)
	return portalRecordSlice(rows, func(row *models.DeclarationChild) *communication.PortalDeclarationChild {
		return portalRecordPointer(row, portalFromRetainedDeclarationChild)
	}), err
}
func (r *announcementRepository) AnnouncementReleaseReminderClaim(ctx context.Context, id int64, sentAt time.Time) (bool, error) {
	rows, err := r.ReleaseReminderClaim(ctx, id, sentAt)
	return rows, err
}
func (r *announcementRepository) AnnouncementDelete(ctx context.Context, id int64) error {
	err := r.Delete(ctx, id)
	return err
}
func (r *announcementRepository) AnnouncementPublishIfDraft(ctx context.Context, id int64, publishedAt time.Time) (bool, error) {
	rows, err := r.PublishIfDraft(ctx, id, publishedAt)
	return rows, err
}
func (r *announcementRepository) AnnouncementListDeclarationVersions(ctx context.Context, tenantID, announcementID int64) ([]*communication.PortalDeclarationVersion, error) {
	rows, err := r.ListDeclarationVersions(ctx, tenantID, announcementID)
	return portalRecordSlice(rows, func(row *models.DeclarationVersion) *communication.PortalDeclarationVersion {
		return portalRecordPointer(row, portalFromRetainedDeclarationVersion)
	}), err
}
func (r *announcementRepository) AnnouncementUnansweredReminderRecipients(ctx context.Context, tenantID, announcementID int64) ([]*communication.PortalAnnouncementPollReminderRecipient, error) {
	rows, err := r.UnansweredReminderRecipients(ctx, tenantID, announcementID)
	return portalRecordSlice(rows, func(row *models.AnnouncementPollReminderRecipient) *communication.PortalAnnouncementPollReminderRecipient {
		return portalRecordPointer(row, portalFromRetainedAnnouncementPollReminderRecipient)
	}), err
}
func (r *announcementRepository) AnnouncementSchoolName(ctx context.Context, tenantID int64) (string, error) {
	rows, err := r.SchoolName(ctx, tenantID)
	return rows, err
}
func (r *announcementRepository) AnnouncementResolveAudienceEmails(ctx context.Context, tenantID, announcementID int64) ([]*communication.PortalAnnouncementRecipient, error) {
	rows, err := r.ResolveAudienceEmails(ctx, tenantID, announcementID)
	return portalRecordSlice(rows, func(row *models.AnnouncementRecipient) *communication.PortalAnnouncementRecipient {
		return portalRecordPointer(row, portalFromRetainedAnnouncementRecipient)
	}), err
}
func (r *announcementRepository) AnnouncementStats(ctx context.Context, tenantID, announcementID int64) (*communication.PortalAnnouncementStats, error) {
	rows, err := r.Stats(ctx, tenantID, announcementID)
	return portalRecordPointer(rows, portalFromRetainedAnnouncementStats), err
}
func (r *announcementRepository) AnnouncementListTargets(ctx context.Context, announcementID int64) ([]*communication.PortalParentAnnouncementTarget, error) {
	rows, err := r.ListTargets(ctx, announcementID)
	return portalRecordSlice(rows, func(row *models.ParentAnnouncementTarget) *communication.PortalParentAnnouncementTarget {
		return portalRecordPointer(row, portalFromRetainedParentAnnouncementTarget)
	}), err
}
func (r *announcementRepository) AnnouncementPollChildren(ctx context.Context, tenantID, announcementID int64) ([]*communication.PortalAnnouncementPollChildStatus, error) {
	rows, err := r.PollChildren(ctx, tenantID, announcementID)
	return portalRecordSlice(rows, func(row *models.AnnouncementPollChildStatus) *communication.PortalAnnouncementPollChildStatus {
		return portalRecordPointer(row, portalFromRetainedAnnouncementPollChildStatus)
	}), err
}
func (r *announcementRepository) AnnouncementUpdate(ctx context.Context, a *communication.PortalParentAnnouncement) error {
	retaineda := portalRecordPointer(a, portalToRetainedParentAnnouncement)
	err := r.Update(ctx, retaineda)
	if a != nil && retaineda != nil {
		*a = portalFromRetainedParentAnnouncement(*retaineda)
	}
	return err
}
func (r *announcementRepository) AnnouncementResolveDeliveryRecipients(ctx context.Context, tenantID, announcementID int64) ([]*communication.PortalAnnouncementDeliveryRecipient, error) {
	rows, err := r.ResolveDeliveryRecipients(ctx, tenantID, announcementID)
	return portalRecordSlice(rows, func(row *models.AnnouncementDeliveryRecipient) *communication.PortalAnnouncementDeliveryRecipient {
		return portalRecordPointer(row, portalFromRetainedAnnouncementDeliveryRecipient)
	}), err
}
func (r *announcementRepository) AnnouncementCountReachableGuardiansForStudents(ctx context.Context, tenantID int64, studentIDs []int64) (int, error) {
	rows, err := r.CountReachableGuardiansForStudents(ctx, tenantID, studentIDs)
	return rows, err
}
func (r *announcementRepository) AnnouncementUnacknowledgedReminderRecipients(ctx context.Context, tenantID, announcementID int64) ([]*communication.PortalAnnouncementPollReminderRecipient, error) {
	rows, err := r.UnacknowledgedReminderRecipients(ctx, tenantID, announcementID)
	return portalRecordSlice(rows, func(row *models.AnnouncementPollReminderRecipient) *communication.PortalAnnouncementPollReminderRecipient {
		return portalRecordPointer(row, portalFromRetainedAnnouncementPollReminderRecipient)
	}), err
}
func (r *announcementRepository) AnnouncementListForTenant(ctx context.Context, includeInactive bool) ([]*communication.PortalParentAnnouncement, error) {
	rows, err := r.ListForTenant(ctx, includeInactive)
	return portalRecordSlice(rows, func(row *models.ParentAnnouncement) *communication.PortalParentAnnouncement {
		return portalRecordPointer(row, portalFromRetainedParentAnnouncement)
	}), err
}
func (r *announcementRepository) AnnouncementCreate(ctx context.Context, a *communication.PortalParentAnnouncement) error {
	retaineda := portalRecordPointer(a, portalToRetainedParentAnnouncement)
	err := r.Create(ctx, retaineda)
	if a != nil && retaineda != nil {
		*a = portalFromRetainedParentAnnouncement(*retaineda)
	}
	return err
}
func (r *announcementRepository) IsParentAnnouncementPublishedConflict(err error) bool {
	return errors.Is(err, models.ErrAnnouncementPublished)
}
