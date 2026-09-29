package compose

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/ports"
)

// The birthday display reads persons, students and staff through a store the
// composition root binds over the retained repositories, and two tenant
// settings it binds over the Settings Platform.
type (
	BirthdayStore    = ports.BirthdayStore
	BirthdaySettings = ports.BirthdaySettings
	BirthdayEntry    = domain.BirthdayEntry
	BirthdayKind     = domain.BirthdayKind
	BirthdayMonthDay = domain.MonthDay
	BirthdayStaff    = domain.BirthdayStaff
)

const (
	BirthdayKindStudent = domain.BirthdayKindStudent
	BirthdayKindStaff   = domain.BirthdayKindStaff
)

// BirthdayDependencies wires the birthday capability.
type BirthdayDependencies struct {
	Store    BirthdayStore
	Settings BirthdaySettings
	Logger   *slog.Logger
	// Now is injectable so tests can pin a weekday; nil means time.Now.
	Now func() time.Time
}

// NewBirthdays returns the birthday display capability.
func NewBirthdays(deps BirthdayDependencies) peopledirectory.Birthdays {
	return birthdays{service: application.NewBirthdayService(deps.Store, deps.Settings, deps.Logger, deps.Now)}
}

type birthdays struct{ service *application.BirthdayService }

func (b birthdays) Overview(ctx context.Context, visibility peopledirectory.BirthdayVisibility) (peopledirectory.BirthdayOverview, error) {
	// A nil visibility stays nil through the conversion: no child is visible.
	value, err := b.service.Overview(ctx, visibility)
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
		Enabled: value.Enabled, IncludeStaff: value.IncludeStaff, Today: value.Today, Celebrations: celebrations,
	}, nil
}

func (b birthdays) GetOptOut(ctx context.Context, accountID int64) (bool, error) {
	optOut, err := b.service.GetOptOut(ctx, accountID)
	return optOut, mapBirthdayError(err)
}

func (b birthdays) SetOptOut(ctx context.Context, accountID int64, optOut bool) error {
	return mapBirthdayError(b.service.SetOptOut(ctx, accountID, optOut))
}

func (b birthdays) ListStaffBirthdays(ctx context.Context, months map[time.Month]bool) ([]peopledirectory.StaffBirthday, error) {
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
	return err
}
