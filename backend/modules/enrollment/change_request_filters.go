package enrollment

import "time"

type ChangeRequestListFilters struct {
	RequestID int64
	Status    string
	Limit     int
}

// ChangeRequestReviewFilters selects the open working list or decided history.
// Results are newest first with keyset pagination.
type ChangeRequestReviewFilters struct {
	// An empty status set returns nothing, not every request.
	Statuses []string
	// Search matches an affected child's full name, case-insensitively.
	Search string
	// History orders by COALESCE(reviewed_at, updated_at), otherwise created_at.
	History bool
	// From and To bound the decision instant for history; zero is unbounded.
	From, To time.Time
	// A zero BeforeInstant selects the first page.
	BeforeInstant time.Time
	BeforeID      int64
	Limit         int
}
