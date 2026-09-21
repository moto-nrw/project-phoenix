package care

import (
	"context"
	"fmt"
	"sort"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
)

// loadCareException merges the pickup and arrival exceptions for one date into
// the parent-facing projection. Returns nil if neither leg has a row.
func (s *Service) loadCareException(ctx context.Context, studentID int64, date timezone.Date) (*CareException, error) {
	pickup, err := s.pickupExceptionForDate(ctx, studentID, careplan.Date(date))
	if err != nil {
		return nil, err
	}
	arrival, err := s.arrivalExceptionForDate(ctx, studentID, careplan.Date(date))
	if err != nil {
		return nil, err
	}
	if pickup == nil && arrival == nil {
		return nil, nil
	}
	out := &CareException{Date: date, Source: scheduleModels.ExceptionSourceGuardian}
	// Any staff-authored leg makes the day staff-owned for display purposes —
	// the portal then shows it read-only rather than as a parent change.
	pickupStaffOwned := applyDatePickupLeg(out, pickup)
	arrivalStaffOwned := applyDateArrivalLeg(out, arrival)
	if pickupStaffOwned || arrivalStaffOwned {
		out.Source = scheduleModels.ExceptionSourceStaff
	}
	return out, nil
}

// applyDatePickupLeg copies the pickup leg of one date into out and reports
// whether the leg is staff-authored. A nil leg changes nothing.
func applyDatePickupLeg(out *CareException, pickup *careplan.PickupException) bool {
	if pickup == nil {
		return false
	}
	out.PickupTime = pickup.PickupTime
	out.PickupSource = pickup.Source
	if pickup.Source == scheduleModels.ExceptionSourceGuardian {
		out.Reason = pickup.Reason
	}
	out.UpdatedAt = pickup.UpdatedAt
	return pickup.Source == scheduleModels.ExceptionSourceStaff
}

// applyDateArrivalLeg copies the arrival leg of one date into out and reports
// whether the leg is staff-authored. A nil leg changes nothing.
func applyDateArrivalLeg(out *CareException, arrival *careplan.ArrivalException) bool {
	if arrival == nil {
		return false
	}
	out.ArrivalTime = arrival.ExpectedArrival
	if out.Reason == nil && arrival.Source == scheduleModels.ExceptionSourceGuardian {
		out.Reason = arrival.Reason
	}
	if arrival.UpdatedAt.After(out.UpdatedAt) {
		out.UpdatedAt = arrival.UpdatedAt
	}
	return arrival.Source == scheduleModels.ExceptionSourceStaff
}

// ListCareExceptions returns the merged pickup/arrival exceptions for the child
// in [from, to], staff- and guardian-authored alike. Unlike SubmitCareException
// this is not gated by parent_pickup_change_enabled: a parent may always see and
// (via DeleteCareException) clear overrides they created, even after the school
// switches the feature off, so existing entries never become stuck.
func (s *Service) ListCareExceptions(ctx context.Context, accountID, studentID int64, from, to timezone.Date) ([]*CareException, error) {
	child, err := s.ResolveOwnedChild(ctx, accountID, studentID)
	if err != nil {
		return nil, err
	}

	var out []*CareException
	txErr := InTenant(ctx, child.TenantID, func(txCtx context.Context) error {
		pickups, err := s.CareExceptions.ListPickupExceptions(txCtx, careplan.StudentScheduleFilter{StudentIDs: []int64{studentID}, From: careplan.Date(from), To: careplan.Date(to)})
		if err != nil {
			return err
		}
		arrivals, err := s.CareExceptions.ListArrivalExceptions(txCtx, careplan.StudentScheduleFilter{StudentIDs: []int64{studentID}, From: careplan.Date(from), To: careplan.Date(to)})
		if err != nil {
			return err
		}
		out = mergeCareExceptions(pickups, arrivals, accountID)
		return nil
	})
	if txErr != nil {
		return nil, fmt.Errorf("parent: list care exceptions: %w", txErr)
	}
	return out, nil
}

// mergeCareExceptions joins pickup and arrival exception rows by date into the
// parent-facing projection, sorted ascending by date.
func mergeCareExceptions(pickups []careplan.PickupException, arrivals []careplan.ArrivalException, accountID int64) []*CareException {
	byDate := make(map[timezone.Date]*CareException)
	order := make([]timezone.Date, 0, len(pickups)+len(arrivals))
	get := func(date timezone.Date) *CareException {
		if ce, ok := byDate[date]; ok {
			return ce
		}
		ce := &CareException{Date: date, Source: scheduleModels.ExceptionSourceGuardian}
		byDate[date] = ce
		order = append(order, date)
		return ce
	}
	for _, p := range pickups {
		applyPickupException(get(timezone.Date(p.ExceptionDate)), p, accountID)
	}
	for _, a := range arrivals {
		applyArrivalException(get(timezone.Date(a.ExceptionDate)), a, accountID)
	}
	sort.Slice(order, func(i, j int) bool { return order[i].Before(order[j]) })
	out := make([]*CareException, 0, len(order))
	for _, d := range order {
		out = append(out, byDate[d])
	}
	return out
}

// applyPickupException folds one pickup exception row into its date's
// parent-facing projection.
func applyPickupException(ce *CareException, p careplan.PickupException, accountID int64) {
	ce.PickupTime = p.PickupTime
	ce.PickupSource = p.Source
	if p.Source == scheduleModels.ExceptionSourceGuardian && guardianAuthoredBy(p.CreatedByGuardian, accountID) {
		ce.Reason = p.Reason
	}
	// A pickup row with no time is an absence marker, not "no override". Carry
	// that distinction to the parent UI so a staff-set "not coming today" row
	// resolves to an absence rather than falling through to the base plan.
	ce.PickupAbsent = p.PickupTime == nil
	if p.UpdatedAt.After(ce.UpdatedAt) {
		ce.UpdatedAt = p.UpdatedAt
	}
	if p.Source == scheduleModels.ExceptionSourceStaff || p.HasManualPartialAbsence() {
		ce.Source = scheduleModels.ExceptionSourceStaff
	}
}

// applyArrivalException folds one arrival exception row into its date's
// parent-facing projection.
func applyArrivalException(ce *CareException, a careplan.ArrivalException, accountID int64) {
	ce.ArrivalTime = a.ExpectedArrival
	if ce.Reason == nil && a.Source == scheduleModels.ExceptionSourceGuardian && guardianAuthoredBy(a.CreatedByGuardian, accountID) {
		ce.Reason = a.Reason
	}
	// An arrival row with no expected time is a "not coming today" absence
	// marker (StudentArrivalException.IsAbsent), the arrival-leg twin of a
	// timeless pickup row. It creates no status day either, so carry the
	// distinction to the parent UI: an arrival-only absence must resolve to
	// an absence, not fall through to a regular pickup time (#1725 review).
	ce.ArrivalAbsent = a.ExpectedArrival == nil
	if a.UpdatedAt.After(ce.UpdatedAt) {
		ce.UpdatedAt = a.UpdatedAt
	}
	if a.Source == scheduleModels.ExceptionSourceStaff {
		ce.Source = scheduleModels.ExceptionSourceStaff
	}
}

func guardianAuthoredBy(author *int64, accountID int64) bool {
	return author != nil && *author == accountID
}
