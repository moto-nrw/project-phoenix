package owners

import (
	"context"
	"errors"

	mealplanModule "github.com/moto-ogs/project-phoenix/modules/mealplan"
	mealplanCompose "github.com/moto-ogs/project-phoenix/modules/mealplan/compose"
	"github.com/moto-ogs/project-phoenix/modules/schoolcalendar"
	settingsModule "github.com/moto-ogs/project-phoenix/modules/settings"
	configModel "github.com/moto-ogs/project-phoenix/modules/settings/records"
	"github.com/moto-ogs/project-phoenix/modules/studentpresence"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
	"github.com/moto-ogs/project-phoenix/tenant"
)

// mealBillingAttendance is the attendance of Student Presence the
// Mensa-Monatsfreigabe counts (#4686).
type mealBillingAttendance interface {
	ListAttendance(context.Context, studentpresence.AttendanceFilter) ([]studentpresence.Attendance, error)
	MealAttendanceMarks
}

// MealAttendanceMarks is Student Presence's first complete attendance day
// of a school under a retention window, in the caller's school transaction
// (#4686): it counts what the retention already deleted.
type MealAttendanceMarks interface {
	AttendanceCompleteFrom(context.Context, int) (calendar.Date, error)
}

// BuildMealBillingMonths composes the Mensa-Monatsfreigabe over the kitchen
// lists of the Meal Plan, the school calendar and the attendance.
func BuildMealBillingMonths(mealPlan mealplanModule.Capability, calendarRecords schoolcalendar.Days, attendance mealBillingAttendance, settings settingsModule.StatisticsRetentionSource) (mealplanModule.BillingMonths, error) {
	if mealPlan == nil || calendarRecords == nil || attendance == nil || settings == nil {
		return nil, errors.New("meal billing months: the meal plan, the school calendar, the attendance and its retention are required")
	}
	return mealplanCompose.NewBillingMonths(mealPlan, mealPlanCalendar(calendarRecords), mealBillingPresence(attendance),
		mealBillingAvailableFrom(settingsModule.NewAttendanceRetention(settings).Days, attendance.AttendanceCompleteFrom))
}

// mealBillingAvailableFrom is the first day whose attendance is complete
// under the school's retention window: Student Presence answers it with
// what its retention already deleted (#4686).
func mealBillingAvailableFrom(retentionDays func(context.Context) (int, error),
	completeFrom func(context.Context, int) (calendar.Date, error),
) mealplanModule.AttendanceAvailableFrom {
	return func(ctx context.Context) (calendar.Date, error) {
		days, err := retentionDays(ctx)
		if err != nil {
			return "", err
		}
		return completeFrom(ctx, days)
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

// MealAttendanceSchoolSettings reads settings of several schools at once.
type MealAttendanceSchoolSettings interface {
	ResolveManyForTenants(ctx context.Context, tenantIDs []int64, keys []string) (map[int64]*configModel.SettingsSnapshot, error)
}

// MealAttendanceFrom names per school the earliest day whose attendance the
// school still keeps, by the rule of the Mensa-Monatsfreigabe (#4686): the
// monthly run shows a month before it as no longer billable. The caller
// runs in its Träger transaction, whose row security hides the settings of
// the schools, so the settings are read in an administrative transaction of
// its own and each school's attendance in a school transaction of its own;
// neither shares anything with the run (ADR 0105).
func MealAttendanceFrom(settings MealAttendanceSchoolSettings, completeFrom func(context.Context, int64, int) (calendar.Date, error),
) func(context.Context, []int64) (map[int64]calendar.Date, error) {
	return func(ctx context.Context, schoolIDs []int64) (map[int64]calendar.Date, error) {
		result := make(map[int64]calendar.Date, len(schoolIDs))
		if len(schoolIDs) == 0 {
			return result, nil
		}
		days, err := schoolRetentionDays(ctx, settings, schoolIDs)
		if err != nil {
			return nil, err
		}
		for _, schoolID := range schoolIDs {
			if result[schoolID], err = completeFrom(ctx, schoolID, days[schoolID]); err != nil {
				return nil, err
			}
		}
		return result, nil
	}
}

// schoolRetentionDays reads the attendance retention of each school.
func schoolRetentionDays(ctx context.Context, settings MealAttendanceSchoolSettings, schoolIDs []int64) (map[int64]int, error) {
	snapshots, err := settings.ResolveManyForTenants(tenant.ContextWithoutTransaction(ctx), schoolIDs,
		[]string{configModel.KeyGDPRAttendanceRetentionDays})
	if err != nil {
		return nil, err
	}
	days := make(map[int64]int, len(schoolIDs))
	for _, schoolID := range schoolIDs {
		snapshot, ok := snapshots[schoolID]
		if !ok {
			return nil, errors.New("meal billing: no settings for a school of the Träger")
		}
		if days[schoolID], err = snapshot.Int(configModel.KeyGDPRAttendanceRetentionDays); err != nil {
			return nil, err
		}
	}
	return days, nil
}

// SchoolAttendanceMarks asks Student Presence for a school's first complete
// attendance day in a school transaction of its own.
func SchoolAttendanceMarks(marks MealAttendanceMarks) func(context.Context, int64, int) (calendar.Date, error) {
	return func(ctx context.Context, schoolID int64, days int) (calendar.Date, error) {
		id, err := tenant.NewTenantID(schoolID)
		if err != nil {
			return "", err
		}
		var complete calendar.Date
		err = tenant.WithinTenant(tenant.ContextWithoutTransaction(ctx), id, func(schoolCtx context.Context) error {
			complete, err = marks.AttendanceCompleteFrom(schoolCtx, days)
			return err
		})
		return complete, err
	}
}
