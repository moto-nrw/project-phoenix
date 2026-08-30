package base

import "time"

// RequestQueueFilters narrows ONE change-request queue in SQL: which children,
// where to resume, and how many rows. It is the shared shape behind the
// aggregated request list (#2432) — the four guardian queues plus the
// direct-correction log all page through it, so the aggregate never has to
// load a whole queue and sift it in Go.
//
// The zero value is "the whole queue from the top", which is what the
// pending-count badge asks for.
type RequestQueueFilters struct {
	// UrgentOnly selects the open queue's urgency phase. Nil leaves urgency
	// unrestricted (history and legacy callers); true/false select exactly one
	// phase so each phase can keep using the existing created_at keyset.
	UrgentOnly *bool
	UrgentDate string
	// StudentIDs limits the queue to a set of children. It exists for the
	// conflict scan, which has to see EVERY open request of the children on
	// one page — not the page itself, whose window would under-count a group.
	// Empty means no restriction; it composes with StudentID.
	StudentIDs []int64
	// StudentID limits the queue to one child — the Kinderkartei's
	// Änderungsprotokoll (#2437). Zero means every child.
	StudentID int64
	// Search matches the child's name case-insensitively as a substring of
	// "first last", the same rule the rendered name follows, so a name that
	// shows up in the list is also a name that finds it.
	Search string
	// BeforeInstant and BeforeID are the keyset position of the last row the
	// caller consumed. A zero instant returns the first page.
	BeforeInstant time.Time
	BeforeID      int64
	// Limit caps the page. Zero or less means unbounded.
	Limit int
}
