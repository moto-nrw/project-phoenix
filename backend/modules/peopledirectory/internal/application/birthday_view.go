package application

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/domain"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

func NewBirthdayView(service *BirthdayService) peopledirectory.Birthdays {
	return birthdayView{service: service}
}

type birthdayView struct{ service *BirthdayService }

func (b birthdayView) Overview(ctx context.Context, visibility peopledirectory.BirthdayVisibility, weekOf *calendar.Date) (peopledirectory.BirthdayOverview, error) {
	// A nil visibility stays nil through the conversion: no child is visible.
	value, err := b.service.Overview(ctx, visibility, weekOf)
	if err != nil {
		return peopledirectory.BirthdayOverview{}, mapBirthdayError(err)
	}
	celebrations := make([]peopledirectory.BirthdayCelebration, 0, len(value.Celebrations))
	for _, c := range value.Celebrations {
		celebrations = append(celebrations, peopledirectory.BirthdayCelebration{
			Kind: peopledirectory.BirthdayKind(c.Kind), ID: c.ID, Name: c.Name,
			GroupName: c.GroupName, SchoolClass: c.SchoolClass, Date: c.Date, Age: c.Age, IsToday: c.IsToday,
		})
	}
	return peopledirectory.BirthdayOverview{
		Enabled: value.Enabled, IncludeStaff: value.IncludeStaff, Today: value.Today,
		WeekStart: value.Week.Start, WeekEnd: value.Week.End,
		EarliestWeekStart: value.EarliestWeekStart, LatestWeekStart: value.LatestWeekStart,
		Celebrations: celebrations,
	}, nil
}

func (b birthdayView) GetOptOut(ctx context.Context, accountID int64) (bool, error) {
	optOut, err := b.service.GetOptOut(ctx, accountID)
	return optOut, mapBirthdayError(err)
}

func (b birthdayView) OwnStaffBirthday(ctx context.Context, accountID int64) (peopledirectory.StaffBirthdayPreference, error) {
	staff, err := b.service.resolveOwnStaff(ctx, accountID)
	return peopledirectory.StaffBirthdayPreference{StaffID: staff.ID, OptOut: staff.OptOut}, mapBirthdayError(err)
}

func (b birthdayView) ListStaffBirthdays(ctx context.Context, months map[time.Month]bool) ([]peopledirectory.StaffBirthday, error) {
	values, err := b.service.ListStaffBirthdays(ctx, months)
	if err != nil {
		return nil, mapBirthdayError(err)
	}
	rows := make([]peopledirectory.StaffBirthday, 0, len(values))
	for _, v := range values {
		rows = append(rows, peopledirectory.StaffBirthday{Name: v.FullName(), Birthday: v.Birthday})
	}
	return rows, nil
}

func mapBirthdayError(err error) error {
	if errors.Is(err, domain.ErrBirthdayStaffNotFound) {
		return fmt.Errorf("%w: %w", peopledirectory.ErrStaffNotFound, err)
	}
	if errors.Is(err, domain.ErrBirthdayWeekOutOfRange) {
		return fmt.Errorf("%w: %w", peopledirectory.ErrBirthdayWeekOutOfRange, err)
	}
	return err
}
