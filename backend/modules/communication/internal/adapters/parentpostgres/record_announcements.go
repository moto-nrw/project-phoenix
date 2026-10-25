package parentpostgres

import (
	"context"
	"errors"
	"sort"
	"time"

	recordports "github.com/moto-nrw/project-phoenix/modules/communication/ports"

	"github.com/moto-nrw/project-phoenix/modules/communication"
	"github.com/moto-nrw/project-phoenix/modules/communication/internal/domain"
	"github.com/uptrace/bun"
)

// NewParentAnnouncementRepository returns the announcement store and audience
// projection behind the existing repository contract. The optional clock
// fixes the calendar day activity-group targets are evaluated on.
//
// SchoolName is not served here: the school's name belongs to Organisation
// and Tenancy, and the composition root decorates it exactly as before.
func NewParentAnnouncementRepository(db *bun.DB, audience recordports.AnnouncementAudienceReads) recordports.ParentAnnouncementRepository {
	if audience == nil {
		panic("communication parent store: audience is required")
	}
	return &announcementRepository{store: NewAnnouncementStore(NewParentRecordDatabase(db)), AnnouncementAudienceReads: audience}
}

type announcementRepository struct {
	store *AnnouncementStore
	recordports.AnnouncementAudienceReads
}

// SurveyRecords is the survey store (#4781) over the announcements'
// database: a parent survey is carried by one of these announcements.
func (r *announcementRepository) SurveyRecords() communication.SurveyRecords {
	return NewSurveyStore(r.store.database)
}

func (r *announcementRepository) Create(ctx context.Context, a *communication.PortalParentAnnouncement) error {
	defer stampAnnouncementKind(a)
	value := announcementValue(a)
	if err := r.store.Create(ctx, value); err != nil {
		return err
	}
	a.ID = value.ID
	a.CreatedAt = value.CreatedAt
	a.UpdatedAt = value.UpdatedAt
	a.ResponseType = value.ResponseType
	a.DeliveryMode = value.DeliveryMode
	a.EmailAudience = value.EmailAudience
	a.TenantID = value.TenantID
	return nil
}

func (r *announcementRepository) Update(ctx context.Context, a *communication.PortalParentAnnouncement) error {
	defer stampAnnouncementKind(a)
	value := announcementValue(a)
	if err := r.store.UpdateDraft(ctx, value); err != nil {
		return publishedError(err)
	}
	a.UpdatedAt = value.UpdatedAt
	return nil
}

func (r *announcementRepository) FindByID(ctx context.Context, id int64) (*communication.PortalParentAnnouncement, error) {
	value, err := r.store.FindByID(ctx, id)
	return announcementModel(value), err
}

func (r *announcementRepository) FindByIDForUpdate(ctx context.Context, id int64) (*communication.PortalParentAnnouncement, error) {
	value, err := r.store.FindByIDForUpdate(ctx, id)
	return announcementModel(value), err
}

func (r *announcementRepository) Delete(ctx context.Context, id int64) error {
	return r.store.Delete(ctx, id)
}

func (r *announcementRepository) ListForTenant(ctx context.Context, includeInactive bool) ([]*communication.PortalParentAnnouncement, error) {
	values, err := r.store.ListForTenant(ctx, includeInactive)
	if err != nil {
		return nil, err
	}
	rows := make([]*communication.PortalParentAnnouncement, 0, len(values))
	for _, value := range values {
		rows = append(rows, announcementModel(value))
	}
	return rows, nil
}

func (r *announcementRepository) SetPublished(ctx context.Context, id int64, publishedAt *time.Time) error {
	return r.store.SetPublished(ctx, id, publishedAt)
}

func (r *announcementRepository) ClearEngagement(ctx context.Context, announcementID int64) error {
	return r.store.ClearEngagement(ctx, announcementID)
}

func (r *announcementRepository) PublishIfDraft(ctx context.Context, id int64, publishedAt time.Time) (bool, error) {
	return r.store.PublishIfDraft(ctx, id, publishedAt)
}

func (r *announcementRepository) SetReminder(ctx context.Context, id int64, reminderAt *time.Time, reminderText *string) (bool, error) {
	return r.store.SetReminder(ctx, id, reminderAt, reminderText)
}

func (r *announcementRepository) ListDueReminders(ctx context.Context, notBefore, dueBefore time.Time) ([]*communication.PortalParentAnnouncement, error) {
	values, err := r.store.ListDueReminders(ctx, notBefore, dueBefore)
	if err != nil {
		return nil, err
	}
	rows := make([]*communication.PortalParentAnnouncement, 0, len(values))
	for _, value := range values {
		rows = append(rows, announcementModel(value))
	}
	return rows, nil
}

func (r *announcementRepository) ClaimReminder(ctx context.Context, id int64, sentAt time.Time) (bool, error) {
	return r.store.ClaimReminder(ctx, id, sentAt)
}

func (r *announcementRepository) ReleaseReminderClaim(ctx context.Context, id int64, sentAt time.Time) (bool, error) {
	return r.store.ReleaseReminderClaim(ctx, id, sentAt)
}

func (r *announcementRepository) ReplaceTargets(ctx context.Context, tenantID, announcementID int64, targets []*communication.PortalParentAnnouncementTarget) error {
	values := make([]*domain.ParentAnnouncementTarget, 0, len(targets))
	for _, target := range targets {
		values = append(values, &domain.ParentAnnouncementTarget{
			ID: target.ID, TenantID: target.TenantID, AnnouncementID: target.AnnouncementID,
			TargetType: target.TargetType, TargetRefID: target.TargetRefID, TargetRefText: target.TargetRefText,
		})
	}
	if err := r.store.ReplaceTargets(ctx, tenantID, announcementID, values); err != nil {
		return publishedError(err)
	}
	for i, value := range values {
		targets[i].ID = value.ID
		targets[i].AnnouncementID = value.AnnouncementID
		targets[i].CreatedAt = value.CreatedAt
		targets[i].TenantID = value.TenantID
	}
	return nil
}

func (r *announcementRepository) ListTargets(ctx context.Context, announcementID int64) ([]*communication.PortalParentAnnouncementTarget, error) {
	values, err := r.store.ListTargets(ctx, announcementID)
	if err != nil {
		return nil, err
	}
	rows := make([]*communication.PortalParentAnnouncementTarget, 0, len(values))
	for _, value := range values {
		rows = append(rows, targetModel(value))
	}
	return rows, nil
}

func (r *announcementRepository) ReplaceOptions(ctx context.Context, tenantID, announcementID int64, options []*communication.PortalParentAnnouncementOption) error {
	values := make([]*domain.ParentAnnouncementOption, 0, len(options))
	for _, option := range options {
		values = append(values, &domain.ParentAnnouncementOption{
			ID: option.ID, TenantID: option.TenantID, AnnouncementID: option.AnnouncementID,
			Label: option.Label, Position: option.Position,
		})
	}
	if err := r.store.ReplaceOptions(ctx, tenantID, announcementID, values); err != nil {
		return publishedError(err)
	}
	for i, value := range values {
		options[i].ID = value.ID
		options[i].AnnouncementID = value.AnnouncementID
		options[i].Position = value.Position
		options[i].CreatedAt = value.CreatedAt
		options[i].TenantID = value.TenantID
	}
	return nil
}

func (r *announcementRepository) ListOptions(ctx context.Context, announcementID int64) ([]*communication.PortalParentAnnouncementOption, error) {
	values, err := r.store.ListOptions(ctx, announcementID)
	return optionModels(values), err
}

func (r *announcementRepository) ListOptionsForAnnouncements(ctx context.Context, announcementIDs []int64) ([]*communication.PortalParentAnnouncementOption, error) {
	values, err := r.store.ListOptionsForAnnouncements(ctx, announcementIDs)
	return optionModels(values), err
}

func (r *announcementRepository) AnswerableChildren(ctx context.Context, accountID int64, announcementIDs []int64) ([]*communication.PortalAnnouncementPollChild, error) {
	values, err := r.AnnouncementAudienceReads.AnswerableChildren(ctx, accountID, announcementIDs)
	if err != nil {
		return nil, err
	}
	rows := make([]*communication.PortalAnnouncementPollChild, 0, len(values))
	for _, value := range values {
		rows = append(rows, &communication.PortalAnnouncementPollChild{
			AnnouncementID: value.AnnouncementID, StudentID: value.StudentID, FirstName: value.FirstName,
			LastName: value.LastName, SelectedOptions: value.SelectedOptions,
		})
	}
	return rows, nil
}

func (r *announcementRepository) AccountMayAnswerForStudent(ctx context.Context, tenantID, announcementID, accountID, studentID int64) (bool, error) {
	return r.AnnouncementAudienceReads.AccountMayAnswerForStudent(ctx, tenantID, announcementID, accountID, studentID)
}

// SetResponse replaces a child's answer for a poll. A transaction-scoped
// advisory lock serializes replacements for one poll and child. The
// audience/relationship guard is held through the projection as a share
// lock on the guardian rows for the rest of the transaction, so a revocation
// either committed before the check (and is seen) or waits behind the
// answer; the liveness/selection guard is part of each write. A correction
// or deadline that changes in between therefore rolls the transaction back
// instead of storing a stale answer, exactly as the single-statement guard
// did before the audience rows moved out of the write.
func (r *announcementRepository) SetResponse(ctx context.Context, tenantID, announcementID, studentID, accountID int64, optionIDs []int64, expectedPublishedAt time.Time) (bool, error) {
	if err := r.store.LockResponse(ctx, announcementID, studentID); err != nil {
		return false, err
	}
	allowed, err := r.HoldAnswerPermission(ctx, tenantID, announcementID, accountID, studentID)
	if err != nil || !allowed {
		return false, err
	}
	live, err := r.store.DeleteResponse(ctx, tenantID, announcementID, studentID, optionIDs, expectedPublishedAt)
	if err != nil || !live || len(optionIDs) == 0 {
		return live, err
	}
	return r.store.InsertResponse(ctx, tenantID, announcementID, studentID, accountID, optionIDs, expectedPublishedAt)
}

func (r *announcementRepository) PollResults(ctx context.Context, tenantID, announcementID int64) (*communication.PortalAnnouncementPollResults, error) {
	value, err := r.AnnouncementAudienceReads.PollResults(ctx, tenantID, announcementID)
	if err != nil {
		return nil, err
	}
	results := &communication.PortalAnnouncementPollResults{
		ChildCount: value.ChildCount, TargetChildCount: value.TargetChildCount, AnsweredCount: value.AnsweredCount,
		Options: make([]*communication.PortalAnnouncementPollOptionResult, 0, len(value.Options)),
	}
	for _, option := range value.Options {
		results.Options = append(results.Options, &communication.PortalAnnouncementPollOptionResult{
			OptionID: option.OptionID, Label: option.Label, Position: option.Position, Count: option.Count,
		})
	}
	return results, nil
}

func (r *announcementRepository) PollChildren(ctx context.Context, tenantID, announcementID int64) ([]*communication.PortalAnnouncementPollChildStatus, error) {
	values, err := r.AnnouncementAudienceReads.PollChildren(ctx, tenantID, announcementID)
	if err != nil {
		return nil, err
	}
	rows := make([]*communication.PortalAnnouncementPollChildStatus, 0, len(values))
	for _, value := range values {
		rows = append(rows, &communication.PortalAnnouncementPollChildStatus{
			StudentID: value.StudentID, FirstName: value.FirstName, LastName: value.LastName,
			SchoolClass: value.SchoolClass, AnswerLabels: value.AnswerLabels, RespondedAt: value.RespondedAt,
			CanAnswer: value.CanAnswer,
		})
	}
	return rows, nil
}

func (r *announcementRepository) UnansweredReminderRecipients(ctx context.Context, tenantID, announcementID int64) ([]*communication.PortalAnnouncementPollReminderRecipient, error) {
	values, err := r.AnnouncementAudienceReads.UnansweredReminderRecipients(ctx, tenantID, announcementID)
	return reminderModels(values), err
}

func (r *announcementRepository) UnacknowledgedReminderRecipients(ctx context.Context, tenantID, announcementID int64) ([]*communication.PortalAnnouncementPollReminderRecipient, error) {
	values, err := r.AnnouncementAudienceReads.UnacknowledgedReminderRecipients(ctx, tenantID, announcementID)
	return reminderModels(values), err
}

func (r *announcementRepository) CountAudience(ctx context.Context, tenantID, announcementID int64) (int, error) {
	return r.AnnouncementAudienceReads.CountAudience(ctx, tenantID, announcementID)
}

func (r *announcementRepository) AccountMatchesAnnouncement(ctx context.Context, tenantID, announcementID, accountID int64) (bool, error) {
	matched, err := r.AnnouncementAudienceReads.AccountMatchesAnnouncement(ctx, tenantID, announcementID, accountID)
	if err != nil || matched {
		return matched, err
	}
	submissions, err := r.store.ListDeclarationSubmissions(ctx, tenantID, announcementID)
	if err != nil {
		return false, err
	}
	return r.HistoricalDeclarationAccess(ctx, tenantID, accountID, declarationHistoryLinksForAccount(submissions, accountID))
}

func (r *announcementRepository) ResolveAudienceEmails(ctx context.Context, tenantID, announcementID int64) ([]*communication.PortalAnnouncementRecipient, error) {
	values, err := r.AnnouncementAudienceReads.ResolveAudienceEmails(ctx, tenantID, announcementID)
	if err != nil {
		return nil, err
	}
	rows := make([]*communication.PortalAnnouncementRecipient, 0, len(values))
	for _, value := range values {
		rows = append(rows, &communication.PortalAnnouncementRecipient{
			AccountID: value.AccountID, Email: value.Email, FirstName: value.FirstName, LastName: value.LastName,
		})
	}
	return rows, nil
}

func (r *announcementRepository) LetterChildStatuses(ctx context.Context, tenantID, announcementID int64) ([]*communication.PortalAnnouncementLetterChildStatus, error) {
	values, err := r.AnnouncementAudienceReads.LetterChildStatuses(ctx, tenantID, announcementID)
	if err != nil {
		return nil, err
	}
	rows := make([]*communication.PortalAnnouncementLetterChildStatus, 0, len(values))
	for _, value := range values {
		rows = append(rows, &communication.PortalAnnouncementLetterChildStatus{
			StudentID: value.StudentID, FirstName: value.FirstName, LastName: value.LastName,
			SchoolClass: value.SchoolClass, CanConfirm: value.CanConfirm, AcknowledgedAt: value.AcknowledgedAt,
			AckFirstName: value.AckFirstName, AckLastName: value.AckLastName,
		})
	}
	return rows, nil
}

func (r *announcementRepository) ResolveDeliveryRecipients(ctx context.Context, tenantID, announcementID int64) ([]*communication.PortalAnnouncementDeliveryRecipient, error) {
	values, err := r.AnnouncementAudienceReads.ResolveDeliveryRecipients(ctx, tenantID, announcementID)
	if err != nil {
		return nil, err
	}
	rows := make([]*communication.PortalAnnouncementDeliveryRecipient, 0, len(values))
	for _, value := range values {
		rows = append(rows, &communication.PortalAnnouncementDeliveryRecipient{
			GuardianProfileID: value.GuardianProfileID, AccountID: value.AccountID, FirstName: value.FirstName,
			LastName: value.LastName, Email: value.Email, HasPortalAccess: value.HasPortalAccess,
			PortalLocale: value.PortalLocale,
		})
	}
	return rows, nil
}

// SchoolName is served by the Organisation and Tenancy decorator at the
// composition root; the store itself does not read school rows.
func (r *announcementRepository) SchoolName(context.Context, int64) (string, error) {
	return "", errors.New("resolve school name through organization tenancy capability")
}

func (r *announcementRepository) AudienceRecipients(ctx context.Context, tenantID, announcementID int64) ([]*communication.PortalAnnouncementRecipientStatus, error) {
	values, err := r.AnnouncementAudienceReads.AudienceRecipients(ctx, tenantID, announcementID)
	if err != nil {
		return nil, err
	}
	rows := make([]*communication.PortalAnnouncementRecipientStatus, 0, len(values))
	for _, value := range values {
		rows = append(rows, &communication.PortalAnnouncementRecipientStatus{
			AccountID: value.AccountID, FirstName: value.FirstName, LastName: value.LastName,
			PortalLocale: value.PortalLocale, ReadAt: value.ReadAt, AcknowledgedAt: value.AcknowledgedAt,
		})
	}
	return rows, nil
}

func (r *announcementRepository) ListFeedForAccount(ctx context.Context, accountID int64, scope communication.PortalAnnouncementFeedScope) ([]*communication.PortalAnnouncementFeedItem, error) {
	scopeValue := feedScope(scope)
	values, err := r.AnnouncementAudienceReads.ListFeedForAccount(ctx, accountID, scopeValue)
	if err != nil {
		return nil, err
	}
	submissions, err := r.store.ListDeclarationSubmissionsForAccountInTenants(ctx, accountID, feedTenantIDs(scopeValue))
	if err != nil {
		return nil, err
	}
	history, err := r.HistoricalDeclarationFeed(ctx, accountID, scopeValue, declarationHistoryLinks(submissions))
	if err != nil {
		return nil, err
	}
	values = mergeFeedItems(values, history)
	rows := make([]*communication.PortalAnnouncementFeedItem, 0, len(values))
	for _, value := range values {
		rows = append(rows, &communication.PortalAnnouncementFeedItem{
			ID: value.ID, TenantID: value.TenantID, Title: value.Title, Body: value.Body, Priority: value.Priority,
			Poll:    domain.AnnouncementResponseIsPoll(value.ResponseType),
			LinkURL: value.LinkURL, RequiresAcknowledgement: value.RequiresAcknowledgement,
			PublishedAt: value.PublishedAt, ExpiresAt: value.ExpiresAt, ResponseType: value.ResponseType,
			DeliveryMode: value.DeliveryMode, ResponseDeadline: value.ResponseDeadline, SystemKind: value.SystemKind,
			ReminderSentAt: value.ReminderSentAt, ReminderText: value.ReminderText,
			ReadAt: value.ReadAt, AcknowledgedAt: value.AcknowledgedAt,
		})
	}
	return rows, nil
}

func declarationHistoryLinksForAccount(submissions []*domain.DeclarationSubmission, accountID int64) []*communication.PortalDeclarationSubmission {
	owned := make([]*domain.DeclarationSubmission, 0, len(submissions))
	for _, submission := range submissions {
		if submission.AccountID != nil && *submission.AccountID == accountID {
			owned = append(owned, submission)
		}
	}
	return declarationHistoryLinks(owned)
}

func mergeFeedItems(current, historical []*communication.PortalAnnouncementFeedItem) []*communication.PortalAnnouncementFeedItem {
	items := make(map[int64]*communication.PortalAnnouncementFeedItem, len(current)+len(historical))
	for _, item := range historical {
		items[item.ID] = item
	}
	for _, item := range current {
		items[item.ID] = item
	}
	merged := make([]*communication.PortalAnnouncementFeedItem, 0, len(items))
	for _, item := range items {
		merged = append(merged, item)
	}
	sort.Slice(merged, func(i, j int) bool {
		return feedItemTime(merged[i]).After(feedItemTime(merged[j])) ||
			(feedItemTime(merged[i]).Equal(feedItemTime(merged[j])) && merged[i].ID > merged[j].ID)
	})
	return merged
}

func feedItemTime(item *communication.PortalAnnouncementFeedItem) time.Time {
	if item.ReminderSentAt != nil && (item.PublishedAt == nil || item.ReminderSentAt.After(*item.PublishedAt)) {
		return *item.ReminderSentAt
	}
	if item.PublishedAt != nil {
		return *item.PublishedAt
	}
	return time.Time{}
}

func (r *announcementRepository) CountUnreadForAccount(ctx context.Context, accountID int64, scope communication.PortalAnnouncementFeedScope) (int, error) {
	return r.CountOutstandingForAccount(ctx, accountID, feedScope(scope))
}

func (r *announcementRepository) CountReachableGuardiansForStudents(ctx context.Context, tenantID int64, studentIDs []int64) (int, error) {
	return r.AnnouncementAudienceReads.CountReachableGuardiansForStudents(ctx, tenantID, studentIDs)
}

func (r *announcementRepository) MarkRead(ctx context.Context, tenantID, announcementID, accountID int64, expectedPublishedAt time.Time) (bool, error) {
	return r.store.MarkRead(ctx, tenantID, announcementID, accountID, expectedPublishedAt)
}

func (r *announcementRepository) MarkAcknowledged(ctx context.Context, tenantID, announcementID, accountID int64, expectedPublishedAt time.Time) (bool, error) {
	return r.store.MarkAcknowledged(ctx, tenantID, announcementID, accountID, expectedPublishedAt)
}

func (r *announcementRepository) Stats(ctx context.Context, tenantID, announcementID int64) (*communication.PortalAnnouncementStats, error) {
	value, err := r.AnnouncementAudienceReads.Stats(ctx, tenantID, announcementID)
	if err != nil {
		return nil, err
	}
	return &communication.PortalAnnouncementStats{
		TargetCount: value.TargetCount, ReadCount: value.ReadCount, AcknowledgedCount: value.AcknowledgedCount,
	}, nil
}

// publishedError keeps the existing sentinel the announcement service maps to
// its published-immutable conflict.
func publishedError(err error) error {
	if errors.Is(err, domain.ErrParentAnnouncementPublished) {
		return domain.ErrParentAnnouncementPublished
	}
	return err
}

func feedScope(scope communication.PortalAnnouncementFeedScope) communication.PortalAnnouncementFeedScope {
	return communication.PortalAnnouncementFeedScope{TenantIDs: scope.TenantIDs, SystemOnlyTenantIDs: scope.SystemOnlyTenantIDs}
}

func feedTenantIDs(scope communication.PortalAnnouncementFeedScope) []int64 {
	ids := make([]int64, 0, len(scope.TenantIDs)+len(scope.SystemOnlyTenantIDs))
	ids = append(ids, scope.TenantIDs...)
	return append(ids, scope.SystemOnlyTenantIDs...)
}

func announcementValue(a *communication.PortalParentAnnouncement) *domain.ParentAnnouncement {
	if a == nil {
		return nil
	}
	return &domain.ParentAnnouncement{
		ID: a.ID, TenantID: a.TenantID, Title: a.Title, Body: a.Body, Priority: a.Priority,
		LinkURL: a.LinkURL, RequiresAcknowledgement: a.RequiresAcknowledgement, SendEmail: a.SendEmail,
		PublishedAt: a.PublishedAt, ExpiresAt: a.ExpiresAt, Active: a.Active, CreatedBy: a.CreatedBy,
		ResponseType: a.ResponseType, ResponseDeadline: a.ResponseDeadline,
		DeliveryMode: a.DeliveryMode, EmailAudience: a.EmailAudience, SystemKind: a.SystemKind,
		ReminderAt: a.ReminderAt, ReminderText: a.ReminderText, ReminderSentAt: a.ReminderSentAt,
		CreatedAt: a.CreatedAt, UpdatedAt: a.UpdatedAt, Declaration: domain.DeclarationSettings(a.Declaration),
	}
}

func announcementModel(value *domain.ParentAnnouncement) *communication.PortalParentAnnouncement {
	if value == nil {
		return nil
	}
	a := &communication.PortalParentAnnouncement{
		Title: value.Title, Body: value.Body, Priority: value.Priority, LinkURL: value.LinkURL,
		RequiresAcknowledgement: value.RequiresAcknowledgement, SendEmail: value.SendEmail,
		PublishedAt: value.PublishedAt, ExpiresAt: value.ExpiresAt, Active: value.Active,
		CreatedBy: value.CreatedBy, ResponseType: value.ResponseType, ResponseDeadline: value.ResponseDeadline,
		DeliveryMode: value.DeliveryMode, EmailAudience: value.EmailAudience, SystemKind: value.SystemKind,
		ReminderAt: value.ReminderAt, ReminderText: value.ReminderText, ReminderSentAt: value.ReminderSentAt,
		Declaration: communication.PortalAnnouncementDeclarationSettings(value.Declaration),
	}
	a.ID = value.ID
	a.CreatedAt = value.CreatedAt
	a.UpdatedAt = value.UpdatedAt
	a.TenantID = value.TenantID
	if value.Targets != nil {
		a.Targets = make([]*communication.PortalParentAnnouncementTarget, 0, len(value.Targets))
		for _, target := range value.Targets {
			a.Targets = append(a.Targets, targetModel(target))
		}
	}
	if value.Options != nil {
		a.Options = optionModels(value.Options)
	}
	stampAnnouncementKind(a)
	return a
}

func targetModel(value *domain.ParentAnnouncementTarget) *communication.PortalParentAnnouncementTarget {
	target := &communication.PortalParentAnnouncementTarget{
		ID: value.ID, AnnouncementID: value.AnnouncementID, TargetType: value.TargetType,
		TargetRefID: value.TargetRefID, TargetRefText: value.TargetRefText, CreatedAt: value.CreatedAt,
	}
	target.TenantID = value.TenantID
	return target
}

func optionModels(values []*domain.ParentAnnouncementOption) []*communication.PortalParentAnnouncementOption {
	if values == nil {
		return nil
	}
	options := make([]*communication.PortalParentAnnouncementOption, 0, len(values))
	for _, value := range values {
		option := &communication.PortalParentAnnouncementOption{
			ID: value.ID, AnnouncementID: value.AnnouncementID, Label: value.Label,
			Position: value.Position, CreatedAt: value.CreatedAt,
		}
		option.TenantID = value.TenantID
		options = append(options, option)
	}
	return options
}

func reminderModels(values []*communication.PortalAnnouncementPollReminderRecipient) []*communication.PortalAnnouncementPollReminderRecipient {
	if values == nil {
		return nil
	}
	rows := make([]*communication.PortalAnnouncementPollReminderRecipient, 0, len(values))
	for _, value := range values {
		rows = append(rows, &communication.PortalAnnouncementPollReminderRecipient{
			AccountID: value.AccountID, Email: value.Email, FirstName: value.FirstName,
			LastName: value.LastName, PortalLocale: value.PortalLocale,
		})
	}
	return rows
}

var _ recordports.ParentAnnouncementRepository = (*announcementRepository)(nil)

func stampAnnouncementKind(a *communication.PortalParentAnnouncement) {
	if a == nil {
		return
	}
	a.Poll = domain.AnnouncementResponseIsPoll(a.ResponseType)
	a.DeclarationMode = domain.AnnouncementIsDeclaration(a.DeliveryMode)
}
