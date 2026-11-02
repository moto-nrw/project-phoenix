package owners

import (
	"context"
	"encoding/json"

	"github.com/moto-nrw/project-phoenix/modules/communication"
	parentinbox "github.com/moto-nrw/project-phoenix/modules/parentmessageinbox/contract"
)

type parentInboxProjection interface {
	ListInboxForStaff(ctx context.Context, accountID int64, reach parentinbox.StaffReach, onlyUnread bool) ([]*parentinbox.ParentInboxThread, error)
	ListThreadsForStudent(ctx context.Context, accountID, studentID int64, reach parentinbox.StaffReach) ([]*parentinbox.ParentInboxThread, error)
	ListThreadsForGuardianStudent(ctx context.Context, accountID, studentID int64, staffAccounts map[int64][]int64) ([]*parentinbox.ParentInboxThread, error)
	ListThreadsForGuardianTenants(ctx context.Context, accountID int64, tenantIDs []int64, staffAccounts map[int64][]int64) ([]*parentinbox.ParentInboxThread, error)
	UnreadMessageCountForStaff(ctx context.Context, accountID int64, reach parentinbox.StaffReach) (int, error)
	UnreadMessageCountForStaffInGroups(ctx context.Context, accountID int64, reach parentinbox.StaffReach, groupIDs []int64) (int, error)
	UnreadMessageCountForGuardianTenants(ctx context.Context, accountID int64, tenantIDs []int64) (int, error)
	FindThreadHeader(ctx context.Context, threadID int64) (*parentinbox.ParentThreadHeader, error)
	LatestReadCursorByOtherStaff(ctx context.Context, threadID, excludeAccountID int64, staffAccounts map[int64][]int64) (*parentinbox.ParentReadCursor, error)
	GuardianReadCursor(ctx context.Context, threadID int64) (*parentinbox.ParentReadCursor, error)
}

type communicationParentInbox struct{ source parentInboxProjection }

// staffReach hands Communication's reader reach to the projection.
func staffReach(reach communication.ParentMessageStaffReach) parentinbox.StaffReach {
	return parentinbox.StaffReach{AllStudents: reach.AllStudents, Leadership: reach.Leadership}
}

func cursorModel(cursor *parentinbox.ParentReadCursor) *communication.ParentReadCursorRecord {
	if cursor == nil {
		return nil
	}
	return &communication.ParentReadCursorRecord{LastReadAt: cursor.LastReadAt, LastReadMessageID: cursor.LastReadMessageID}
}
func parentInboxPayload(facts map[string]any) (map[string]json.RawMessage, error) {
	if facts == nil {
		return nil, nil
	}
	raw, err := json.Marshal(facts)
	if err != nil {
		return nil, err
	}
	var out map[string]json.RawMessage
	err = json.Unmarshal(raw, &out)
	return out, err
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
		payload, err := parentInboxPayload(facts)
		if err != nil {
			return nil, err
		}
		thread := &communication.PortalInboxThread{
			ThreadID: row.ThreadID, TenantID: row.TenantID, StudentID: row.StudentID,
			StudentName: row.StudentName, SchoolClass: row.SchoolClass, GroupID: row.GroupID,
			GuardianAccountID: row.GuardianAccountID, GuardianName: row.GuardianName,
			RelationshipType: row.RelationshipType, Audience: row.Audience, LastMessageAt: row.LastMessageAt,
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
func (r communicationParentInbox) ListInboxForStaff(ctx context.Context, accountID int64, reach communication.ParentMessageStaffReach, onlyUnread bool) ([]*communication.PortalInboxThread, error) {
	rows, err := r.source.ListInboxForStaff(ctx, accountID, staffReach(reach), onlyUnread)
	mapped, mapErr := inboxModels(rows)
	if err != nil {
		return mapped, err
	}
	return mapped, mapErr
}
func (r communicationParentInbox) ListThreadsForStudent(ctx context.Context, accountID, studentID int64, reach communication.ParentMessageStaffReach) ([]*communication.PortalInboxThread, error) {
	rows, err := r.source.ListThreadsForStudent(ctx, accountID, studentID, staffReach(reach))
	mapped, mapErr := inboxModels(rows)
	if err != nil {
		return mapped, err
	}
	return mapped, mapErr
}
func (r communicationParentInbox) ListThreadsForGuardianStudentWithStaff(ctx context.Context, accountID, studentID int64, staffAccounts map[int64][]int64) ([]*communication.PortalInboxThread, error) {
	rows, err := r.source.ListThreadsForGuardianStudent(ctx, accountID, studentID, staffAccounts)
	mapped, mapErr := inboxModels(rows)
	if err != nil {
		return mapped, err
	}
	return mapped, mapErr
}
func (r communicationParentInbox) ListThreadsForGuardianTenantsWithStaff(ctx context.Context, accountID int64, tenantIDs []int64, staffAccounts map[int64][]int64) ([]*communication.PortalInboxThread, error) {
	rows, err := r.source.ListThreadsForGuardianTenants(ctx, accountID, tenantIDs, staffAccounts)
	mapped, mapErr := inboxModels(rows)
	if err != nil {
		return mapped, err
	}
	return mapped, mapErr
}
func (r communicationParentInbox) UnreadMessageCountForStaff(ctx context.Context, accountID int64, reach communication.ParentMessageStaffReach) (int, error) {
	return r.source.UnreadMessageCountForStaff(ctx, accountID, staffReach(reach))
}
func (r communicationParentInbox) UnreadMessageCountForStaffInGroups(ctx context.Context, accountID int64, reach communication.ParentMessageStaffReach, groupIDs []int64) (int, error) {
	return r.source.UnreadMessageCountForStaffInGroups(ctx, accountID, staffReach(reach), groupIDs)
}
func (r communicationParentInbox) UnreadMessageCountForGuardianTenants(ctx context.Context, accountID int64, tenantIDs []int64) (int, error) {
	return r.source.UnreadMessageCountForGuardianTenants(ctx, accountID, tenantIDs)
}
func (r communicationParentInbox) FindThreadHeader(ctx context.Context, threadID int64) (*communication.ParentThreadHeaderRecord, error) {
	header, err := r.source.FindThreadHeader(ctx, threadID)
	if err != nil || header == nil {
		return nil, err
	}
	return &communication.ParentThreadHeaderRecord{StudentName: header.StudentName, GuardianName: header.GuardianName, RelationshipType: header.RelationshipType}, nil
}
func (r communicationParentInbox) LatestReadCursorByOtherStaff(ctx context.Context, threadID, excludeAccountID int64, staffAccounts map[int64][]int64) (*communication.ParentReadCursorRecord, error) {
	cursor, err := r.source.LatestReadCursorByOtherStaff(ctx, threadID, excludeAccountID, staffAccounts)
	return cursorModel(cursor), err
}
func (r communicationParentInbox) GuardianReadCursor(ctx context.Context, threadID int64) (*communication.ParentReadCursorRecord, error) {
	cursor, err := r.source.GuardianReadCursor(ctx, threadID)
	return cursorModel(cursor), err
}
