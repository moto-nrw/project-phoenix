package schedule

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// CalendarPeriodService defines operations for managing calendar periods
type CalendarPeriodService interface {
	// CRUD
	GetAllPeriods(ctx context.Context) ([]*schedule.CalendarPeriod, error)
	GetActivePeriods(ctx context.Context) ([]*schedule.CalendarPeriod, error)
	GetPeriodByID(ctx context.Context, id int64) (*schedule.CalendarPeriod, error)
	CreatePeriod(ctx context.Context, period *schedule.CalendarPeriod) error
	UpdatePeriod(ctx context.Context, period *schedule.CalendarPeriod) error
	DeletePeriod(ctx context.Context, id int64) error

	// EnsureDefaultSchoolYear guarantees the tenant has at least one calendar
	// period. If none exists, it creates the current German school year
	// (Aug 1 – Jul 31) as an active period. Idempotent and race-safe;
	// created reports whether this call inserted the default period.
	EnsureDefaultSchoolYear(ctx context.Context) (periods []*schedule.CalendarPeriod, created bool, err error)

	// FindActiveOverlaps returns the active periods whose date range overlaps
	// the given period (excluding the period itself). Returns nil when the
	// period is inactive — only active/active collisions are advisory-worthy.
	FindActiveOverlaps(ctx context.Context, period *schedule.CalendarPeriod) ([]*schedule.CalendarPeriod, error)

	// GetUsageCounts reports how many rows reference each calendar period of
	// the current tenant through nullable calendar_period_id FKs.
	// Counts drive list display and delete warnings. A linked care offering may
	// separately block an update/delete when clearing or shrinking the period
	// would invalidate its timetable series.
	GetUsageCounts(ctx context.Context) (map[int64]schedule.CalendarPeriodUsage, error)

	// A/B week resolution — weekPattern: 0=every, 1=week A, 2=week B
	ShouldMaterialize(weekPattern int, instanceDate timezone.Date, period *schedule.CalendarPeriod) bool
}

// calendarPeriodService implements CalendarPeriodService
type calendarPeriodService struct {
	repo                         schedule.CalendarPeriodRepository
	db                           *bun.DB
	validateCareOfferingChange   func(context.Context, int64, *schedule.CalendarPeriod) error
	lockTenantRecurrenceMutation func(context.Context) error
	logger                       *slog.Logger
}

// CalendarPeriodServiceConfig wires mutation-time dependencies. Production supplies DB and
// ValidateCareOfferingChange so updates/deletes share the tenant recurrence
// gate with template and care-offering mutations.
type CalendarPeriodServiceConfig struct {
	Repo                       schedule.CalendarPeriodRepository
	DB                         *bun.DB
	ValidateCareOfferingChange func(context.Context, int64, *schedule.CalendarPeriod) error
	Logger                     *slog.Logger
}

// NewCalendarPeriodServiceWithConfig creates the production calendar-period
// service with transaction-scoped recurrence locking and care-link preflight
// validation.
func NewCalendarPeriodServiceWithConfig(cfg CalendarPeriodServiceConfig) CalendarPeriodService {
	var lockMutation func(context.Context) error
	if cfg.DB != nil {
		lockMutation = func(ctx context.Context) error {
			return lockTenantRecurrenceWrites(ctx, cfg.DB)
		}
	}
	return &calendarPeriodService{
		repo:                         cfg.Repo,
		db:                           cfg.DB,
		validateCareOfferingChange:   cfg.ValidateCareOfferingChange,
		lockTenantRecurrenceMutation: lockMutation,
		logger:                       cfg.Logger,
	}
}

func (s *calendarPeriodService) getLogger() *slog.Logger {
	return cmp.Or(s.logger, slog.Default())
}

// GetAllPeriods returns all calendar periods for the current tenant
func (s *calendarPeriodService) GetAllPeriods(ctx context.Context) ([]*schedule.CalendarPeriod, error) {
	periods, err := s.repo.FindByTenantID(ctx)
	if err != nil {
		return nil, &ScheduleError{Op: "get all calendar periods", Err: err}
	}
	return periods, nil
}

// GetActivePeriods returns all active calendar periods for the current tenant
func (s *calendarPeriodService) GetActivePeriods(ctx context.Context) ([]*schedule.CalendarPeriod, error) {
	periods, err := s.repo.FindActiveByTenantID(ctx)
	if err != nil {
		return nil, &ScheduleError{Op: "get active calendar periods", Err: err}
	}
	return periods, nil
}

// GetPeriodByID returns a calendar period by its ID
func (s *calendarPeriodService) GetPeriodByID(ctx context.Context, id int64) (*schedule.CalendarPeriod, error) {
	period, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, &ScheduleError{Op: "get calendar period by id", Err: err}
	}
	return period, nil
}

// CreatePeriod creates a new calendar period. The duplicate-name and
// same-type overlap checks run under the tenant recurrence gate: without it,
// two concurrent creates could both pass the overlap check before either
// insert commits, violating the hard active/same-type invariant (#1837).
func (s *calendarPeriodService) CreatePeriod(ctx context.Context, period *schedule.CalendarPeriod) error {
	const op = "create calendar period"
	if err := period.Validate(); err != nil {
		return &ScheduleError{Op: op, Err: err}
	}

	return s.withRecurrenceGate(ctx, op, func(txCtx context.Context) error {
		// Check for duplicate name
		existing, err := s.repo.FindByName(txCtx, period.Name)
		if err != nil {
			return &ScheduleError{Op: op, Err: err}
		}
		if existing != nil {
			return &ScheduleError{Op: op, Err: schedule.ErrCalendarPeriodNameConflict}
		}

		if err := s.ensureNoActiveSameTypeOverlap(txCtx, period); err != nil {
			return err
		}

		period.SetTenantID(tenant.FromContext(txCtx))
		if err := s.repo.Create(txCtx, period); err != nil {
			return &ScheduleError{Op: op, Err: err}
		}

		s.getLogger().Info("calendar period created",
			slog.Int64("period_id", period.ID),
			slog.String("name", period.Name),
			slog.String("period_type", period.PeriodType),
		)

		return nil
	})
}

// UpdatePeriod updates an existing calendar period
func (s *calendarPeriodService) UpdatePeriod(ctx context.Context, period *schedule.CalendarPeriod) error {
	if err := period.Validate(); err != nil {
		return &ScheduleError{Op: "update calendar period", Err: err}
	}

	// Check for duplicate name (excluding self)
	existing, err := s.repo.FindByName(ctx, period.Name)
	if err != nil {
		return &ScheduleError{Op: "update calendar period", Err: err}
	}
	if existing != nil && existing.ID != period.ID {
		return &ScheduleError{Op: "update calendar period", Err: schedule.ErrCalendarPeriodNameConflict}
	}

	return s.mutatePeriod(ctx, "update calendar period", period.ID, period, func(txCtx context.Context) error {
		current, err := s.repo.FindByID(txCtx, period.ID)
		if err != nil {
			return &ScheduleError{Op: "update calendar period", Err: err}
		}
		// The hard overlap rule only guards changes to the scheduling-relevant
		// fields (dates, active flag, and type — the check runs against the NEW
		// type, so re-typing a period out of a conflict stays possible).
		// Rename-only edits of a period that already overlaps (legacy data from
		// before the rule) stay possible; the advisory warning keeps surfacing
		// those.
		if current.StartDate != period.StartDate || current.EndDate != period.EndDate ||
			current.IsActive != period.IsActive || current.PeriodType != period.PeriodType {
			if err := s.ensureNoActiveSameTypeOverlap(txCtx, period); err != nil {
				return err
			}
		}
		if err := s.repo.Update(txCtx, period); err != nil {
			return &ScheduleError{Op: "update calendar period", Err: err}
		}
		return nil
	})
}

// ensureNoActiveSameTypeOverlap rejects a period whose resulting state would
// be active and overlap another active period of the same period_type
// (#1837). Inactive periods never conflict. Cross-type overlaps (holidays
// inside a school year) stay legal — the advisory FindActiveOverlaps warning
// covers those.
func (s *calendarPeriodService) ensureNoActiveSameTypeOverlap(ctx context.Context, period *schedule.CalendarPeriod) error {
	if period == nil || !period.IsActive {
		return nil
	}
	overlaps, err := s.repo.FindActiveOverlappingByType(ctx, period.PeriodType, period.StartDate, period.EndDate, period.ID)
	if err != nil {
		return &ScheduleError{Op: "check same-type period overlap", Err: err}
	}
	if len(overlaps) > 0 {
		return &ScheduleError{Op: "check same-type period overlap", Err: &schedule.CalendarPeriodOverlapError{Overlaps: overlaps}}
	}
	return nil
}

// GetUsageCounts reports per-period reference counts for the current tenant.
// Thin by design: the repository is the data-access boundary for the
// cross-schema counting query (analog PhaseService.DeleteImpact).
func (s *calendarPeriodService) GetUsageCounts(ctx context.Context) (map[int64]schedule.CalendarPeriodUsage, error) {
	usage, err := s.repo.UsageCounts(ctx)
	if err != nil {
		return nil, &ScheduleError{Op: "get calendar period usage counts", Err: err}
	}
	return usage, nil
}

// DeletePeriod deletes a calendar period by ID
func (s *calendarPeriodService) DeletePeriod(ctx context.Context, id int64) error {
	return s.mutatePeriod(ctx, "delete calendar period", id, nil, func(txCtx context.Context) error {
		if err := s.repo.Delete(txCtx, id); err != nil {
			return &ScheduleError{Op: "delete calendar period", Err: err}
		}
		return nil
	})
}

// mutatePeriod serializes period changes with every recurrence-derived write,
// validates care-offering links before the first mutation, and then executes
// the repository write in the same tenant transaction. Preflight is deliberate:
// a domain conflict maps to HTTP 409, and TenantTxMiddleware commits non-5xx
// responses unless explicitly marked for rollback.
func (s *calendarPeriodService) mutatePeriod(
	ctx context.Context,
	op string,
	periodID int64,
	replacement *schedule.CalendarPeriod,
	mutate func(context.Context) error,
) error {
	if s.db != nil && s.validateCareOfferingChange == nil {
		return &ScheduleError{Op: op, Err: errors.New("calendar period care-offering validation is not configured")}
	}
	return s.withRecurrenceGate(ctx, op, func(txCtx context.Context) error {
		// nil only on the legacy DB-less unit-test path, where the gate above
		// already guarantees no care-offering dependency was wired.
		if s.validateCareOfferingChange != nil {
			if err := s.validateCareOfferingChange(txCtx, periodID, replacement); err != nil {
				return &ScheduleError{Op: op + ": validate linked care offerings", Err: err}
			}
		}
		return mutate(txCtx)
	})
}

// withRecurrenceGate runs mutate inside a tenant transaction while holding the
// tenant-wide recurrence advisory lock, serializing the change against every
// other recurrence-derived write (creates, updates, deletes, re-planning,
// materialization). Without a configured DB (legacy unit-test constructor)
// mutate runs directly, provided no gate dependency was wired.
func (s *calendarPeriodService) withRecurrenceGate(
	ctx context.Context,
	op string,
	mutate func(context.Context) error,
) error {
	if s.db == nil {
		if s.validateCareOfferingChange != nil || s.lockTenantRecurrenceMutation != nil {
			return &ScheduleError{Op: op, Err: errors.New("calendar period mutation database is not configured")}
		}
		return mutate(ctx)
	}
	tenantID := tenant.FromContext(ctx)
	if tenantID <= 0 {
		return &ScheduleError{Op: op, Err: errors.New("no tenant in context")}
	}
	return tenant.WithTenantTx(ctx, s.db, tenantID, func(txCtx context.Context, _ bun.Tx) error {
		if s.lockTenantRecurrenceMutation == nil {
			return &ScheduleError{Op: op, Err: errors.New("calendar period recurrence lock is not configured")}
		}
		if err := s.lockTenantRecurrenceMutation(txCtx); err != nil {
			return &ScheduleError{Op: op + ": lock recurrence", Err: err}
		}
		return mutate(txCtx)
	})
}

// defaultSchoolYearBounds computes the German school year containing the
// given calendar day: it starts on August 1st of the current year when the
// day is in August or later, otherwise on August 1st of the previous year,
// and always ends on July 31st of the following year.
//
// MUST stay in sync with the frontend helper schoolYearPeriodDefaults in
// frontend/src/app/[tenant]/(protected)/timetables/page.tsx — both sides
// derive the same name ("Schuljahr YYYY/YYYY+1") and the same date bounds
// so the bootstrap endpoint and the client-side prefill never diverge.
//
// Extracted as a pure function so the year-boundary logic is testable
// without injecting a clock.
func defaultSchoolYearBounds(today timezone.Date) (name string, start, end timezone.Date) {
	startYear := today.Year
	if today.Month < time.August {
		startYear--
	}
	name = fmt.Sprintf("Schuljahr %d/%d", startYear, startYear+1)
	return name, timezone.NewDate(startYear, time.August, 1), timezone.NewDate(startYear+1, time.July, 31)
}

// EnsureDefaultSchoolYear guarantees the tenant has at least one calendar
// period (WP-B1). When periods already exist, it returns them unchanged with
// created=false. Otherwise it inserts the current school year via the
// race-safe CreateIfAbsent upsert, re-lists, and returns the fresh state.
// The read + insert run under the same tenant recurrence gate as CreatePeriod:
// without it, a bootstrap racing a normal period create could list zero
// periods, then insert the default school year next to a just-committed
// overlapping active period of the same type, bypassing the hard overlap
// invariant (#1837). Two concurrent calls yield exactly one row and no errors.
func (s *calendarPeriodService) EnsureDefaultSchoolYear(ctx context.Context) ([]*schedule.CalendarPeriod, bool, error) {
	const op = "ensure default school year"
	var periods []*schedule.CalendarPeriod
	var created bool

	err := s.withRecurrenceGate(ctx, op, func(txCtx context.Context) error {
		var err error
		periods, err = s.repo.FindByTenantID(txCtx)
		if err != nil {
			return &ScheduleError{Op: op, Err: err}
		}
		if len(periods) > 0 {
			return nil
		}

		name, start, end := defaultSchoolYearBounds(timezone.TodayDate())
		period := &schedule.CalendarPeriod{
			Name:            name,
			PeriodType:      schedule.PeriodTypeSchoolYear,
			StartDate:       start,
			EndDate:         end,
			WeekCycleLength: 1,
			IsActive:        true,
		}
		period.SetTenantID(tenant.FromContext(txCtx))

		if err := s.ensureNoActiveSameTypeOverlap(txCtx, period); err != nil {
			return err
		}

		created, err = s.repo.CreateIfAbsent(txCtx, period)
		if err != nil {
			return &ScheduleError{Op: op, Err: err}
		}
		if created {
			s.getLogger().Info("default school year period created",
				slog.Int64("period_id", period.ID),
				slog.String("name", period.Name),
			)
		}

		periods, err = s.repo.FindByTenantID(txCtx)
		if err != nil {
			return &ScheduleError{Op: op, Err: err}
		}
		return nil
	})
	if err != nil {
		return nil, false, err
	}
	return periods, created, nil
}

// FindActiveOverlaps returns the active periods overlapping the given period
// (WP-B2, QA #1577 H3). The check is advisory only — callers attach the
// result as a warning, never as a save blocker. Inactive periods cannot
// collide with anything, so the repo round-trip is skipped for them.
func (s *calendarPeriodService) FindActiveOverlaps(ctx context.Context, period *schedule.CalendarPeriod) ([]*schedule.CalendarPeriod, error) {
	if period == nil || !period.IsActive {
		return nil, nil
	}
	overlaps, err := s.repo.FindActiveOverlapping(ctx, period.StartDate, period.EndDate, period.ID)
	if err != nil {
		return nil, &ScheduleError{Op: "find active overlapping periods", Err: err}
	}
	return overlaps, nil
}

// ShouldMaterialize determines whether a schedule with the given weekPattern should
// produce an instance on instanceDate, considering the period's A/B week cycle.
//
// Uses day-based difference calculation (NOT ISO week numbers) to avoid
// year-boundary bugs. See timetable-system-plan.md §6.1 for algorithm details.
//
// timezone.Date.DaysUntil anchors both calendar days at UTC midnight before
// subtracting, so DST transitions in Europe/Berlin (167- or 169-hour weeks at
// the end of March/October) can never skew the day count.
//
// weekPattern: 0=every week, 1=week A, 2=week B (maps to currentPattern 1, 2, ...)
func (s *calendarPeriodService) ShouldMaterialize(weekPattern int, instanceDate timezone.Date, period *schedule.CalendarPeriod) bool {
	return ShouldMaterializeWeekPattern(weekPattern, instanceDate, period)
}

// ShouldMaterializeWeekPattern is the pure A/B-cycle predicate shared by
// materialization and cross-domain care-offering recurrence validation.
// Keeping one implementation prevents a catalog link from being accepted for
// dates the materializer would later skip.
func ShouldMaterializeWeekPattern(weekPattern int, instanceDate timezone.Date, period *schedule.CalendarPeriod) bool {
	if weekPattern == 0 {
		return true // every week
	}
	if period == nil || period.WeekCycleLength <= 1 {
		return true // no alternation configured
	}
	if period.WeekCycleAnchor == nil {
		return true // no anchor set, can't compute — allow by default
	}

	daysDiff := period.WeekCycleAnchor.DaysUntil(instanceDate)
	weeksDiff := daysDiff / 7
	// Go's integer division truncates toward zero, so for negative daysDiff
	// we need an explicit floor when the division isn't exact.
	if daysDiff < 0 && daysDiff%7 != 0 {
		weeksDiff--
	}

	// Modulo that handles negative values correctly.
	cycleLen := period.WeekCycleLength
	currentPattern := ((weeksDiff % cycleLen) + cycleLen) % cycleLen
	currentPattern++ // 1-based: 1=A, 2=B, etc.

	return currentPattern == weekPattern
}
