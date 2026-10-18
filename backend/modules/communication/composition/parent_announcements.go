package compose

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/communication"
	deliveryPorts "github.com/moto-nrw/project-phoenix/modules/communication/internal/application"
	staff "github.com/moto-nrw/project-phoenix/modules/communication/internal/staffannouncements"
)

// ParentAnnouncementConfig is the compatibility composition seam for the
// users.parent_* persistence that moves in #2675. Callers receive only the
// Communication public capability.
type ParentAnnouncementOutbox interface{ deliveryPorts.MailOutbox }

type ParentAnnouncementPushOutbox interface {
	CancelPendingByRelatedEntity(context.Context, string, int64, string) (int64, error)
}

type ParentAnnouncementDeliveryRecorder interface {
	ReplaceForEntity(context.Context, int64, string, int64, []communication.ParentAnnouncementEmailDelivery) error
	DeleteForEntity(context.Context, int64, string, int64) (int64, error)
	ListForEntity(context.Context, int64, string, int64) ([]communication.ParentAnnouncementEmailDeliveryStatus, error)
	AttachOutbox(context.Context, int64, int64, int64) error
	ClaimFailedDelivery(context.Context, int64, int64) (bool, error)
}

type ParentAnnouncementConfig struct {
	Repo             deliveryPorts.StaffAnnouncementSource
	Settings         staff.Settings
	Outbox           ParentAnnouncementOutbox
	PushOutbox       ParentAnnouncementPushOutbox
	Notifier         func(context.Context, communication.NotificationEvent) error
	ReminderNotifier func(context.Context, communication.NotificationEvent) (int, error)
	Preferences      deliveryPorts.NotificationPreferences
	Deliveries       ParentAnnouncementDeliveryRecorder
	Fingerprints     staff.Fingerprints
	ParentsURL       string
	Logger           *slog.Logger
}

type ParentAnnouncementEmailConfig communication.MailRendererConfig

// NewParentAnnouncementsWithAttachments composes the parent announcements
// over the file side of their attachments, which File Storage builds from
// its database and object store before it builds its attachment routes
// over these announcements (#4080). A missing file side is a construction
// error.
func NewParentAnnouncementsWithAttachments(cfg ParentAnnouncementConfig, attachments communication.ParentAnnouncementAttachmentPurger, runtime staff.Runtime, policy deliveryPorts.ParentDeliveryPolicy) (communication.ParentAnnouncementCapability, error) {
	if attachments == nil {
		return nil, errors.New("parent announcements compose: the attachment purger is required")
	}
	var deliveries staff.DeliveryRecorder
	if cfg.Deliveries != nil {
		deliveries = parentAnnouncementDeliveryAdapter{recorder: cfg.Deliveries}
	}
	return &parentAnnouncements{service: staff.NewService(staff.ServiceConfig{
		Repo: staff.NewRepository(cfg.Repo), Settings: cfg.Settings, Outbox: cfg.Outbox, PushOutbox: cfg.PushOutbox, Notifier: deliveryPorts.NewNotificationSender(cfg.Notifier),
		ReminderNotifier: deliveryPorts.NewDurableNotificationSender(cfg.ReminderNotifier),
		Preferences:      cfg.Preferences, Deliveries: deliveries,
		ParentsURL: cfg.ParentsURL, Logger: cfg.Logger, Fingerprints: cfg.Fingerprints, Attachments: attachments,
	}, runtime, policy)}, nil
}

func NewParentAnnouncementRenderer(cfg ParentAnnouncementEmailConfig) communication.MailRenderer {
	return communication.MailRenderer(staff.NewAnnouncementRenderer(staff.EmailConfig(cfg)))
}

type parentAnnouncementDeliveryAdapter struct {
	recorder ParentAnnouncementDeliveryRecorder
}

func (a parentAnnouncementDeliveryAdapter) ReplaceForEntity(ctx context.Context, tenantID int64, relatedType string, relatedID int64, rows []staff.EmailDelivery) error {
	if a.recorder == nil {
		return nil
	}
	values := make([]communication.ParentAnnouncementEmailDelivery, 0, len(rows))
	for _, row := range rows {
		values = append(values, communication.ParentAnnouncementEmailDelivery{
			OutboxID: row.OutboxID, GuardianProfileID: row.GuardianProfileID, AccountID: row.AccountID,
			RecipientEmail: row.RecipientEmail, Reachability: row.Reachability,
		})
	}
	return a.recorder.ReplaceForEntity(ctx, tenantID, relatedType, relatedID, values)
}

func (a parentAnnouncementDeliveryAdapter) DeleteForEntity(ctx context.Context, tenantID int64, relatedType string, relatedID int64) (int64, error) {
	if a.recorder == nil {
		return 0, nil
	}
	return a.recorder.DeleteForEntity(ctx, tenantID, relatedType, relatedID)
}

func (a parentAnnouncementDeliveryAdapter) ListForEntity(ctx context.Context, tenantID int64, relatedType string, relatedID int64) ([]staff.EmailDeliveryStatus, error) {
	if a.recorder == nil {
		return nil, nil
	}
	rows, err := a.recorder.ListForEntity(ctx, tenantID, relatedType, relatedID)
	if err != nil {
		return nil, err
	}
	result := make([]staff.EmailDeliveryStatus, 0, len(rows))
	for _, row := range rows {
		result = append(result, staff.EmailDeliveryStatus{
			DeliveryID: row.DeliveryID, GuardianProfileID: row.GuardianProfileID, AccountID: row.AccountID,
			FirstName: row.FirstName, LastName: row.LastName, RecipientEmail: row.RecipientEmail,
			Reachability: row.Reachability, EmailStatus: row.EmailStatus, LastError: row.LastError,
			SentAt: row.SentAt, Attempts: row.Attempts,
		})
	}
	return result, nil
}

func (a parentAnnouncementDeliveryAdapter) AttachOutbox(ctx context.Context, tenantID, deliveryID, outboxID int64) error {
	if a.recorder == nil {
		return nil
	}
	return a.recorder.AttachOutbox(ctx, tenantID, deliveryID, outboxID)
}

func (a parentAnnouncementDeliveryAdapter) ClaimFailedDelivery(ctx context.Context, tenantID, deliveryID int64) (bool, error) {
	if a.recorder == nil {
		return false, nil
	}
	return a.recorder.ClaimFailedDelivery(ctx, tenantID, deliveryID)
}

type parentAnnouncements struct{ service staff.Service }

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
	row, err := p.service.UpdateReminder(ctx, id, staff.ReminderInput{ReminderAt: input.ReminderAt, ReminderText: input.ReminderText})
	return mapParentAnnouncement(row), mapParentAnnouncementError(err)
}

func (p *parentAnnouncements) SendDueParentAnnouncementReminders(ctx context.Context, notBefore, dueBefore time.Time) (int, *time.Time, error) {
	count, retryFrom, err := p.service.SendDueReminders(ctx, notBefore, dueBefore)
	return count, retryFrom, mapParentAnnouncementError(err)
}

func (p *parentAnnouncements) PublishCareCancellation(ctx context.Context, input communication.CareCancellationInput) (*communication.CareCancellationResult, error) {
	result, err := p.service.PublishCareCancellation(ctx, staff.CareCancellationInput{
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

func mapParentAnnouncementInput(input communication.ParentAnnouncementInput) staff.Input {
	targets := make([]staff.TargetInput, 0, len(input.Targets))
	for _, target := range input.Targets {
		targets = append(targets, staff.TargetInput{TargetType: target.TargetType, RefID: target.RefID, RefText: target.RefText})
	}
	return staff.Input{
		Title: input.Title, Body: input.Body, Priority: input.Priority, LinkURL: input.LinkURL,
		RequiresAcknowledgement: input.RequiresAcknowledgement, SendEmail: input.SendEmail,
		ExpiresAt: input.ExpiresAt, Targets: targets, ResponseType: input.ResponseType,
		ResponseDeadline: input.ResponseDeadline, Options: input.Options,
		DeliveryMode: input.DeliveryMode, EmailAudience: input.EmailAudience,
		ReminderAt: input.ReminderAt, ReminderText: input.ReminderText,
		Declaration: staff.AnnouncementDeclarationSettings(input.Declaration),
	}
}

func mapParentAnnouncement(row *staff.ParentAnnouncement) *communication.ParentAnnouncement {
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
		{staff.ErrNotFound, communication.ErrParentAnnouncementNotFound},
		{staff.ErrValidation, communication.ErrParentAnnouncementValidation},
		{staff.ErrNewsDisabled, communication.ErrParentNewsDisabled},
		{staff.ErrSystemAnnouncementImmutable, communication.ErrSystemParentAnnouncementImmutable},
		{staff.ErrPublishedImmutable, communication.ErrPublishedParentAnnouncement},
		{staff.ErrNotPublished, communication.ErrParentAnnouncementNotPublished},
		{staff.ErrNothingOutstanding, communication.ErrParentAnnouncementNothingDue},
		{staff.ErrNotAPoll, communication.ErrParentAnnouncementNotPoll},
		{staff.ErrPollNotOpen, communication.ErrParentAnnouncementPollClosed},
		{staff.ErrReminderAlreadySent, communication.ErrParentAnnouncementReminderSent},
		{staff.ErrDeclarationHasSubmissions, communication.ErrDeclarationHasSubmissions},
		{staff.ErrNotDeclaration, communication.ErrNotDeclaration},
	} {
		if errors.Is(err, pair.internal) {
			return &mappedParentAnnouncementError{public: pair.public, original: err}
		}
	}
	return err
}
