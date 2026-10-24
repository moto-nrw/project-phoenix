// Package messaging holds the staff-side (tenant portal) business logic for
// the parent-OGS messaging feature: the central inbox, per-thread chats, and
// staff replies / new conversations. A thread is one conversation between the
// OGS and a single guardian about one child (email-like, identified by a
// subject). The parent side lives in workflows/parentportal/messaging; both operate on the same
// users.parent_message_* tables.
//
// All methods run inside the request's tenant transaction (routes are mounted
// with TenantTxMiddleware), so tenant scoping is enforced by RLS.
package messaging

import (
	"context"
	"fmt"
	"log/slog"
)

func (s *Service) finishStaffReply(ctx context.Context, thread *ParentMessageThread, accountID, handledUpToMessageID, messageID int64, visibleMessages []*ParentMessage) ([]*ParentMessage, error) {
	messages, err := s.MessageRepo.ListByThread(ctx, thread.ID, 0)
	if err != nil {
		return nil, fmt.Errorf("messaging: list messages: %w", err)
	}
	if err := MarkStaffHandledToVisible(ctx, s.ReadRepo, thread.TenantID, thread.ID, handledUpToMessageID, visibleMessages); err != nil {
		return nil, fmt.Errorf("messaging: mark handled: %w", err)
	}
	// Advance the staff reader's cursor over this returned snapshot, exactly as the
	// GET path (markReadAndBuild) does. The client applies this list with
	// revalidate:false, so a guardian message that became visible since the last GET
	// (an SSE echo missed/delayed before this send) would otherwise stay lit in the
	// sidebar/inbox unread count though it is already on screen. Bounded to the
	// newest GUARDIAN row in the snapshot (never NOW(), never our own just-sent
	// message), so it can't leap the cursor to ~now and swallow a guardian message
	// committing concurrently in a still-open tx. See parentmessaging.MarkReadToNewest.
	if _, err := MarkReadToNewest(ctx, s.ReadRepo, thread.TenantID, thread.ID, accountID, true, messages); err != nil {
		return nil, fmt.Errorf("messaging: mark read: %w", err)
	}
	if err := s.clearStaffUnreadMark(ctx, thread); err != nil {
		return nil, err
	}
	// Re-stamp the "Gelesen" receipts on the returned snapshot: the client applies
	// this list optimistically (revalidate:false), so without it the staff's older,
	// guardian-read messages would lose their receipt until the next GET/SSE refresh.
	// The just-sent message is unread by the guardian (cursor unchanged by our send),
	// so it correctly stays unstamped.
	DecorateGuardianReadReceipts(ctx, s.ReadRepo, s.Logger, thread.ID, messages)
	if err := s.decorateGroupThreadTitles(ctx, messages); err != nil {
		return nil, err
	}
	s.broadcastAfterCommit(ctx, thread)
	s.notifyGuardianDevice(ctx, thread, messageID)
	return messages, nil
}

func (s *Service) guardianPushLocale(ctx context.Context, thread *ParentMessageThread) string {
	locale := ""
	if s.GuardianProfiles != nil {
		profile, profileErr := s.GuardianProfiles.FindMessageGuardianProfile(ctx, thread.GuardianAccountID)
		if profileErr != nil {
			s.Logger.Warn("messaging: load guardian locale for push failed",
				slog.Int64("thread_id", thread.ID),
				slog.String("error", profileErr.Error()),
			)
		} else if profile != nil && profile.PortalLocale != nil {
			locale = *profile.PortalLocale
		}
	}
	return locale
}
