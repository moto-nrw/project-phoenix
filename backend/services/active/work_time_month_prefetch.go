package active

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	configModels "github.com/moto-nrw/project-phoenix/models/config"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
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
	from, to timezone.Date

	staff          map[int64]*userModels.Staff
	sessions       map[int64][]*activeModels.WorkSession
	breaks         map[int64][]*activeModels.WorkSessionBreak
	absences       map[int64][]*activeModels.StaffAbsence
	adjustments    map[int64][]*activeModels.StaffBalanceAdjustment
	shifts         map[int64][]*scheduleModels.StaffShift
	schedules      map[int64][]*configModels.StaffWorkSchedule
	scheduleHist   map[int64]bool
	workTimeModels map[int64]*configModels.WorkTimeModel
	holidays       map[timezone.Date]bool
	snapshots      map[int64][]*activeModels.StaffMonthBalanceSnapshot
	settings       *memoSettingsResolver
}

// memoSettingsResolver resolves each settings key once per request. The
// settings service issues one query per Resolve call with no cache, which would
// otherwise cost one query per staff member for the account-start anchor alone.
type memoSettingsResolver struct {
	inner monthSettingsResolver

	mu     sync.Mutex
	values map[string]string
	errs   map[string]error
}

func newMemoSettingsResolver(inner monthSettingsResolver) *memoSettingsResolver {
	return &memoSettingsResolver{
		inner:  inner,
		values: make(map[string]string),
		errs:   make(map[string]error),
	}
}

func (m *memoSettingsResolver) ResolveString(ctx context.Context, key string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if value, ok := m.values[key]; ok {
		return value, m.errs[key]
	}
	if m.inner == nil {
		return "", nil
	}
	value, err := m.inner.ResolveString(ctx, key)
	m.values[key], m.errs[key] = value, err
	return value, err
}

// These are programming errors, not data conditions: the prefetch is built
// from the very staff list the overview iterates, so a miss means the caller
// asked for something it never loaded. Failing loudly beats reporting a
// silently wrong Stundenkonto.
var (
	errUnexpectedPrefetchID       = errors.New("prefetched staff reader expects an int64 id")
	errStaffNotPrefetched         = errors.New("staff member is not part of the prefetch")
	errWorkTimeModelNotPrefetched = errors.New("work time model is not part of the prefetch")
)

// --- readers ---------------------------------------------------------------

type prefetchedStaffReader struct{ p *monthPrefetch }

func (r prefetchedStaffReader) FindByID(_ context.Context, id any) (*userModels.Staff, error) {
	staffID, ok := id.(int64)
	if !ok {
		return nil, errUnexpectedPrefetchID
	}
	staff, ok := r.p.staff[staffID]
	if !ok {
		return nil, errStaffNotPrefetched
	}
	return staff, nil
}

type prefetchedSessionReader struct{ p *monthPrefetch }

func (r prefetchedSessionReader) ListOverlappingByStaffID(_ context.Context, staffID int64, from time.Time, to *time.Time) ([]*activeModels.WorkSession, error) {
	sessions := r.p.sessions[staffID]
	result := make([]*activeModels.WorkSession, 0, len(sessions))
	for _, session := range sessions {
		end := time.Now()
		if session.CheckOutTime != nil {
			end = *session.CheckOutTime
		}
		if !end.After(from) || to != nil && !session.CheckInTime.Before(*to) {
			continue
		}
		result = append(result, session)
	}
	return result, nil
}

type prefetchedBreakReader struct{ p *monthPrefetch }

func (r prefetchedBreakReader) GetBySessionID(_ context.Context, sessionID int64) ([]*activeModels.WorkSessionBreak, error) {
	return r.p.breaks[sessionID], nil
}

func (r prefetchedBreakReader) GetBySessionIDs(_ context.Context, sessionIDs []int64) (map[int64][]*activeModels.WorkSessionBreak, error) {
	result := make(map[int64][]*activeModels.WorkSessionBreak, len(sessionIDs))
	for _, sessionID := range sessionIDs {
		result[sessionID] = r.p.breaks[sessionID]
	}
	return result, nil
}

type prefetchedAbsenceReader struct{ p *monthPrefetch }

func (r prefetchedAbsenceReader) GetByStaffAndDateRange(_ context.Context, staffID int64, _, _ timezone.Date) ([]*activeModels.StaffAbsence, error) {
	return r.p.absences[staffID], nil
}

type prefetchedAdjustmentReader struct{ p *monthPrefetch }

func (r prefetchedAdjustmentReader) GetByStaffAndDateRange(_ context.Context, staffID int64, from, to timezone.Date) ([]*activeModels.StaffBalanceAdjustment, error) {
	adjustments := r.p.adjustments[staffID]
	result := make([]*activeModels.StaffBalanceAdjustment, 0, len(adjustments))
	for _, adjustment := range adjustments {
		if adjustment.EffectiveDate.Before(from) || adjustment.EffectiveDate.After(to) {
			continue
		}
		result = append(result, adjustment)
	}
	return result, nil
}

type prefetchedShiftReader struct{ p *monthPrefetch }

func (r prefetchedShiftReader) FindByStaffAndDateRange(_ context.Context, staffID int64, _, _ timezone.Date) ([]*scheduleModels.StaffShift, error) {
	return r.p.shifts[staffID], nil
}

type prefetchedScheduleReader struct{ p *monthPrefetch }

func (r prefetchedScheduleReader) FindByStaffIDsValidInRange(_ context.Context, staffIDs []int64, _, _ timezone.Date) ([]*configModels.StaffWorkSchedule, error) {
	var entries []*configModels.StaffWorkSchedule
	for _, staffID := range staffIDs {
		entries = append(entries, r.p.schedules[staffID]...)
	}
	return entries, nil
}

func (r prefetchedScheduleReader) HasScheduleHistory(_ context.Context, staffID int64) (bool, error) {
	return r.p.scheduleHist[staffID], nil
}

type prefetchedModelReader struct{ p *monthPrefetch }

func (r prefetchedModelReader) FindByID(_ context.Context, id int64) (*configModels.WorkTimeModel, error) {
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

func (r prefetchedHolidayReader) HolidayDates(_ context.Context, _, _ timezone.Date) (map[timezone.Date]bool, error) {
	return r.p.holidays, nil
}

// prefetchedSnapshotReader answers "newest active snapshot at or before this
// month" from memory. It must be prefetched like everything else: the splice
// asks per staff member AND per month, so a repository-backed reader here would
// silently reintroduce a per-staff query (which
// TestTimeTrackingOverview_QueryCountIsConstant catches).
type prefetchedSnapshotReader struct{ p *monthPrefetch }

func (r prefetchedSnapshotReader) GetLatestClosedThrough(_ context.Context, staffID int64, year, month int) (*activeModels.StaffMonthBalanceSnapshot, error) {
	limit := year*12 + month
	var best *activeModels.StaffMonthBalanceSnapshot
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
// one, including every setter, so no code path differs.
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
	)
	svc.SetHolidayReader(prefetchedHolidayReader{p})
	svc.SetAdjustmentReader(prefetchedAdjustmentReader{p})
	svc.SetSnapshotReader(prefetchedSnapshotReader{p})
	return svc
}
