package staffmessaging

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/services/notifications"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// notifyAfterCommit wakes the conversation's participants and pushes to the
// recipient's devices. Both are side effects of a successful send: never let
// either fail the send itself.
func (s *Service) notifyAfterCommit(ctx context.Context, thread *usersModels.StaffMessageThread, message *usersModels.StaffMessage, senderAccountID int64) {
	if thread == nil || message == nil {
		return
	}
	s.broadcastAfterCommit(ctx, thread, senderAccountID)
	s.notifyRecipients(ctx, thread, senderAccountID)
}

// broadcastAfterCommit queues the SSE wake-up to fire only AFTER the request
// transaction commits. Firing inside the transaction would wake the recipient's
// tab before the message is visible to any other transaction, so its refetch
// would come back without the message and the badge would stay stale until the
// next event.
//
// The fan-out addresses the thread's participants only. Staff and school
// portal clients use separate indexes, so a chat refresh reaches both
// immediately and never depends on notification consent or delivery hours.
func (s *Service) broadcastAfterCommit(ctx context.Context, thread *usersModels.StaffMessageThread, senderAccountID int64) {
	if s.Broadcaster == nil {
		return
	}
	tenantID := thread.TenantID
	threadID := thread.ID

	participants, err := s.ThreadRepo.ParticipantAccountIDs(ctx, threadID)
	if err != nil {
		s.Logger.Warn("staffmessaging: resolve participants for broadcast failed",
			slog.Int64("thread_id", threadID),
			slog.String("error", err.Error()),
		)
		return
	}

	tenant.RegisterAfterCommit(ctx, func() {
		event := realtime.NewStaffMessageEvent(senderAccountID, threadID)
		if err := s.Broadcaster.BroadcastToStaffAccounts(tenantID, participants, event); err != nil {
			s.Logger.Warn("staffmessaging: broadcast failed",
				slog.Int64("thread_id", threadID),
				slog.String("error", err.Error()),
			)
		}
		if err := s.Broadcaster.BroadcastToSchoolAccounts(tenantID, participants, event); err != nil {
			s.Logger.Warn("staffmessaging: school broadcast failed",
				slog.Int64("thread_id", threadID),
				slog.String("error", err.Error()),
			)
		}
	})
}

// notifyRecipients pushes the message to the OTHER participants' devices.
//
// Not wrapped in an after-commit hook, deliberately: Notify defers its own
// fan-out until the surrounding tenant transaction commits, and the consent
// read it performs has to happen inside that transaction to be RLS-scoped at
// all.
//
// The copy names neither the sender nor the message. A push payload leaves the
// backend and is rendered on a lock screen; the conversation behind the deep
// link is authenticated, and that is where the content belongs.
func (s *Service) notifyRecipients(ctx context.Context, thread *usersModels.StaffMessageThread, senderAccountID int64) {
	if s.Notifier == nil || s.Preferences == nil {
		return
	}

	participants, err := s.ThreadRepo.ParticipantAccountIDs(ctx, thread.ID)
	if err != nil {
		s.Logger.Warn("staffmessaging: resolve participants for push failed",
			slog.Int64("thread_id", thread.ID),
			slog.String("error", err.Error()),
		)
		return
	}

	// Zusaetzlich zur Einwilligung: nur an erreichbares Personal. Der Sendepfad
	// prueft das bereits, aber eine Zustellung an ein inzwischen deaktiviertes
	// Konto waere der teuerste Weg, das zu bemerken - ein Push landet auf einem
	// Sperrbildschirm.
	candidates := make([]int64, 0, len(participants))
	for _, id := range participants {
		if id == senderAccountID {
			continue
		}
		reachable, err := s.ReadRepo.IsMessageableStaff(ctx, id)
		if err != nil {
			s.Logger.Warn("staffmessaging: recipient reachability check failed",
				slog.Int64("thread_id", thread.ID),
				slog.String("error", err.Error()),
			)
			continue
		}
		if reachable {
			candidates = append(candidates, id)
		}
	}
	if len(candidates) == 0 {
		return
	}

	// The single enforcement point for "nothing without consent": build the
	// relation-based candidate set first, then narrow it here — never the other
	// way round.
	optedIn, err := s.Preferences.FilterOptedIn(ctx, notifications.TypeStaffMessage, candidates)
	if err != nil {
		s.Logger.Warn("staffmessaging: filter opted-in recipients failed",
			slog.Int64("thread_id", thread.ID),
			slog.String("error", err.Error()),
		)
		return
	}
	if len(optedIn) == 0 {
		return
	}

	err = s.Notifier.Notify(ctx, notifications.Event{
		Type:     notifications.TypeStaffMessage,
		Title:    "Neue Nachricht aus dem Team",
		Body:     "Jemand aus dem Team hat Ihnen geschrieben.",
		DeepLink: fmt.Sprintf("/team-chat/%d", thread.ID),
		// The same conversation in "moto schule" (#2208); the school host
		// strips the /school prefix itself.
		SchoolDeepLink: fmt.Sprintf("/school/nachrichten/%d", thread.ID),
		Priority:       notifications.PriorityNormal,
		Audience: notifications.Audience{
			TenantID:        thread.TenantID,
			Scope:           notifications.ScopeStaff,
			StaffAccountIDs: optedIn,
		},
	})
	switch {
	case errors.Is(err, notifications.ErrDisabled), errors.Is(err, notifications.ErrOutsideActiveWindow):
		s.Logger.Info("staffmessaging: push suppressed by tenant notification gate",
			slog.Int64("thread_id", thread.ID),
			slog.String("reason", err.Error()),
		)
	case err != nil:
		s.Logger.Warn("staffmessaging: push failed",
			slog.Int64("thread_id", thread.ID),
			slog.String("error", err.Error()),
		)
	}
}
