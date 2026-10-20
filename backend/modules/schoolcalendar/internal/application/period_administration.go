package application

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"

	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar/internal/ports"
)

// Administration holds the collaborators of the administrative period write
// path. Transaction and Today are required; the gate and the guard are
// optional so a composition without recurrence-derived planning (unit
// fixtures, the unobserved test graph) still administers periods.
type Administration struct {
	Observe           ports.Observer
	FederalState      func(context.Context) (string, error)
	Transaction       ports.WriteTransaction
	RecurrenceGate    ports.RecurrenceGate
	CareOfferingGuard ports.CareOfferingGuard
	Today             ports.Today
}

// AddCalendarPeriod creates a period under the recurrence gate. The
// duplicate-name and same-type overlap checks run inside the gate: without
// it, two concurrent adds could both pass the overlap check before either
// insert commits, violating the hard active/same-type invariant (#1837).

// ChangeCalendarPeriod replaces the period's fields. The care-offering guard
// sees the proposed row before any foreign key can be cleared, and the hard
// overlap rule only guards changes to the scheduling-relevant fields (dates,
// active flag and type, checked against the NEW type so re-typing a period
// out of a conflict stays possible). Rename-only edits of a period that
// already overlaps (legacy data from before the rule) stay possible; the
// advisory overlap listing keeps surfacing those.

// ensureChangeAllowed runs the rules a change must pass in order: the name
// stays unique, the linked care offerings accept the proposed row, the
// period exists, and a scheduling-relevant change keeps the same-type
// overlap invariant.
func (s *Service) ensureChangeAllowed(ctx context.Context, id int64, fields schoolcalendar.CalendarPeriodFields, stats *domain.OperationStats) error {
	if err := s.ensureUniqueName(ctx, fields.Name, id, stats); err != nil {
		return err
	}
	if err := s.guardCareOfferings(ctx, id, &fields); err != nil {
		return err
	}
	current, found, findStats, err := s.store.FindCalendarPeriod(ctx, id)
	stats.Add(findStats)
	if err != nil {
		return err
	}
	if !found {
		return schoolcalendar.ErrCalendarPeriodNotFound
	}
	if !schedulingFieldsChanged(current, fields) {
		return nil
	}
	return s.ensureNoActiveSameTypeOverlap(ctx, fields, id, stats)
}

// schedulingFieldsChanged reports whether the change touches the fields the
// overlap rule guards; a rename alone never does.
func schedulingFieldsChanged(current schoolcalendar.CalendarPeriod, fields schoolcalendar.CalendarPeriodFields) bool {
	return current.StartDate != fields.StartDate || current.EndDate != fields.EndDate ||
		current.IsActive != fields.IsActive || current.PeriodType != fields.PeriodType
}

// RemoveCalendarPeriod deletes the period once no care offering needs it:
// the guard sees the removal (a nil replacement) before the row goes.

// EnsureDefaultSchoolYear guarantees at least one period (WP-B1). The read
// and the insert run under the same recurrence gate as AddCalendarPeriod:
// without it, a bootstrap racing a normal add could list zero periods, then
// insert the default school year next to a just-committed overlapping
// active period of the same type. The insert itself is the race-free
// if-absent upsert, so two concurrent calls yield exactly one row and no
// error.

// underRecurrenceGate runs mutate inside the tenant transaction while holding
// the tenant-wide recurrence advisory lock, when one is configured.
func (s *Service) underRecurrenceGate(ctx context.Context, mutate func(context.Context) error) error {
	return s.administration.Transaction(ctx, func(txCtx context.Context) error {
		if s.administration.RecurrenceGate != nil {
			if err := s.administration.RecurrenceGate(txCtx); err != nil {
				return fmt.Errorf("lock recurrence: %w", err)
			}
		}
		return mutate(txCtx)
	})
}

func (s *Service) guardCareOfferings(ctx context.Context, id int64, replacement *schoolcalendar.CalendarPeriodFields) error {
	if s.administration.CareOfferingGuard == nil {
		return nil
	}
	if err := s.administration.CareOfferingGuard(ctx, id, replacement); err != nil {
		return fmt.Errorf("validate linked care offerings: %w", err)
	}
	return nil
}

// ensureUniqueName keeps the per-tenant name rule ahead of the insert so a
// collision answers with the domain sentinel instead of a constraint error.
func (s *Service) ensureUniqueName(ctx context.Context, name string, selfID int64, stats *domain.OperationStats) error {
	existing, listStats, err := s.store.ListCalendarPeriods(ctx, schoolcalendar.CalendarPeriodFilter{Name: name})
	stats.Add(listStats)
	if err != nil {
		return err
	}
	for _, period := range existing {
		if period.ID != selfID {
			return schoolcalendar.ErrCalendarPeriodNameConflict
		}
	}
	return nil
}

// ensureNoActiveSameTypeOverlap rejects a period whose resulting state would
// be active and overlap another active period of the same period_type
// (#1837). Inactive periods never conflict; cross-type overlaps (holidays
// inside a school year) stay legal and are only advisory.
func (s *Service) ensureNoActiveSameTypeOverlap(ctx context.Context, fields schoolcalendar.CalendarPeriodFields, selfID int64, stats *domain.OperationStats) error {
	if !fields.IsActive {
		return nil
	}
	overlaps, listStats, err := s.store.ListCalendarPeriods(ctx, schoolcalendar.CalendarPeriodFilter{
		ActiveOnly: true, PeriodType: fields.PeriodType,
		OverlappingFrom: fields.StartDate, OverlappingTo: fields.EndDate, ExcludeID: selfID,
	})
	stats.Add(listStats)
	if err != nil {
		return err
	}
	if len(overlaps) > 0 {
		return &schoolcalendar.CalendarPeriodOverlapError{Overlaps: overlaps}
	}
	return nil
}
