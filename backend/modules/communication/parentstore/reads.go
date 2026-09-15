package parentstore

import (
	"context"
	"time"

	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/communication/internal/adapters/parentinbox"
	"github.com/moto-nrw/project-phoenix/modules/communication/internal/adapters/parentpostgres"
	"github.com/moto-nrw/project-phoenix/modules/communication/internal/domain"
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
	inbox   *parentinbox.Projection
}

// NewParentMessageReadRepository builds the read-side store.
func NewParentMessageReadRepository(db *bun.DB) *ParentMessageReads {
	return &ParentMessageReads{
		cursors: parentpostgres.NewReadCursorStore(postgresDatabase(db)),
		threads: parentpostgres.NewThreadStore(postgresDatabase(db)),
		inbox:   parentinbox.New(inboxDatabase(db)),
	}
}

func (r *ParentMessageReads) MarkReadUpTo(ctx context.Context, tenantID, threadID, accountID int64, readAt time.Time, readMessageID int64) (bool, error) {
	return r.cursors.MarkReadUpTo(ctx, tenantID, threadID, accountID, readAt, readMessageID)
}

func (r *ParentMessageReads) MarkStaffHandledUpTo(ctx context.Context, tenantID, threadID int64, handledAt time.Time, handledMessageID int64) error {
	return r.threads.MarkStaffHandledUpTo(ctx, tenantID, threadID, handledAt, handledMessageID)
}

func (r *ParentMessageReads) UnreadMessageCountForStaff(ctx context.Context, accountID int64, allStudents bool) (int, error) {
	return r.inbox.UnreadMessageCountForStaff(ctx, accountID, allStudents)
}

func (r *ParentMessageReads) ListInboxForStaff(ctx context.Context, accountID int64, allStudents, onlyUnread bool) ([]*usersModels.InboxThread, error) {
	rows, err := r.inbox.ListInboxForStaff(ctx, accountID, allStudents, onlyUnread)
	return inboxModels(rows), err
}

func (r *ParentMessageReads) ListThreadsForStudent(ctx context.Context, accountID, studentID int64) ([]*usersModels.InboxThread, error) {
	rows, err := r.inbox.ListThreadsForStudent(ctx, accountID, studentID)
	return inboxModels(rows), err
}

func (r *ParentMessageReads) ListThreadsForGuardianStudentWithStaff(ctx context.Context, accountID, studentID int64, staffAccounts map[int64][]int64) ([]*usersModels.InboxThread, error) {
	rows, err := r.inbox.ListThreadsForGuardianStudent(ctx, accountID, studentID, staffAccounts)
	return inboxModels(rows), err
}

func (r *ParentMessageReads) ListThreadsForGuardianTenantsWithStaff(ctx context.Context, accountID int64, tenantIDs []int64, staffAccounts map[int64][]int64) ([]*usersModels.InboxThread, error) {
	rows, err := r.inbox.ListThreadsForGuardianTenants(ctx, accountID, tenantIDs, staffAccounts)
	return inboxModels(rows), err
}

func (r *ParentMessageReads) UnreadMessageCountForGuardianTenants(ctx context.Context, accountID int64, tenantIDs []int64) (int, error) {
	return r.inbox.UnreadMessageCountForGuardianTenants(ctx, accountID, tenantIDs)
}

func (r *ParentMessageReads) FindThreadHeader(ctx context.Context, threadID int64) (*usersModels.ThreadHeader, error) {
	header, err := r.inbox.FindThreadHeader(ctx, threadID)
	if err != nil || header == nil {
		return nil, err
	}
	return &usersModels.ThreadHeader{
		StudentName: header.StudentName, GuardianName: header.GuardianName,
		RelationshipType: header.RelationshipType,
	}, nil
}

func (r *ParentMessageReads) LatestReadCursorByOtherStaff(ctx context.Context, threadID, excludeAccountID int64, staffAccounts map[int64][]int64) (*usersModels.ReadCursor, error) {
	cursor, err := r.inbox.LatestReadCursorByOtherStaff(ctx, threadID, excludeAccountID, staffAccounts)
	return cursorModel(cursor), err
}

func (r *ParentMessageReads) GuardianReadCursor(ctx context.Context, threadID int64) (*usersModels.ReadCursor, error) {
	cursor, err := r.inbox.GuardianReadCursor(ctx, threadID)
	return cursorModel(cursor), err
}

func cursorModel(cursor *domain.ParentReadCursor) *usersModels.ReadCursor {
	if cursor == nil {
		return nil
	}
	return &usersModels.ReadCursor{LastReadAt: cursor.LastReadAt, LastReadMessageID: cursor.LastReadMessageID}
}

func inboxModels(rows []*domain.ParentInboxThread) []*usersModels.InboxThread {
	if rows == nil {
		return nil
	}
	threads := make([]*usersModels.InboxThread, 0, len(rows))
	for _, row := range rows {
		threads = append(threads, &usersModels.InboxThread{
			ThreadID: row.ThreadID, TenantID: row.TenantID, StudentID: row.StudentID,
			StudentName: row.StudentName, SchoolClass: row.SchoolClass, GroupID: row.GroupID,
			GuardianAccountID: row.GuardianAccountID, GuardianName: row.GuardianName,
			RelationshipType: row.RelationshipType, LastMessageAt: row.LastMessageAt,
			LastSenderKind: row.LastSenderKind, LastMessageBody: row.LastMessageBody,
			LastMessageKind: row.LastMessageKind, LastEventType: row.LastEventType,
			LastRequestType: row.LastRequestType, LastRequestStatus: row.LastRequestStatus,
			LastMessagePayload:     row.LastMessagePayload,
			LastMessageReadByStaff: row.LastMessageReadByStaff,
			UnreadCount:            row.UnreadCount,
		})
	}
	return threads
}
