package users

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

// StaffMessageParticipant links an account to a conversation. For a direct chat
// there are exactly two rows, mirroring StaffMessageThread.ParticipantKey.
type StaffMessageParticipant struct {
	bun.BaseModel `bun:"table:users.staff_message_participants,alias:smp"`
	base.TenantModel
	ThreadID  int64     `bun:"thread_id,pk" json:"thread_id"`
	AccountID int64     `bun:"account_id,pk" json:"account_id"`
	CreatedAt time.Time `bun:"created_at,nullzero,notnull,default:current_timestamp" json:"created_at"`
}

// StaffMessageRead is one reader's cursor in one conversation. Unread is every
// message after the cursor that the reader did not send.
//
// The cursor is the COMPOSITE (LastReadAt, LastReadMessageID): messages can
// share a created_at, so a timestamp-only cursor would treat a tied message
// that committed after the reader's snapshot as already read and silently drop
// it from the unread badge.
type StaffMessageRead struct {
	bun.BaseModel `bun:"table:users.staff_message_reads,alias:smr"`
	base.TenantModel
	ThreadID          int64     `bun:"thread_id,pk" json:"thread_id"`
	AccountID         int64     `bun:"account_id,pk" json:"account_id"`
	LastReadAt        time.Time `bun:"last_read_at,notnull" json:"last_read_at"`
	LastReadMessageID int64     `bun:"last_read_message_id,notnull" json:"last_read_message_id"`
}

// StaffInboxThread is the inbox projection: one row per conversation the
// viewer takes part in, with the counterpart resolved and the viewer's unread
// count applied. It is a query result, not a table.
type StaffInboxThread struct {
	ThreadID int64 `bun:"thread_id" json:"thread_id"`
	TenantID int64 `bun:"tenant_id" json:"-"`
	// CounterpartAccountID / CounterpartName describe the OTHER person in a
	// direct chat, resolved per viewer — the same thread row renders as "Anna"
	// for Ben and as "Ben" for Anna.
	CounterpartAccountID int64  `bun:"counterpart_account_id" json:"counterpart_account_id"`
	CounterpartName      string `bun:"counterpart_name" json:"counterpart_name"`
	// CounterpartRoleKind is resolved by the service (StaffRoleKinds).
	CounterpartRoleKind string     `bun:"-" json:"counterpart_role_kind"`
	LastMessageAt       *time.Time `bun:"last_message_at" json:"last_message_at,omitempty"`
	LastMessageBody     string     `bun:"last_message_body" json:"last_message_body,omitempty"`
	LastSenderAccountID *int64     `bun:"last_sender_account_id" json:"last_sender_account_id,omitempty"`
	UnreadCount         int        `bun:"unread_count" json:"unread_count"`
}

// MessageableStaff is one addressable colleague in the recipient picker.
type MessageableStaff struct {
	AccountID int64  `bun:"account_id" json:"account_id"`
	Name      string `bun:"name" json:"name"`
	// RoleKind is resolved by the service (StaffRoleKinds), not by the picker
	// query - see the StaffRoleKind constants.
	RoleKind string `bun:"-" json:"role_kind"`
}

// StaffRoleKind tells a reader which side of the school a colleague sits on
// (#2208). With Lehrkräfte in the same conversations as the OGS team, a name
// alone no longer says who someone is: a Lehrkraft looking for "the OGS
// leadership" and a Betreuungskraft answering "the class teacher" both need
// the kind next to the name. Deliberately coarse - three values, not the
// school's whole role catalogue.
const (
	// StaffRoleKindLehrkraft: holds the platform lehrkraft system role here.
	StaffRoleKindLehrkraft = "lehrkraft"
	// StaffRoleKindAdmin: holds an admin-based role here (the OGS leadership
	// and administration). Wins over lehrkraft for a dual-role account: such
	// a person answers for the OGS, and that is what a Lehrkraft needs to see.
	StaffRoleKindAdmin = "admin"
	// StaffRoleKindStaff: every other colleague (Betreuungskräfte).
	StaffRoleKindStaff = "staff"
)

// StaffMessageReadRepository is the tenant-scoped data-access contract for read
// cursors and the unread projections derived from them. All methods must run
// inside a tenant transaction.
type StaffMessageReadRepository interface {
	// MarkReadUpTo advances the account's cursor in one thread to the supplied
	// composite. Monotonic: an older (at, messageID) pair never moves the cursor
	// backwards, so an out-of-order request cannot resurrect read messages.
	MarkReadUpTo(ctx context.Context, threadID, accountID int64, at time.Time, messageID int64) error
	// UnreadCount is the account's total unread messages across every thread it
	// takes part in — the sidebar badge.
	UnreadCount(ctx context.Context, accountID int64) (int, error)
	// ListInbox projects the account's conversations, newest activity first,
	// with the counterpart and the per-thread unread count resolved.
	ListInbox(ctx context.Context, accountID int64, onlyUnread bool) ([]*StaffInboxThread, error)
	// ListMessageableStaff returns the accounts the viewer may write to: active
	// members of the current tenant, excluding the viewer.
	ListMessageableStaff(ctx context.Context, viewerAccountID int64) ([]*MessageableStaff, error)
	// IsMessageableStaff reports whether the account may take part in an
	// internal conversation at the current school. MUST be the same predicate
	// ListMessageableStaff filters by - an active tenant mapping alone also
	// covers guardian accounts, which are not colleagues.
	IsMessageableStaff(ctx context.Context, accountID int64) (bool, error)
	// StaffRoleKinds resolves the StaffRoleKind of each account at the
	// current tenant. Accounts without a role row here come back as
	// StaffRoleKindStaff; the map always has one entry per requested id.
	StaffRoleKinds(ctx context.Context, accountIDs []int64) (map[int64]string, error)
}
