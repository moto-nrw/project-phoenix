package owners

import (
	"context"
	"errors"
	"log/slog"

	"github.com/moto-ogs/project-phoenix/modules/enrollment"
	"github.com/moto-ogs/project-phoenix/modules/schoolcalendar"
	schoolCalendarCompose "github.com/moto-ogs/project-phoenix/modules/schoolcalendar/compose"
	configModels "github.com/moto-ogs/project-phoenix/modules/settings/records"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
	"github.com/moto-ogs/project-phoenix/tenant"
	notification "github.com/moto-ogs/project-phoenix/workflows/holidayclosurenotification"
	notificationPorts "github.com/moto-ogs/project-phoenix/workflows/holidayclosurenotification/ports"
)

// HolidayClosureHolidays is what the worker reads of School Calendar's
// statutory holidays: those of one federal state.
type HolidayClosureHolidays interface {
	ValidHolidayRegion(string) bool
	ListHolidays(ctx context.Context, region, from, to string) ([]schoolcalendar.Holiday, error)
}

// NewHolidayClosureNotifications binds the holiday closure notification
// (ADR 0196) to School Calendar's proposals and commands, Enrollment's
// closures and the units of work. The root builds it where it mounts a
// route or the Worker's job.
func NewHolidayClosureNotifications(closures enrollment.HolidayClosures, schools notificationPorts.Schools, mails notificationPorts.Mails, logger *slog.Logger) notification.Notifications {
	if closures == nil || schools == nil || mails == nil {
		panic("holiday closure notification: Enrollment's closures and the Träger's schools are required")
	}
	events := schoolCalendarCompose.NewHolidayClosureEvents()
	return notification.New(notification.Dependencies{
		Proposals: events, Queue: events, Closures: closures, Mails: mails,
		Schools: schools, Units: holidayClosureUnits{}, Logger: logger,
	})
}

// NewHolidayClosureCalendar binds School Calendar's guarded no-care capability
// to Enrollment's consumer contract and the settings registry default.
func NewHolidayClosureCalendar(holidays HolidayClosureHolidays) enrollment.HolidayClosureCalendar {
	if holidays == nil {
		panic("holiday closure calendar: School Calendar's holidays are required")
	}
	var defaultRegion string
	if definition := configModels.DefaultRegistry().GetDefinition(configModels.KeyFederalState); definition != nil {
		defaultRegion, _ = definition.Default.(string)
	}
	return holidayClosureCalendar{calendar: schoolCalendarCompose.NewHolidayClosureCalendar(holidays, defaultRegion)}
}

type holidayClosureCalendar struct {
	calendar schoolcalendar.HolidayClosureCareCalendar
}

func (c holidayClosureCalendar) HolidayClosureNoCareDays(ctx context.Context, organizationID int64, schoolIDs []int64, from, to calendar.Date, revisions map[int64]int64) (map[int64][]enrollment.HolidayNoCareDay, error) {
	schools, err := c.calendar.HolidayClosureNoCareDays(ctx, organizationID, schoolIDs, from, to, revisions)
	if errors.Is(err, schoolcalendar.ErrHolidayClosureCalendarChanged) {
		return nil, enrollment.ErrHolidayClosureChanged
	}
	if err != nil {
		return nil, err
	}
	result := make(map[int64][]enrollment.HolidayNoCareDay, len(schools))
	for school, days := range schools {
		result[school] = make([]enrollment.HolidayNoCareDay, 0, len(days))
		for _, day := range days {
			mapped := enrollment.HolidayNoCareDay{Date: day.Date, Kind: day.Kind, Name: day.Name}
			for _, source := range day.ClosingDays {
				mapped.ClosingDays = append(mapped.ClosingDays, enrollment.HolidayClosingDayRef{ID: source.ID, Traeger: source.Traeger})
			}
			result[school] = append(result[school], mapped)
		}
	}
	return result, nil
}

// holidayClosureUnits opens each unit on its own: the caller's session
// binds a school or a Träger, which a unit of the other kind masks, and no
// unit runs inside another.
type holidayClosureUnits struct{}

func (holidayClosureUnits) WithinSchool(ctx context.Context, schoolID int64, fn func(context.Context) error) error {
	id, err := tenant.NewTenantID(schoolID)
	if err != nil {
		return err
	}
	return tenant.WithinTenant(tenant.ContextWithoutTransaction(ctx), id, fn)
}

func (holidayClosureUnits) WithinTraeger(ctx context.Context, organizationID int64, fn func(context.Context) error) error {
	return tenant.WithinTraeger(tenant.ContextWithoutTenant(tenant.ContextWithoutTransaction(ctx)), organizationID, fn)
}

func (holidayClosureUnits) WithinAdmin(ctx context.Context, fn func(context.Context) error) error {
	return tenant.WithinAdmin(tenant.ContextWithoutTenant(tenant.ContextWithoutTransaction(ctx)), fn)
}
