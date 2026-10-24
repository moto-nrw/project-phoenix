package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	enrollmentNative "github.com/moto-nrw/project-phoenix/modules/enrollment"
)

var _ enrollmentNative.SettledEnrollmentRetention = (*RejectedCleanup)(nil)

// settledCutoffs are the instants before which a withdrawal and a rejection
// are past their windows.
type settledCutoffs struct {
	withdrawn, rejected time.Time
	withdrawnDays       int
}

// CleanupSettledEnrollments deletes the withdrawn and lapsed requests no
// child was created from once the school's window after settling passed, and
// the invitations to enroll late settled before it (#3960). Each request goes
// the way of a rejected one: its mails are cancelled and the deletion is
// recorded per request.
func (s *RejectedCleanup) CleanupSettledEnrollments(ctx context.Context) (enrollmentNative.SettledEnrollmentCleanupResult, error) {
	d := s.deps
	if d.Requests == nil || d.Children == nil || d.LateInvites == nil || d.Delivery == nil || d.Settings == nil || d.Runtime.InTransaction == nil {
		return enrollmentNative.SettledEnrollmentCleanupResult{}, errors.New("settled enrollment cleanup is not configured")
	}
	cutoffs, err := s.settledCutoffs(ctx)
	if err != nil {
		return enrollmentNative.SettledEnrollmentCleanupResult{}, err
	}
	deleted := enrollmentNative.RejectedEnrollmentCleanupResult{}
	var invites int64
	err = s.runInTx(ctx, func(txCtx context.Context) error {
		requestIDs, listErr := d.Requests.SettledRequestsBefore(txCtx, cutoffs.withdrawn, cutoffs.rejected)
		if listErr != nil {
			return listErr
		}
		settled := func(children []*enrollmentNative.RequestChild) bool {
			return childrenRemainSettledBefore(children, cutoffs)
		}
		for _, requestID := range requestIDs {
			if err := s.cleanupRequest(txCtx, requestID, settled, &deleted); err != nil {
				return fmt.Errorf("settled enrollment cleanup of request %d: %w", requestID, err)
			}
		}
		var inviteErr error
		invites, inviteErr = d.LateInvites.DeleteLateInvitesSettledBefore(txCtx, cutoffs.withdrawn)
		if inviteErr != nil {
			return fmt.Errorf("delete settled enrollment late invites: %w", inviteErr)
		}
		return nil
	})
	if err != nil {
		return enrollmentNative.SettledEnrollmentCleanupResult{}, err
	}
	result := enrollmentNative.SettledEnrollmentCleanupResult{
		DeletedRequests:    deleted.DeletedRequests,
		DeletedLateInvites: deleted.DeletedLateInvites + invites,
		RetentionDays:      cutoffs.withdrawnDays,
	}
	d.Logger.InfoContext(ctx, "settled enrollment cleanup completed",
		slog.Int("deleted_requests", result.DeletedRequests),
		slog.Int64("deleted_late_invites", result.DeletedLateInvites),
		slog.Int64("cancelled_outbox_rows", deleted.DeletedOutboxRows))
	return result, nil
}

// settledCutoffs resolves both windows. It never guesses: an unreadable or
// non-positive window fails the run, so nothing is deleted on a wrong period.
func (s *RejectedCleanup) settledCutoffs(ctx context.Context) (settledCutoffs, error) {
	withdrawnDays, err := s.deps.Settings.WithdrawnRetentionDays(ctx)
	if err != nil {
		return settledCutoffs{}, fmt.Errorf("resolve withdrawn enrollment retention: %w", err)
	}
	rejectedDays, err := s.deps.Settings.RejectedRetentionDays(ctx)
	if err != nil {
		return settledCutoffs{}, fmt.Errorf("resolve rejected enrollment retention: %w", err)
	}
	if withdrawnDays <= 0 || rejectedDays <= 0 {
		return settledCutoffs{}, fmt.Errorf("invalid enrollment retention days: withdrawn %d, rejected %d", withdrawnDays, rejectedDays)
	}
	now := time.Now()
	return settledCutoffs{
		withdrawn:     now.Add(-time.Duration(withdrawnDays) * 24 * time.Hour),
		rejected:      now.Add(-time.Duration(rejectedDays) * 24 * time.Hour),
		withdrawnDays: withdrawnDays,
	}, nil
}

// childrenRemainSettledBefore reports whether the locked children still make
// a settled request without a child: each is settled past its window and at
// least one is withdrawn.
func childrenRemainSettledBefore(children []*enrollmentNative.RequestChild, cutoffs settledCutoffs) bool {
	withdrawn := false
	for _, child := range children {
		if !childSettledBefore(child, cutoffs) {
			return false
		}
		withdrawn = withdrawn || child.Status == enrollmentNative.ChildStatusWithdrawn
	}
	return withdrawn
}

// childSettledBefore reports whether no child record was created from child
// and it is withdrawn or rejected past its window. A parent withdrawal stamps
// reviewed_at; the rollover deadline lapses a renewal without a review stamp.
func childSettledBefore(child *enrollmentNative.RequestChild, cutoffs settledCutoffs) bool {
	if child == nil || (child.CreatedStudentID != nil && *child.CreatedStudentID > 0) {
		return false
	}
	switch child.Status {
	case enrollmentNative.ChildStatusWithdrawn:
		settledAt := child.UpdatedAt
		if child.ReviewedAt != nil {
			settledAt = *child.ReviewedAt
		}
		return settledAt.Before(cutoffs.withdrawn)
	case enrollmentNative.ChildStatusRejected:
		return child.ReviewedAt != nil && child.ReviewedAt.Before(cutoffs.rejected)
	default:
		return false
	}
}
