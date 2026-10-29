package application

import (
	"context"
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The roster impact of an adjustment (#5102). The resync rewrites only the
// rows a booking created; the office learns which Regeltermine followed and
// which staff maintain by hand and still keep the child on a weekday the
// change dropped a booking on, where no remaining booking carries them
// (#5342).

// adjustedRosterRows lists the roster rows this request child materialized
// for its student.
func (m *BookingMaterialization) adjustedRosterRows(ctx context.Context, work *offeringAdjustmentWork) ([]ports.RosterEnrollment, error) {
	rows, err := m.deps.Rosters.StudentEnrollments(ctx, *work.child.CreatedStudentID)
	if err != nil {
		return nil, fmt.Errorf("decision: list enrollments for roster impact: %w", err)
	}
	tagged := make([]ports.RosterEnrollment, 0, len(rows))
	for _, row := range rows {
		if taggedWithRequestChild(row, work.child.ID) {
			tagged = append(tagged, row)
		}
	}
	return tagged, nil
}

// offeringAdjustmentImpact compares the request child's rows before and
// after the adjustment and asks Timetable where the child stays planned by
// hand on the weekdays the change dropped a booking on, without a remaining
// booking that carries the Regeltermin.
func (m *BookingMaterialization) offeringAdjustmentImpact(
	ctx context.Context, work *offeringAdjustmentWork, before []ports.RosterEnrollment,
) (careplan.RosterImpact, error) {
	from := m.enrollmentRewriteBoundary(work.effectiveFrom)
	impact := careplan.RosterImpact{EffectiveFrom: from, Adjusted: []careplan.AdjustedRosterTemplate{}, NotAdjusted: []careplan.ManualRosterTemplate{}}
	after, err := m.adjustedRosterRows(ctx, work)
	if err != nil {
		return impact, err
	}
	if impact.Adjusted, err = m.adjustedRosterTemplates(ctx, before, after, from); err != nil {
		return impact, err
	}
	change := bookingDayChange(work, from)
	if len(change.DroppedWeekdays) == 0 || m.deps.ManualPlans == nil {
		return impact, nil
	}
	manual, err := m.deps.ManualPlans.ManualStudentPlan(ctx, *work.child.CreatedStudentID, from, change)
	if err != nil {
		return impact, fmt.Errorf("decision: list hand-maintained templates: %w", err)
	}
	impact.NotAdjusted = append(impact.NotAdjusted, manual...)
	return impact, nil
}

// bookingDayChange summarizes what the change did to the child's weekdays.
func bookingDayChange(work *offeringAdjustmentWork, from calendar.Date) careplan.BookingDayChange {
	dropped, netLoss := changedBookingWeekdays(work)
	return careplan.BookingDayChange{
		DroppedWeekdays: dropped, NetLossWeekdays: netLoss,
		Remaining: remainingDayBookings(work.replacement, work.offeringByID, from),
	}
}

// changedBookingWeekdays names the ISO weekdays on which the change drops an
// offering the child booked before, and those on which it drops more
// offerings than it newly books. Whether a hand-maintained Regeltermin on a
// dropped day lost its booking is Timetable's decision with the remaining
// bookings and their pickup times (#5342). The net loss decides only where
// a pickup time or a Regeltermin end is missing: a switch between offerings
// then offsets the dropped one, as before.
func changedBookingWeekdays(work *offeringAdjustmentWork) (dropped, netLoss []int) {
	before := bookedWeekdaysByOffering(work.beforeLinks, work.offeringByID)
	after := bookedWeekdaysByOffering(work.replacement, work.offeringByID)
	balance := make(map[int]int)
	droppedDays := make(map[int]bool)
	for offeringID, days := range before {
		for weekday := range days {
			if !after[offeringID][weekday] {
				balance[weekday]++
				droppedDays[weekday] = true
			}
		}
	}
	for offeringID, days := range after {
		for weekday := range days {
			if !before[offeringID][weekday] {
				balance[weekday]--
			}
		}
	}
	lostDays := make(map[int]bool)
	for weekday, count := range balance {
		if count > 0 {
			lostDays[weekday] = true
		}
	}
	return sortedISOWeekdays(droppedDays), sortedISOWeekdays(lostDays)
}

// remainingDayBookings lists, per weekday, the offerings the child books
// after the change from the date on, with the offering's kind flags and
// pickup time that day. A link whose days cannot be resolved names no weekday, as in
// bookedWeekdaysByOffering.
func remainingDayBookings(links []*careplan.BookedOffering, offerings map[int64]*careplan.CareOffering, from calendar.Date) []careplan.RemainingDayBooking {
	result := make([]careplan.RemainingDayBooking, 0, len(links))
	for _, link := range links {
		offering := offerings[link.CareOfferingID]
		if offering == nil || (link.ValidUntil != nil && !link.ValidUntil.After(from)) {
			continue
		}
		days, err := effectiveOfferingDaysForEnrollment(offering, link)
		if err != nil {
			continue
		}
		for _, day := range days {
			weekday, ok := offeringDayWeekday(day)
			if !ok {
				continue
			}
			result = append(result, careplan.RemainingDayBooking{
				Weekday: weekday, CountsAsCare: offering.CountsAsCare, IncludesLunch: offering.IncludesLunch,
				PickupTime: offering.PickupTimes[strings.ToLower(strings.TrimSpace(day))],
			})
		}
	}
	return result
}

// bookedWeekdaysByOffering resolves each link's booked weekdays. The impact is
// a report: a link whose days cannot be resolved (a legacy parent-choice row
// without days) names no weekday instead of failing the booking change.
func bookedWeekdaysByOffering(links []*careplan.BookedOffering, offerings map[int64]*careplan.CareOffering) map[int64]map[int]bool {
	result := make(map[int64]map[int]bool, len(links))
	for _, link := range links {
		offering := offerings[link.CareOfferingID]
		if offering == nil {
			continue
		}
		days, err := effectiveOfferingDaysForEnrollment(offering, link)
		if err != nil {
			continue
		}
		if result[link.CareOfferingID] == nil {
			result[link.CareOfferingID] = make(map[int]bool)
		}
		for _, day := range days {
			if weekday, ok := offeringDayWeekday(day); ok {
				result[link.CareOfferingID][weekday] = true
			}
		}
	}
	return result
}

// adjustedRosterTemplates names the series whose booking-created weekdays
// from the date on differ before and after the change.
func (m *BookingMaterialization) adjustedRosterTemplates(
	ctx context.Context, before, after []ports.RosterEnrollment, from calendar.Date,
) ([]careplan.AdjustedRosterTemplate, error) {
	ids := rosterTemplateIDs(before, after)
	if len(ids) == 0 {
		return []careplan.AdjustedRosterTemplate{}, nil
	}
	scheduleDays, err := m.templateScheduleWeekdays(ctx, ids)
	if err != nil {
		return nil, err
	}
	groups, err := m.deps.Templates.Groups(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("decision: list templates for roster impact: %w", err)
	}
	series := newRosterSeries(groups)
	beforeDays := series.weekdays(before, from, scheduleDays)
	afterDays := series.weekdays(after, from, scheduleDays)
	result := make([]careplan.AdjustedRosterTemplate, 0, len(series.latest))
	for root, latest := range series.latest {
		removed := weekdayDifference(beforeDays[root], afterDays[root])
		added := weekdayDifference(afterDays[root], beforeDays[root])
		if len(removed) == 0 && len(added) == 0 {
			continue
		}
		result = append(result, careplan.AdjustedRosterTemplate{
			TemplateID: latest.ID, Name: latest.Name, RemovedWeekdays: removed, AddedWeekdays: added,
		})
	}
	sort.Slice(result, func(i, j int) bool {
		if left, right := strings.ToLower(result[i].Name), strings.ToLower(result[j].Name); left != right {
			return left < right
		}
		return result[i].TemplateID < result[j].TemplateID
	})
	return result, nil
}

func rosterTemplateIDs(rowSets ...[]ports.RosterEnrollment) []int64 {
	ids := make([]int64, 0)
	for _, rows := range rowSets {
		for _, row := range rows {
			ids = append(ids, row.ActivityGroupID)
		}
	}
	slices.Sort(ids)
	return slices.Compact(ids)
}

func (m *BookingMaterialization) templateScheduleWeekdays(ctx context.Context, ids []int64) (map[int64]map[int]bool, error) {
	schedules, err := m.catalog.deps.Timetable.GroupSchedules(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("decision: list template schedules for roster impact: %w", err)
	}
	result := make(map[int64]map[int]bool, len(ids))
	for _, sch := range schedules {
		if result[sch.GroupID] == nil {
			result[sch.GroupID] = make(map[int]bool)
		}
		result[sch.GroupID][sch.Weekday] = true
	}
	return result, nil
}

// rosterSeries maps every template to its split-series root and remembers
// the latest segment per series.
type rosterSeries struct {
	rootOf map[int64]int64
	latest map[int64]ports.SourcedTemplate
}

func newRosterSeries(groups []ports.SourcedTemplate) rosterSeries {
	series := rosterSeries{rootOf: make(map[int64]int64, len(groups)), latest: make(map[int64]ports.SourcedTemplate, len(groups))}
	for _, group := range groups {
		root := group.ID
		if group.SeriesRootID != nil {
			root = *group.SeriesRootID
		}
		series.rootOf[group.ID] = root
		if current, ok := series.latest[root]; !ok || group.ID > current.ID {
			series.latest[root] = group
		}
	}
	return series
}

// weekdays collects, per series, the weekdays the rows plan from the date on.
func (s rosterSeries) weekdays(rows []ports.RosterEnrollment, from calendar.Date, scheduleDays map[int64]map[int]bool) map[int64]map[int]bool {
	result := make(map[int64]map[int]bool)
	for _, row := range rows {
		root, known := s.rootOf[row.ActivityGroupID]
		if !known || (row.ValidUntil != nil && !row.ValidUntil.After(from)) {
			continue
		}
		if result[root] == nil {
			result[root] = make(map[int]bool)
		}
		for weekday := range rosterRowWeekdays(row, scheduleDays[row.ActivityGroupID]) {
			result[root][weekday] = true
		}
	}
	return result
}

// rosterRowWeekdays is the weekday set a row plans: its single weekday
// scope, its selected weekdays, or every weekday the template runs on.
func rosterRowWeekdays(row ports.RosterEnrollment, scheduleDays map[int]bool) map[int]bool {
	if row.Weekday != nil {
		return map[int]bool{*row.Weekday: true}
	}
	if len(row.SelectedWeekdays) == 0 {
		return scheduleDays
	}
	result := make(map[int]bool, len(row.SelectedWeekdays))
	for _, weekday := range row.SelectedWeekdays {
		result[weekday] = true
	}
	return result
}

func weekdayDifference(left, right map[int]bool) []int {
	result := make(map[int]bool)
	for weekday := range left {
		if !right[weekday] {
			result[weekday] = true
		}
	}
	return sortedISOWeekdays(result)
}

func sortedISOWeekdays(set map[int]bool) []int {
	result := make([]int, 0, len(set))
	for weekday := range set {
		result = append(result, weekday)
	}
	slices.Sort(result)
	return result
}
