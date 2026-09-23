package schoolmembership

import (
	"context"
	"errors"
	"fmt"
)

// ChildQuotaReachedCode is the stable error code of a write refused because
// the Kinderkontingent is reached (#3567). Clients map it to their own text.
const ChildQuotaReachedCode = "students.child_quota_reached"

// ErrChildQuotaReached classifies ChildQuotaReachedError with errors.Is.
var ErrChildQuotaReached = errors.New("child quota reached")

// ChildQuotaReachedError refuses a write that would raise the Kontingentzahl
// above the school's Kinderkontingent. Booked is the Kinderkontingent,
// Occupied the Kontingentzahl before the write and Requested the number of
// children the write would add. The write is not applied.
//
// It is a business rejection: ErrorCode and ErrorDetails let an HTTP adapter
// answer 409 with the code and the numbers without importing this package.
type ChildQuotaReachedError struct {
	Booked    int
	Occupied  int
	Requested int
}

func (e *ChildQuotaReachedError) Error() string {
	return fmt.Sprintf("child quota reached: %d of %d places occupied, %d requested", e.Occupied, e.Booked, e.Requested)
}

func (e *ChildQuotaReachedError) Unwrap() error { return ErrChildQuotaReached }

func (e *ChildQuotaReachedError) ErrorCode() string { return ChildQuotaReachedCode }

// ChildQuotaDetails are the numbers a refusal names, in wire form.
type ChildQuotaDetails struct {
	BookedPlaces    int `json:"booked_places"`
	OccupiedPlaces  int `json:"occupied_places"`
	RequestedPlaces int `json:"requested_places"`
}

func (e *ChildQuotaReachedError) ErrorDetails() any {
	return ChildQuotaDetails{BookedPlaces: e.Booked, OccupiedPlaces: e.Occupied, RequestedPlaces: e.Requested}
}

// ChildQuotaCheck says whether a write that can raise the Kontingentzahl is
// checked against the Kinderkontingent. It has no usable zero value: a caller
// names its choice, so skipping the check is never a silent default.
type ChildQuotaCheck int

const (
	// EnforceChildQuota checks the write; every ordinary caller uses it.
	EnforceChildQuota ChildQuotaCheck = iota + 1
	// SkipChildQuotaForGradeTransitionRevert lets reverting a grade
	// transition bring its children back even when the Kinderkontingent is
	// full, so a school never stays stuck in a wrong state.
	SkipChildQuotaForGradeTransitionRevert
)

func (c ChildQuotaCheck) valid() bool {
	return c == EnforceChildQuota || c == SkipChildQuotaForGradeTransitionRevert
}

// ChildQuotaUsage is the school's Kinderkontingent next to its
// Kontingentzahl, the numbers the Datenverwaltung shows (#3569). Booked is
// the Kinderkontingent, Occupied the Kontingentzahl of today.
type ChildQuotaUsage struct {
	Booked   int
	Occupied int
}

// ChildQuotaUsages reads the Kinderkontingent of the tenant in context.
// limited is false when the school has none; the usage is then empty and
// the Kontingentzahl is not counted.
type ChildQuotaUsages interface {
	ChildQuotaUsage(context.Context) (usage ChildQuotaUsage, limited bool, err error)
}

func (m *Module) ChildQuotaUsage(ctx context.Context) (ChildQuotaUsage, bool, error) {
	if err := ctx.Err(); err != nil {
		return ChildQuotaUsage{}, false, err
	}
	return m.engine.ChildQuotaUsage(ctx)
}
