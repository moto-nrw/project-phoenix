package application

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/communication"
)

type parentAnnouncementDeliveryAdapter struct {
	ParentAnnouncementDeliveryRecorder
}

func (a parentAnnouncementDeliveryAdapter) ReplaceForEntity(ctx context.Context, tenantID int64, relatedType string, relatedID int64, rows []EmailDelivery) error {
	if a.ParentAnnouncementDeliveryRecorder == nil {
		return nil
	}
	values := make([]communication.ParentAnnouncementEmailDelivery, 0, len(rows))
	for _, row := range rows {
		values = append(values, communication.ParentAnnouncementEmailDelivery{
			OutboxID: row.OutboxID, GuardianProfileID: row.GuardianProfileID, AccountID: row.AccountID,
			RecipientEmail: row.RecipientEmail, Reachability: row.Reachability,
		})
	}
	return a.ParentAnnouncementDeliveryRecorder.ReplaceForEntity(ctx, tenantID, relatedType, relatedID, values)
}

func (a parentAnnouncementDeliveryAdapter) ListForEntity(ctx context.Context, tenantID int64, relatedType string, relatedID int64) ([]EmailDeliveryStatus, error) {
	if a.ParentAnnouncementDeliveryRecorder == nil {
		return nil, nil
	}
	rows, err := a.ParentAnnouncementDeliveryRecorder.ListForEntity(ctx, tenantID, relatedType, relatedID)
	if err != nil {
		return nil, err
	}
	result := make([]EmailDeliveryStatus, 0, len(rows))
	for _, row := range rows {
		result = append(result, EmailDeliveryStatus{
			DeliveryID: row.DeliveryID, GuardianProfileID: row.GuardianProfileID, AccountID: row.AccountID,
			FirstName: row.FirstName, LastName: row.LastName, RecipientEmail: row.RecipientEmail,
			Reachability: row.Reachability, EmailStatus: row.EmailStatus, LastError: row.LastError,
			SentAt: row.SentAt, Attempts: row.Attempts,
		})
	}
	return result, nil
}

// parentAnnouncements promotes the service's retention unchanged.
type parentAnnouncements struct {
	communication.ParentAnnouncementRetention
	service StaffService
}

func (p *parentAnnouncements) ListParentAnnouncements(ctx context.Context, includeInactive bool) ([]communication.ParentAnnouncement, error) {
	rows, err := p.service.List(ctx, includeInactive)
	if err != nil {
		return nil, mapParentAnnouncementError(err)
	}
	result := make([]communication.ParentAnnouncement, 0, len(rows))
	for _, row := range rows {
		result = append(result, *mapParentAnnouncement(row))
	}
	return result, nil
}

func (p *parentAnnouncements) GetParentAnnouncement(ctx context.Context, id int64) (*communication.ParentAnnouncement, error) {
	row, err := p.service.Get(ctx, id)
	if err != nil {
		return nil, mapParentAnnouncementError(err)
	}
	announcement := mapParentAnnouncement(row)
	if row.IsDeclaration() {
		frozen, err := p.service.DeclarationFrozen(ctx, row)
		if err != nil {
			return nil, err
		}
		announcement.DeclarationFrozen = frozen
	}
	return announcement, nil
}

func (p *parentAnnouncements) CreateParentAnnouncement(ctx context.Context, createdBy int64, input communication.ParentAnnouncementInput) (*communication.ParentAnnouncement, error) {
	row, err := p.service.Create(ctx, createdBy, mapParentAnnouncementInput(input))
	return mapParentAnnouncement(row), mapParentAnnouncementError(err)
}

func (p *parentAnnouncements) UpdateParentAnnouncement(ctx context.Context, id int64, input communication.ParentAnnouncementInput) (*communication.ParentAnnouncement, error) {
	row, err := p.service.Update(ctx, id, mapParentAnnouncementInput(input))
	return mapParentAnnouncement(row), mapParentAnnouncementError(err)
}

func (p *parentAnnouncements) DeleteParentAnnouncement(ctx context.Context, id int64) error {
	return mapParentAnnouncementError(p.service.Delete(ctx, id))
}

func (p *parentAnnouncements) PublishParentAnnouncement(ctx context.Context, id int64) (*communication.ParentAnnouncement, error) {
	row, err := p.service.Publish(ctx, id)
	return mapParentAnnouncement(row), mapParentAnnouncementError(err)
}

func (p *parentAnnouncements) UnpublishParentAnnouncement(ctx context.Context, id int64) (*communication.ParentAnnouncement, error) {
	row, err := p.service.Unpublish(ctx, id)
	return mapParentAnnouncement(row), mapParentAnnouncementError(err)
}

func (p *parentAnnouncements) ParentAnnouncementStats(ctx context.Context, id int64) (*communication.ParentAnnouncementStats, error) {
	stats, err := p.service.Stats(ctx, id)
	if stats == nil {
		return nil, mapParentAnnouncementError(err)
	}
	return &communication.ParentAnnouncementStats{
		TargetCount: stats.TargetCount, ReadCount: stats.ReadCount, AcknowledgedCount: stats.AcknowledgedCount,
	}, mapParentAnnouncementError(err)
}

func (p *parentAnnouncements) ParentAnnouncementRecipients(ctx context.Context, id int64) ([]communication.ParentAnnouncementRecipient, error) {
	rows, err := p.service.Recipients(ctx, id)
	if err != nil {
		return nil, mapParentAnnouncementError(err)
	}
	result := make([]communication.ParentAnnouncementRecipient, 0, len(rows))
	for _, row := range rows {
		result = append(result, communication.ParentAnnouncementRecipient{
			AccountID: row.AccountID, FirstName: row.FirstName, LastName: row.LastName,
			ReadAt: row.ReadAt, AcknowledgedAt: row.AcknowledgedAt,
		})
	}
	return result, nil
}

func (p *parentAnnouncements) ParentAnnouncementPollResults(ctx context.Context, id int64) (*communication.ParentAnnouncementPollResults, error) {
	rows, err := p.service.PollResults(ctx, id)
	if rows == nil {
		return nil, mapParentAnnouncementError(err)
	}
	result := &communication.ParentAnnouncementPollResults{
		ChildCount: rows.ChildCount, TargetChildCount: rows.TargetChildCount, AnsweredCount: rows.AnsweredCount,
		Options: make([]communication.ParentAnnouncementPollOption, 0, len(rows.Options)),
	}
	for _, option := range rows.Options {
		result.Options = append(result.Options, communication.ParentAnnouncementPollOption{
			OptionID: option.OptionID, Label: option.Label, Count: option.Count,
		})
	}
	return result, mapParentAnnouncementError(err)
}

func (p *parentAnnouncements) ParentAnnouncementPollChildren(ctx context.Context, id int64) ([]communication.ParentAnnouncementPollChild, error) {
	rows, err := p.service.PollChildren(ctx, id)
	if err != nil {
		return nil, mapParentAnnouncementError(err)
	}
	result := make([]communication.ParentAnnouncementPollChild, 0, len(rows))
	for _, row := range rows {
		result = append(result, communication.ParentAnnouncementPollChild{
			StudentID: row.StudentID, FirstName: row.FirstName, LastName: row.LastName,
			SchoolClass: row.SchoolClass, AnswerLabels: row.AnswerLabels,
			RespondedAt: row.RespondedAt, CanAnswer: row.CanAnswer,
		})
	}
	return result, nil
}

func (p *parentAnnouncements) RemindParentAnnouncement(ctx context.Context, id int64) (int, error) {
	count, err := p.service.RemindOutstanding(ctx, id)
	return count, mapParentAnnouncementError(err)
}

func (p *parentAnnouncements) ParentAnnouncementLetterStatus(ctx context.Context, id int64) (*communication.ParentAnnouncementLetterStatus, error) {
	status, err := p.service.LetterStatus(ctx, id)
	if status == nil {
		return nil, mapParentAnnouncementError(err)
	}
	result := &communication.ParentAnnouncementLetterStatus{
		Recipients: make([]communication.ParentAnnouncementLetterRecipient, 0, len(status.Recipients)),
		Children:   make([]communication.ParentAnnouncementLetterChild, 0, len(status.Children)),
		Summary: communication.ParentAnnouncementLetterSummary{
			ChildrenTotal: status.Summary.ChildrenTotal, ChildrenConfirmable: status.Summary.ChildrenConfirmable,
			ChildrenFulfilled: status.Summary.ChildrenFulfilled, ChildrenOpen: status.Summary.ChildrenOpen,
			ChildrenWithoutPortal: status.Summary.ChildrenWithoutPortal, RecipientsTotal: status.Summary.RecipientsTotal,
			EmailsSent: status.Summary.EmailsSent, EmailsPending: status.Summary.EmailsPending,
			EmailsFailed: status.Summary.EmailsFailed, WithoutEmail: status.Summary.WithoutEmail,
			WithoutPortal: status.Summary.WithoutPortal,
		},
	}
	for _, recipient := range status.Recipients {
		result.Recipients = append(result.Recipients, communication.ParentAnnouncementLetterRecipient{
			FirstName: recipient.FirstName, LastName: recipient.LastName, RecipientEmail: recipient.RecipientEmail,
			EmailStatus: recipient.EmailStatus, Reachability: recipient.Reachability,
			LastError: recipient.LastError, SentAt: recipient.SentAt,
		})
	}
	for _, child := range status.Children {
		result.Children = append(result.Children, communication.ParentAnnouncementLetterChild{
			StudentID: child.StudentID, FirstName: child.FirstName, LastName: child.LastName,
			SchoolClass: child.SchoolClass, CanConfirm: child.CanConfirm,
			AcknowledgedAt: child.AcknowledgedAt, AckFirstName: child.AckFirstName, AckLastName: child.AckLastName,
		})
	}
	return result, mapParentAnnouncementError(err)
}

func (p *parentAnnouncements) ResendParentAnnouncementEmails(ctx context.Context, id int64) (int, error) {
	count, err := p.service.ResendFailedEmails(ctx, id)
	return count, mapParentAnnouncementError(err)
}

func (p *parentAnnouncements) UpdateParentAnnouncementReminder(ctx context.Context, id int64, input communication.ParentAnnouncementReminderInput) (*communication.ParentAnnouncement, error) {
	row, err := p.service.UpdateReminder(ctx, id, ReminderInput{ReminderAt: input.ReminderAt, ReminderText: input.ReminderText})
	return mapParentAnnouncement(row), mapParentAnnouncementError(err)
}

func (p *parentAnnouncements) SendDueParentAnnouncementReminders(ctx context.Context, notBefore, dueBefore time.Time) (int, *time.Time, error) {
	count, retryFrom, err := p.service.SendDueReminders(ctx, notBefore, dueBefore)
	return count, retryFrom, mapParentAnnouncementError(err)
}

func (p *parentAnnouncements) PublishCareCancellation(ctx context.Context, input communication.CareCancellationInput) (*communication.CareCancellationResult, error) {
	result, err := p.service.PublishCareCancellation(ctx, CareCancellationInput{
		StudentIDs: input.StudentIDs, Title: input.Title, Body: input.Body, CreatedBy: input.CreatedBy,
	})
	if result == nil {
		return nil, mapParentAnnouncementError(err)
	}
	return &communication.CareCancellationResult{
		AnnouncementID: result.Announcement.ID, RecipientCount: result.RecipientCount,
	}, mapParentAnnouncementError(err)
}

func (p *parentAnnouncements) CareCancellationReachFor(ctx context.Context, studentIDs []int64) (*communication.CareCancellationReach, error) {
	reach, err := p.service.CareCancellationReachFor(ctx, studentIDs)
	if reach == nil {
		return nil, mapParentAnnouncementError(err)
	}
	return &communication.CareCancellationReach{FamilyCount: reach.FamilyCount}, mapParentAnnouncementError(err)
}

func (p *parentAnnouncements) AnnouncementExists(ctx context.Context, id int64) (bool, error) {
	exists, err := p.service.AnnouncementExists(ctx, id)
	return exists, mapParentAnnouncementError(err)
}

func (p *parentAnnouncements) AnnouncementEditable(ctx context.Context, id int64) (bool, error) {
	editable, err := p.service.AnnouncementEditable(ctx, id)
	return editable, mapParentAnnouncementError(err)
}

func (p *parentAnnouncements) LockAnnouncementForAttachmentChange(ctx context.Context, id int64) (bool, bool, error) {
	exists, editable, err := p.service.LockAnnouncementForAttachmentChange(ctx, id)
	return exists, editable, mapParentAnnouncementError(err)
}

func (p *parentAnnouncements) ResetAnnouncementEngagement(ctx context.Context, id int64) error {
	return mapParentAnnouncementError(p.service.ResetAnnouncementEngagement(ctx, id))
}

func mapParentAnnouncementInput(input communication.ParentAnnouncementInput) Input {
	targets := make([]TargetInput, 0, len(input.Targets))
	for _, target := range input.Targets {
		targets = append(targets, TargetInput{TargetType: target.TargetType, RefID: target.RefID, RefText: target.RefText})
	}
	return Input{
		Title: input.Title, Body: input.Body, Priority: input.Priority, LinkURL: input.LinkURL,
		RequiresAcknowledgement: input.RequiresAcknowledgement, SendEmail: input.SendEmail,
		ExpiresAt: input.ExpiresAt, Targets: targets, ResponseType: input.ResponseType,
		ResponseDeadline: input.ResponseDeadline, Options: input.Options,
		DeliveryMode: input.DeliveryMode, EmailAudience: input.EmailAudience,
		ReminderAt: input.ReminderAt, ReminderText: input.ReminderText,
		Declaration: StaffAnnouncementDeclarationSettings(input.Declaration),
		Sender:      (*ClassLetterSender)(input.Sender),
	}
}

func mapParentAnnouncement(row *StaffParentAnnouncement) *communication.ParentAnnouncement {
	if row == nil {
		return nil
	}
	targets := make([]communication.ParentAnnouncementTarget, 0, len(row.Targets))
	for _, target := range row.Targets {
		targets = append(targets, communication.ParentAnnouncementTarget{
			TargetType: target.TargetType, RefID: target.TargetRefID, RefText: target.TargetRefText,
		})
	}
	options := make([]communication.ParentAnnouncementOption, 0, len(row.Options))
	for _, option := range row.Options {
		options = append(options, communication.ParentAnnouncementOption{ID: option.ID, Label: option.Label})
	}
	return &communication.ParentAnnouncement{
		ID: row.ID, Title: row.Title, Body: row.Body, Priority: row.Priority, LinkURL: row.LinkURL,
		RequiresAcknowledgement: row.RequiresAcknowledgement, SendEmail: row.SendEmail,
		PublishedAt: row.PublishedAt, ExpiresAt: row.ExpiresAt, Active: row.Active,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, Targets: targets,
		ResponseType: row.ResponseType, ResponseDeadline: row.ResponseDeadline, Options: options,
		DeliveryMode: row.DeliveryMode, EmailAudience: row.EmailAudience, SystemKind: row.SystemKind,
		ReminderAt: row.ReminderAt, ReminderText: row.ReminderText, ReminderSentAt: row.ReminderSentAt,
		Declaration: communication.ParentDeclarationSettings(row.Declaration),
		CreatedBy:   row.CreatedBy, Sender: (*communication.ParentLetterSender)(classLetterSenderOf(row)),
	}
}

var _ communication.ParentAnnouncementCapability = (*parentAnnouncements)(nil)

type mappedParentAnnouncementError struct {
	public   error
	original error
}

func (e *mappedParentAnnouncementError) Error() string   { return e.original.Error() }
func (e *mappedParentAnnouncementError) Unwrap() []error { return []error{e.public, e.original} }

func mapParentAnnouncementError(err error) error {
	if err == nil {
		return nil
	}
	for _, pair := range []struct{ internal, public error }{
		{ErrNotFound, communication.ErrParentAnnouncementNotFound},
		{ErrValidation, communication.ErrParentAnnouncementValidation},
		{ErrNewsDisabled, communication.ErrParentNewsDisabled},
		{ErrSystemAnnouncementImmutable, communication.ErrSystemParentAnnouncementImmutable},
		{ErrPublishedImmutable, communication.ErrPublishedParentAnnouncement},
		{ErrNotPublished, communication.ErrParentAnnouncementNotPublished},
		{ErrNothingOutstanding, communication.ErrParentAnnouncementNothingDue},
		{ErrNotAPoll, communication.ErrParentAnnouncementNotPoll},
		{ErrPollNotOpen, communication.ErrParentAnnouncementPollClosed},
		{ErrReminderAlreadySent, communication.ErrParentAnnouncementReminderSent},
		{ErrDeclarationHasSubmissions, communication.ErrDeclarationHasSubmissions},
		{ErrNotDeclaration, communication.ErrNotDeclaration},
		{ErrClassLetterReadOnly, communication.ErrClassLetterReadOnly},
	} {
		if errors.Is(err, pair.internal) {
			return &mappedParentAnnouncementError{public: pair.public, original: err}
		}
	}
	return err
}

func NewParentAnnouncementFacade(service StaffService) communication.ParentAnnouncementCapability {
	return &parentAnnouncements{ParentAnnouncementRetention: service, service: service}
}
func NewParentAnnouncementDeliveryAdapter(recorder ParentAnnouncementDeliveryRecorder) DeliveryRecorder {
	return parentAnnouncementDeliveryAdapter{ParentAnnouncementDeliveryRecorder: recorder}
}

type ParentAnnouncementDeliveryRecorder interface {
	ReplaceForEntity(context.Context, int64, string, int64, []communication.ParentAnnouncementEmailDelivery) error
	DeleteForEntity(context.Context, int64, string, int64) (int64, error)
	ListForEntity(context.Context, int64, string, int64) ([]communication.ParentAnnouncementEmailDeliveryStatus, error)
	AttachOutbox(context.Context, int64, int64, int64) error
	ClaimFailedDelivery(context.Context, int64, int64) (bool, error)
}
