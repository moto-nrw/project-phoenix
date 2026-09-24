package careplan

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The error contract of the care-offering catalog (#3559). Handlers render
// err.Error() to administrators, so every text is byte-identical to the
// services/enrollment value it replaces; services/enrollment points its
// legacy names at these values while its routes still speak enrollment rows.
var (
	// ErrCareOfferingConfigInvalid classifies a catalog or timetable-link
	// configuration an administrator can correct. Infrastructure failures
	// never wrap it, so handlers can answer 400 without leaking database
	// details on the 500 path.
	ErrCareOfferingConfigInvalid = errors.New("invalid care offering configuration")
	// ErrCareOfferingTemplatePeriodMismatch means a linked timetable
	// template's planning period does not contain the enrollment phase's
	// service dates.
	ErrCareOfferingTemplatePeriodMismatch = errors.New("care offering phase must be within the linked timetable template period")
	// ErrCareOfferingGroupRuleConflict means a save would leave two offerings
	// of one selection group with different non-optional selection rules.
	ErrCareOfferingGroupRuleConflict = errors.New("care offerings in the same selection group must share one selection rule")
	// ErrCareOfferingDaysRequired marks a save without any weekday (#1885).
	ErrCareOfferingDaysRequired = errors.New("available_days must contain at least one day")
	// ErrCareOfferingPickupTimesRequired marks an active offering whose
	// weekday plan has no unambiguous pickup time.
	ErrCareOfferingPickupTimesRequired = errors.New("active care offering requires pickup_times for every weekday")
	// ErrCalendarPeriodCareOfferingConflict refuses a calendar-period change
	// a linked, still materializable care offering needs.
	ErrCalendarPeriodCareOfferingConflict = errors.New("calendar period is required by a linked care offering")
)

// CareOfferingCatalog is the administration of the per-tenant care-offering
// catalog. Every write validates the offering against the tenant's grade
// range, the linked timetable template and the selection-group rules before
// it reaches the Care Plan records.
type CareOfferingCatalog interface {
	ListCatalog(ctx context.Context) ([]CareOffering, error)
	ListByPhase(ctx context.Context, phaseID int64) ([]CareOffering, error)
	FindOffering(ctx context.Context, id int64) (CareOffering, error)
	CreateOffering(ctx context.Context, offering CareOffering) (CareOffering, error)
	UpdateOffering(ctx context.Context, offering CareOffering) (CareOffering, error)
	DeleteOffering(ctx context.Context, id int64) error
	// Clone copies an offering into a target phase. Linked timetable
	// templates are cleared across phases so the admin relinks one for the
	// new phase.
	Clone(ctx context.Context, sourceID, targetPhaseID int64) (CareOffering, error)
	// ListBookingStats reports how full each offering of the phase is and
	// how its bookings distribute across grade levels. Aggregates only; no
	// child data leaves Care Plan.
	ListBookingStats(ctx context.Context, phaseID int64) ([]CareOfferingBookingStat, error)
}

// CareOfferingBookingStat is one offering's admin-facing booking summary.
type CareOfferingBookingStat struct {
	OfferingID int64
	// Capacity is nil for an unlimited offering.
	Capacity *int
	// Booked is the peak number of children holding a slot simultaneously
	// inside the phase's remaining capacity window — the number the
	// capacity gate compares against Capacity when a booking is saved.
	Booked int
	// GradeLevels maps a child's target grade level to how many booked
	// children carry it. Bookings without a grade are counted in
	// UnknownGradeCount; an availability rule never matches a missing grade.
	GradeLevels       map[int]int
	UnknownGradeCount int
}

// CareOfferingGuards protect the resources a linked care offering
// materializes from. The owners of templates, rooms, timeframes, phases and
// calendar periods ask before they change or delete one; a refusal wraps
// ErrCareOfferingConfigInvalid (or ErrCalendarPeriodCareOfferingConflict for
// a calendar period).
type CareOfferingGuards interface {
	// ValidateTemplateSeries checks every offering linked to a live segment
	// of groupID's split series against the complete post-split series.
	ValidateTemplateSeries(ctx context.Context, groupID int64) error
	// ValidateTemplateOfferingSource guards a template's offering-source
	// rule (#2137) against a calendar period (nil skips the period check).
	// storedOfferingIDs are the ids already persisted on the template: a
	// vanished id is tolerated only when stored.
	ValidateTemplateOfferingSource(ctx context.Context, offeringIDs, storedOfferingIDs []int64, calendarPeriodID *int64) error
	ValidateRoomDeletion(ctx context.Context, roomID int64) error
	// ValidateTimeframeChange simulates an edit of the timeframe; a nil
	// replacement is its deletion.
	ValidateTimeframeChange(ctx context.Context, timeframeID int64, replacement *TimeframeReplacement) error
	// ValidatePhaseChange evaluates every materializable offering of the
	// phase against its proposed service window.
	ValidatePhaseChange(ctx context.Context, phaseID int64, replacement *OfferingPhase) error
	// ValidateCalendarPeriodChange simulates the effective period of every
	// linked template segment; a nil replacement is the period's deletion.
	ValidateCalendarPeriodChange(ctx context.Context, periodID int64, replacement *CalendarPeriodReplacement) error
}

// CareOfferingRollover copies a catalog into a follow-up phase (#2249).
type CareOfferingRollover interface {
	// CloneCatalogForRollover copies every offering of the source phase,
	// plus the carried offerings of earlier phases, into the target phase
	// and returns the source→target id mapping. Unlike Clone it keeps the
	// linked timetable template; one that cannot cover the target phase
	// fails the rollover. Auto-add triggers are remapped inside the clone.
	CloneCatalogForRollover(ctx context.Context, sourcePhaseID, targetPhaseID int64, carriedOfferingIDs []int64) (map[int64]int64, error)
}

// CareOfferingCatalogCapability is everything the catalog serves.
type CareOfferingCatalogCapability interface {
	CareOfferingCatalog
	CareOfferingGuards
	CareOfferingRollover
	CareOfferingLinks
}

// TimeframeReplacement is the proposed state of a timeframe an edit would
// leave behind. Clock values are HH:MM:SS.
type TimeframeReplacement struct {
	StartTime   string
	EndTime     *string
	IsActive    bool
	Description string
}

// CalendarPeriodReplacement is the proposed state of a School Calendar
// planning period.
type CalendarPeriodReplacement struct {
	StartDate       calendar.Date
	EndDate         calendar.Date
	IsActive        bool
	WeekCycleLength int
	// WeekCycleAnchor is empty when no anchor is set.
	WeekCycleAnchor string
}
