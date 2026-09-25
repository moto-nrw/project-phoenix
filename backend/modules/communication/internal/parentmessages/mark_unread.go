package messaging

import (
	"context"
	"fmt"
	"log/slog"

	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// MarkUnread marks the conversation unread for the whole team (#3654). Every
// staff member who may read the child sees it as unread again, including the
// person who marked it, until someone opens or answers it.
//
// It checks exactly what reading the thread checks. The mark is its own state
// on the thread: the personal read cursors and the team handled boundary stay
// where they are, so the parent-facing "Von der OGS gelesen" receipt does not
// change. Repeating the call keeps the thread marked and only renews the mark,
// so an open that loaded the older mark cannot remove the newer one.
func (s *Service) MarkUnread(ctx context.Context, threadID int64) error {
	thread, err := s.loadAuthorizedThread(ctx, threadID)
	if err != nil {
		return err
	}
	if err := s.ReadRepo.MarkStaffUnread(ctx, thread.TenantID, thread.ID, accountIDFromCtx(ctx)); err != nil {
		return fmt.Errorf("messaging: mark unread: %w", err)
	}
	s.broadcastStaffUnreadAfterCommit(ctx, thread)
	return nil
}

// clearStaffUnreadMark ends the team-wide unread mark when a staff member opens
// or answers the conversation. It only removes the mark the thread row carried
// when this request loaded it, so a colleague's newer mark survives. A cleared
// mark wakes the other staff tabs so their badges drop right away.
func (s *Service) clearStaffUnreadMark(ctx context.Context, thread *usersModels.ParentMessageThread) error {
	if thread == nil || thread.StaffMarkedUnreadAt == nil {
		return nil
	}
	cleared, err := s.ReadRepo.ClearStaffUnreadMark(ctx, thread.TenantID, thread.ID, *thread.StaffMarkedUnreadAt)
	if err != nil {
		return fmt.Errorf("messaging: clear unread mark: %w", err)
	}
	if cleared {
		s.broadcastStaffUnreadAfterCommit(ctx, thread)
	}
	return nil
}

// broadcastStaffUnreadAfterCommit wakes the staff tabs of the school so their
// unread badges, inboxes and child cards refetch. It uses its own event type:
// an open conversation must not reload on it, because loading would end the
// mark at once. No guardian is addressed; parents see no change.
func (s *Service) broadcastStaffUnreadAfterCommit(ctx context.Context, thread *usersModels.ParentMessageThread) {
	if s.Broadcaster == nil || thread.TenantID <= 0 {
		return
	}
	tenantID := thread.TenantID
	threadID := thread.ID
	tenant.RegisterAfterCommit(ctx, func() {
		if err := s.Broadcaster.BroadcastParentMessage(tenantID, 0, realtime.NewParentMessageUnreadChangedEvent()); err != nil {
			s.Logger.Warn("messaging: failed to broadcast unread mark change",
				slog.Int64("tenant_id", tenantID),
				slog.Int64("thread_id", threadID),
				slog.String("error", err.Error()),
			)
		}
	})
}
