package timetracking

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The adapters in this file let the Monatskarte math run over data loaded once
// for MANY staff members (#1417). Each satisfies one of the narrow reader
// interfaces workTimeMonthService depends on, serving from a prefetched map
// instead of issuing a query per staff member. There is deliberately NO
// arithmetic here: the cross-staff overview must produce byte-identical numbers
// to the per-staff detail view, so it runs the very same code with the very
// same readers.
//
// This works because the prefetch window is the same for every staff member of
// a tenant: computeAggregates ranges over [first, last], where `first` derives
// from the account-start anchor — a TENANT-wide setting, not a per-staff value
// — and `last` from the requested month. One load therefore covers everybody.
// TestStaffOverview_PrefetchWindowIsTenantWide pins that property.
//
// Callers must never request a WIDER range than the prefetch covers; the
// adapters serve their whole slice and cannot detect the mistake.
type monthPrefetch struct {
	from, to calendar.Date

	staff          map[int64]*StaffScheduleAssignment
	sessions       map[int64][]*WorkSession
	breaks         map[int64][]*WorkSessionBreak
	absences       map[int64][]*StaffAbsence
	adjustments    map[int64][]*StaffBalanceAdjustment
	shifts         map[int64][]*TimeTrackingShift
	schedules      map[int64]WorkScheduleTargets
	scheduleHist   map[int64]bool
	workTimeModels map[int64]*WorkTimeTargetModel
	holidays       map[calendar.Date]bool
	overrides      workforce.TargetOverrideDays
	snapshots      map[int64][]*MonthSnapshot
	settings       *memoSettingsResolver
}

// memoSettingsResolver resolves each settings key once per request. The
// settings service issues one query per Resolve call with no cache, which would
// otherwise cost one query per staff member for the account-start anchor alone.
type memoSettingsResolver struct {
	inner monthSettingsResolver

	mu     sync.Mutex
	loaded bool
	value  string
	err    error
}

func newMemoSettingsResolver(inner monthSettingsResolver) *memoSettingsResolver {
	return &memoSettingsResolver{inner: inner}
}

func (m *memoSettingsResolver) AccountStartDate(ctx context.Context) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.loaded {
		return m.value, m.err
	}
	if m.inner == nil {
		return "", nil
	}
	m.value, m.err = m.inner.AccountStartDate(ctx)
	m.loaded = true
	return m.value, m.err
}

// These are programming errors, not data conditions: the prefetch is built
// from the very staff list the overview iterates, so a miss means the caller
// asked for something it never loaded. Failing loudly beats reporting a
// silently wrong Stundenkonto.
var (
	errStaffNotPrefetched         = errors.New("staff member is not part of the prefetch")
	errWorkTimeModelNotPrefetched = errors.New("work time model is not part of the prefetch")
)

// --- readers ---------------------------------------------------------------

type prefetchedStaffReader struct{ p *monthPrefetch }

func (r prefetchedStaffReader) ScheduleAssignment(_ context.Context, staffID int64) (*StaffScheduleAssignment, error) {
	staff, ok := r.p.staff[staffID]
	if !ok {
		return nil, errStaffNotPrefetched
	}
	return staff, nil
}

type prefetchedSessionReader struct{ p *monthPrefetch }

func (r prefetchedSessionReader) ListOverlappingWorkSessions(_ context.Context, staffIDs []int64, from time.Time, to *time.Time) ([]workforce.WorkSession, error) {
	result := make([]workforce.WorkSession, 0)
	for _, staffID := range staffIDs {
		for _, session := range r.p.sessions[staffID] {
			end := time.Now()
			if session.CheckOutTime != nil {
				end = *session.CheckOutTime
			}
			if !end.After(from) || to != nil && !session.CheckInTime.Before(*to) {
				continue
			}
			result = append(result, WorkSessionToCapability(session))
		}
	}
	return result, nil
}

type prefetchedBreakReader struct{ p *monthPrefetch }

func (r prefetchedBreakReader) ListWorkSessionBreaks(_ context.Context, filter workforce.WorkSessionBreakFilter) ([]workforce.WorkSessionBreak, error) {
	result := make([]workforce.WorkSessionBreak, 0)
	ids := filter.SessionIDs
	if filter.SessionID != 0 {
		ids = []int64{filter.SessionID}
	}
	for _, id := range ids {
		for _, entry := range r.p.breaks[id] {
			result = append(result, WorkSessionBreakToCapability(entry))
		}
	}
	return result, nil
}

type prefetchedAbsenceReader struct{ p *monthPrefetch }

func (r prefetchedAbsenceReader) ListStaffAbsences(_ context.Context, filter workforce.StaffAbsenceFilter) ([]workforce.StaffAbsence, error) {
	values := r.p.absences[filter.StaffID]
	result := make([]workforce.StaffAbsence, 0, len(values))
	for _, value := range values {
		result = append(result, StaffAbsenceToCapability(value))
	}
	return result, nil
}

type prefetchedAdjustmentReader struct{ p *monthPrefetch }

func (r prefetchedAdjustmentReader) ListStaffBalanceAdjustments(_ context.Context, filter workforce.StaffBalanceAdjustmentFilter) ([]workforce.StaffBalanceAdjustment, error) {
	from, to := calendar.Date(filter.EffectiveFrom), calendar.Date(filter.EffectiveTo)
	adjustments := r.p.adjustments[filter.StaffID]
	result := make([]workforce.StaffBalanceAdjustment, 0, len(adjustments))
	for _, adjustment := range adjustments {
		if adjustment.EffectiveDate.Before(from) || adjustment.EffectiveDate.After(to) {
			continue
		}
		result = append(result, StaffBalanceAdjustmentToCapability(adjustment))
	}
	return result, nil
}

type prefetchedShiftReader struct{ p *monthPrefetch }

func (r prefetchedShiftReader) FindByStaffAndDateRange(_ context.Context, staffID int64, _, _ calendar.Date) ([]*TimeTrackingShift, error) {
	return r.p.shifts[staffID], nil
}

type prefetchedScheduleReader struct{ p *monthPrefetch }

func (r prefetchedScheduleReader) TargetsForStaff(_ context.Context, staffID int64, _, _ calendar.Date) (WorkScheduleTargets, error) {
	return r.p.schedules[staffID], nil
}

func (r prefetchedScheduleReader) HasScheduleHistory(_ context.Context, staffID int64) (bool, error) {
	return r.p.scheduleHist[staffID], nil
}

type prefetchedModelReader struct{ p *monthPrefetch }

func (r prefetchedModelReader) FindByID(_ context.Context, id int64) (*WorkTimeTargetModel, error) {
	model, ok := r.p.workTimeModels[id]
	if !ok {
		return nil, errWorkTimeModelNotPrefetched
	}
	return model, nil
}

// prefetchedHolidayReader serves the non-working-day set. targetFor only does a
// map lookup, so a map covering a superset of the requested range is
// behaviourally identical to a per-range query.
type prefetchedHolidayReader struct{ p *monthPrefetch }

func (r prefetchedHolidayReader) HolidayDates(_ context.Context, _, _ calendar.Date) (map[calendar.Date]bool, error) {
	return r.p.holidays, nil
}

// prefetchedOverrideReader serves the Sonderarbeitszeit days loaded for every
// staff member at once; the resolver only looks up days of its own range.
type prefetchedOverrideReader struct{ p *monthPrefetch }

func (r prefetchedOverrideReader) StaffTargetOverrideDays(_ context.Context, staffIDs []int64, _, _ string) (workforce.TargetOverrideDays, error) {
	result := make(workforce.TargetOverrideDays, len(staffIDs))
	for _, staffID := range staffIDs {
		result[staffID] = r.p.overrides[staffID]
	}
	return result, nil
}

// prefetchedSnapshotReader answers "newest active snapshot at or before this
// month" from memory. It must be prefetched like everything else: the splice
// asks per staff member AND per month, so a repository-backed reader here would
// silently reintroduce a per-staff query (which
// TestTimeTrackingOverview_QueryCountIsConstant catches).
type prefetchedSnapshotReader struct{ p *monthPrefetch }

func (r prefetchedSnapshotReader) LatestClosedMonth(_ context.Context, staffID int64, year, month int) (*MonthSnapshot, error) {
	limit := year*12 + month
	var best *MonthSnapshot
	for _, snapshot := range r.p.snapshots[staffID] {
		ordinal := snapshot.Year*12 + snapshot.Month
		if ordinal > limit {
			continue
		}
		if best == nil || ordinal > best.Year*12+best.Month {
			best = snapshot
		}
	}
	return best, nil
}

// newPrefetchedMonthService builds a month service whose readers all serve from
// the prefetch. Constructed the same way services/factory.go builds the real
// one, with the same optional readers, so no code path differs.
func newPrefetchedMonthService(p *monthPrefetch, logger *slog.Logger) WorkTimeMonthService {
	svc := NewWorkTimeMonthService(
		prefetchedSessionReader{p},
		prefetchedBreakReader{p},
		prefetchedAbsenceReader{p},
		prefetchedStaffReader{p},
		prefetchedScheduleReader{p},
		prefetchedModelReader{p},
		prefetchedShiftReader{p},
		p.settings,
		logger,
		WithMonthHolidays(prefetchedHolidayReader{p}),
		WithMonthTargetOverrides(prefetchedOverrideReader{p}),
		WithMonthAdjustments(prefetchedAdjustmentReader{p}),
		WithMonthSnapshots(prefetchedSnapshotReader{p}),
	)
	return svc
}
