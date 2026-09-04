// Package parentmessaging holds the small shared core of the parent-OGS
// messaging feature used by BOTH the staff-side Communication adapter and the
// parent side (services/parent). It exists so the load-bearing rules — append a
// message and advance the read cursor off the DB-stamped created_at (never the
// app clock), and the SSE fan-out contract — live in ONE place instead of being
// hand-mirrored in two services where a fix to one could silently miss the
// other.
package parentmessaging

import (
	"cmp"
	"context"
	"log/slog"

	configModels "github.com/moto-nrw/project-phoenix/models/config"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/realtime"
)

// SettingsResolver resolves a boolean setting inside the current tenant
// transaction. Satisfied by services/config.SettingsService; declared here as a
// narrow interface so this shared core does not depend on the whole settings
// service (and to keep the import graph acyclic).
type SettingsResolver interface {
	ResolveBool(ctx context.Context, key string) (bool, error)
}

// TenantSettingsResolver resolves a boolean setting for an explicit tenant,
// opening its own tenant transaction. Used by callers OUTSIDE tenant middleware
// (the cross-tenant parent badge, the unauthenticated tenant-resolve handler).
type TenantSettingsResolver interface {
	ResolveBoolForTenant(ctx context.Context, tenantID int64, key string) (bool, error)
}

// MessagingEnabled reports whether parent messaging is on for the CURRENT tenant
// transaction. It is the single home for BOTH the setting key and the fail-OPEN
// direction, so every read- and write-path across the staff and parent services
// agrees: a transient settings-resolve error counts as ENABLED.
//
// Failing open is deliberate and uniform. The unread badge, the inbox/row pills,
// the compose-button visibility, and the reply path all gate on this one flag; if
// they disagreed during a config-DB blip the staffer would see and read unread
// messages while the "Neue Nachricht" button vanished and every reply 500'd — a
// half-disabled UI that contradicts the still-rendering inbox. Over-permitting on
// a rare blip beats that split brain: messaging is a soft, non-destructive feature
// flag, not a security boundary. (Unlike the photos/NFC flags, which fail closed
// for opt-out safety because enabling them surfaces data a school opted out of.)
func MessagingEnabled(ctx context.Context, settings SettingsResolver, logger *slog.Logger) bool {
	enabled, err := settings.ResolveBool(ctx, configModels.KeyParentNotesEnabled)
	if err != nil {
		loggerOr(logger).Warn("parent messaging: resolve enabled failed, failing open (counting as enabled)",
			slog.String("error", err.Error()),
		)
		return true
	}
	return enabled
}

// MessagingEnabledForTenant is MessagingEnabled for an explicit tenant, for
// callers running outside tenant middleware. Same key, same fail-OPEN contract.
func MessagingEnabledForTenant(ctx context.Context, settings TenantSettingsResolver, tenantID int64, logger *slog.Logger) bool {
	enabled, err := settings.ResolveBoolForTenant(ctx, tenantID, configModels.KeyParentNotesEnabled)
	if err != nil {
		loggerOr(logger).Warn("parent messaging: resolve enabled for tenant failed, failing open (counting as enabled)",
			slog.Int64("tenant_id", tenantID),
			slog.String("error", err.Error()),
		)
		return true
	}
	return enabled
}

func loggerOr(logger *slog.Logger) *slog.Logger {
	return cmp.Or(logger, slog.Default())
}

// AppendMessage serializes the thread, persists an already-built message, then
// updates the last-activity preview. The caller has already authorized and
// stamped the sender and tenant fields.
//
// The thread touch is driven off the inserted row's DB-stamped created_at
// (msg.CreatedAt), NOT a Go time.Now(): messages.created_at defaults to the
// Postgres clock, so seeding it from the app clock desyncs the preview whenever
// the app host's clock leads Postgres (multi-host deploy) and the monotonic
// preview guard (TouchLastMessage) could then keep the older message. Using the
// row's own created_at keeps every comparison on one clock.
//
// It deliberately does not advance the sender's read cursor. Own messages are
// excluded by sender_account_id, while read paths advance only to a snapshot the
// client actually received.
func AppendMessage(
	ctx context.Context,
	msgRepo usersModels.ParentMessageRepository,
	threadRepo usersModels.ParentMessageThreadRepository,
	msg *usersModels.ParentMessage,
) error {
	if err := threadRepo.LockForMessageAppend(ctx, msg.ThreadID); err != nil {
		return err
	}
	if err := msgRepo.Create(ctx, msg); err != nil {
		return err
	}
	return threadRepo.TouchLastMessage(ctx, msg.ThreadID, msg.CreatedAt, msg.ID, msg.SenderKind, msg.Body)
}

// MarkReadToNewest advances the reader's read cursor to the newest COUNTERPART
// message in the supplied snapshot that the reader did NOT author — and ONLY to
// that message, never to NOW(), never to one of the reader's OWN rows, and never
// to a later non-counterpart row. staffReader selects which side counts as the
// counterpart (true: the guardian side is the counterpart; false: the staff
// side), matching usersModels.IsCounterpartMessage / the unread SQL's
// counterpartUnread. Both portals call this off the SAME slice they return to the
// client.
//
// Three hazards make the "newest non-authored COUNTERPART" bound load-bearing,
// not a nicety:
//
//  1. Never NOW(): a counterpart message that committed between the caller's
//     ListByThread snapshot and this mark is absent from `messages`, so a NOW()
//     cursor would fall past it and silently drop it from the reader's unread badge
//     though they never saw it (and the refetch that would heal it is what advanced
//     the cursor).
//
//  2. Never the reader's OWN row: created_at defaults to the inserting
//     transaction's start instant, so a counterpart whose tx began earlier (lower
//     created_at) but commits LATER sits below a message stamped now. On a send
//     path the snapshot's newest element is the reader's just-inserted message at
//     ~now; advancing the cursor onto it would leap past that still-in-flight
//     counterpart and mark it read once it commits, even though the sender never
//     saw it. This is the same reason AppendMessage deliberately does not move the
//     sender's cursor on insert.
//
//  3. Never a later NON-counterpart row: the unread query only counts
//     counterpart-authored rows (counterpartUnread), so the cursor must be bounded
//     to those. A snapshot can contain a later staff/system row from ANOTHER
//     account — not the reader's, but also not a counterpart — and binding the
//     cursor to it (as a bare "not authored by me" test did) leaps past an earlier
//     counterpart message that can still commit with a lower created_at in a
//     concurrent send/read race. That guardian message would then never surface as
//     unread for this reader. Bounding to the newest counterpart row the reader did
//     not author is exactly the membership of the unread set, so it is both
//     sufficient and the only safe choice.
//
// A snapshot with no qualifying counterpart message (empty thread, or only the
// reader's own / same-side rows) leaves the cursor untouched: there is nothing the
// reader needs to mark seen. Messages are ordered oldest-first, so the last
// qualifying element is the newest counterpart. MarkReadUpTo itself is monotonic,
// so a stale snapshot never rolls the cursor back. This rule used to be
// hand-mirrored in Communication's staff adapter and services/parent;
// keeping it here is what stops the two portals' unread counts from drifting.
// It returns whether the cursor actually ADVANCED (false when there was nothing
// to mark, or the snapshot was already read). The read-receipt SSE push gates on
// this so a refetch that marks nothing new does not emit an event — the bound that
// stops the receipt event from ping-ponging with the refetch it triggers on the
// counterpart.
func MarkReadToNewest(
	ctx context.Context,
	readRepo usersModels.ParentMessageReadRepository,
	tenantID, threadID, accountID int64,
	staffReader bool,
	messages []*usersModels.ParentMessage,
) (bool, error) {
	var newest *usersModels.ParentMessage
	for _, msg := range messages {
		if msg == nil {
			continue
		}
		// Oldest-first order: keep overwriting so newest ends up last. Bound to the
		// reader's unread set exactly — a COUNTERPART row (hazard 3) the reader did
		// NOT author (hazard 2). Both legs mirror the unread SQL (counterpartUnread
		// ∧ notReaderAuthored) via their model helpers; the author skip still matters
		// for a dual-role staff+guardian account, whose own counterpart-side PLAIN
		// message must not move the cursor. IsReaderAuthored exempts system events
		// from that skip exactly as notReaderAuthored does — a confirm/reject/
		// withdrawal that account triggered is attributed by side, so on the opposite
		// portal it is genuinely unread and MUST be allowed to advance the cursor;
		// the bare account check stranded it as permanently unread once viewed.
		if usersModels.IsCounterpartMessage(msg, staffReader) && !usersModels.IsReaderAuthored(msg, accountID) {
			newest = msg
		}
	}
	if newest == nil {
		return false, nil
	}
	return readRepo.MarkReadUpTo(ctx, tenantID, threadID, accountID, newest.CreatedAt, newest.ID)
}

// MarkStaffHandledToVisible advances the shared team boundary only through the
// last timeline row the replying client displayed. Missing or stale boundaries
// are safe no-ops, so a concurrent message can stay open but is never skipped.
func MarkStaffHandledToVisible(
	ctx context.Context,
	readRepo usersModels.ParentMessageReadRepository,
	tenantID, threadID int64,
	handledUpToMessageID int64,
	messages []*usersModels.ParentMessage,
) error {
	if handledUpToMessageID <= 0 {
		return nil
	}
	var newest *usersModels.ParentMessage
	for _, msg := range messages {
		if msg == nil {
			continue
		}
		if usersModels.IsCounterpartMessage(msg, true) {
			newest = msg
		}
		if msg.ID == handledUpToMessageID {
			if newest == nil {
				return nil
			}
			return readRepo.MarkStaffHandledUpTo(ctx, tenantID, threadID, newest.CreatedAt, newest.ID)
		}
	}
	return nil
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
		loggerOr(logger).Warn("parent messaging: read-receipt lookup failed",
			slog.Int64("thread_id", threadID),
			slog.String("error", err.Error()),
		)
		return
	}
	usersModels.StampStaffReadReceipts(messages, cutoff)
}

// DecorateGuardianReadReceipts is the staff-side mirror of DecorateReadReceipts:
// it stamps ReadByGuardian (the staff-facing "von den Eltern gelesen" indicator)
// on every staff-authored message the thread's guardian has already read, using
// the guardian's read cursor. The staff chat runs its snapshot through this so
// staff can see whether the parent has read their reply — the symmetric
// counterpart of the parent-facing "OGS hat gelesen" receipt. Like the staff
// version, a transient lookup failure is logged and the indicator simply stays
// hidden until the next load rather than blanking the chat. Guardian-only: the
// parent never needs to be told it read its own conversation.
func DecorateGuardianReadReceipts(
	ctx context.Context,
	readRepo usersModels.ParentMessageReadRepository,
	logger *slog.Logger,
	threadID int64,
	messages []*usersModels.ParentMessage,
) {
	cutoff, err := readRepo.GuardianReadCursor(ctx, threadID)
	if err != nil {
		loggerOr(logger).Warn("parent messaging: guardian read-receipt lookup failed",
			slog.Int64("thread_id", threadID),
			slog.String("error", err.Error()),
		)
		return
	}
	usersModels.StampGuardianReadReceipts(messages, cutoff)
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
		loggerOr(logger).Warn("parent messaging: failed to broadcast parent message",
			slog.Int64("tenant_id", tenantID),
			slog.Int64("guardian_account_id", guardianAccountID),
			slog.String("error", err.Error()),
		)
	}
}

// BroadcastRead fires the read-receipt SSE wake-up over the SAME guardian+staff
// fan-out as Broadcast, but as an EventParentMessageRead so the far side refreshes
// only its "Gelesen" receipts (no new message, no unread-badge change). Callers
// schedule it AFTER commit and ONLY when the read cursor actually advanced (see
// MarkReadToNewest) — so it can't loop with the receipt refetch it triggers.
// Fire-and-forget: a nil broadcaster or non-positive tenant is a no-op and a
// delivery error is logged, never returned.
func BroadcastRead(
	broadcaster realtime.Broadcaster,
	logger *slog.Logger,
	tenantID, guardianAccountID, threadID, studentID int64,
) {
	if broadcaster == nil || tenantID <= 0 {
		return
	}
	event := realtime.NewParentMessageReadEvent(guardianAccountID, threadID, studentID)
	if err := broadcaster.BroadcastParentMessage(tenantID, guardianAccountID, event); err != nil {
		loggerOr(logger).Warn("parent messaging: failed to broadcast read receipt",
			slog.Int64("tenant_id", tenantID),
			slog.Int64("guardian_account_id", guardianAccountID),
			slog.String("error", err.Error()),
		)
	}
}
