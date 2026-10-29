package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/communication"
)

type parentMessaging[I, T, M, G any] struct {
	service ParentMessageService[I, T, M, G]
	rows    ParentMessageRows[I, T, M, G]
	errors  ParentMessageErrors
	observe messageObservation
}
type staffMessaging[I, T, M, G any, R retentionCounts] struct {
	service StaffMessageService[I, T, M, G, R]
	rows    StaffMessageRows[I, T, M, G, R]
	errors  StaffMessageErrors
	observe messageObservation
}

func NewParentMessagingFacade[I, T, M, G any](service ParentMessageService[I, T, M, G], rows ParentMessageRows[I, T, M, G], kinds ParentMessageErrors, observe messageObservation) communication.ParentMessagingCapability {
	return &parentMessaging[I, T, M, G]{service: service, rows: rows, errors: kinds, observe: observe}
}
func NewStaffMessagingFacade[I, T, M, G any, R retentionCounts](service StaffMessageService[I, T, M, G, R], rows StaffMessageRows[I, T, M, G, R], kinds StaffMessageErrors, observe messageObservation) communication.StaffMessagingRuntime {
	return &staffMessaging[I, T, M, G, R]{service: service, rows: rows, errors: kinds, observe: observe}
}
func (m *parentMessaging[I, T, M, G]) mapError(err error) error {
	return MapParentMessagingError(err, m.errors)
}
func (m *staffMessaging[I, T, M, G, R]) mapError(err error) error {
	return MapStaffMessagingError(err, m.errors)
}
func observeMessageResult[T any](ctx context.Context, observe messageObservation, operation string, run func(context.Context) (T, error)) (result T, err error) {
	runCtx, finish := observe(ctx, operation)
	defer func() { finish(err) }()
	return run(runCtx)
}
func projectMessageRows[A, B any](rows []*A, convert func(*A) *B) []*B {
	if rows == nil {
		return nil
	}
	out := make([]*B, 0, len(rows))
	for _, row := range rows {
		out = append(out, convert(row))
	}
	return out
}
func mapNativeParentThread[T, M any](thread *T, view func(*T) (InboxThread, []*M), convert func(*M) *ParentMessage) (*communication.ParentMessageThread, error) {
	if thread == nil {
		return nil, nil
	}
	header, rows := view(thread)
	messages, err := mapParentMessages(projectMessageRows(rows, convert))
	if err != nil {
		return nil, err
	}
	return &communication.ParentMessageThread{ThreadID: header.ThreadID, StudentID: header.StudentID, StudentName: header.StudentName, GuardianName: header.GuardianName, RelationshipType: header.RelationshipType, Messages: messages}, nil
}

// mapStaffThreadView maps the chat window onto the public contract,
// including a group's members and each message's read receipt (#4776).
func mapStaffThreadView(view *StaffThreadView) *communication.StaffMessageThread {
	if view == nil {
		return nil
	}
	result := &communication.StaffMessageThread{
		ThreadID: view.Header.ThreadID, CounterpartAccountID: view.Header.CounterpartAccountID,
		CounterpartName: view.Header.CounterpartName, CounterpartRoleKind: view.Header.CounterpartRoleKind,
		Kind: view.Kind, GroupName: view.GroupName, CanManage: view.CanManage,
		Messages: make([]communication.StaffMessage, 0, len(view.Messages)),
	}
	for _, member := range view.Members {
		result.Members = append(result.Members, communication.StaffThreadMember{AccountID: member.AccountID, Name: member.Name, RoleKind: member.RoleKind})
	}
	for _, message := range view.Messages {
		mapped := mapStaffMessage(message)
		if receipt, ok := view.Receipts[message.ID]; ok {
			mapped.ReadByAccountIDs = receipt.ReadBy
			mapped.EligibleReaderAccountIDs = receipt.EligibleReaderAccountIDs
			mapped.ReaderCount = receipt.ReaderCount
		}
		result.Messages = append(result.Messages, mapped)
	}
	return result
}

func (m *parentMessaging[I, T, M, G]) ListParentMessageInbox(ctx context.Context, onlyUnread bool) ([]communication.ParentMessageInboxThread, error) {
	return observeMessageResult(ctx, m.observe, "parent_messages.list_inbox", func(runCtx context.Context) ([]communication.ParentMessageInboxThread, error) {
		rows, err := m.service.ListInbox(runCtx, onlyUnread)
		return mapParentMessageInbox(projectMessageRows(rows, m.rows.Inbox)), m.mapError(err)
	})
}
func (m *parentMessaging[I, T, M, G]) CountUnreadParentMessages(ctx context.Context) (int, error) {
	return observeMessageResult(ctx, m.observe, "parent_messages.count_unread", func(runCtx context.Context) (int, error) {
		count, err := m.service.UnreadMessageCount(runCtx)
		return count, m.mapError(err)
	})
}
func (m *parentMessaging[I, T, M, G]) GetParentMessageThread(ctx context.Context, threadID int64) (*communication.ParentMessageThread, error) {
	return observeMessageResult(ctx, m.observe, "parent_messages.get_thread", func(runCtx context.Context) (*communication.ParentMessageThread, error) {
		thread, err := m.service.GetThread(runCtx, threadID)
		if err != nil {
			return nil, m.mapError(err)
		}
		return mapNativeParentThread(thread, m.rows.Thread, m.rows.Message)
	})
}
func (m *parentMessaging[I, T, M, G]) ListMessageableGuardians(ctx context.Context, studentID int64) ([]communication.MessageableGuardian, error) {
	return observeMessageResult(ctx, m.observe, "parent_messages.list_guardians", func(runCtx context.Context) ([]communication.MessageableGuardian, error) {
		rows, err := m.service.ListGuardians(runCtx, studentID)
		if err != nil {
			return nil, m.mapError(err)
		}
		result := make([]communication.MessageableGuardian, 0, len(rows))
		for _, row := range projectMessageRows(rows, m.rows.Guardian) {
			result = append(result, communication.MessageableGuardian{
				AccountID: row.AccountID, Name: row.Name,
				RelationshipType: row.RelationshipType, IsPrimary: row.IsPrimary,
			})
		}
		return result, nil
	})
}
func (m *parentMessaging[I, T, M, G]) ListParentMessageReachableStudents(ctx context.Context) ([]int64, error) {
	return observeMessageResult(ctx, m.observe, "parent_messages.list_reachable_students", func(runCtx context.Context) ([]int64, error) {
		ids, err := m.service.ListReachableStudentIDs(runCtx)
		return ids, m.mapError(err)
	})
}
func (m *parentMessaging[I, T, M, G]) ListParentMessageThreadsForStudent(ctx context.Context, studentID int64) ([]communication.ParentMessageInboxThread, error) {
	return observeMessageResult(ctx, m.observe, "parent_messages.list_student_threads", func(runCtx context.Context) ([]communication.ParentMessageInboxThread, error) {
		rows, err := m.service.ListStudentThreads(runCtx, studentID)
		return mapParentMessageInbox(projectMessageRows(rows, m.rows.Inbox)), m.mapError(err)
	})
}
func (m *parentMessaging[I, T, M, G]) StartParentMessageThread(ctx context.Context, studentID, guardianAccountID int64, body string) (*communication.ParentMessageThread, error) {
	return observeMessageResult(ctx, m.observe, "parent_messages.start_thread", func(runCtx context.Context) (*communication.ParentMessageThread, error) {
		thread, err := m.service.StartThread(runCtx, studentID, guardianAccountID, body)
		if err != nil {
			return nil, m.mapError(err)
		}
		return mapNativeParentThread(thread, m.rows.Thread, m.rows.Message)
	})
}
func (m *parentMessaging[I, T, M, G]) OpenParentMessageThread(ctx context.Context, studentID, guardianAccountID int64) (*communication.ParentMessageThread, error) {
	return observeMessageResult(ctx, m.observe, "parent_messages.open_thread", func(runCtx context.Context) (*communication.ParentMessageThread, error) {
		thread, err := m.service.OpenThread(runCtx, studentID, guardianAccountID)
		if err != nil {
			return nil, m.mapError(err)
		}
		return mapNativeParentThread(thread, m.rows.Thread, m.rows.Message)
	})
}
func (m *parentMessaging[I, T, M, G]) PostParentMessage(ctx context.Context, threadID int64, body string, handledUpToMessageID int64) ([]communication.ParentMessage, error) {
	return observeMessageResult(ctx, m.observe, "parent_messages.post_message", func(runCtx context.Context) ([]communication.ParentMessage, error) {
		rows, err := m.service.PostMessage(runCtx, threadID, body, handledUpToMessageID)
		if err != nil {
			return nil, m.mapError(err)
		}
		return mapParentMessages(projectMessageRows(rows, m.rows.Message))
	})
}
func (m *parentMessaging[I, T, M, G]) MarkParentMessageThreadUnread(ctx context.Context, threadID int64) error {
	_, err := observeMessageResult(ctx, m.observe, "parent_messages.mark_unread", func(runCtx context.Context) (struct{}, error) {
		return struct{}{}, m.mapError(m.service.MarkUnread(runCtx, threadID))
	})
	return err
}
func (m *parentMessaging[I, T, M, G]) MarkAllParentMessagesRead(ctx context.Context) (int, error) {
	return observeMessageResult(ctx, m.observe, "parent_messages.mark_all_read", func(runCtx context.Context) (int, error) {
		count, err := m.service.MarkAllRead(runCtx)
		return count, m.mapError(err)
	})
}
func (m *parentMessaging[I, T, M, G]) ParentMessageCountScope(ctx context.Context) (communication.ParentMessageCountSetting, error) {
	return observeMessageResult(ctx, m.observe, "parent_messages.count_scope", func(runCtx context.Context) (communication.ParentMessageCountSetting, error) {
		scope, err := m.service.CountScope(runCtx)
		if err != nil {
			return communication.ParentMessageCountSetting{}, m.mapError(err)
		}
		hasOwnGroups, err := m.service.HasOwnGroups(runCtx)
		if err != nil {
			return communication.ParentMessageCountSetting{}, m.mapError(err)
		}
		return communication.ParentMessageCountSetting{Scope: scope, HasOwnGroups: hasOwnGroups}, nil
	})
}
func (m *parentMessaging[I, T, M, G]) SetParentMessageCountScope(ctx context.Context, scope string) error {
	_, err := observeMessageResult(ctx, m.observe, "parent_messages.set_count_scope", func(runCtx context.Context) (struct{}, error) {
		return struct{}{}, m.mapError(m.service.SetCountScope(runCtx, scope))
	})
	return err
}
func (m *staffMessaging[I, T, M, G, R]) ListStaffMessageInbox(ctx context.Context, onlyUnread bool) ([]communication.StaffMessageInboxThread, error) {
	return observeMessageResult(ctx, m.observe, "staff_messages.list_inbox", func(runCtx context.Context) ([]communication.StaffMessageInboxThread, error) {
		rows, err := m.service.ListInbox(runCtx, onlyUnread)
		if err != nil {
			return nil, m.mapError(err)
		}
		result := make([]communication.StaffMessageInboxThread, 0, len(rows))
		for _, row := range projectMessageRows(rows, m.rows.Inbox) {
			result = append(result, communication.StaffMessageInboxThread{
				ThreadID: row.ThreadID, CounterpartAccountID: row.CounterpartAccountID,
				CounterpartName: row.CounterpartName, CounterpartRoleKind: row.CounterpartRoleKind,
				LastMessageAt: row.LastMessageAt, LastMessageBody: row.LastMessageBody,
				LastSenderAccountID: row.LastSenderAccountID, UnreadCount: row.UnreadCount,
				Kind: row.Kind, GroupName: row.GroupName, MemberCount: row.MemberCount, LastSenderName: row.LastSenderName,
			})
		}
		return result, nil
	})
}
func (m *staffMessaging[I, T, M, G, R]) CountUnreadStaffMessages(ctx context.Context) (int, error) {
	return observeMessageResult(ctx, m.observe, "staff_messages.count_unread", func(runCtx context.Context) (int, error) {
		count, err := m.service.UnreadMessageCount(runCtx)
		return count, m.mapError(err)
	})
}
func (m *staffMessaging[I, T, M, G, R]) ListMessageableStaff(ctx context.Context) ([]communication.MessageableStaff, error) {
	return observeMessageResult(ctx, m.observe, "staff_messages.list_recipients", func(runCtx context.Context) ([]communication.MessageableStaff, error) {
		rows, err := m.service.ListMessageableStaff(runCtx)
		if err != nil {
			return nil, m.mapError(err)
		}
		result := make([]communication.MessageableStaff, 0, len(rows))
		for _, row := range projectMessageRows(rows, m.rows.Colleague) {
			result = append(result, communication.MessageableStaff{AccountID: row.AccountID, Name: row.Name, RoleKind: row.RoleKind})
		}
		return result, nil
	})
}
func (m *staffMessaging[I, T, M, G, R]) OpenStaffMessageThread(ctx context.Context, counterpartAccountID int64) (*communication.StaffMessageThread, error) {
	return observeMessageResult(ctx, m.observe, "staff_messages.open_thread", func(runCtx context.Context) (*communication.StaffMessageThread, error) {
		thread, err := m.service.OpenThread(runCtx, counterpartAccountID)
		if err != nil || thread == nil {
			return nil, m.mapError(err)
		}
		return mapStaffThreadView(m.rows.Thread(thread)), nil
	})
}
func (m *staffMessaging[I, T, M, G, R]) GetStaffMessageThread(ctx context.Context, threadID int64) (*communication.StaffMessageThread, error) {
	return observeMessageResult(ctx, m.observe, "staff_messages.get_thread", func(runCtx context.Context) (*communication.StaffMessageThread, error) {
		thread, err := m.service.GetThread(runCtx, threadID)
		if err != nil || thread == nil {
			return nil, m.mapError(err)
		}
		return mapStaffThreadView(m.rows.Thread(thread)), nil
	})
}
func (m *staffMessaging[I, T, M, G, R]) PostStaffMessage(ctx context.Context, threadID int64, body string) (*communication.StaffMessage, error) {
	return observeMessageResult(ctx, m.observe, "staff_messages.post_message", func(runCtx context.Context) (*communication.StaffMessage, error) {
		result, err := m.service.PostMessageWithReceipt(runCtx, threadID, body)
		if result == nil || result.Message == nil {
			return nil, m.mapError(err)
		}
		mapped := mapNativeStaffMessage(result.Message, m.rows.Message)
		if receipt := result.Receipt; receipt != nil {
			mapped.ReadByAccountIDs = receipt.ReadBy
			mapped.EligibleReaderAccountIDs = receipt.EligibleReaderAccountIDs
			mapped.ReaderCount = receipt.ReaderCount
		}
		return &mapped, m.mapError(err)
	})
}
func (m *staffMessaging[I, T, M, G, R]) CreateStaffMessageGroup(ctx context.Context, name string, memberAccountIDs []int64) (*communication.StaffMessageThread, error) {
	return observeMessageResult(ctx, m.observe, "staff_messages.create_group", func(runCtx context.Context) (*communication.StaffMessageThread, error) {
		thread, err := m.service.CreateGroup(runCtx, name, memberAccountIDs)
		if err != nil || thread == nil {
			return nil, m.mapError(err)
		}
		return mapStaffThreadView(m.rows.Thread(thread)), nil
	})
}
func (m *staffMessaging[I, T, M, G, R]) RenameStaffMessageGroup(ctx context.Context, threadID int64, name string) (*communication.StaffMessageThread, error) {
	return observeMessageResult(ctx, m.observe, "staff_messages.rename_group", func(runCtx context.Context) (*communication.StaffMessageThread, error) {
		thread, err := m.service.RenameGroup(runCtx, threadID, name)
		if err != nil || thread == nil {
			return nil, m.mapError(err)
		}
		return mapStaffThreadView(m.rows.Thread(thread)), nil
	})
}
func (m *staffMessaging[I, T, M, G, R]) AddStaffMessageGroupMembers(ctx context.Context, threadID int64, accountIDs []int64) (*communication.StaffMessageThread, error) {
	return observeMessageResult(ctx, m.observe, "staff_messages.add_group_members", func(runCtx context.Context) (*communication.StaffMessageThread, error) {
		thread, err := m.service.AddGroupMembers(runCtx, threadID, accountIDs)
		if err != nil || thread == nil {
			return nil, m.mapError(err)
		}
		return mapStaffThreadView(m.rows.Thread(thread)), nil
	})
}
func (m *staffMessaging[I, T, M, G, R]) RemoveStaffMessageGroupMember(ctx context.Context, threadID, accountID int64) (*communication.StaffMessageThread, error) {
	return observeMessageResult(ctx, m.observe, "staff_messages.remove_group_member", func(runCtx context.Context) (*communication.StaffMessageThread, error) {
		thread, err := m.service.RemoveGroupMember(runCtx, threadID, accountID)
		if err != nil || thread == nil {
			return nil, m.mapError(err)
		}
		return mapStaffThreadView(m.rows.Thread(thread)), nil
	})
}
func (m *staffMessaging[I, T, M, G, R]) CleanupExpiredStaffMessages(ctx context.Context) (communication.StaffMessageCleanupResult, error) {
	return observeMessageResult(ctx, m.observe, "staff_messages.cleanup", func(runCtx context.Context) (communication.StaffMessageCleanupResult, error) {
		result, err := m.service.CleanupExpiredMessages(runCtx)
		deleted, threads, days := result.CleanupCounts()
		return communication.StaffMessageCleanupResult{MessagesDeleted: deleted, ThreadsDeleted: threads, RetentionDays: days}, m.mapError(err)
	})
}
func mapParentMessageInbox(rows []*InboxThread) []communication.ParentMessageInboxThread {
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
func mapParentMessages(rows []*ParentMessage) ([]communication.ParentMessage, error) {
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
func mapParentMessage(row *ParentMessage) (communication.ParentMessage, error) {
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
		GroupThreadID: row.GroupThreadID, GroupThreadTitle: row.GroupThreadTitle,
	}, nil
}
func mapStaffMessage(message *StaffMessage) communication.StaffMessage {
	return communication.StaffMessage{
		ID: message.ID, SenderAccountID: message.SenderAccountID, SenderName: message.SenderName,
		Body: message.Body, CreatedAt: message.CreatedAt,
	}
}
func MapParentMessagingError(err error, kinds ParentMessageErrors) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, kinds.ErrForbidden):
		return fmt.Errorf("%w: %w", communication.ErrParentMessagingForbidden, err)
	case errors.Is(err, kinds.ErrThreadNotFound):
		return fmt.Errorf("%w: %w", communication.ErrParentMessageThreadNotFound, err)
	case errors.Is(err, kinds.ErrEmptyBody):
		return fmt.Errorf("%w: %w", communication.ErrParentMessageEmptyBody, err)
	case errors.Is(err, kinds.ErrBodyTooLong):
		return fmt.Errorf("%w: %w", communication.ErrParentMessageBodyTooLong, err)
	case errors.Is(err, kinds.ErrHandledBoundaryRequired):
		return fmt.Errorf("%w: %w", communication.ErrParentMessageHandledBoundaryRequired, err)
	case errors.Is(err, kinds.ErrInvalidGuardian):
		return fmt.Errorf("%w: %w", communication.ErrParentMessageInvalidGuardian, err)
	case errors.Is(err, kinds.ErrGuardianAccessRevoked):
		return fmt.Errorf("%w: %w", communication.ErrParentMessageGuardianAccessRevoked, err)
	case errors.Is(err, kinds.ErrMessagingDisabled):
		return fmt.Errorf("%w: %w", communication.ErrParentMessagingDisabled, err)
	case errors.Is(err, kinds.ErrInvalidCountScope):
		return fmt.Errorf("%w: %w", communication.ErrParentMessageInvalidCountScope, err)
	default:
		return err
	}
}
func MapStaffMessagingError(err error, kinds StaffMessageErrors) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, kinds.ErrMessagingDisabled):
		return fmt.Errorf("%w: %w", communication.ErrStaffMessagingDisabled, err)
	case errors.Is(err, kinds.ErrNotParticipant):
		return fmt.Errorf("%w: %w", communication.ErrStaffMessagingNotParticipant, err)
	case errors.Is(err, kinds.ErrThreadNotFound):
		return fmt.Errorf("%w: %w", communication.ErrStaffMessageThreadNotFound, err)
	case errors.Is(err, kinds.ErrRecipientNotAvailable):
		return fmt.Errorf("%w: %w", communication.ErrStaffMessageRecipientNotAvailable, err)
	case errors.Is(err, kinds.ErrCounterpartUnavailable):
		return fmt.Errorf("%w: %w", communication.ErrStaffMessageCounterpartUnavailable, err)
	case errors.Is(err, kinds.ErrSelfConversation):
		return fmt.Errorf("%w: %w", communication.ErrStaffMessageSelfConversation, err)
	case errors.Is(err, kinds.ErrEmptyMessage):
		return fmt.Errorf("%w: %w", communication.ErrStaffMessageEmptyBody, err)
	case errors.Is(err, kinds.ErrMessageTooLong):
		return fmt.Errorf("%w: %w", communication.ErrStaffMessageBodyTooLong, err)
	case errors.Is(err, kinds.ErrNoActor):
		return fmt.Errorf("%w: %w", communication.ErrStaffMessagingNoActor, err)
	case errors.Is(err, kinds.ErrRetentionUnresolved):
		return fmt.Errorf("%w: %w", communication.ErrStaffMessageRetentionUnresolved, err)
	case errors.Is(err, kinds.ErrGroupCreateForbidden):
		return fmt.Errorf("%w: %w", communication.ErrStaffMessageGroupCreateForbidden, err)
	case errors.Is(err, kinds.ErrGroupManageForbidden):
		return fmt.Errorf("%w: %w", communication.ErrStaffMessageGroupManageForbidden, err)
	case errors.Is(err, kinds.ErrGroupNameInvalid):
		return communication.NewStaffGroupNameInvalidError(kinds.GroupNameMaxCharacters,
			fmt.Errorf("%w: %w", communication.ErrStaffMessageGroupNameInvalid, err))
	case errors.Is(err, kinds.ErrGroupMembersRequired):
		return fmt.Errorf("%w: %w", communication.ErrStaffMessageGroupMembersRequired, err)
	case errors.Is(err, kinds.ErrGroupTooLarge):
		return communication.NewStaffGroupTooLargeError(kinds.GroupMaxMembers,
			fmt.Errorf("%w: %w", communication.ErrStaffMessageGroupTooLarge, err))
	default:
		return err
	}
}

func mapNativeStaffMessage[M any](row *M, convert func(*M) *StaffMessage) communication.StaffMessage {
	return mapStaffMessage(convert(row))
}
