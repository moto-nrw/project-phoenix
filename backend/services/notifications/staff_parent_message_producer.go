package notifications

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	userModel "github.com/moto-nrw/project-phoenix/models/users"
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
	StudentID      int64
	ActorAccountID int64
}

// StaffParentMessageNotifier tells responsible staff that a guardian wrote in
// a child-related thread.
type StaffParentMessageNotifier interface {
	// NotifyStaffParentMessage is fire-and-forget. The message is already
	// committed when callers invoke it, so notification failures are only logged.
	NotifyStaffParentMessage(ctx context.Context, report StaffParentMessageReport)
}

type staffParentMessageNotifier struct {
	notifier   Service
	recipients StaffRecipientResolver
	threads    userModel.ParentMessageThreadRepository
	db         *bun.DB
	logger     *slog.Logger
}

// NewStaffParentMessageNotifier builds the parent-message producer.
func NewStaffParentMessageNotifier(
	notifier Service,
	recipients StaffRecipientResolver,
	threads userModel.ParentMessageThreadRepository,
	db *bun.DB,
	logger *slog.Logger,
) StaffParentMessageNotifier {
	return &staffParentMessageNotifier{
		notifier:   notifier,
		recipients: recipients,
		threads:    threads,
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

func (n *staffParentMessageNotifier) NotifyStaffParentMessage(ctx context.Context, report StaffParentMessageReport) {
	if report.TenantID <= 0 {
		return
	}

	var err error
	if n.db == nil {
		err = n.notify(ctx, report)
	} else {
		err = tenant.WithTenantTx(ctx, n.db, report.TenantID, func(txCtx context.Context, _ bun.Tx) error {
			return n.notify(txCtx, report)
		})
	}
	if err == nil || errors.Is(err, ErrDisabled) || errors.Is(err, ErrOutsideActiveWindow) {
		return
	}
	n.getLogger().Warn("staff parent-message notification failed",
		slog.Int64("tenant_id", report.TenantID),
		slog.Int64("thread_id", report.ThreadID),
		slog.String("error", err.Error()),
	)
}

func (n *staffParentMessageNotifier) notify(ctx context.Context, report StaffParentMessageReport) error {
	if n.notifier == nil || n.recipients == nil || n.threads == nil {
		return nil
	}
	if report.ThreadID <= 0 || report.StudentID <= 0 {
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
	scopes, err := n.recipients.Resolve(ctx, StaffRecipientRequest{
		StudentIDs:         []int64{report.StudentID},
		NotificationType:   TypeStaffParentMessage,
		ExcludedAccountIDs: []int64{report.ActorAccountID},
	})
	if err != nil {
		return nil, err
	}
	accountIDs := make([]int64, 0, len(scopes))
	for _, scope := range scopes {
		accountIDs = append(accountIDs, scope.AccountID)
	}
	return accountIDs, nil
}

func staffParentMessageEvent(report StaffParentMessageReport, accountIDs []int64) Event {
	return Event{
		Type:     TypeStaffParentMessage,
		Title:    "Neue Nachricht von Eltern",
		Body:     "Eltern haben der OGS eine neue Nachricht geschrieben.",
		DeepLink: fmt.Sprintf("/messages/%d", report.ThreadID),
		Priority: PriorityNormal,
		Audience: Audience{
			TenantID:        report.TenantID,
			Scope:           ScopeStaff,
			StaffAccountIDs: accountIDs,
		},
	}
}
