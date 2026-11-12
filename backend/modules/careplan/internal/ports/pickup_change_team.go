package ports

import (
	"context"
	"encoding/json"
	"time"

	careplanNative "github.com/moto-ogs/project-phoenix/modules/careplan"
	"github.com/moto-ogs/project-phoenix/modules/careplan/internal/domain"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

// PickupChangeNotice and PickupChangeNotifier tell the child's team about an
// approved or taken-back one-day pickup time (#6855).
type (
	PickupChangeNotice    = careplanNative.PickupChangeNotice
	PickupChangeNotifier  = careplanNative.PickupChangeNotifier
	PickupChangeTeamReach = careplanNative.PickupChangeTeamReach
	PickupChangeRevision  = careplanNative.PickupChangeNoticeRevision
)

// PickupChangeNoticeRow is one unread marker as the store reads it: the
// marker, its request's current state and the child's first name.
type PickupChangeNoticeRow struct {
	RequestID        int64
	StudentID        int64
	StudentFirstName string
	Status           string
	Payload          json.RawMessage
	InformedAt       time.Time
}

// PickupChangeTeamStore keeps the unread markers of the changed pickup times
// in the caller's tenant transaction.
type PickupChangeTeamStore interface {
	// ClearPickupChangeNotice removes a reviewer's marker of the request
	// they just decided, including a marker from an earlier decision.
	ClearPickupChangeNotice(ctx context.Context, requestID, reviewerAccountID int64) (domain.OperationStats, error)
	// UpsertPickupChangeNotices stores one unread marker per recipient for
	// the request; a recipient who read an earlier one gets it unread again.
	UpsertPickupChangeNotices(ctx context.Context, requestID int64, date calendar.Date, recipientAccountIDs []int64) (domain.OperationStats, error)
	// ListUnreadPickupChangeNotices reads the reader's unread markers on days
	// from `from` on, newest first, at most limit, in one statement.
	ListUnreadPickupChangeNotices(ctx context.Context, readerAccountID int64, from calendar.Date, limit int) ([]PickupChangeNoticeRow, domain.OperationStats, error)
	// MarkPickupChangeNoticesRead clears the reader's observed markers of one
	// child.
	MarkPickupChangeNoticesRead(ctx context.Context, readerAccountID, studentID int64, observed []PickupChangeRevision) (domain.OperationStats, error)
}
