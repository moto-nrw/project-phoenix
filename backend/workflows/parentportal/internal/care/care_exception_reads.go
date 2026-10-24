package care

import (
	"context"
	"fmt"
	"sort"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

const (
	exceptionSourceGuardian = "guardian"
	exceptionSourceStaff    = "staff"
)

// loadCareException merges the pickup and arrival exceptions for one date into
// the parent-facing projection. Returns nil if neither leg has a row.
func (s *Service) loadCareException(ctx context.Context, studentID int64, date calendar.Date) (*CareException, error) {
	pickup, err := s.pickupExceptionForDate(ctx, studentID, calendar.Date(date))
	if err != nil {
		return nil, err
	}
	arrival, err := s.arrivalExceptionForDate(ctx, studentID, calendar.Date(date))
	if err != nil {
		return nil, err
	}
	if pickup == nil && arrival == nil {
		return nil, nil
	}
	out := &CareException{Date: date, Source: exceptionSourceGuardian}
	// Any staff-authored leg makes the day staff-owned for display purposes —
	// the portal then shows it read-only rather than as a parent change.
	pickupStaffOwned := applyDatePickupLeg(out, pickup)
	arrivalStaffOwned := applyDateArrivalLeg(out, arrival)
	if pickupStaffOwned || arrivalStaffOwned {
		out.Source = exceptionSourceStaff
	}
	return out, nil
}

// applyDatePickupLeg copies the pickup leg of one date into out and reports
// whether the leg is staff-authored. A nil leg changes nothing.
func applyDatePickupLeg(out *CareException, pickup *PickupException) bool {
	if pickup == nil {
		return false
	}
	out.PickupTime = pickup.PickupTime
	out.PickupSource = pickup.Source
	if pickup.Source == exceptionSourceGuardian {
		out.Reason = pickup.Reason
	}
	out.UpdatedAt = pickup.UpdatedAt
	return pickup.Source == exceptionSourceStaff
}

// applyDateArrivalLeg copies the arrival leg of one date into out and reports
// whether the leg is staff-authored. A nil leg changes nothing.
func applyDateArrivalLeg(out *CareException, arrival *ArrivalException) bool {
	if arrival == nil {
		return false
	}
	out.ArrivalTime = arrival.ExpectedArrival
	if out.Reason == nil && arrival.Source == exceptionSourceGuardian {
		out.Reason = arrival.Reason
	}
	if arrival.UpdatedAt.After(out.UpdatedAt) {
		out.UpdatedAt = arrival.UpdatedAt
	}
	return arrival.Source == exceptionSourceStaff
}

// ListCareExceptions returns the merged pickup/arrival exceptions for the child
// in [from, to], staff- and guardian-authored alike. Unlike SubmitCareException
// this is not gated by parent_pickup_change_enabled: a parent may always see and
// (via DeleteCareException) clear overrides they created, even after the school
// switches the feature off, so existing entries never become stuck.
func (s *Service) ListCareExceptions(ctx context.Context, accountID, studentID int64, from, to calendar.Date) ([]*CareException, error) {
	child, err := s.ResolveOwnedChild(ctx, accountID, studentID)
	if err != nil {
		return nil, err
	}

	var out []*CareException
	txErr := s.Runtime.WithinTenant(ctx, child.TenantID, func(txCtx context.Context) error {
		pickups, err := s.CareExceptions.ListPickupExceptions(txCtx, StudentScheduleFilter{StudentIDs: []int64{studentID}, From: calendar.Date(from), To: calendar.Date(to)})
		if err != nil {
			return err
		}
		arrivals, err := s.CareExceptions.ListArrivalExceptions(txCtx, StudentScheduleFilter{StudentIDs: []int64{studentID}, From: calendar.Date(from), To: calendar.Date(to)})
		if err != nil {
			return err
		}
		departures, err := s.CareExceptions.ListDepartureExceptions(txCtx, StudentScheduleFilter{StudentIDs: []int64{studentID}, From: calendar.Date(from), To: calendar.Date(to)})
		if err != nil {
			return err
		}
		out = mergeCareExceptions(pickups, arrivals, accountID)
		out = mergeDepartureExceptions(out, departures)
		return nil
	})
	if txErr != nil {
		return nil, fmt.Errorf("parent: list care exceptions: %w", txErr)
	}
	return out, nil
}

// mergeCareExceptions joins pickup and arrival exception rows by date into the
// parent-facing projection, sorted ascending by date.
func mergeCareExceptions(pickups []PickupException, arrivals []ArrivalException, accountID int64) []*CareException {
	byDate := make(map[calendar.Date]*CareException)
	order := make([]calendar.Date, 0, len(pickups)+len(arrivals))
	get := func(date calendar.Date) *CareException {
		if ce, ok := byDate[date]; ok {
			return ce
		}
		ce := &CareException{Date: date, Source: exceptionSourceGuardian}
		byDate[date] = ce
		order = append(order, date)
		return ce
	}
	for _, p := range pickups {
		applyPickupException(get(calendar.Date(p.ExceptionDate)), p, accountID)
	}
	for _, a := range arrivals {
		applyArrivalException(get(calendar.Date(a.ExceptionDate)), a, accountID)
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
func applyPickupException(ce *CareException, p PickupException, accountID int64) {
	ce.PickupTime = p.PickupTime
	ce.PickupSource = p.Source
	if p.Source == exceptionSourceGuardian && guardianAuthoredBy(p.CreatedByGuardian, accountID) {
		ce.Reason = p.Reason
	}
	// A pickup row with no time is an absence marker, not "no override". Carry
	// that distinction to the parent UI so a staff-set "not coming today" row
	// resolves to an absence rather than falling through to the base plan.
	ce.PickupAbsent = p.PickupTime == nil
	if p.UpdatedAt.After(ce.UpdatedAt) {
		ce.UpdatedAt = p.UpdatedAt
	}
	if p.Source == exceptionSourceStaff || p.HasManualPartialAbsence() {
		ce.Source = exceptionSourceStaff
	}
}

// applyArrivalException folds one arrival exception row into its date's
// parent-facing projection.
func applyArrivalException(ce *CareException, a ArrivalException, accountID int64) {
	ce.ArrivalTime = a.ExpectedArrival
	if ce.Reason == nil && a.Source == exceptionSourceGuardian && guardianAuthoredBy(a.CreatedByGuardian, accountID) {
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
	if a.Source == exceptionSourceStaff {
		ce.Source = exceptionSourceStaff
	}
}

// mergeDepartureExceptions adds the day Abholart (#3945) to the merged days.
// A day with only an Abholart becomes its own entry; its Source is staff,
// because only the OGS writes the value.
func mergeDepartureExceptions(days []*CareException, departures []DepartureException) []*CareException {
	if len(departures) == 0 {
		return days
	}
	byDate := make(map[calendar.Date]*CareException, len(days))
	for _, day := range days {
		byDate[day.Date] = day
	}
	for _, departure := range departures {
		date := calendar.Date(departure.ExceptionDate)
		day, ok := byDate[date]
		if !ok {
			day = &CareException{Date: date, Source: exceptionSourceStaff}
			byDate[date] = day
			days = append(days, day)
		}
		mode := departure.DepartureMode
		day.DepartureMode = &mode
		if departure.UpdatedAt.After(day.UpdatedAt) {
			day.UpdatedAt = departure.UpdatedAt
		}
	}
	sort.Slice(days, func(i, j int) bool { return days[i].Date.Before(days[j].Date) })
	return days
}

func guardianAuthoredBy(author *int64, accountID int64) bool {
	return author != nil && *author == accountID
}
