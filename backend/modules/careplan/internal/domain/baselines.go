package domain

import (
	"fmt"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	timezone "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

type ClassArrivalBaseline struct {
	SchoolClass  string
	ArrivalTimes map[int]time.Time
}

func (c *ClassArrivalBaseline) TimeForWeekday(day int) (time.Time, bool) {
	if c == nil {
		return time.Time{}, false
	}
	v, ok := c.ArrivalTimes[day]
	return v, ok
}
func canonicalDayToISOWeekday(day string) (int, bool) {
	for i, key := range []string{"mon", "tue", "wed", "thu", "fri", "sat", "sun"} {
		if day == key {
			return i + 1, true
		}
	}
	return 0, false
}
func isCareDay(
	careDays careplan.CareDayIndex,
	studentID int64,
	date timezone.Date,
	weekday int,
	row *careplan.ArrivalSchedule,
) bool {
	if careDays != nil {
		return careDays.Covers(studentID, date, weekday)
	}
	return row != nil
}

// classArrivalRow builds the row the class timetable would supply, if any.

func classArrivalRow(
	studentID int64,
	weekday int,
	classTimes *ClassArrivalBaseline,
) *careplan.ArrivalSchedule {
	if classTimes == nil {
		return nil
	}
	arrival, ok := classTimes.TimeForWeekday(weekday)
	if !ok {
		return nil
	}
	return &careplan.ArrivalSchedule{
		StudentID:       studentID,
		Weekday:         weekday,
		ExpectedArrival: arrival,
		Source:          careplan.ScheduleSourceClassSchedule,
		SourceClass:     classTimes.SchoolClass,
	}
}

// EffectiveArrivalRow picks the row that is actually in force: a per-child
// deviation wins, a row without its own time takes the class time, and a care
// day whose class carries no time keeps the day without an arrival time.

func EffectiveArrivalRow(
	studentID int64,
	weekday int,
	row *careplan.ArrivalSchedule,
	classRow *careplan.ArrivalSchedule,
) *careplan.ArrivalSchedule {
	if row == nil {
		// Booking mode only: the booking put the child in care that day and
		// no row was ever entered. The projected row has no ID because there
		// is nothing stored behind it.
		if classRow != nil {
			return classRow
		}
		return &careplan.ArrivalSchedule{
			StudentID: studentID,
			Weekday:   weekday,
		}
	}
	effective := *row
	if !row.ExpectedArrival.IsZero() {
		effective.Source = careplan.ScheduleSourceStaff
		return &effective
	}
	// Inheriting: keep the stored row's identity and notes, take the time and
	// the provenance label from the class. Callers that delete or edit the row
	// still address the real one.
	if classRow == nil {
		return &effective
	}
	effective.Source = careplan.ScheduleSourceClassSchedule
	effective.ExpectedArrival = classRow.ExpectedArrival
	effective.SourceClass = classRow.SourceClass
	return &effective
}

// loadStoredRows returns the care-day markers a person entered. Two
// independent facts meet in the projection above:
//
//	care day  — with the booking mode on, the approved booking links decide,
//	            so a stale row on an unbooked weekday is IGNORED rather than
//	            deleted (the same treatment ADR 0001 gives legacy pickup rows)
//	            and a newly booked weekday needs no manual row at all. With
//	            the mode off, the stored rows are the care days, exactly as
//	            before this change.
//	time      — the row's own value if it carries one, otherwise the class
//	            timetable. A care day whose class has no time yet stays a care
//	            day and simply has no arrival time.

func ProjectCareDayIndex(links []*careplan.ApprovedBooking, offerings map[int64]*careplan.CareOffering, from, to timezone.Date) careplan.CareDayIndex {
	index := make(careplan.CareDayIndex, len(links))
	for _, entry := range links {
		if entry == nil || entry.Link == nil {
			continue
		}
		offering := offerings[entry.Link.CareOfferingID]
		if !activeCareOffering(offering) {
			continue
		}
		weekdays := bookedCareWeekdays(entry.Link, offering)
		for date := from; !date.After(to); date = date.AddDays(1) {
			if offeringLinkCovers(entry.Link, date) {
				addBookedCareDays(index, entry.StudentID, date, weekdays)
			}
		}
	}
	return index
}
func bookedCareWeekdays(link *careplan.BookingSelection, offering *careplan.CareOffering) []int {
	var days []int
	for _, day := range OfferingPickupDays(link, offering) {
		weekday, ok := canonicalDayToISOWeekday(strings.ToLower(strings.TrimSpace(day)))
		if ok && weekday <= 5 {
			days = append(days, weekday)
		}
	}
	return days
}
func addBookedCareDays(index careplan.CareDayIndex, id int64, date timezone.Date, weekdays []int) {
	if len(weekdays) == 0 {
		return
	}
	if index[id] == nil {
		index[id] = make(map[timezone.Date]map[int]bool)
	}
	if index[id][date] == nil {
		index[id][date] = make(map[int]bool)
	}
	for _, weekday := range weekdays {
		index[id][date][weekday] = true
	}
}

func MergePickupPlans(
	projection *careplan.PickupBaselineProjection,
	studentIDs []int64,
	manual map[int64]map[int]*careplan.PickupSchedule,
	offering careplan.PickupPlansByStudent,
	from, to timezone.Date,
) {
	for _, studentID := range studentIDs {
		byDate := make(careplan.PickupPlanByDate)
		for date := from; !date.After(to); date = date.AddDays(1) {
			byWeekday := make(careplan.PickupWeek, len(manual[studentID])+len(offering[studentID][date]))
			for weekday, row := range offering[studentID][date] {
				byWeekday[weekday] = row
			}
			for weekday, row := range manual[studentID] {
				if !projection.AllowsPickupForWeekday(studentID, date, weekday) {
					continue
				}
				byWeekday[weekday] = row
			}
			byDate[date] = byWeekday
		}
		projection.WeeklyByStudentDate[studentID] = byDate
		projection.OfferingByStudentDate[studentID] = offering[studentID]
	}
}

func ProjectOfferingLinks(
	links []*careplan.ApprovedBooking,
	offeringByID map[int64]*careplan.CareOffering,
	from, to timezone.Date,
) (careplan.PickupPlansByStudent, error) {
	out := make(careplan.PickupPlansByStudent)
	for _, entry := range links {
		if entry == nil || entry.Link == nil {
			continue
		}
		offering := offeringByID[entry.Link.CareOfferingID]
		if offering == nil || !offering.IsActive || !offering.CountsAsCare || len(offering.PickupTimes) == 0 {
			continue
		}
		for date := from; !date.After(to); date = date.AddDays(1) {
			if !offeringLinkCovers(entry.Link, date) {
				continue
			}
			if err := projectOfferingWeek(out, entry.StudentID, date, entry.Link, offering); err != nil {
				return nil, err
			}
		}
	}
	return out, nil
}

func projectOfferingWeek(
	out careplan.PickupPlansByStudent,
	studentID int64,
	date timezone.Date,
	link *careplan.BookingSelection,
	offering *careplan.CareOffering,
) error {
	for _, day := range OfferingPickupDays(link, offering) {
		weekday, row, ok, err := ProjectedOfferingPickup(studentID, day, offering)
		if err != nil {
			return err
		}
		if !ok {
			continue
		}
		if out[studentID] == nil {
			out[studentID] = make(careplan.PickupPlanByDate)
		}
		if out[studentID][date] == nil {
			out[studentID][date] = make(careplan.PickupWeek)
		}
		current := out[studentID][date][weekday]
		if current != nil && !timezone.NormalizeWallClock(current.PickupTime).Before(row.PickupTime) {
			continue
		}
		out[studentID][date][weekday] = row
	}
	return nil
}

func OfferingPickupDays(
	link *careplan.BookingSelection,
	offering *careplan.CareOffering,
) []string {
	if len(link.SelectedDays) > 0 || offering.DaysOfWeekMode != "fixed" {
		return link.SelectedDays
	}
	return offering.AvailableDays
}

func ProjectedOfferingPickup(
	studentID int64,
	day string,
	offering *careplan.CareOffering,
) (int, *careplan.PickupSchedule, bool, error) {
	key := strings.ToLower(strings.TrimSpace(day))
	hhmm := offering.PickupTimes[key]
	weekday, ok := canonicalDayToISOWeekday(key)
	if !ok || weekday > 5 || hhmm == "" {
		return 0, nil, false, nil
	}
	parsed, err := time.Parse("15:04", hhmm)
	if err != nil {
		return 0, nil, false, fmt.Errorf("project pickup baselines: offering %d has invalid pickup time %q for %s: %w", offering.ID, hhmm, key, err)
	}
	offeringID := offering.ID
	return weekday, &careplan.PickupSchedule{
		StudentID: studentID, Weekday: weekday, PickupTime: timezone.NormalizeWallClock(parsed),
		Source: careplan.ScheduleSourceCareOffering, CareOfferingID: &offeringID,
		CareOfferingName: offering.Name,
	}, true, nil
}

func offeringLinkCovers(link *careplan.BookingSelection, date timezone.Date) bool {
	return link != nil &&
		(link.ValidFrom == nil || !date.Before(timezone.Date(*link.ValidFrom))) &&
		(link.ValidUntil == nil || date.Before(timezone.Date(*link.ValidUntil)))
}

func ProjectArrivalWeeks(id int64, stored careplan.ArrivalWeek, times *ClassArrivalBaseline, days careplan.CareDayIndex, from, to timezone.Date) (careplan.ArrivalPlanByDate, careplan.ArrivalPlanByDate) {
	weekly, derived := make(careplan.ArrivalPlanByDate), make(careplan.ArrivalPlanByDate)
	for date := from; !date.After(to); date = date.AddDays(1) {
		weekly[date], derived[date] = arrivalWeek(id, date, stored, times, days)
	}
	return weekly, derived
}
func arrivalWeek(id int64, date timezone.Date, stored careplan.ArrivalWeek, times *ClassArrivalBaseline, days careplan.CareDayIndex) (careplan.ArrivalWeek, careplan.ArrivalWeek) {
	weekly, derived := make(careplan.ArrivalWeek, 5), make(careplan.ArrivalWeek, 5)
	for weekday := 1; weekday <= 5; weekday++ {
		row := stored[weekday]
		if !isCareDay(days, id, date, weekday, row) {
			continue
		}
		classRow := classArrivalRow(id, weekday, times)
		if classRow != nil {
			derived[weekday] = classRow
		}
		if effective := EffectiveArrivalRow(id, weekday, row, classRow); effective != nil {
			weekly[weekday] = effective
		}
	}
	return weekly, derived
}
func HasOfferingPickupForWeekday(studentID int64, weekday int, links []*careplan.ApprovedBooking, offerings map[int64]*careplan.CareOffering) (bool, error) {
	for _, entry := range links {
		if entry == nil || entry.Link == nil {
			continue
		}
		offering := offerings[entry.Link.CareOfferingID]
		if !activeCareOffering(offering) {
			continue
		}
		found, err := offeringHasPickup(studentID, weekday, entry.Link, offering)
		if err != nil || found {
			return found, err
		}
	}
	return false, nil
}
func offeringHasPickup(studentID int64, weekday int, link *careplan.BookingSelection, offering *careplan.CareOffering) (bool, error) {
	for _, day := range OfferingPickupDays(link, offering) {
		projected, _, ok, err := ProjectedOfferingPickup(studentID, day, offering)
		if err != nil {
			return false, err
		}
		if ok && projected == weekday {
			return true, nil
		}
	}
	return false, nil
}
func activeCareOffering(offering *careplan.CareOffering) bool {
	return offering != nil && offering.IsActive && offering.CountsAsCare
}
