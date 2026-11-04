package notifications

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

const staffParentMessageCooldown = time.Minute

// StaffParentMessageReport identifies one guardian-authored thread message.
// IDs are used only for recipient resolution, debounce and the authenticated
// deep link; none is rendered in the notification copy.
type StaffParentMessageReport struct {
	TenantID       int64
	ThreadID       int64
	MessageID      int64
	StudentID      int64
	ActorAccountID int64
	// Leadership marks a message in a conversation only the leadership reads
	// (#5200): only the leadership persons are told, never the group
	// supervisors.
	Leadership bool
}

// LeadershipAccounts lists the accounts of the leadership persons of the
// school in context: the staff whose account holds parent_messages:leadership.
// The root binds it.
type LeadershipAccounts interface {
	LeadershipAccountIDs(ctx context.Context) ([]int64, error)
}

// StaffParentMessageNotifier tells responsible staff that a guardian wrote in
// a child-related thread.
type StaffParentMessageNotifier interface {
	// NotifyStaffParentMessage enqueues the delivery in the caller's transaction.
	// A caller without an ambient transaction gets a narrow compatibility
	// transaction around recipient resolution and enqueue.
	NotifyStaffParentMessage(ctx context.Context, report StaffParentMessageReport) error
}

// StaffMessageClaims debounces the staff notification of one thread: a
// claim succeeds at most once per cooldown. The root binds it to the
// thread store.
type StaffMessageClaims interface {
	ClaimStaffMessageNotification(ctx context.Context, threadID int64, cooldown time.Duration) (bool, error)
}

type staffParentMessageNotifier struct {
	notifier   Service
	recipients StaffRecipientResolver
	threads    StaffMessageClaims
	leaders    LeadershipAccounts
	db         *bun.DB
	logger     *slog.Logger
}

// NewStaffParentMessageNotifier builds the parent-message producer.
// Without leaders a leadership message reaches nobody rather than the team.
func NewStaffParentMessageNotifier(
	notifier Service,
	recipients StaffRecipientResolver,
	threads StaffMessageClaims,
	leaders LeadershipAccounts,
	db *bun.DB,
	logger *slog.Logger,
) StaffParentMessageNotifier {
	return &staffParentMessageNotifier{
		notifier:   notifier,
		recipients: recipients,
		threads:    threads,
		leaders:    leaders,
		db:         db,
		logger:     logger,
	}
}

func (n *staffParentMessageNotifier) getLogger() *slog.Logger {
	if n.logger == nil {
		return slog.Default()
	}
	return n.logger
}

func (n *staffParentMessageNotifier) NotifyStaffParentMessage(ctx context.Context, report StaffParentMessageReport) error {
	if report.TenantID <= 0 {
		return nil
	}

	var err error
	if n.db == nil {
		err = n.notify(ctx, report)
	} else if _, active := tenant.TransactionFromContext(ctx); active {
		err = n.notify(ctx, report)
	} else {
		err = tenant.WithTenantTx(ctx, n.db, report.TenantID, func(txCtx context.Context, _ bun.Tx) error {
			return n.notify(txCtx, report)
		})
	}
	if err == nil || errors.Is(err, ErrDisabled) || errors.Is(err, ErrOutsideActiveWindow) {
		return nil
	}
	n.getLogger().Warn("staff parent-message notification failed",
		slog.Int64("tenant_id", report.TenantID),
		slog.Int64("thread_id", report.ThreadID),
		slog.String("error", err.Error()),
	)
	return err
}

func (n *staffParentMessageNotifier) notify(ctx context.Context, report StaffParentMessageReport) error {
	if n.notifier == nil || n.recipients == nil || n.threads == nil {
		return nil
	}
	if report.ThreadID <= 0 || report.MessageID <= 0 || report.StudentID <= 0 {
		return nil
	}

	accountIDs, err := n.resolveAccountIDs(ctx, report)
	if err != nil || len(accountIDs) == 0 {
		return err
	}
	claimed, err := n.threads.ClaimStaffMessageNotification(ctx, report.ThreadID, staffParentMessageCooldown)
	if err != nil || !claimed {
		return err
	}
	return n.notifier.Notify(ctx, staffParentMessageEvent(report, accountIDs))
}

func (n *staffParentMessageNotifier) resolveAccountIDs(ctx context.Context, report StaffParentMessageReport) ([]int64, error) {
	request := StaffRecipientRequest{
		StudentIDs:         []int64{report.StudentID},
		NotificationType:   TypeStaffParentMessage,
		ExcludedAccountIDs: []int64{report.ActorAccountID},
	}
	if report.Leadership {
		leaders, err := n.leadershipAccountIDs(ctx)
		if err != nil {
			return nil, err
		}
		request.OnlyAccounts, request.AccountIDs = true, leaders
	}
	scopes, err := n.recipients.Resolve(ctx, request)
	if err != nil {
		return nil, err
	}
	accountIDs := make([]int64, 0, len(scopes))
	for _, scope := range scopes {
		accountIDs = append(accountIDs, scope.AccountID)
	}
	return accountIDs, nil
}

func (n *staffParentMessageNotifier) leadershipAccountIDs(ctx context.Context) ([]int64, error) {
	if n.leaders == nil {
		return nil, nil
	}
	ids, err := n.leaders.LeadershipAccountIDs(ctx)
	if err != nil {
		return nil, fmt.Errorf("resolve leadership accounts: %w", err)
	}
	return ids, nil
}

func staffParentMessageEvent(report StaffParentMessageReport, accountIDs []int64) Event {
	return Event{
		Type:           TypeStaffParentMessage,
		IdempotencyKey: fmt.Sprintf("staff-parent-message:%d:%d", report.ThreadID, report.MessageID),
		RelatedType:    "parent_message_thread",
		RelatedID:      report.ThreadID,
		Title:          "Neue Nachricht von Eltern",
		Body:           "Eltern haben der OGS eine neue Nachricht geschrieben.",
		DeepLink:       fmt.Sprintf("/messages/%d", report.ThreadID),
		Priority:       PriorityNormal,
		Audience: Audience{
			TenantID:        report.TenantID,
			Scope:           ScopeStaff,
			StaffAccountIDs: accountIDs,
		},
	}
}
