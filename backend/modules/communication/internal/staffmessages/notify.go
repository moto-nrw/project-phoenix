package staffmessaging

import (
	"context"
	"fmt"
	"log/slog"

	deliveryPorts "github.com/moto-nrw/project-phoenix/modules/communication/internal/application"
	"github.com/moto-nrw/project-phoenix/realtime"
)

// notifyAfterCommit wakes the conversation's participants and pushes to the
// recipient's devices. Both are side effects of a successful send: never let
// either fail the send itself.
func (s *Service) notifyAfterCommit(ctx context.Context, thread *StaffMessageThread, message *StaffMessage, senderAccountID int64) {
	if thread == nil || message == nil || message.SendReplayed {
		return
	}
	s.broadcastAfterCommit(ctx, thread, senderAccountID)
	s.notifyRecipients(ctx, thread, message.ID, senderAccountID)
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
func (s *Service) broadcastAfterCommit(ctx context.Context, thread *StaffMessageThread, senderAccountID int64) {
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

	s.runtime.RegisterAfterCommit(ctx, func() {
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
func (s *Service) notifyRecipients(ctx context.Context, thread *StaffMessageThread, messageID, senderAccountID int64) {
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

	candidates := s.reachablePushRecipients(ctx, thread, participants, senderAccountID)
	if len(candidates) == 0 {
		return
	}

	// The single enforcement point for "nothing without consent": build the
	// relation-based candidate set first, then narrow it here — never the other
	// way round.
	optedIn, err := s.Preferences.FilterOptedIn(ctx, deliveryPorts.TypeStaffMessage, candidates)
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

	s.sendStaffMessagePush(ctx, thread, messageID, optedIn)
}

func (s *Service) reachablePushRecipients(ctx context.Context, thread *StaffMessageThread, participants []int64, senderAccountID int64) []int64 {
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
	return candidates
}

func (s *Service) sendStaffMessagePush(ctx context.Context, thread *StaffMessageThread, messageID int64, optedIn []int64) {
	title, body := "Neue Nachricht aus dem Team", "Jemand aus dem Team hat Ihnen geschrieben."
	if thread.Kind == groupKind {
		// Not even the group name: it can say more than the lock screen should.
		title, body = "Neue Nachricht im Gruppenchat", "Jemand hat in einer Gruppe des Teams geschrieben."
	}
	err := s.Notifier.Notify(ctx, deliveryPorts.NotificationEvent{
		Type:           deliveryPorts.TypeStaffMessage,
		IdempotencyKey: fmt.Sprintf("staff-message:%d:%d", thread.ID, messageID),
		RelatedType:    "staff_message_thread", RelatedID: thread.ID,
		Title:    title,
		Body:     body,
		DeepLink: fmt.Sprintf("/team-chat/%d", thread.ID),
		// The same conversation in "moto schule" (#2208); the school host
		// strips the /school prefix itself.
		SchoolDeepLink: fmt.Sprintf("/school/nachrichten/%d", thread.ID),
		Priority:       deliveryPorts.PriorityNormal,
		Audience: deliveryPorts.NotificationAudience{
			TenantID:        thread.TenantID,
			Scope:           deliveryPorts.ScopeStaff,
			StaffAccountIDs: optedIn,
		},
	}.Public())
	switch {
	case s.deliveryPolicy.IsNotificationsDisabled(err), s.deliveryPolicy.IsOutsideActiveWindow(err):
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
