package users

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
)

// StaffMessageThreadKindDirect is the only conversation shape in V1: exactly
// two staff accounts. Group threads would add a kind here plus rows in
// users.staff_message_participants — the schema needs no change for them.
const StaffMessageThreadKindDirect = "direct"

// StaffMessageThread is ONE continuous conversation between staff accounts of a
// single school (chat model, no subject). For a direct chat the pair is
// identified by ParticipantKey, so the send path is get-or-create against a
// single unique index.
type StaffMessageThread struct {
	base.Model `bun:"schema:users,table:staff_message_threads"`
	base.TenantModel
	// ParticipantKey identifies the conversation independently of the row's id.
	// For a direct chat it is DirectParticipantKey(a, b) — the two account ids
	// sorted ascending, colon-joined. Unique per tenant.
	ParticipantKey string     `bun:"participant_key,notnull" json:"-"`
	Kind           string     `bun:"kind,notnull" json:"kind"`
	LastMessageAt  *time.Time `bun:"last_message_at" json:"last_message_at,omitempty"`
	// LastMessageID is the id of whichever message last set LastMessageAt. It is
	// the second half of the (created_at, id) composite the message list orders
	// by, so TouchLastMessage can break ties when two messages share a
	// created_at (clock_timestamp() can collide): the higher-id message wins.
	// Every path that updates LastMessageAt MUST set this in the same UPDATE.
	LastMessageID *int64 `bun:"last_message_id" json:"last_message_id,omitempty"`
	// LastMessageBody denormalizes the preview so the inbox projection reads it
	// off the thread row instead of a correlated subquery per row. Same rule as
	// LastMessageID: written in the same UPDATE as LastMessageAt.
	LastMessageBody     string `bun:"last_message_body" json:"last_message_body,omitempty"`
	LastSenderAccountID *int64 `bun:"last_sender_account_id" json:"last_sender_account_id,omitempty"`
}

// DirectParticipantKey builds the stable identity of a two-person conversation:
// the account ids sorted ascending and colon-joined, so (17, 42) and (42, 17)
// address the same thread regardless of who writes first.
func DirectParticipantKey(a, b int64) string {
	ids := []int64{a, b}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	parts := make([]string, 0, len(ids))
	for _, id := range ids {
		parts = append(parts, fmt.Sprintf("%d", id))
	}
	return strings.Join(parts, ":")
}

// StaffMessageThreadRepository is the tenant-scoped data-access contract for
// conversations. All methods must run inside a tenant transaction.
type StaffMessageThreadRepository interface {
	// GetOrCreateDirect returns the conversation between the two accounts in the
	// current tenant, atomically creating it (and both participant rows) when
	// absent. Concurrency-safe: two simultaneous first messages converge on one
	// thread via the (tenant_id, participant_key) unique constraint.
	GetOrCreateDirect(ctx context.Context, accountA, accountB int64) (*StaffMessageThread, error)
	// FindByID returns the thread by id (tenant-scoped), or nil when absent.
	FindByID(ctx context.Context, id int64) (*StaffMessageThread, error)
	// LockForMessageAppend serializes message inserts for one thread, so message
	// order also reflects commit order. Acquire before inserting.
	LockForMessageAppend(ctx context.Context, threadID int64) error
	// TouchLastMessage atomically advances the denormalized last-activity fields
	// that drive the inbox preview and ordering, but ONLY when the message's
	// (at, messageID) composite is newer than the stored pair. Single monotonic
	// write path for those fields. Callers insert the message row separately.
	TouchLastMessage(ctx context.Context, threadID int64, at time.Time, messageID, senderAccountID int64, body string) error
	// ParticipantAccountIDs returns every account in the thread, ascending.
	ParticipantAccountIDs(ctx context.Context, threadID int64) ([]int64, error)
	// IsParticipant reports whether the account belongs to the thread. This is
	// the authorization predicate for reading or posting.
	IsParticipant(ctx context.Context, threadID, accountID int64) (bool, error)
	// DeleteEmpty removes threads created before the cutoff that hold no
	// messages any more. The cutoff is a grace period: OpenThread creates a
	// thread before its first message, so a just-opened conversation is
	// legitimately empty and must survive the sweep. Returns rows deleted.
	DeleteEmpty(ctx context.Context, createdBefore time.Time) (int64, error)
}
