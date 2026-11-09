package api

import (
	"context"

	mealplanModule "github.com/moto-nrw/project-phoenix/modules/mealplan"
	mealplanCompose "github.com/moto-nrw/project-phoenix/modules/mealplan/compose"
	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	"github.com/moto-nrw/project-phoenix/roots/owners"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// parentBillingCareMeals translates the released Mensa months of Meal Plan
// for the monthly run (#4686). Both run in the caller's Träger transaction.
type parentBillingCareMeals struct {
	months     mealplanModule.TraegerMealMonths
	attendance func(context.Context, []int64) (map[int64]calendar.Date, error)
}

func newParentBillingCareMeals(schoolSettings owners.MealAttendanceSchoolSettings, marks owners.MealAttendanceMarks) parentBillingCareMeals {
	return parentBillingCareMeals{months: mealplanCompose.NewTraegerMealMonths(),
		attendance: owners.MealAttendanceFrom(schoolSettings, owners.SchoolAttendanceMarks(marks))}
}

// MealAttendanceFrom is the attendance window of each school, by the rule of
// the Mensa-Monatsfreigabe.
func (m parentBillingCareMeals) MealAttendanceFrom(ctx context.Context, schoolIDs []int64) (map[int64]calendar.Date, error) {
	return m.attendance(ctx, schoolIDs)
}

func (m parentBillingCareMeals) ReleasedMealMonths(ctx context.Context, organizationID int64, until calendar.Date) ([]parentbilling.CareMealRelease, error) {
	months, err := m.months.ReleasedMealMonths(ctx, organizationID, until)
	if err != nil {
		return nil, err
	}
	releases := make([]parentbilling.CareMealRelease, len(months))
	for i, month := range months {
		children := make([]parentbilling.CareMealChild, len(month.Children))
		for j, child := range month.Children {
			children[j] = parentbilling.CareMealChild(child)
		}
		releases[i] = parentbilling.CareMealRelease{SchoolID: month.SchoolID, Month: month.Month, Version: month.Version, Children: children}
	}
	return releases, nil
}
