// Package care coordinates the guardian portal's child flows: the today status,
// the weekly care plan and its requests, booked care offerings and courses,
// absences and one-day pickup changes, the meal plan, the Stammdaten and their
// requests, guardian contacts, related accounts, consents and the guardian's
// own profile (#3227, #3420).
//
// The package owns no data. Every flow resolves the guardian's child, checks
// the relationship's parent_portal.* permission and the school's settings,
// opens one tenant unit of work from the request context and calls the owners'
// commands inside it: Care Plan for status days, pickup exceptions and change
// requests, People Directory for student and guardian rows, Audit Platform for
// the guardian change trail. Notifications run after the commit.
package care

import (
	"context"
	"errors"
)

// Request types of the parent request-sharing ledger. Wire-stable values;
// the messaging package and the workflow root re-export them.
const (
	RequestShareMasterData   = "master_data"
	RequestShareCareSchedule = "care_schedule"
	RequestSharePickupChange = "pickup_change"
	RequestShareOffering     = "offering"
	RequestShareExcused      = "excused"
)

// MaxParentNoteLen bounds a single note so a parent can't paste a novel
// the staff card then has to render. Generous for a "kurze Nachricht".
const MaxParentNoteLen = 2000

var (
	// ErrNotesDisabled means operations.parent_notes_enabled is off for
	// the child's tenant.
	ErrNotesDisabled = errors.New("parent: parent notes disabled for this school")
	// ErrNoteTooLong means the note body exceeded MaxParentNoteLen.
	ErrNoteTooLong = errors.New("parent: note body too long")
	// ErrEmptyNote means the note body was blank after trimming.
	ErrEmptyNote = errors.New("parent: note body must not be empty")
	// ErrPickupChangeCutoffPassed means the school's same-day cutoff
	// (operations.parent_pickup_change_cutoff_time) has passed, so today's
	// pickup time is closed for guardians (#3163). Later days stay open.
	ErrPickupChangeCutoffPassed = errors.New("parent: same-day pickup change cutoff has passed")
)

// RequestShareVisibility answers whether an account may see one request of
// the child, given the family's current sharing choices.
type RequestShareVisibility interface {
	Allows(requestType string, requestID, accountID, submittedBy int64) bool
}

// RequestSharer is the port to the parent request-sharing ledger, which this
// package does not own. ShareRequestInTx writes the family's recipient choice
// inside the caller's transaction; LoadRequestShareVisibility reads the
// current shares inside the caller's tenant transaction.
type RequestSharer interface {
	ShareRequestInTx(ctx context.Context, accountID, studentID int64, requestType string, requestID int64, recipientProfileIDs []int64) error
	LoadRequestShareVisibility(ctx context.Context, studentID int64) (RequestShareVisibility, error)
}
