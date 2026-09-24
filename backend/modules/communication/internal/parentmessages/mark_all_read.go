package messaging

import (
	"context"
	"fmt"

	usersModels "github.com/moto-nrw/project-phoenix/models/users"
)

// MarkAllRead marks every conversation the staff member sees as unread in the
// inbox as read for their own account (#3663). It uses the inbox's own unread
// filter, so it covers exactly the conversations the inbox counts and never a
// child the account may not read.
//
// Only the caller's read cursors move, each to the newest guardian message by
// the rules of a single open. The team handled boundary and a team-wide unread
// mark (#3654) stay: colleagues see no change, and a marked conversation stays
// marked until someone opens or answers it. The parent-facing "Von der OGS
// gelesen" receipt follows the staff cursors, so it appears on the messages
// read here. Repeating the call changes nothing.
//
// It returns the caller's unread count afterwards. It is above zero only when
// team-marked conversations remain, so the client can say why.
func (s *Service) MarkAllRead(ctx context.Context) (int, error) {
	accountID := accountIDFromCtx(ctx)
	rows, err := s.ReadRepo.ListInboxForStaff(ctx, accountID, s.scope(ctx), true)
	if err != nil {
		return 0, fmt.Errorf("messaging: list unread inbox: %w", err)
	}
	threadsByTenant := map[int64][]int64{}
	byID := make(map[int64]*usersModels.InboxThread, len(rows))
	for _, row := range rows {
		threadsByTenant[row.TenantID] = append(threadsByTenant[row.TenantID], row.ThreadID)
		byID[row.ThreadID] = row
	}
	for tenantID, threadIDs := range threadsByTenant {
		advanced, err := s.ReadRepo.MarkThreadsReadForStaff(ctx, tenantID, accountID, threadIDs)
		if err != nil {
			return 0, fmt.Errorf("messaging: mark all read: %w", err)
		}
		for _, threadID := range advanced {
			if row := byID[threadID]; row != nil {
				// Wake the guardian's open chat so its "Gelesen" receipt updates,
				// the same push a single open sends when the cursor advanced.
				s.broadcastReadAfterCommit(ctx, inboxThreadRef(row))
			}
		}
	}
	return s.UnreadMessageCount(ctx)
}

// inboxThreadRef carries the identity of an inbox row the read broadcast needs.
func inboxThreadRef(row *usersModels.InboxThread) *usersModels.ParentMessageThread {
	thread := &usersModels.ParentMessageThread{StudentID: row.StudentID, GuardianAccountID: row.GuardianAccountID}
	thread.ID = row.ThreadID
	thread.SetTenantID(row.TenantID)
	return thread
}
