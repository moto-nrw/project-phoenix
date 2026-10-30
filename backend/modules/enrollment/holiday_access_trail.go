package enrollment

import (
	"context"
	"time"
)

// The access trail of the holiday registrations (#5326): who of the
// Träger-Büro read or changed the personal data of a Träger's holiday
// registrations, and when. It reads enrollment.holiday_registration_audit
// for the access log of the Träger-Portal, which the Träger projection
// serves (ADR 0109). A registration belongs to the Träger, not to a school
// (ADR 0114 point 6), so the trail has no school. Like the Audit Platform's
// access trail, it joins the projection's administrative transaction and
// every statement filters on the Träger it is given.

// HolidayAccessFilter selects trail entries between the instants From
// (inclusive) and To (exclusive). AccountID and Kind narrow further when
// set; Kind is "read" for the reads (list, detail, export, proof) and
// "change" for the changes (confirm, cancel, rebook, freeze, reopen). Limit
// bounds the number of entries, newest first.
type HolidayAccessFilter struct {
	From      time.Time
	To        time.Time
	AccountID int64
	Kind      string
	Limit     int
}

// HolidayAccessEntry is one read or change. Data is
// "holiday_registration_" followed by the action of the audit trail;
// AccountID is nil for an account deleted since.
type HolidayAccessEntry struct {
	ID         int64
	OccurredAt time.Time
	AccountID  *int64
	Kind       string
	Data       string
}

// HolidayAccessTrail reads the access trail of the holiday registrations of
// one Träger. It joins the caller's administrative transaction and never
// opens one.
type HolidayAccessTrail interface {
	// ListHolidayAccess lists the entries of the filter, newest first.
	ListHolidayAccess(ctx context.Context, organizationID int64, filter HolidayAccessFilter) ([]HolidayAccessEntry, error)
	// ListHolidayAccessActors lists the accounts with an entry between from
	// (inclusive) and to (exclusive).
	ListHolidayAccessActors(ctx context.Context, organizationID int64, from, to time.Time) ([]int64, error)
}
