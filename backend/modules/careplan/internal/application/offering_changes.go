package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// OfferingChangeDependencies are the ports the offering-change review
// (#1665, #3561) reads and writes through. Every call runs in the caller's
// tenant transaction. Messenger, Ledger and Shares may be nil; the effect is
// then skipped.
type OfferingChangeDependencies struct {
	Rows       ports.OfferingChangeRows
	Catalog    ports.OfferingChangeCatalog
	Enrollment ports.OfferingChangeEnrollment
	Students   ports.OfferingChangeStudents
	Settings   ports.OfferingChangeSettings
	Planning   ports.OfferingChangePlanning
	Bookings   ports.OfferingChangeBookings
	Scope      ports.ReviewScopeResolver
	Hooks      ports.TransactionHooks

	Messenger ports.RequestMessenger
	Ledger    ports.RequestLedger
	Shares    ports.ShareVisibility

	Today  func() calendar.Date
	Logger *slog.Logger
}

// OfferingChanges decides post-enrollment offering changes, course requests
// and the staff's direct booking replacements (#3561).
type OfferingChanges struct {
	deps OfferingChangeDependencies
}

var _ careplan.OfferingChangeCapability = (*OfferingChanges)(nil)

// NewOfferingChanges builds the offering-change review over its ports.
func NewOfferingChanges(deps OfferingChangeDependencies) (*OfferingChanges, error) {
	if deps.Rows == nil || deps.Catalog == nil || deps.Enrollment == nil || deps.Students == nil ||
		deps.Settings == nil || deps.Planning == nil || deps.Bookings == nil || deps.Scope == nil || deps.Hooks == nil {
		return nil, errors.New("offering changes: rows, catalog, enrollment, students, settings, planning, bookings, review scope and transaction hooks are required")
	}
	if deps.Logger == nil {
		deps.Logger = slog.Default()
	}
	if deps.Today == nil {
		deps.Today = calendar.TodayDate
	}
	return &OfferingChanges{deps: deps}, nil
}

func (s *OfferingChanges) todayDate() calendar.Date {
	return s.deps.Today()
}

func (s *OfferingChanges) logger() *slog.Logger {
	return s.deps.Logger
}

// EarliestEffectiveFrom is the first date a switch may take effect under the
// school's configured lead time.
func (s *OfferingChanges) EarliestEffectiveFrom(ctx context.Context) (calendar.Date, error) {
	leadDays, err := s.leadDays(ctx)
	if err != nil {
		return calendar.Date(""), err
	}
	return careplan.EarliestOfferingChangeDate(s.todayDate(), leadDays), nil
}

// leadDays reads the notice period, stored as a string setting, as a
// non-negative number of days.
func (s *OfferingChanges) leadDays(ctx context.Context) (int, error) {
	raw, err := s.deps.Settings.OfferingChangeLeadDays(ctx)
	if err != nil {
		return 0, fmt.Errorf("offering change: resolve lead days: %w", err)
	}
	return parseOfferingChangeLeadDays(raw)
}

func parseOfferingChangeLeadDays(raw string) (int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, fmt.Errorf("offering change: lead-days setting is empty")
	}
	days, convErr := strconv.Atoi(raw)
	if convErr != nil || days < 0 {
		return 0, fmt.Errorf("offering change: lead-days setting must be a non-negative integer")
	}
	return days, nil
}

// changesEnabled fails closed: a school that never switched offering changes
// on must not collect requests because a config read blipped.
func (s *OfferingChanges) changesEnabled(ctx context.Context) error {
	enabled, err := s.deps.Settings.OfferingChangesEnabled(ctx)
	if err != nil {
		s.logger().Warn("offering change: resolve enabled failed, failing closed",
			slog.String("error", err.Error()),
		)
		return careplan.ErrOfferingChangeDisabled
	}
	if !enabled {
		return careplan.ErrOfferingChangeDisabled
	}
	careOfferingsEnabled, err := s.deps.Settings.CareOfferingsEnabled(ctx)
	if err != nil {
		s.logger().Warn("offering change: resolve care offerings enabled failed, failing closed",
			slog.String("error", err.Error()),
		)
		return careplan.ErrCareOfferingsDisabled
	}
	if !careOfferingsEnabled {
		return careplan.ErrCareOfferingsDisabled
	}
	return nil
}

// bookingsAuthoritative reads whether bookings drive the arrival
// expectations, and with them whether a complete withdrawal may apply.
func (s *OfferingChanges) bookingsAuthoritative(ctx context.Context) (bool, error) {
	authoritative, err := s.deps.Settings.BookingsAuthoritative(ctx)
	if err != nil {
		return false, fmt.Errorf("offering change: resolve booking authority: %w", err)
	}
	return authoritative, nil
}

// carePeriodScope is the approved enrollment an offering change applies to,
// with its phase.
type carePeriodScope struct {
	period *ports.OfferingCarePeriod
	phase  *ports.BookingPhase
}

func (s *OfferingChanges) carePeriods(ctx context.Context, studentID int64) ([]ports.OfferingCarePeriod, error) {
	periods, err := s.deps.Enrollment.StudentCarePeriods(ctx, studentID)
	if err != nil {
		return nil, fmt.Errorf("offering change: list care periods: %w", err)
	}
	return periods, nil
}

func (s *OfferingChanges) periodPhase(ctx context.Context, period ports.OfferingCarePeriod) (*carePeriodScope, error) {
	phase, err := s.deps.Enrollment.Phase(ctx, period.PhaseID)
	if err != nil || phase == nil {
		return nil, fmt.Errorf("offering change: load phase: %w", err)
	}
	return &carePeriodScope{period: &period, phase: phase}, nil
}

func carePeriodCovers(period ports.OfferingCarePeriod, onDate calendar.Date) bool {
	return !period.ServiceStart.After(onDate) && !period.ServiceEnd.Before(onDate)
}

// carePeriodAt resolves the approved enrollment that covers onDate. A
// request may have to take effect in the next approved period when the
// notice period crosses a care-period boundary.
func (s *OfferingChanges) carePeriodAt(ctx context.Context, studentID int64, onDate calendar.Date) (*carePeriodScope, error) {
	periods, err := s.carePeriods(ctx, studentID)
	if err != nil {
		return nil, err
	}
	for _, candidate := range periods {
		if carePeriodCovers(candidate, onDate) {
			return s.periodPhase(ctx, candidate)
		}
	}
	return nil, careplan.ErrOfferingChangeNoEnrollment
}

// carePeriodAtOrNext resolves the period covering onDate, or the nearest
// approved period beginning after it, so the catalog can offer the first
// valid date when a lead time falls before the next period.
func (s *OfferingChanges) carePeriodAtOrNext(ctx context.Context, studentID int64, onDate calendar.Date) (*carePeriodScope, []ports.OfferingCarePeriod, error) {
	periods, err := s.carePeriods(ctx, studentID)
	if err != nil {
		return nil, nil, err
	}
	var next *ports.OfferingCarePeriod
	for i, candidate := range periods {
		if carePeriodCovers(candidate, onDate) {
			scope, phaseErr := s.periodPhase(ctx, candidate)
			return scope, periods, phaseErr
		}
		if candidate.ServiceStart.After(onDate) && (next == nil || candidate.ServiceStart.Before(next.ServiceStart)) {
			next = &periods[i]
		}
	}
	if next == nil {
		return nil, nil, careplan.ErrOfferingChangeNoEnrollment
	}
	scope, err := s.periodPhase(ctx, *next)
	return scope, periods, err
}

func (s *OfferingChanges) carePeriodForEarliestEffectiveDate(ctx context.Context, studentID int64) (*carePeriodScope, calendar.Date, []ports.OfferingCarePeriod, error) {
	earliest, err := s.EarliestEffectiveFrom(ctx)
	if err != nil {
		return nil, calendar.Date(""), nil, err
	}
	scope, periods, err := s.carePeriodAtOrNext(ctx, studentID, earliest)
	if err != nil {
		return nil, calendar.Date(""), nil, err
	}
	if earliest.Before(scope.phase.ServiceStart) {
		earliest = scope.phase.ServiceStart
	}
	return scope, earliest, periods, nil
}

// latestContiguousApprovedCarePeriodEnd returns the end of the uninterrupted
// approved care beginning with initial. The date picker cannot offer a date
// in a gap because CatalogAt requires an enrollment on the chosen date.
func (s *OfferingChanges) latestContiguousApprovedCarePeriodEnd(ctx context.Context, studentID int64, initial *ports.OfferingCarePeriod) (calendar.Date, error) {
	periods, err := s.carePeriods(ctx, studentID)
	if err != nil {
		return calendar.Date(""), err
	}
	if initial == nil {
		return calendar.Date(""), careplan.ErrOfferingChangeNoEnrollment
	}
	return contiguousCarePeriodEnd(periods, *initial), nil
}

func contiguousCarePeriodEnd(periods []ports.OfferingCarePeriod, initial ports.OfferingCarePeriod) calendar.Date {
	latest := initial.ServiceEnd
	// Periods are returned newest first. Repeat the scan because an earlier
	// overlapping period can extend the range far enough to join another one.
	for extended := true; extended; {
		extended = false
		for _, period := range periods {
			if !period.ServiceStart.After(latest.AddDays(1)) && latest.Before(period.ServiceEnd) {
				latest = period.ServiceEnd
				extended = true
			}
		}
	}
	return latest
}

// reviewAllows applies the caller's parent-request review scope to one
// student.
func (s *OfferingChanges) reviewAllows(ctx context.Context, student *ports.ReviewStudent) (bool, error) {
	scope, err := s.deps.Scope(ctx)
	if err != nil {
		return false, fmt.Errorf("offering change: resolve request reviewer scope: %w", err)
	}
	return scope.Allows(student), nil
}

// careEnded reports whether the student's care ended before the day.
func careEnded(student *ports.ReviewStudent, day calendar.Date) bool {
	return student != nil && student.CareEndedOn(careplan.Date(day))
}
