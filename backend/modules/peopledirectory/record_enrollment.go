package peopledirectory

import (
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/departure"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

func AccompaniedWeekdays(allowed AllowedDepartureModes, days DepartureDays) map[string]bool {
	return departure.Plan{AllowedDepartureModes: allowed, DepartureDays: days}.AccompaniedDays()
}
func WithAccompaniedDays(allowed AllowedDepartureModes, days []string) AllowedDepartureModes {
	return departure.WithAccompaniedDays(allowed, days)
}

var CompanionWeekdayKeys = map[int]string{1: PickupDayMonday, 2: PickupDayTuesday, 3: PickupDayWednesday, 4: PickupDayThursday, 5: PickupDayFriday}

func EnrolledOn(student *StudentState, date, today calendar.Date) bool {
	if student == nil {
		return false
	}
	if student.EnrolledFrom != nil && date.Before(*student.EnrolledFrom) {

		if student.Status != StudentStatusActive || date.Before(today) {
			return false
		}
	}
	if student.EnrolledUntil != nil && date.After(*student.EnrolledUntil) {
		return false
	}
	if student.EnrolledFrom == nil && student.EnrolledUntil == nil {
		return student.Status != StudentStatusInactive
	}
	return true
}
