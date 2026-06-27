// Package parentmessaging holds the small shared core of the parent-OGS
// messaging feature used by BOTH the staff side (services/messaging) and the
// parent side (services/parent). It exists so the load-bearing rules — append a
// message and advance the read cursor off the DB-stamped created_at (never the
// app clock), and the SSE fan-out contract — live in ONE place instead of being
// hand-mirrored in two services where a fix to one could silently miss the
// other.
package parentmessaging

import (
	"context"
	"log/slog"

	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/realtime"
)

// AppendMessage persists an already-built ParentMessage, then updates the
// thread's last-activity preview and the sender's own read cursor in lockstep.
// The caller has already authorized/validated the body and stamped
// SenderKind / SenderName / tenant on msg.
//
// Both the thread touch and the read cursor are driven off the inserted row's
// DB-stamped created_at (msg.CreatedAt), NOT a Go time.Now(): messages.created_at
// defaults to the Postgres clock, so seeding these from the app clock desyncs the
// two whenever the app host's clock leads Postgres (multi-host deploy) — a
// counterpart message arriving within the skew window would be marked read though
// never seen, and the monotonic preview guard (TouchLastMessage) could keep the
// older message. Using the row's own created_at keeps every comparison on one
// clock; it is also exactly the cursor a dual-role (staff+guardian) sender needs
// so their own just-sent message is not counted as unread for themselves.
//
// TouchLastMessage and MarkReadUpTo are both guarded/monotonic: a concurrent
// counterpart send that committed with a newer instant is never clobbered by an
// older one.
func AppendMessage(
	ctx context.Context,
	msgRepo usersModels.ParentMessageRepository,
	threadRepo usersModels.ParentMessageThreadRepository,
	readRepo usersModels.ParentMessageReadRepository,
	msg *usersModels.ParentMessage,
) error {
	if err := msgRepo.Create(ctx, msg); err != nil {
		return err
	}
	at := msg.CreatedAt
	if err := threadRepo.TouchLastMessage(ctx, msg.ThreadID, at, msg.SenderKind, msg.Body); err != nil {
		return err
	}
	return readRepo.MarkReadUpTo(ctx, msg.GetTenantID(), msg.ThreadID, msg.SenderAccountID, at, msg.ID)
}

// DecorateReadReceipts stamps the "OGS hat gelesen" indicator (ReadByStaff) on
// every guardian-authored message the staff side has already read, using the
// newest read cursor in the thread held by an account OTHER than the querying
// one. BOTH portals run their message snapshot through this — the staff chat and
// the parent read/write paths — so the receipt rule can never drift between the
// two services (it used to be hand-mirrored in each, differing only by log
// prefix). A transient lookup failure is logged, not fatal: the indicator simply
// stays hidden until the next load rather than blanking the whole chat.
//
// otherAccountID is the account whose reads count as "the other side read it":
// the thread's guardian for the staff view, the querying guardian's own account
// excluded for the parent view (LatestReadCursorByOther positively gates on staff
// membership, so a guardian's cursor never satisfies the receipt either way).
func DecorateReadReceipts(
	ctx context.Context,
	readRepo usersModels.ParentMessageReadRepository,
	logger *slog.Logger,
	threadID, otherAccountID int64,
	messages []*usersModels.ParentMessage,
) {
	cutoff, err := readRepo.LatestReadCursorByOther(ctx, threadID, otherAccountID)
	if err != nil {
		if logger == nil {
			logger = slog.Default()
		}
		logger.Warn("parent messaging: read-receipt lookup failed",
			slog.Int64("thread_id", threadID),
			slog.String("error", err.Error()),
		)
		return
	}
	usersModels.StampStaffReadReceipts(messages, cutoff)
}

// Broadcast fires the parent-OGS SSE wake-up: the addressed guardian's own tabs
// plus the tenant's staff (whose access-filtered inboxes refetch). Fire-and-forget
// — a nil broadcaster or non-positive tenant is a no-op, and a delivery error is
// logged, never returned (the message has already committed). Callers schedule
// this AFTER commit so a woken client never reads the pre-commit snapshot.
func Broadcast(
	broadcaster realtime.Broadcaster,
	logger *slog.Logger,
	tenantID, guardianAccountID, threadID, studentID int64,
) {
	if broadcaster == nil || tenantID <= 0 {
		return
	}
	event := realtime.NewParentMessageEvent(guardianAccountID, threadID, studentID)
	if err := broadcaster.BroadcastParentMessage(tenantID, guardianAccountID, event); err != nil {
		if logger == nil {
			logger = slog.Default()
		}
		logger.Warn("parent messaging: failed to broadcast parent message",
			slog.Int64("tenant_id", tenantID),
			slog.Int64("guardian_account_id", guardianAccountID),
			slog.String("error", err.Error()),
		)
	}
}
