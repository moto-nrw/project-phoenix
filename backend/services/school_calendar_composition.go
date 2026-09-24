package services

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/modules/planexport"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	schoolCalendarCompose "github.com/moto-nrw/project-phoenix/modules/schoolcalendar/compose"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	timetableHTTPAdapter "github.com/moto-nrw/project-phoenix/modules/timetable/compose/httpadapter"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/services/enrollment"
)

// federalStateResolver is the slice of config.SettingsService the tenant
// holiday reads need.
type federalStateResolver interface {
	ResolveString(ctx context.Context, key string) (string, error)
}

// schoolCalendarAdministration binds the owner collaborators the School
// Calendar period administration and the tenant holiday reads resolve at
// call time: the federal-state setting (#1418 3a), the shared tenant
// recurrence gate and Enrollment's care-offering guard, whose refusal is
// mapped onto the calendar's own sentinel.
func schoolCalendarAdministration(
	settings federalStateResolver,
	lockRecurrence func(context.Context) error,
	careOfferings enrollment.CareOfferingCalendarPeriodValidator,
) schoolCalendarCompose.AdministrationRuntime {
	runtime := schoolCalendarCompose.AdministrationRuntime{
		FederalState: func(ctx context.Context) (string, error) {
			region, err := settings.ResolveString(ctx, configModel.KeyFederalState)
			if err != nil {
				return "", fmt.Errorf("failed to resolve federal state setting: %w", err)
			}
			return region, nil
		},
		RecurrenceGate: lockRecurrence,
	}
	if careOfferings != nil {
		runtime.CareOfferingGuard = func(ctx context.Context, periodID int64, replacement *schoolcalendar.CalendarPeriodFields) error {
			err := careOfferings.ValidateCalendarPeriodFieldsChange(ctx, periodID, replacement)
			if enrollment.IsCareOfferingCalendarPeriodConflict(err) {
				return fmt.Errorf("%w: %w", schoolcalendar.ErrCalendarPeriodRequiredByCareOffering, err)
			}
			return err
		}
	}
	return runtime
}

// SchoolCalendarAdministration is the runtime the School Calendar module
// resolves its period administration from once this factory exists: the
// care-offering guard is an Enrollment service composed here, long after the
// root composed the calendar itself.
func (f *Factory) SchoolCalendarAdministration() schoolCalendarCompose.AdministrationRuntime {
	guard, ok := f.EnrollmentCareOffering.(enrollment.CareOfferingCalendarPeriodValidator)
	runtime := schoolCalendarAdministration(f.Settings, f.lockTenantRecurrence, guard)
	if !ok {
		// newFactory refuses this composition; keep the refusal loud on the
		// write path rather than administering periods unguarded.
		runtime.CareOfferingGuard = func(context.Context, int64, *schoolcalendar.CalendarPeriodFields) error {
			return errors.New("calendar period care-offering validation is not configured")
		}
	}
	return runtime
}

// lockTenantRecurrence is the tenant-wide recurrence gate every
// recurrence-derived write of this factory serializes on.
func (f *Factory) lockTenantRecurrence(ctx context.Context) error {
	if f.TimetableData.RecurrenceLock == nil {
		return errors.New("timetable recurrence lock is not configured")
	}
	return f.TimetableData.RecurrenceLock.LockRecurrenceWrites(ctx)
}

// nonWorkingDays serves the retained Soll consumers (time tracking, the
// Dienstplan overview) with the union of statutory holidays and closing
// days keyed by calendar date (#1418 3a/3b): a day that is both collapses
// into one key, so it can never be subtracted twice.
type nonWorkingDays struct {
	calendar schoolcalendar.NonWorkingDayQuery
}

func (d nonWorkingDays) HolidayDates(ctx context.Context, from, to timezone.Date) (map[timezone.Date]bool, error) {
	return calendarDateSet(d.calendar.NonWorkingDayDates(ctx, from.String(), to.String()))
}

// tenantHolidays serves the consumers that must only see statutory
// holidays (the holidays endpoint, the Feiertagsarbeit warning, the
// statistics reports): closing days never leak in.
type tenantHolidays struct {
	calendar schoolcalendar.NonWorkingDayQuery
}

func (h tenantHolidays) HolidayDates(ctx context.Context, from, to timezone.Date) (map[timezone.Date]bool, error) {
	return calendarDateSet(h.calendar.TenantHolidayDates(ctx, from.String(), to.String()))
}

// tenantClosingDays expands the school's closure ranges into a date set.
type tenantClosingDays struct {
	calendar schoolcalendar.NonWorkingDayQuery
}

func (c tenantClosingDays) ClosingDayDates(ctx context.Context, from, to timezone.Date) (map[timezone.Date]bool, error) {
	return calendarDateSet(c.calendar.ClosingDayDates(ctx, from.String(), to.String()))
}

func calendarDateSet(values map[string]bool, err error) (map[timezone.Date]bool, error) {
	if err != nil {
		return nil, err
	}
	result := make(map[timezone.Date]bool, len(values))
	for day := range values {
		result[timezone.Date(day)] = true
	}
	return result, nil
}

// planningCalendarCapability serves workforce.PlanningCalendar from the
// School Calendar: statutory holidays and closing days of the tenant.
type planningCalendarCapability struct{ calendar schoolcalendar.Calendar }

// PlanningCalendarCapability adapts the School Calendar; a nil calendar
// leaves the planning calendar empty.
func PlanningCalendarCapability(calendar schoolcalendar.Calendar) workforce.PlanningCalendar {
	return planningCalendarCapability{calendar: calendar}
}

func (c planningCalendarCapability) HolidaysInRange(ctx context.Context, from, to string) ([]workforce.PublicHoliday, error) {
	if c.calendar == nil {
		return []workforce.PublicHoliday{}, nil
	}
	if _, _, err := parseCapabilityRange(from, to); err != nil {
		return nil, err
	}
	holidays, err := c.calendar.TenantHolidays(ctx, from, to)
	if err != nil {
		return nil, err
	}
	result := make([]workforce.PublicHoliday, 0, len(holidays))
	for _, holiday := range holidays {
		result = append(result, workforce.PublicHoliday{Date: holiday.Date, Name: holiday.Name})
	}
	return result, nil
}

func (c planningCalendarCapability) ClosingDaysInRange(ctx context.Context, from, to string) ([]workforce.ClosingPeriod, error) {
	if c.calendar == nil {
		return []workforce.ClosingPeriod{}, nil
	}
	if _, _, err := parseCapabilityRange(from, to); err != nil {
		return nil, err
	}
	days, err := c.calendar.ListClosingDays(ctx, schoolcalendar.ClosingDayFilter{OverlappingFrom: from, OverlappingTo: to})
	if err != nil {
		return nil, err
	}
	result := make([]workforce.ClosingPeriod, 0, len(days))
	for _, day := range days {
		result = append(result, workforce.ClosingPeriod{StartDate: day.StartDate, EndDate: day.EndDate, Reason: day.Reason})
	}
	return result, nil
}

// planExportClosingDays and planExportHolidays serve the plan export's
// non-working-day labels from the School Calendar.
type planExportClosingDays struct{ calendar schoolcalendar.Calendar }

func (a planExportClosingDays) ClosingDaysInRange(ctx context.Context, from, to planexport.Date) ([]*planexport.ClosingPeriod, error) {
	if _, _, err := parseCapabilityRange(string(from), string(to)); err != nil {
		return nil, fmt.Errorf("plan export range: %w", err)
	}
	ranges, err := a.calendar.ListClosingDays(ctx, schoolcalendar.ClosingDayFilter{OverlappingFrom: string(from), OverlappingTo: string(to)})
	if err != nil {
		return nil, err
	}
	out := make([]*planexport.ClosingPeriod, 0, len(ranges))
	for _, closing := range ranges {
		out = append(out, &planexport.ClosingPeriod{
			StartDate: planexport.Date(closing.StartDate), EndDate: planexport.Date(closing.EndDate), Reason: closing.Reason,
		})
	}
	return out, nil
}

type planExportHolidays struct{ calendar schoolcalendar.Calendar }

func (a planExportHolidays) HolidaysInRange(ctx context.Context, from, to planexport.Date) ([]planexport.Holiday, error) {
	if _, _, err := parseCapabilityRange(string(from), string(to)); err != nil {
		return nil, fmt.Errorf("plan export range: %w", err)
	}
	holidays, err := a.calendar.TenantHolidays(ctx, string(from), string(to))
	if err != nil {
		return nil, err
	}
	out := make([]planexport.Holiday, 0, len(holidays))
	for _, holiday := range holidays {
		out = append(out, planexport.Holiday{Date: planexport.Date(holiday.Date), Name: holiday.Name})
	}
	return out, nil
}

// TimeframeChangeGuard binds the schedules API's timeframe edit and delete
// guard: the change is serialized under the tenant recurrence gate and
// refused while a linked care offering still needs the timeframe.
func TimeframeChangeGuard(
	lockRecurrence func(context.Context) error,
	validate func(ctx context.Context, timeframeID int64, replacement *enrollment.TimeframeReplacement) error,
) timetableHTTPAdapter.TimeframeChangeGuard {
	return func(ctx context.Context, timeframeID int64, replacement *timetable.TimeframeInput) error {
		if err := lockRecurrence(ctx); err != nil {
			return fmt.Errorf("lock timetable recurrence: %w", err)
		}
		var proposed *enrollment.TimeframeReplacement
		if replacement != nil {
			proposed = &enrollment.TimeframeReplacement{
				StartTime: replacement.StartTime, EndTime: replacement.EndTime,
				IsActive: replacement.IsActive, Description: replacement.Description,
			}
		}
		if err := validate(ctx, timeframeID, proposed); err != nil {
			if enrollment.IsCareOfferingInvalid(err) {
				return fmt.Errorf("%w: %w", timetable.ErrTimeframeRequiredByCareOffering, err)
			}
			return fmt.Errorf("validate care offerings: %w", err)
		}
		return nil
	}
}
