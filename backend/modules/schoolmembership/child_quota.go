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

// Free is the number of children the Kinderkontingent still takes; never
// negative when the Kontingentzahl is already above it.
func (u ChildQuotaUsage) Free() int { return max(u.Booked-u.Occupied, 0) }

// Admit judges requested new children against this usage by the rule every
// counting write follows (domain.CheckChildQuota): adding nobody always passes. It takes no lock, so it
// is a preflight only (#3571); the write itself stays checked under the quota
// lock. It returns the refusal the write would raise, or nil.
func (u ChildQuotaUsage) Admit(requested int) error {
	if requested <= 0 || u.Occupied+requested <= u.Booked {
		return nil
	}
	return &ChildQuotaReachedError{Booked: u.Booked, Occupied: u.Occupied, Requested: requested}
}

// CountsTowardChildQuota says whether one membership belongs to the
// Kontingentzahl on day (#3571): an active child whose care has not ended, or
// a pending one whose care starts after day and has not ended. It is the Go
// twin of childQuotaPredicate (internal/adapters/postgres/student_counts.go)
// for a membership that is not written yet, such as an import preview row.
// Dates are YYYY-MM-DD; an empty date is none.
func CountsTowardChildQuota(status, enrolledFrom, enrolledUntil, day string) bool {
	notEnded := enrolledUntil == "" || enrolledUntil >= day
	switch status {
	case "active":
		return notEnded
	case "pending":
		return notEnded && enrolledFrom > day
	default:
		return false
	}
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
