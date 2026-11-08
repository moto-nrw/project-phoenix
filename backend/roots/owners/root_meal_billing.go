package owners

import (
	"context"
	"errors"

	mealplanModule "github.com/moto-nrw/project-phoenix/modules/mealplan"
	mealplanCompose "github.com/moto-nrw/project-phoenix/modules/mealplan/compose"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	settingsModule "github.com/moto-nrw/project-phoenix/modules/settings"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// mealBillingAttendance is the attendance of Student Presence the
// Mensa-Monatsfreigabe counts (#4686).
type mealBillingAttendance interface {
	ListAttendance(context.Context, studentpresence.AttendanceFilter) ([]studentpresence.Attendance, error)
}

// BuildMealBillingMonths composes the Mensa-Monatsfreigabe over the kitchen
// lists of the Meal Plan, the school calendar and the attendance.
func BuildMealBillingMonths(mealPlan mealplanModule.Capability, calendarRecords schoolcalendar.Days, attendance mealBillingAttendance, settings settingsModule.StatisticsRetentionSource) (mealplanModule.BillingMonths, error) {
	if mealPlan == nil || calendarRecords == nil || attendance == nil || settings == nil {
		return nil, errors.New("meal billing months: the meal plan, the school calendar, the attendance and its retention are required")
	}
	return mealplanCompose.NewBillingMonths(mealPlan, mealPlanCalendar(calendarRecords), mealBillingPresence(attendance),
		mealBillingAvailableFrom(settingsModule.NewAttendanceRetention(settings).Days, calendar.TodayDate))
}

func mealBillingAvailableFrom(retentionDays func(context.Context) (int, error), today func() calendar.Date) mealplanModule.AttendanceAvailableFrom {
	return func(ctx context.Context) (calendar.Date, error) {
		days, err := retentionDays(ctx)
		if err != nil {
			return "", err
		}
		if days <= 0 {
			return "", studentpresence.ErrAttendanceRetentionWindow
		}
		return today().AddDays(-days), nil
	}
}

// mealBillingPresence names per day the children with an attendance row,
// a checkout to class included: the child was at school that day.
func mealBillingPresence(attendance mealBillingAttendance) mealplanModule.PresenceFinder {
	return func(ctx context.Context, from, to calendar.Date) (map[calendar.Date]map[int64]bool, error) {
		rows, err := attendance.ListAttendance(ctx, studentpresence.AttendanceFilter{FromDate: from.String(), UntilDate: to.String()})
		if err != nil {
			return nil, err
		}
		present := map[calendar.Date]map[int64]bool{}
		for _, row := range rows {
			day, err := calendar.ParseDate(row.Date)
			if err != nil {
				return nil, err
			}
			if present[day] == nil {
				present[day] = map[int64]bool{}
			}
			present[day][row.StudentID] = true
		}
		return present, nil
	}
}
