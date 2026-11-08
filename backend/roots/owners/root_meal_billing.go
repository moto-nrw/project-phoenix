package owners

import (
	"context"
	"errors"

	mealplanModule "github.com/moto-nrw/project-phoenix/modules/mealplan"
	mealplanCompose "github.com/moto-nrw/project-phoenix/modules/mealplan/compose"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	settingsModule "github.com/moto-nrw/project-phoenix/modules/settings"
	configModel "github.com/moto-nrw/project-phoenix/modules/settings/records"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
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
		return mealAttendanceCutoff(days, today)
	}
}

// mealAttendanceCutoff is the earliest day a retention of days still keeps;
// the school side and the Träger side of the Mensa billing share it.
func mealAttendanceCutoff(days int, today func() calendar.Date) (calendar.Date, error) {
	if days <= 0 {
		return "", studentpresence.ErrAttendanceRetentionWindow
	}
	return today().AddDays(-days), nil
}

// MealAttendanceSchoolSettings reads settings of several schools at once.
type MealAttendanceSchoolSettings interface {
	ResolveManyForTenants(ctx context.Context, tenantIDs []int64, keys []string) (map[int64]*configModel.SettingsSnapshot, error)
}

// MealAttendanceFrom names per school the earliest day whose attendance the
// school still keeps, by the rule of the Mensa-Monatsfreigabe (#4686): the
// monthly run shows a month before it as no longer billable. The caller
// runs in its Träger transaction, whose row security hides the settings of
// the schools, so the read runs in an administrative transaction of its
// own; it shares nothing with the run (ADR 0105).
func MealAttendanceFrom(settings MealAttendanceSchoolSettings, today func() calendar.Date) func(context.Context, []int64) (map[int64]calendar.Date, error) {
	return func(ctx context.Context, schoolIDs []int64) (map[int64]calendar.Date, error) {
		result := make(map[int64]calendar.Date, len(schoolIDs))
		if len(schoolIDs) == 0 {
			return result, nil
		}
		snapshots, err := settings.ResolveManyForTenants(tenant.ContextWithoutTransaction(ctx), schoolIDs,
			[]string{configModel.KeyGDPRAttendanceRetentionDays})
		if err != nil {
			return nil, err
		}
		for _, schoolID := range schoolIDs {
			snapshot, ok := snapshots[schoolID]
			if !ok {
				return nil, errors.New("meal billing: no settings for a school of the Träger")
			}
			days, err := snapshot.Int(configModel.KeyGDPRAttendanceRetentionDays)
			if err != nil {
				return nil, err
			}
			if result[schoolID], err = mealAttendanceCutoff(days, today); err != nil {
				return nil, err
			}
		}
		return result, nil
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
