package domain

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// StaffMessageThread is one continuous conversation between staff accounts of
// a single school (#2598). ParticipantKey identifies the conversation
// independently of its id; for a direct chat it is the two account ids sorted
// ascending and colon-joined, unique per school.
type StaffMessageThread struct {
	ID             int64
	TenantID       int64
	ParticipantKey string
	Kind           string
	LastMessageAt  *time.Time
	// LastMessageID is the second half of the (created_at, id) composite the
	// message list orders by. Every write of LastMessageAt sets it in the same
	// statement, so ties on created_at resolve to the higher id.
	LastMessageID       *int64
	LastMessageBody     string
	LastSenderAccountID *int64
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

// StaffMessage is one append-only entry in a staff conversation. SenderName
// is frozen at send time so history stays readable after a rename.
type StaffMessage struct {
	ID              int64
	TenantID        int64
	ThreadID        int64
	SenderAccountID int64
	SenderName      string
	Body            string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// StaffMessageThreadKindDirect is the two-account conversation kind.
const StaffMessageThreadKindDirect = "direct"

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
