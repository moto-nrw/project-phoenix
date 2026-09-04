package compose

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/email"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/communication"
	parentMessages "github.com/moto-nrw/project-phoenix/modules/communication/internal/parentmessages"
	"github.com/moto-nrw/project-phoenix/modules/communication/internal/ports"
	staffMessages "github.com/moto-nrw/project-phoenix/modules/communication/internal/staffmessages"
	"github.com/moto-nrw/project-phoenix/modules/delivery/application/notifications"
	"github.com/moto-nrw/project-phoenix/realtime"
	configService "github.com/moto-nrw/project-phoenix/services/config"
	userContextService "github.com/moto-nrw/project-phoenix/services/usercontext"
	userService "github.com/moto-nrw/project-phoenix/services/users"
	"github.com/uptrace/bun"
)

type ParentMessagingConfig struct {
	ThreadRepo       usersModels.ParentMessageThreadRepository
	MessageRepo      usersModels.ParentMessageRepository
	ReadRepo         usersModels.ParentMessageReadRepository
	Persons          userService.PersonService
	UserContext      userContextService.UserContextService
	Settings         configService.SettingsService
	Broadcaster      realtime.Broadcaster
	DB               *bun.DB
	Logger           *slog.Logger
	Notifier         notifications.Service
	Preferences      notifications.PreferenceService
	Outbox           platformModels.OutboxEnqueuer
	GuardianProfiles parentMessages.GuardianProfileFinder
	Schools          parentMessages.SchoolFinder
	LoginImages      parentMessages.LoginImageResolver
	ParentsURL       string
	Observe          func(Observation)
}

func NewParentMessaging(cfg ParentMessagingConfig) communication.ParentMessagingCapability {
	return &parentMessaging{
		service: parentMessages.NewService(parentMessages.Config{
			ThreadRepo: cfg.ThreadRepo, MessageRepo: cfg.MessageRepo, ReadRepo: cfg.ReadRepo,
			Persons: cfg.Persons, UserContext: cfg.UserContext, Settings: cfg.Settings,
			Broadcaster: cfg.Broadcaster, DB: cfg.DB, Logger: cfg.Logger,
			Notifier: cfg.Notifier, Preferences: cfg.Preferences, Outbox: cfg.Outbox,
			GuardianProfiles: cfg.GuardianProfiles, Schools: cfg.Schools,
			LoginImages: cfg.LoginImages, ParentsURL: cfg.ParentsURL,
		}),
		observe: cfg.Observe,
	}
}

type ParentMessageRendererConfig struct{ DefaultFrom email.Email }

func NewParentMessageRenderer(cfg ParentMessageRendererConfig) func(context.Context, *platformModels.EmailOutbox) (*email.Message, error) {
	return parentMessages.NewParentMessageRenderer(parentMessages.ParentMessageRendererConfig{DefaultFrom: cfg.DefaultFrom})
}

type parentMessaging struct {
	service *parentMessages.Service
	observe func(Observation)
}

func (m *parentMessaging) ListParentMessageInbox(ctx context.Context, onlyUnread bool) ([]communication.ParentMessageInboxThread, error) {
	return observeResult(ctx, m.observe, "parent_messages.list_inbox", func(runCtx context.Context) ([]communication.ParentMessageInboxThread, error) {
		rows, err := m.service.ListInbox(runCtx, onlyUnread)
		return mapParentMessageInbox(rows), mapParentMessagingError(err)
	})
}

func (m *parentMessaging) CountUnreadParentMessages(ctx context.Context) (int, error) {
	return observeResult(ctx, m.observe, "parent_messages.count_unread", func(runCtx context.Context) (int, error) {
		count, err := m.service.UnreadMessageCount(runCtx)
		return count, mapParentMessagingError(err)
	})
}

func (m *parentMessaging) GetParentMessageThread(ctx context.Context, threadID int64) (*communication.ParentMessageThread, error) {
	return observeResult(ctx, m.observe, "parent_messages.get_thread", func(runCtx context.Context) (*communication.ParentMessageThread, error) {
		thread, err := m.service.GetThread(runCtx, threadID)
		if err != nil {
			return nil, mapParentMessagingError(err)
		}
		return mapParentMessageThread(thread)
	})
}

func (m *parentMessaging) ListMessageableGuardians(ctx context.Context, studentID int64) ([]communication.MessageableGuardian, error) {
	return observeResult(ctx, m.observe, "parent_messages.list_guardians", func(runCtx context.Context) ([]communication.MessageableGuardian, error) {
		rows, err := m.service.ListGuardians(runCtx, studentID)
		if err != nil {
			return nil, mapParentMessagingError(err)
		}
		result := make([]communication.MessageableGuardian, 0, len(rows))
		for _, row := range rows {
			result = append(result, communication.MessageableGuardian{
				AccountID: row.AccountID, Name: row.Name,
				RelationshipType: row.RelationshipType, IsPrimary: row.IsPrimary,
			})
		}
		return result, nil
	})
}

func (m *parentMessaging) ListParentMessageThreadsForStudent(ctx context.Context, studentID int64) ([]communication.ParentMessageInboxThread, error) {
	return observeResult(ctx, m.observe, "parent_messages.list_student_threads", func(runCtx context.Context) ([]communication.ParentMessageInboxThread, error) {
		rows, err := m.service.ListStudentThreads(runCtx, studentID)
		return mapParentMessageInbox(rows), mapParentMessagingError(err)
	})
}

func (m *parentMessaging) StartParentMessageThread(ctx context.Context, studentID, guardianAccountID int64, body string) (*communication.ParentMessageThread, error) {
	return observeResult(ctx, m.observe, "parent_messages.start_thread", func(runCtx context.Context) (*communication.ParentMessageThread, error) {
		thread, err := m.service.StartThread(runCtx, studentID, guardianAccountID, body)
		if err != nil {
			return nil, mapParentMessagingError(err)
		}
		return mapParentMessageThread(thread)
	})
}

func (m *parentMessaging) OpenParentMessageThread(ctx context.Context, studentID, guardianAccountID int64) (*communication.ParentMessageThread, error) {
	return observeResult(ctx, m.observe, "parent_messages.open_thread", func(runCtx context.Context) (*communication.ParentMessageThread, error) {
		thread, err := m.service.OpenThread(runCtx, studentID, guardianAccountID)
		if err != nil {
			return nil, mapParentMessagingError(err)
		}
		return mapParentMessageThread(thread)
	})
}

func (m *parentMessaging) PostParentMessage(ctx context.Context, threadID int64, body string, handledUpToMessageID int64) ([]communication.ParentMessage, error) {
	return observeResult(ctx, m.observe, "parent_messages.post_message", func(runCtx context.Context) ([]communication.ParentMessage, error) {
		rows, err := m.service.PostMessage(runCtx, threadID, body, handledUpToMessageID)
		if err != nil {
			return nil, mapParentMessagingError(err)
		}
		return mapParentMessages(rows)
	})
}

type StaffMessagingConfig struct {
	ThreadRepo  usersModels.StaffMessageThreadRepository
	MessageRepo usersModels.StaffMessageRepository
	ReadRepo    usersModels.StaffMessageReadRepository
	Persons     userService.PersonService
	Settings    configService.SettingsService
	Broadcaster realtime.Broadcaster
	DB          *bun.DB
	Logger      *slog.Logger
	Notifier    notifications.Service
	Preferences notifications.PreferenceService
	Observe     func(Observation)
}

func NewStaffMessaging(cfg StaffMessagingConfig) communication.StaffMessagingRuntime {
	return &staffMessaging{
		service: staffMessages.NewService(staffMessages.Config{
			ThreadRepo: cfg.ThreadRepo, MessageRepo: cfg.MessageRepo, ReadRepo: cfg.ReadRepo,
			Persons: cfg.Persons, Settings: cfg.Settings, Broadcaster: cfg.Broadcaster,
			DB: cfg.DB, Logger: cfg.Logger, Notifier: cfg.Notifier, Preferences: cfg.Preferences,
		}),
		observe: cfg.Observe,
	}
}

type staffMessaging struct {
	service *staffMessages.Service
	observe func(Observation)
}

func (m *staffMessaging) ListStaffMessageInbox(ctx context.Context, onlyUnread bool) ([]communication.StaffMessageInboxThread, error) {
	return observeResult(ctx, m.observe, "staff_messages.list_inbox", func(runCtx context.Context) ([]communication.StaffMessageInboxThread, error) {
		rows, err := m.service.ListInbox(runCtx, onlyUnread)
		if err != nil {
			return nil, mapStaffMessagingError(err)
		}
		result := make([]communication.StaffMessageInboxThread, 0, len(rows))
		for _, row := range rows {
			result = append(result, communication.StaffMessageInboxThread{
				ThreadID: row.ThreadID, CounterpartAccountID: row.CounterpartAccountID,
				CounterpartName: row.CounterpartName, CounterpartRoleKind: row.CounterpartRoleKind,
				LastMessageAt: row.LastMessageAt, LastMessageBody: row.LastMessageBody,
				LastSenderAccountID: row.LastSenderAccountID, UnreadCount: row.UnreadCount,
			})
		}
		return result, nil
	})
}

func (m *staffMessaging) CountUnreadStaffMessages(ctx context.Context) (int, error) {
	return observeResult(ctx, m.observe, "staff_messages.count_unread", func(runCtx context.Context) (int, error) {
		count, err := m.service.UnreadMessageCount(runCtx)
		return count, mapStaffMessagingError(err)
	})
}

func (m *staffMessaging) ListMessageableStaff(ctx context.Context) ([]communication.MessageableStaff, error) {
	return observeResult(ctx, m.observe, "staff_messages.list_recipients", func(runCtx context.Context) ([]communication.MessageableStaff, error) {
		rows, err := m.service.ListMessageableStaff(runCtx)
		if err != nil {
			return nil, mapStaffMessagingError(err)
		}
		result := make([]communication.MessageableStaff, 0, len(rows))
		for _, row := range rows {
			result = append(result, communication.MessageableStaff{AccountID: row.AccountID, Name: row.Name, RoleKind: row.RoleKind})
		}
		return result, nil
	})
}

func (m *staffMessaging) OpenStaffMessageThread(ctx context.Context, counterpartAccountID int64) (*communication.StaffMessageThread, error) {
	return observeResult(ctx, m.observe, "staff_messages.open_thread", func(runCtx context.Context) (*communication.StaffMessageThread, error) {
		thread, err := m.service.OpenThread(runCtx, counterpartAccountID)
		return mapStaffMessageThread(thread), mapStaffMessagingError(err)
	})
}

func (m *staffMessaging) GetStaffMessageThread(ctx context.Context, threadID int64) (*communication.StaffMessageThread, error) {
	return observeResult(ctx, m.observe, "staff_messages.get_thread", func(runCtx context.Context) (*communication.StaffMessageThread, error) {
		thread, err := m.service.GetThread(runCtx, threadID)
		return mapStaffMessageThread(thread), mapStaffMessagingError(err)
	})
}

func (m *staffMessaging) PostStaffMessage(ctx context.Context, threadID int64, body string) (*communication.StaffMessage, error) {
	return observeResult(ctx, m.observe, "staff_messages.post_message", func(runCtx context.Context) (*communication.StaffMessage, error) {
		message, err := m.service.PostMessage(runCtx, threadID, body)
		if message == nil {
			return nil, mapStaffMessagingError(err)
		}
		mapped := mapStaffMessage(message)
		return &mapped, mapStaffMessagingError(err)
	})
}

func (m *staffMessaging) CleanupExpiredStaffMessages(ctx context.Context) (communication.StaffMessageCleanupResult, error) {
	return observeResult(ctx, m.observe, "staff_messages.cleanup", func(runCtx context.Context) (communication.StaffMessageCleanupResult, error) {
		result, err := m.service.CleanupExpiredMessages(runCtx)
		return communication.StaffMessageCleanupResult{
			MessagesDeleted: result.MessagesDeleted,
			ThreadsDeleted:  result.ThreadsDeleted,
			RetentionDays:   result.RetentionDays,
		}, mapStaffMessagingError(err)
	})
}

func observeResult[T any](ctx context.Context, observe func(Observation), operation string, run func(context.Context) (T, error)) (result T, err error) {
	started := time.Now()
	runCtx, queryStats := withMessageQueryStats(ctx)
	defer func() {
		if observe != nil {
			stats := queryStats()
			if err != nil {
				stats.Rows = 0
			}
			observe(ports.Observation{Operation: operation, Duration: time.Since(started), Stats: stats, Err: err})
		}
	}()
	return run(runCtx)
}

func mapParentMessageInbox(rows []*usersModels.InboxThread) []communication.ParentMessageInboxThread {
	result := make([]communication.ParentMessageInboxThread, 0, len(rows))
	for _, row := range rows {
		result = append(result, communication.ParentMessageInboxThread{
			ThreadID: row.ThreadID, StudentID: row.StudentID, StudentName: row.StudentName,
			SchoolClass: row.SchoolClass, GroupName: row.GroupName, GuardianName: row.GuardianName,
			RelationshipType: row.RelationshipType, LastMessageAt: row.LastMessageAt,
			LastSenderKind: row.LastSenderKind, LastMessageBody: row.LastMessageBody, UnreadCount: row.UnreadCount,
		})
	}
	return result
}

func mapParentMessageThread(thread *parentMessages.ThreadDetail) (*communication.ParentMessageThread, error) {
	if thread == nil {
		return nil, nil
	}
	messages, err := mapParentMessages(thread.Messages)
	if err != nil {
		return nil, err
	}
	return &communication.ParentMessageThread{
		ThreadID: thread.ThreadID, StudentID: thread.StudentID, StudentName: thread.StudentName,
		GuardianName: thread.GuardianName, RelationshipType: thread.RelationshipType, Messages: messages,
	}, nil
}

func mapParentMessages(rows []*usersModels.ParentMessage) ([]communication.ParentMessage, error) {
	result := make([]communication.ParentMessage, 0, len(rows))
	for _, row := range rows {
		mapped, err := mapParentMessage(row)
		if err != nil {
			return nil, err
		}
		result = append(result, mapped)
	}
	return result, nil
}

func mapParentMessage(row *usersModels.ParentMessage) (communication.ParentMessage, error) {
	var payload json.RawMessage
	if row.Payload != nil {
		encoded, err := json.Marshal(row.Payload)
		if err != nil {
			return communication.ParentMessage{}, fmt.Errorf("communication: encode parent message payload: %w", err)
		}
		payload = encoded
	}
	return communication.ParentMessage{
		ID: row.ID, SenderKind: row.SenderKind, SenderName: row.SenderName,
		Body: row.Body, CreatedAt: row.CreatedAt, Kind: row.Kind, EventType: row.EventType,
		RequestType: row.RequestType, RequestStatus: row.RequestStatus, Payload: payload,
		RefTable: row.RefTable, RefID: row.RefID, AppliedAt: row.AppliedAt, AppliedBy: row.AppliedBy,
		DecisionReason: row.DecisionReason, ReadByStaff: row.ReadByStaff, ReadByGuardian: row.ReadByGuardian,
	}, nil
}

func mapStaffMessageThread(thread *staffMessages.ThreadDetail) *communication.StaffMessageThread {
	if thread == nil {
		return nil
	}
	result := &communication.StaffMessageThread{
		ThreadID: thread.ThreadID, CounterpartAccountID: thread.CounterpartAccountID,
		CounterpartName: thread.CounterpartName, CounterpartRoleKind: thread.CounterpartRoleKind,
		Messages: make([]communication.StaffMessage, 0, len(thread.Messages)),
	}
	for _, message := range thread.Messages {
		result.Messages = append(result.Messages, mapStaffMessage(message))
	}
	return result
}

func mapStaffMessage(message *usersModels.StaffMessage) communication.StaffMessage {
	return communication.StaffMessage{
		ID: message.ID, SenderAccountID: message.SenderAccountID, SenderName: message.SenderName,
		Body: message.Body, CreatedAt: message.CreatedAt,
	}
}

func mapParentMessagingError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, parentMessages.ErrForbidden):
		return fmt.Errorf("%w: %w", communication.ErrParentMessagingForbidden, err)
	case errors.Is(err, parentMessages.ErrThreadNotFound):
		return fmt.Errorf("%w: %w", communication.ErrParentMessageThreadNotFound, err)
	case errors.Is(err, parentMessages.ErrEmptyBody):
		return fmt.Errorf("%w: %w", communication.ErrParentMessageEmptyBody, err)
	case errors.Is(err, parentMessages.ErrBodyTooLong):
		return fmt.Errorf("%w: %w", communication.ErrParentMessageBodyTooLong, err)
	case errors.Is(err, parentMessages.ErrHandledBoundaryRequired):
		return fmt.Errorf("%w: %w", communication.ErrParentMessageHandledBoundaryRequired, err)
	case errors.Is(err, parentMessages.ErrInvalidGuardian):
		return fmt.Errorf("%w: %w", communication.ErrParentMessageInvalidGuardian, err)
	case errors.Is(err, parentMessages.ErrGuardianAccessRevoked):
		return fmt.Errorf("%w: %w", communication.ErrParentMessageGuardianAccessRevoked, err)
	case errors.Is(err, parentMessages.ErrMessagingDisabled):
		return fmt.Errorf("%w: %w", communication.ErrParentMessagingDisabled, err)
	default:
		return err
	}
}

func mapStaffMessagingError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, staffMessages.ErrMessagingDisabled):
		return fmt.Errorf("%w: %w", communication.ErrStaffMessagingDisabled, err)
	case errors.Is(err, staffMessages.ErrNotParticipant):
		return fmt.Errorf("%w: %w", communication.ErrStaffMessagingNotParticipant, err)
	case errors.Is(err, staffMessages.ErrThreadNotFound):
		return fmt.Errorf("%w: %w", communication.ErrStaffMessageThreadNotFound, err)
	case errors.Is(err, staffMessages.ErrRecipientNotAvailable):
		return fmt.Errorf("%w: %w", communication.ErrStaffMessageRecipientNotAvailable, err)
	case errors.Is(err, staffMessages.ErrCounterpartUnavailable):
		return fmt.Errorf("%w: %w", communication.ErrStaffMessageCounterpartUnavailable, err)
	case errors.Is(err, staffMessages.ErrSelfConversation):
		return fmt.Errorf("%w: %w", communication.ErrStaffMessageSelfConversation, err)
	case errors.Is(err, staffMessages.ErrEmptyMessage):
		return fmt.Errorf("%w: %w", communication.ErrStaffMessageEmptyBody, err)
	case errors.Is(err, staffMessages.ErrMessageTooLong):
		return fmt.Errorf("%w: %w", communication.ErrStaffMessageBodyTooLong, err)
	case errors.Is(err, staffMessages.ErrNoActor):
		return fmt.Errorf("%w: %w", communication.ErrStaffMessagingNoActor, err)
	case errors.Is(err, staffMessages.ErrRetentionUnresolved):
		return fmt.Errorf("%w: %w", communication.ErrStaffMessageRetentionUnresolved, err)
	default:
		return err
	}
}

var (
	_ communication.ParentMessagingCapability = (*parentMessaging)(nil)
	_ communication.StaffMessagingRuntime     = (*staffMessaging)(nil)
)
