package messaging

import (
	"context"
	"fmt"

	usersModels "github.com/moto-nrw/project-phoenix/models/users"
)

// MarkAllRead clears the caller's own unread numbers for every conversation
// the inbox shows them as unread (#3663). It uses the inbox's own unread
// filter, so it covers exactly the conversations the inbox counts and never a
// child the account may not read.
//
// Only the caller's personal clear boundary moves, each to the newest
// guardian message of the inbox snapshot. It is not a read (#3673): the read
// cursor stays, so the parent-facing "Von der OGS gelesen" receipt appears
// only once someone actually opens the conversation. The team handled
// boundary and a team-wide unread mark (#3654) stay as well: colleagues see no
// change, and a marked conversation stays marked until someone opens or
// answers it. Repeating the call changes nothing.
//
// It returns the caller's counter afterwards. It is above zero only when
// team-marked conversations remain, so the client can say why.
func (s *Service) MarkAllRead(ctx context.Context) (int, error) {
	if err := s.requireEnabled(ctx); err != nil {
		return 0, err
	}
	accountID := accountIDFromCtx(ctx)
	rows, err := s.ReadRepo.ListInboxForStaff(ctx, accountID, s.scope(ctx), true)
	if err != nil {
		return 0, fmt.Errorf("messaging: list unread inbox: %w", err)
	}
	boundsByTenant := map[int64][]usersModels.ReadCursorBound{}
	for _, row := range rows {
		if row.ReadBound != nil {
			boundsByTenant[row.TenantID] = append(boundsByTenant[row.TenantID], *row.ReadBound)
		}
	}
	for tenantID, bounds := range boundsByTenant {
		if _, err := s.ReadRepo.ClearUnreadForStaff(ctx, tenantID, accountID, bounds); err != nil {
			return 0, fmt.Errorf("messaging: mark all read: %w", err)
		}
	}
	return s.UnreadMessageCount(ctx)
}
