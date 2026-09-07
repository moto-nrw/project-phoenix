package parentstore

import (
	"context"
	"errors"
	"time"

	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/communication/internal/adapters/parentaudience"
	"github.com/moto-nrw/project-phoenix/modules/communication/internal/adapters/parentpostgres"
	"github.com/moto-nrw/project-phoenix/modules/communication/internal/domain"
	"github.com/uptrace/bun"
)

// PendingAnnouncementApplicant is one undecided enrollment application the
// pending_enrollment target reaches. Enrollment owns those rows; the
// composition root converts them so Communication never reads its tables.
type PendingAnnouncementApplicant = domain.PendingAnnouncementApplicant

// PendingApplicantSource supplies Enrollment's undecided applications for
// the caller's school and for an explicit school set.
type PendingApplicantSource interface {
	PendingAnnouncementApplicants(context.Context) ([]PendingAnnouncementApplicant, error)
	PendingAnnouncementApplicantsForSchools(context.Context, []int64) ([]PendingAnnouncementApplicant, error)
}

// NewParentAnnouncementRepository returns the announcement store and audience
// projection behind the existing repository contract. The optional clock
// fixes the calendar day activity-group targets are evaluated on.
//
// SchoolName is not served here: the school's name belongs to Organisation
// and Tenancy, and the composition root decorates it exactly as before.
func NewParentAnnouncementRepository(db *bun.DB, applicants PendingApplicantSource, clocks ...func() time.Time) usersModels.ParentAnnouncementRepository {
	if applicants == nil {
		panic("communication parent store: pending applicant source is required")
	}
	return &announcementRepository{
		store:    parentpostgres.NewAnnouncementStore(postgresDatabase(db)),
		audience: parentaudience.New(audienceDatabase(db), applicants, clocks...),
	}
}

func audienceDatabase(db *bun.DB) parentaudience.Database {
	return func(ctx context.Context) (bun.IDB, int64, error) { return resolve(ctx, db) }
}

type announcementRepository struct {
	store    *parentpostgres.AnnouncementStore
	audience *parentaudience.Projection
}

func (r *announcementRepository) Create(ctx context.Context, a *usersModels.ParentAnnouncement) error {
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
	a.SetTenantID(value.TenantID)
	return nil
}

func (r *announcementRepository) Update(ctx context.Context, a *usersModels.ParentAnnouncement) error {
	value := announcementValue(a)
	if err := r.store.UpdateDraft(ctx, value); err != nil {
		return publishedError(err)
	}
	a.UpdatedAt = value.UpdatedAt
	return nil
}

func (r *announcementRepository) FindByID(ctx context.Context, id int64) (*usersModels.ParentAnnouncement, error) {
	value, err := r.store.FindByID(ctx, id)
	return announcementModel(value), err
}

func (r *announcementRepository) FindByIDForUpdate(ctx context.Context, id int64) (*usersModels.ParentAnnouncement, error) {
	value, err := r.store.FindByIDForUpdate(ctx, id)
	return announcementModel(value), err
}

func (r *announcementRepository) Delete(ctx context.Context, id int64) error {
	return r.store.Delete(ctx, id)
}

func (r *announcementRepository) ListForTenant(ctx context.Context, includeInactive bool) ([]*usersModels.ParentAnnouncement, error) {
	values, err := r.store.ListForTenant(ctx, includeInactive)
	if err != nil {
		return nil, err
	}
	rows := make([]*usersModels.ParentAnnouncement, 0, len(values))
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

func (r *announcementRepository) ReplaceTargets(ctx context.Context, tenantID, announcementID int64, targets []*usersModels.ParentAnnouncementTarget) error {
	values := make([]*domain.ParentAnnouncementTarget, 0, len(targets))
	for _, target := range targets {
		values = append(values, &domain.ParentAnnouncementTarget{
			ID: target.ID, TenantID: target.GetTenantID(), AnnouncementID: target.AnnouncementID,
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
		targets[i].SetTenantID(value.TenantID)
	}
	return nil
}

func (r *announcementRepository) ListTargets(ctx context.Context, announcementID int64) ([]*usersModels.ParentAnnouncementTarget, error) {
	values, err := r.store.ListTargets(ctx, announcementID)
	if err != nil {
		return nil, err
	}
	rows := make([]*usersModels.ParentAnnouncementTarget, 0, len(values))
	for _, value := range values {
		rows = append(rows, targetModel(value))
	}
	return rows, nil
}

func (r *announcementRepository) ReplaceOptions(ctx context.Context, tenantID, announcementID int64, options []*usersModels.ParentAnnouncementOption) error {
	values := make([]*domain.ParentAnnouncementOption, 0, len(options))
	for _, option := range options {
		values = append(values, &domain.ParentAnnouncementOption{
			ID: option.ID, TenantID: option.GetTenantID(), AnnouncementID: option.AnnouncementID,
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
		options[i].SetTenantID(value.TenantID)
	}
	return nil
}

func (r *announcementRepository) ListOptions(ctx context.Context, announcementID int64) ([]*usersModels.ParentAnnouncementOption, error) {
	values, err := r.store.ListOptions(ctx, announcementID)
	return optionModels(values), err
}

func (r *announcementRepository) ListOptionsForAnnouncements(ctx context.Context, announcementIDs []int64) ([]*usersModels.ParentAnnouncementOption, error) {
	values, err := r.store.ListOptionsForAnnouncements(ctx, announcementIDs)
	return optionModels(values), err
}

func (r *announcementRepository) AnswerableChildren(ctx context.Context, accountID int64, announcementIDs []int64) ([]*usersModels.AnnouncementPollChild, error) {
	values, err := r.audience.AnswerableChildren(ctx, accountID, announcementIDs)
	if err != nil {
		return nil, err
	}
	rows := make([]*usersModels.AnnouncementPollChild, 0, len(values))
	for _, value := range values {
		rows = append(rows, &usersModels.AnnouncementPollChild{
			AnnouncementID: value.AnnouncementID, StudentID: value.StudentID, FirstName: value.FirstName,
			LastName: value.LastName, SelectedOptions: value.SelectedOptions,
		})
	}
	return rows, nil
}

func (r *announcementRepository) AccountMayAnswerForStudent(ctx context.Context, tenantID, announcementID, accountID, studentID int64) (bool, error) {
	return r.audience.AccountMayAnswerForStudent(ctx, tenantID, announcementID, accountID, studentID)
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
	allowed, err := r.audience.HoldAnswerPermission(ctx, tenantID, announcementID, accountID, studentID)
	if err != nil || !allowed {
		return false, err
	}
	live, err := r.store.DeleteResponse(ctx, tenantID, announcementID, studentID, optionIDs, expectedPublishedAt)
	if err != nil || !live || len(optionIDs) == 0 {
		return live, err
	}
	return r.store.InsertResponse(ctx, tenantID, announcementID, studentID, accountID, optionIDs, expectedPublishedAt)
}

func (r *announcementRepository) PollResults(ctx context.Context, tenantID, announcementID int64) (*usersModels.AnnouncementPollResults, error) {
	value, err := r.audience.PollResults(ctx, tenantID, announcementID)
	if err != nil {
		return nil, err
	}
	results := &usersModels.AnnouncementPollResults{
		ChildCount: value.ChildCount, TargetChildCount: value.TargetChildCount, AnsweredCount: value.AnsweredCount,
		Options: make([]*usersModels.AnnouncementPollOptionResult, 0, len(value.Options)),
	}
	for _, option := range value.Options {
		results.Options = append(results.Options, &usersModels.AnnouncementPollOptionResult{
			OptionID: option.OptionID, Label: option.Label, Position: option.Position, Count: option.Count,
		})
	}
	return results, nil
}

func (r *announcementRepository) PollChildren(ctx context.Context, tenantID, announcementID int64) ([]*usersModels.AnnouncementPollChildStatus, error) {
	values, err := r.audience.PollChildren(ctx, tenantID, announcementID)
	if err != nil {
		return nil, err
	}
	rows := make([]*usersModels.AnnouncementPollChildStatus, 0, len(values))
	for _, value := range values {
		rows = append(rows, &usersModels.AnnouncementPollChildStatus{
			StudentID: value.StudentID, FirstName: value.FirstName, LastName: value.LastName,
			SchoolClass: value.SchoolClass, AnswerLabels: value.AnswerLabels, RespondedAt: value.RespondedAt,
			CanAnswer: value.CanAnswer,
		})
	}
	return rows, nil
}

func (r *announcementRepository) UnansweredReminderRecipients(ctx context.Context, tenantID, announcementID int64) ([]*usersModels.AnnouncementPollReminderRecipient, error) {
	values, err := r.audience.UnansweredReminderRecipients(ctx, tenantID, announcementID)
	return reminderModels(values), err
}

func (r *announcementRepository) UnacknowledgedReminderRecipients(ctx context.Context, tenantID, announcementID int64) ([]*usersModels.AnnouncementPollReminderRecipient, error) {
	values, err := r.audience.UnacknowledgedReminderRecipients(ctx, tenantID, announcementID)
	return reminderModels(values), err
}

func (r *announcementRepository) CountAudience(ctx context.Context, tenantID, announcementID int64) (int, error) {
	return r.audience.CountAudience(ctx, tenantID, announcementID)
}

func (r *announcementRepository) AccountMatchesAnnouncement(ctx context.Context, tenantID, announcementID, accountID int64) (bool, error) {
	return r.audience.AccountMatchesAnnouncement(ctx, tenantID, announcementID, accountID)
}

func (r *announcementRepository) ResolveAudienceEmails(ctx context.Context, tenantID, announcementID int64) ([]*usersModels.AnnouncementRecipient, error) {
	values, err := r.audience.ResolveAudienceEmails(ctx, tenantID, announcementID)
	if err != nil {
		return nil, err
	}
	rows := make([]*usersModels.AnnouncementRecipient, 0, len(values))
	for _, value := range values {
		rows = append(rows, &usersModels.AnnouncementRecipient{
			AccountID: value.AccountID, Email: value.Email, FirstName: value.FirstName, LastName: value.LastName,
		})
	}
	return rows, nil
}

func (r *announcementRepository) LetterChildStatuses(ctx context.Context, tenantID, announcementID int64) ([]*usersModels.AnnouncementLetterChildStatus, error) {
	values, err := r.audience.LetterChildStatuses(ctx, tenantID, announcementID)
	if err != nil {
		return nil, err
	}
	rows := make([]*usersModels.AnnouncementLetterChildStatus, 0, len(values))
	for _, value := range values {
		rows = append(rows, &usersModels.AnnouncementLetterChildStatus{
			StudentID: value.StudentID, FirstName: value.FirstName, LastName: value.LastName,
			SchoolClass: value.SchoolClass, CanConfirm: value.CanConfirm, AcknowledgedAt: value.AcknowledgedAt,
			AckFirstName: value.AckFirstName, AckLastName: value.AckLastName,
		})
	}
	return rows, nil
}

func (r *announcementRepository) ResolveDeliveryRecipients(ctx context.Context, tenantID, announcementID int64) ([]*usersModels.AnnouncementDeliveryRecipient, error) {
	values, err := r.audience.ResolveDeliveryRecipients(ctx, tenantID, announcementID)
	if err != nil {
		return nil, err
	}
	rows := make([]*usersModels.AnnouncementDeliveryRecipient, 0, len(values))
	for _, value := range values {
		rows = append(rows, &usersModels.AnnouncementDeliveryRecipient{
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

func (r *announcementRepository) AudienceRecipients(ctx context.Context, tenantID, announcementID int64) ([]*usersModels.AnnouncementRecipientStatus, error) {
	values, err := r.audience.AudienceRecipients(ctx, tenantID, announcementID)
	if err != nil {
		return nil, err
	}
	rows := make([]*usersModels.AnnouncementRecipientStatus, 0, len(values))
	for _, value := range values {
		rows = append(rows, &usersModels.AnnouncementRecipientStatus{
			AccountID: value.AccountID, FirstName: value.FirstName, LastName: value.LastName,
			PortalLocale: value.PortalLocale, ReadAt: value.ReadAt, AcknowledgedAt: value.AcknowledgedAt,
		})
	}
	return rows, nil
}

func (r *announcementRepository) ListFeedForAccount(ctx context.Context, accountID int64, scope usersModels.AnnouncementFeedScope) ([]*usersModels.AnnouncementFeedItem, error) {
	values, err := r.audience.ListFeedForAccount(ctx, accountID, feedScope(scope))
	if err != nil {
		return nil, err
	}
	rows := make([]*usersModels.AnnouncementFeedItem, 0, len(values))
	for _, value := range values {
		rows = append(rows, &usersModels.AnnouncementFeedItem{
			ID: value.ID, TenantID: value.TenantID, Title: value.Title, Body: value.Body, Priority: value.Priority,
			LinkURL: value.LinkURL, RequiresAcknowledgement: value.RequiresAcknowledgement,
			PublishedAt: value.PublishedAt, ExpiresAt: value.ExpiresAt, ResponseType: value.ResponseType,
			DeliveryMode: value.DeliveryMode, ResponseDeadline: value.ResponseDeadline, SystemKind: value.SystemKind,
			ReadAt: value.ReadAt, AcknowledgedAt: value.AcknowledgedAt,
		})
	}
	return rows, nil
}

func (r *announcementRepository) CountUnreadForAccount(ctx context.Context, accountID int64, scope usersModels.AnnouncementFeedScope) (int, error) {
	return r.audience.CountOutstandingForAccount(ctx, accountID, feedScope(scope))
}

func (r *announcementRepository) CountReachableGuardiansForStudents(ctx context.Context, tenantID int64, studentIDs []int64) (int, error) {
	return r.audience.CountReachableGuardiansForStudents(ctx, tenantID, studentIDs)
}

func (r *announcementRepository) MarkRead(ctx context.Context, tenantID, announcementID, accountID int64, expectedPublishedAt time.Time) (bool, error) {
	return r.store.MarkRead(ctx, tenantID, announcementID, accountID, expectedPublishedAt)
}

func (r *announcementRepository) MarkAcknowledged(ctx context.Context, tenantID, announcementID, accountID int64, expectedPublishedAt time.Time) (bool, error) {
	return r.store.MarkAcknowledged(ctx, tenantID, announcementID, accountID, expectedPublishedAt)
}

func (r *announcementRepository) Stats(ctx context.Context, tenantID, announcementID int64) (*usersModels.AnnouncementStats, error) {
	value, err := r.audience.Stats(ctx, tenantID, announcementID)
	if err != nil {
		return nil, err
	}
	return &usersModels.AnnouncementStats{
		TargetCount: value.TargetCount, ReadCount: value.ReadCount, AcknowledgedCount: value.AcknowledgedCount,
	}, nil
}

// publishedError keeps the existing sentinel the announcement service maps to
// its published-immutable conflict.
func publishedError(err error) error {
	if errors.Is(err, domain.ErrParentAnnouncementPublished) {
		return usersModels.ErrAnnouncementPublished
	}
	return err
}

func feedScope(scope usersModels.AnnouncementFeedScope) domain.ParentAnnouncementFeedScope {
	return domain.ParentAnnouncementFeedScope{TenantIDs: scope.TenantIDs, SystemOnlyTenantIDs: scope.SystemOnlyTenantIDs}
}

func announcementValue(a *usersModels.ParentAnnouncement) *domain.ParentAnnouncement {
	if a == nil {
		return nil
	}
	return &domain.ParentAnnouncement{
		ID: a.ID, TenantID: a.GetTenantID(), Title: a.Title, Body: a.Body, Priority: a.Priority,
		LinkURL: a.LinkURL, RequiresAcknowledgement: a.RequiresAcknowledgement, SendEmail: a.SendEmail,
		PublishedAt: a.PublishedAt, ExpiresAt: a.ExpiresAt, Active: a.Active, CreatedBy: a.CreatedBy,
		ResponseType: a.ResponseType, ResponseDeadline: a.ResponseDeadline,
		DeliveryMode: a.DeliveryMode, EmailAudience: a.EmailAudience, SystemKind: a.SystemKind,
		CreatedAt: a.CreatedAt, UpdatedAt: a.UpdatedAt,
	}
}

func announcementModel(value *domain.ParentAnnouncement) *usersModels.ParentAnnouncement {
	if value == nil {
		return nil
	}
	a := &usersModels.ParentAnnouncement{
		Title: value.Title, Body: value.Body, Priority: value.Priority, LinkURL: value.LinkURL,
		RequiresAcknowledgement: value.RequiresAcknowledgement, SendEmail: value.SendEmail,
		PublishedAt: value.PublishedAt, ExpiresAt: value.ExpiresAt, Active: value.Active,
		CreatedBy: value.CreatedBy, ResponseType: value.ResponseType, ResponseDeadline: value.ResponseDeadline,
		DeliveryMode: value.DeliveryMode, EmailAudience: value.EmailAudience, SystemKind: value.SystemKind,
	}
	a.ID = value.ID
	a.CreatedAt = value.CreatedAt
	a.UpdatedAt = value.UpdatedAt
	a.SetTenantID(value.TenantID)
	if value.Targets != nil {
		a.Targets = make([]*usersModels.ParentAnnouncementTarget, 0, len(value.Targets))
		for _, target := range value.Targets {
			a.Targets = append(a.Targets, targetModel(target))
		}
	}
	if value.Options != nil {
		a.Options = optionModels(value.Options)
	}
	return a
}

func targetModel(value *domain.ParentAnnouncementTarget) *usersModels.ParentAnnouncementTarget {
	target := &usersModels.ParentAnnouncementTarget{
		ID: value.ID, AnnouncementID: value.AnnouncementID, TargetType: value.TargetType,
		TargetRefID: value.TargetRefID, TargetRefText: value.TargetRefText, CreatedAt: value.CreatedAt,
	}
	target.SetTenantID(value.TenantID)
	return target
}

func optionModels(values []*domain.ParentAnnouncementOption) []*usersModels.ParentAnnouncementOption {
	if values == nil {
		return nil
	}
	options := make([]*usersModels.ParentAnnouncementOption, 0, len(values))
	for _, value := range values {
		option := &usersModels.ParentAnnouncementOption{
			ID: value.ID, AnnouncementID: value.AnnouncementID, Label: value.Label,
			Position: value.Position, CreatedAt: value.CreatedAt,
		}
		option.SetTenantID(value.TenantID)
		options = append(options, option)
	}
	return options
}

func reminderModels(values []*domain.ParentAnnouncementReminderRecipient) []*usersModels.AnnouncementPollReminderRecipient {
	if values == nil {
		return nil
	}
	rows := make([]*usersModels.AnnouncementPollReminderRecipient, 0, len(values))
	for _, value := range values {
		rows = append(rows, &usersModels.AnnouncementPollReminderRecipient{
			AccountID: value.AccountID, Email: value.Email, FirstName: value.FirstName,
			LastName: value.LastName, PortalLocale: value.PortalLocale,
		})
	}
	return rows
}

var _ usersModels.ParentAnnouncementRepository = (*announcementRepository)(nil)
