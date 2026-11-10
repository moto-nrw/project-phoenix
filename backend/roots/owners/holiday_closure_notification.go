package owners

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	schoolCalendarCompose "github.com/moto-nrw/project-phoenix/modules/schoolcalendar/compose"
	configModels "github.com/moto-nrw/project-phoenix/modules/settings/records"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
	notification "github.com/moto-nrw/project-phoenix/workflows/holidayclosurenotification"
	notificationPorts "github.com/moto-nrw/project-phoenix/workflows/holidayclosurenotification/ports"
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
func NewHolidayClosureNotifications(closures enrollment.HolidayClosures, schools notificationPorts.Schools, logger *slog.Logger) notification.Notifications {
	if closures == nil || schools == nil {
		panic("holiday closure notification: Enrollment's closures and the Träger's schools are required")
	}
	events := schoolCalendarCompose.NewHolidayClosureEvents()
	return notification.New(notification.Dependencies{
		Proposals: events, Queue: events, Closures: closures,
		Schools: schools, Units: holidayClosureUnits{}, Logger: logger,
	})
}

// NewHolidayClosureCalendar binds Enrollment's calendar of a school
// closing day operation (ADR 0196) to School Calendar's read channel of the
// worker in the caller's Träger transaction: the statutory holidays of the
// school's stored federal state, of the registry default when it stored
// none, and its closing days with their sources. An unknown state fails the
// command visibly; there is no other fallback.
func NewHolidayClosureCalendar(holidays HolidayClosureHolidays) enrollment.HolidayClosureCalendar {
	if holidays == nil {
		panic("holiday closure calendar: School Calendar's holidays are required")
	}
	return holidayClosureCalendar{holidays: holidays, reads: schoolCalendarCompose.NewHolidayClosureEvents()}
}

type holidayClosureCalendar struct {
	holidays HolidayClosureHolidays
	reads    schoolcalendar.HolidayClosureEventQueue
}

func (c holidayClosureCalendar) HolidayClosureNoCareDays(ctx context.Context, organizationID, schoolID int64, from, to calendar.Date) ([]enrollment.HolidayNoCareDay, error) {
	school, err := c.reads.HolidayClosureCalendar(ctx, organizationID, schoolID, from, to)
	if err != nil {
		return nil, err
	}
	region, err := holidayClosureRegion(school.FederalState)
	if err != nil {
		return nil, err
	}
	if !c.holidays.ValidHolidayRegion(region) {
		return nil, fmt.Errorf("%w: unsupported region %q", schoolcalendar.ErrFederalStateUnavailable, region)
	}
	listed, err := c.holidays.ListHolidays(ctx, region, from.String(), to.String())
	if err != nil {
		return nil, err
	}
	holidayDays := make([]enrollment.HolidayNoCareDay, 0, len(listed))
	for _, holiday := range listed {
		day, err := calendar.ParseDate(holiday.Date)
		if err != nil {
			return nil, fmt.Errorf("holiday closure calendar: %w", err)
		}
		holidayDays = append(holidayDays, enrollment.HolidayNoCareDay{Date: day, Kind: enrollment.HolidayNoCareHoliday, Name: holiday.Name})
	}
	var closing []enrollment.HolidayNoCareDay
	for _, closingDay := range school.ClosingDays {
		for day := max(closingDay.StartsOn, from); !day.After(min(closingDay.EndsOn, to)); day = day.AddDays(1) {
			closing = append(closing, enrollment.HolidayNoCareDay{Date: day, Kind: enrollment.HolidayNoCareClosingDay, Name: closingDay.Reason,
				ClosingDays: []enrollment.HolidayClosingDayRef{{ID: closingDay.SourceID, Traeger: closingDay.Traeger}}})
		}
	}
	return mergeNoCareDays(holidayDays, closing), nil
}

// holidayClosureRegion is the stored federal state or the registry default
// of the settings contract.
func holidayClosureRegion(stored string) (string, error) {
	if stored != "" {
		return stored, nil
	}
	definition := configModels.DefaultRegistry().GetDefinition(configModels.KeyFederalState)
	if definition == nil {
		return "", fmt.Errorf("%w: no registry default", schoolcalendar.ErrFederalStateUnavailable)
	}
	value, ok := definition.Default.(string)
	if !ok || value == "" {
		return "", fmt.Errorf("%w: registry default is no state", schoolcalendar.ErrFederalStateUnavailable)
	}
	return value, nil
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
