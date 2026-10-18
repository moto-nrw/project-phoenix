package parentstore

import (
	"context"
	"encoding/json"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/communication"
	"github.com/moto-nrw/project-phoenix/modules/communication/internal/adapters/parentpostgres"
	values "github.com/moto-nrw/project-phoenix/modules/communication/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/communication/internal/domain"
	parentinbox "github.com/moto-nrw/project-phoenix/modules/parentmessageinbox/contract"
	"github.com/uptrace/bun"
)

// ParentMessageReads answers the inbox, unread, header and receipt reads plus
// the two cursor writes.
//
// The three staff-gated projections need the school's staff accounts, which
// School Membership owns, so they are exposed as *WithStaff methods and the
// composition root completes the repository contract over that owner.
type ParentMessageReads struct {
	cursors *parentpostgres.ReadCursorStore
	threads *parentpostgres.ThreadStore
	inbox   ParentInbox
}

// NewParentMessageReadRepository builds the read-side store over the
// guardian-link row source.
func NewParentMessageReadRepository(db *bun.DB, inbox ParentInbox) *ParentMessageReads {
	return &ParentMessageReads{
		cursors: parentpostgres.NewReadCursorStore(postgresDatabase(db)),
		threads: parentpostgres.NewThreadStore(postgresDatabase(db)),
		inbox:   inbox,
	}
}

func (r *ParentMessageReads) MarkReadUpTo(ctx context.Context, tenantID, threadID, accountID int64, readAt time.Time, readMessageID int64) (bool, error) {
	return r.cursors.MarkReadUpTo(ctx, tenantID, threadID, accountID, readAt, readMessageID)
}

func (r *ParentMessageReads) ClearUnreadForStaff(ctx context.Context, tenantID, accountID int64, bounds []communication.PortalReadCursorBound) ([]int64, error) {
	selected := make([]domain.ReadCursorBound, 0, len(bounds))
	for _, bound := range bounds {
		selected = append(selected, domain.ReadCursorBound{
			ThreadID: bound.ThreadID, ReadAt: bound.ReadAt, MessageID: bound.MessageID,
		})
	}
	return r.cursors.ClearUnreadForStaff(ctx, tenantID, accountID, selected)
}

func (r *ParentMessageReads) MarkStaffHandledUpTo(ctx context.Context, tenantID, threadID int64, handledAt time.Time, handledMessageID int64) error {
	return r.threads.MarkStaffHandledUpTo(ctx, tenantID, threadID, handledAt, handledMessageID)
}

func (r *ParentMessageReads) MarkStaffUnread(ctx context.Context, tenantID, threadID, accountID int64) error {
	return r.threads.MarkStaffUnread(ctx, tenantID, threadID, accountID)
}

func (r *ParentMessageReads) ClearStaffUnreadMark(ctx context.Context, tenantID, threadID int64, observedAt time.Time) (bool, error) {
	return r.threads.ClearStaffUnreadMark(ctx, tenantID, threadID, observedAt)
}

func (r *ParentMessageReads) UnreadMessageCountForStaff(ctx context.Context, accountID int64, allStudents bool) (int, error) {
	return r.inbox.UnreadMessageCountForStaff(ctx, accountID, allStudents)
}

func (r *ParentMessageReads) UnreadMessageCountForStaffInGroups(ctx context.Context, accountID int64, allStudents bool, groupIDs []int64) (int, error) {
	return r.inbox.UnreadMessageCountForStaffInGroups(ctx, accountID, allStudents, groupIDs)
}

func (r *ParentMessageReads) ListInboxForStaff(ctx context.Context, accountID int64, allStudents, onlyUnread bool) ([]*communication.PortalInboxThread, error) {
	rows, err := r.inbox.ListInboxForStaff(ctx, accountID, allStudents, onlyUnread)
	mapped, mapErr := inboxModels(rows)
	if err != nil {
		return mapped, err
	}
	return mapped, mapErr
}

func (r *ParentMessageReads) ListThreadsForStudent(ctx context.Context, accountID, studentID int64) ([]*communication.PortalInboxThread, error) {
	rows, err := r.inbox.ListThreadsForStudent(ctx, accountID, studentID)
	mapped, mapErr := inboxModels(rows)
	if err != nil {
		return mapped, err
	}
	return mapped, mapErr
}

func (r *ParentMessageReads) ListThreadsForGuardianStudentWithStaff(ctx context.Context, accountID, studentID int64, staffAccounts map[int64][]int64) ([]*communication.PortalInboxThread, error) {
	rows, err := r.inbox.ListThreadsForGuardianStudent(ctx, accountID, studentID, staffAccounts)
	mapped, mapErr := inboxModels(rows)
	if err != nil {
		return mapped, err
	}
	return mapped, mapErr
}

func (r *ParentMessageReads) ListThreadsForGuardianTenantsWithStaff(ctx context.Context, accountID int64, tenantIDs []int64, staffAccounts map[int64][]int64) ([]*communication.PortalInboxThread, error) {
	rows, err := r.inbox.ListThreadsForGuardianTenants(ctx, accountID, tenantIDs, staffAccounts)
	mapped, mapErr := inboxModels(rows)
	if err != nil {
		return mapped, err
	}
	return mapped, mapErr
}

func (r *ParentMessageReads) UnreadMessageCountForGuardianTenants(ctx context.Context, accountID int64, tenantIDs []int64) (int, error) {
	return r.inbox.UnreadMessageCountForGuardianTenants(ctx, accountID, tenantIDs)
}

func (r *ParentMessageReads) FindThreadHeader(ctx context.Context, threadID int64) (*communication.ParentThreadHeaderRecord, error) {
	header, err := r.inbox.FindThreadHeader(ctx, threadID)
	if err != nil || header == nil {
		return nil, err
	}
	return &communication.ParentThreadHeaderRecord{
		StudentName: header.StudentName, GuardianName: header.GuardianName,
		RelationshipType: header.RelationshipType,
	}, nil
}

func (r *ParentMessageReads) LatestReadCursorByOtherStaff(ctx context.Context, threadID, excludeAccountID int64, staffAccounts map[int64][]int64) (*communication.ParentReadCursorRecord, error) {
	cursor, err := r.inbox.LatestReadCursorByOtherStaff(ctx, threadID, excludeAccountID, staffAccounts)
	return cursorModel(cursor), err
}

func (r *ParentMessageReads) GuardianReadCursor(ctx context.Context, threadID int64) (*communication.ParentReadCursorRecord, error) {
	cursor, err := r.inbox.GuardianReadCursor(ctx, threadID)
	return cursorModel(cursor), err
}

func cursorModel(cursor *parentinbox.ParentReadCursor) *communication.ParentReadCursorRecord {
	if cursor == nil {
		return nil
	}
	return &communication.ParentReadCursorRecord{LastReadAt: cursor.LastReadAt, LastReadMessageID: cursor.LastReadMessageID}
}

func inboxModels(rows []*parentinbox.ParentInboxThread) ([]*communication.PortalInboxThread, error) {
	if rows == nil {
		return nil, nil
	}
	threads := make([]*communication.PortalInboxThread, 0, len(rows))
	for _, row := range rows {
		var facts map[string]any
		if err := json.Unmarshal(row.LastMessagePayload, &facts); err != nil && len(row.LastMessagePayload) > 0 {
			return nil, err
		}
		payload, err := values.ParentPayloadFromFacts(facts)
		if err != nil {
			return nil, err
		}
		thread := &communication.PortalInboxThread{
			ThreadID: row.ThreadID, TenantID: row.TenantID, StudentID: row.StudentID,
			StudentName: row.StudentName, SchoolClass: row.SchoolClass, GroupID: row.GroupID,
			GuardianAccountID: row.GuardianAccountID, GuardianName: row.GuardianName,
			RelationshipType: row.RelationshipType, LastMessageAt: row.LastMessageAt,
			LastSenderKind: row.LastSenderKind, LastMessageBody: row.LastMessageBody,
			LastMessageKind: row.LastMessageKind, LastEventType: row.LastEventType,
			LastRequestType: row.LastRequestType, LastRequestStatus: row.LastRequestStatus,
			LastMessagePayload:     payload,
			LastMessageReadByStaff: row.LastMessageReadByStaff,
			UnreadCount:            row.UnreadCount,
		}
		if row.ReadBoundAt != nil && row.ReadBoundMessageID != nil {
			thread.ReadBound = &communication.PortalReadCursorBound{ThreadID: row.ThreadID, ReadAt: *row.ReadBoundAt, MessageID: *row.ReadBoundMessageID}
		}
		threads = append(threads, thread)
	}
	return threads, nil
}
