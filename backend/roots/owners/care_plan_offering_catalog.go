package owners

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	careplanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	enrollmentOwner "github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	configModels "github.com/moto-nrw/project-phoenix/modules/settings/records"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// careOfferingPhaseGuard binds the catalog's phase-window check to the phase
// service, which describes the proposed phase in Enrollment's words.
func careOfferingPhaseGuard(guards careplan.CareOfferingGuards) func(context.Context, int64, *enrollmentOwner.Phase) error {
	return func(ctx context.Context, phaseID int64, replacement *enrollmentOwner.Phase) error {
		var proposed *careplan.OfferingPhase
		if replacement != nil {
			proposed = &careplan.OfferingPhase{
				ID: replacement.ID, Name: replacement.Name,
				ServiceStart: calendar.Date(replacement.ServiceStartDate), ServiceEnd: calendar.Date(replacement.ServiceEndDate),
			}
		}
		return guards.ValidatePhaseChange(ctx, phaseID, proposed)
	}
}

type CareOfferingPhaseReads interface {
	Phase(ctx context.Context, id int64) (*enrollmentOwner.Phase, error)
}

// careOfferingCatalogPhases serves the phase service windows. The Enrollment
// owner reports a missing phase as a plain error, which the catalog has
// always treated as a failure rather than as a correctable input.
type CareOfferingCatalogPhases struct{ Phases CareOfferingPhaseReads }

func (p CareOfferingCatalogPhases) Phase(ctx context.Context, id int64) (careplan.OfferingPhase, error) {
	phase, err := p.Phases.Phase(ctx, id)
	if err != nil {
		return careplan.OfferingPhase{}, err
	}
	if phase == nil {
		return careplan.OfferingPhase{}, careplanCompose.CatalogRowNotFound(fmt.Errorf("phase %d not found", id))
	}
	return careplan.OfferingPhase{
		ID: phase.ID, Name: phase.Name,
		ServiceStart: calendar.Date(phase.ServiceStartDate), ServiceEnd: calendar.Date(phase.ServiceEndDate),
	}, nil
}

type CareOfferingBookingReads interface {
	OfferingGradeCounts(context.Context, []int64, enrollmentOwner.Date, enrollmentOwner.Date) ([]*enrollmentOwner.OfferingGradeCount, error)
	OfferingCapacityPeaks(context.Context, []int64, enrollmentOwner.Date, enrollmentOwner.Date) (map[int64]int, error)
	MaterializableOfferingCount(context.Context, int64, enrollmentOwner.Date) (int, error)
}

type CareOfferingCatalogBookings struct{ Bookings CareOfferingBookingReads }

func (b CareOfferingCatalogBookings) OfferingGradeCounts(ctx context.Context, offeringIDs []int64, from, until calendar.Date) ([]careplanCompose.OfferingGradeCount, error) {
	rows, err := b.Bookings.OfferingGradeCounts(ctx, offeringIDs, enrollmentOwner.Date(from), enrollmentOwner.Date(until))
	if err != nil {
		return nil, err
	}
	result := make([]careplanCompose.OfferingGradeCount, 0, len(rows))
	for _, row := range rows {
		if row != nil {
			result = append(result, careplanCompose.OfferingGradeCount{CareOfferingID: row.CareOfferingID, GradeLevel: row.GradeLevel, Count: row.Count})
		}
	}
	return result, nil
}

func (b CareOfferingCatalogBookings) OfferingCapacityPeaks(ctx context.Context, offeringIDs []int64, from, until calendar.Date) (map[int64]int, error) {
	return b.Bookings.OfferingCapacityPeaks(ctx, offeringIDs, enrollmentOwner.Date(from), enrollmentOwner.Date(until))
}

func (b CareOfferingCatalogBookings) MaterializableOfferingCount(ctx context.Context, offeringID int64, today calendar.Date) (int, error) {
	return b.Bookings.MaterializableOfferingCount(ctx, offeringID, enrollmentOwner.Date(today))
}

type CareOfferingTimetableReads interface {
	FindGroup(context.Context, int64) (timetable.Group, error)
	ListGroups(context.Context, timetable.GroupFilter) ([]timetable.Group, error)
	ListSchedules(context.Context, timetable.ScheduleFilter) ([]timetable.Schedule, error)
	ListTimeframes(context.Context, timetable.TimeframeFilter) ([]timetable.Timeframe, error)
	ListActivityExceptions(context.Context, timetable.ActivityExceptionFilter) ([]timetable.ActivityException, error)
}

// careOfferingCatalogTimetable reads the Timetable rows a linked offering
// materializes from, with the filters the retained repositories used.
type CareOfferingCatalogTimetable struct{ Timetable CareOfferingTimetableReads }

func (t CareOfferingCatalogTimetable) FindGroup(ctx context.Context, id int64) (careplan.LinkedGroup, error) {
	group, err := t.Timetable.FindGroup(ctx, id)
	if err != nil {
		if errors.Is(err, timetable.ErrGroupNotFound) || errors.Is(err, timetable.ErrInvalidGroupQuery) {
			return careplan.LinkedGroup{}, careplanCompose.CatalogRowNotFound(err)
		}
		return careplan.LinkedGroup{}, err
	}
	return linkedGroup(group), nil
}

func (t CareOfferingCatalogTimetable) TemplateSeries(ctx context.Context, groupID int64) ([]careplan.LinkedGroup, error) {
	groups, err := t.Timetable.ListGroups(ctx, timetable.ActiveTemplateSeries(groupID))
	if err != nil {
		return nil, err
	}
	result := make([]careplan.LinkedGroup, 0, len(groups))
	for _, group := range groups {
		result = append(result, linkedGroup(group))
	}
	return result, nil
}

func linkedGroup(group timetable.Group) careplan.LinkedGroup {
	return careplan.LinkedGroup{
		ID: group.ID, IsTemplate: group.IsTemplate, Archived: group.ArchivedAt != nil,
		CalendarPeriodID: group.CalendarPeriodID, PlannedRoomID: group.PlannedRoomID,
	}
}

func (t CareOfferingCatalogTimetable) GroupSchedules(ctx context.Context, groupIDs []int64) ([]careplan.LinkedSchedule, error) {
	if len(groupIDs) == 0 {
		return []careplan.LinkedSchedule{}, nil
	}
	schedules, err := t.Timetable.ListSchedules(ctx, timetable.ScheduleFilter{GroupIDs: groupIDs})
	if err != nil {
		return nil, err
	}
	result := make([]careplan.LinkedSchedule, 0, len(schedules))
	for _, schedule := range schedules {
		result = append(result, careplan.LinkedSchedule{
			GroupID: schedule.ActivityGroupID, Weekday: schedule.Weekday, TimeframeID: schedule.TimeframeID,
			CalendarPeriodID: schedule.CalendarPeriodID, WeekPattern: schedule.WeekPattern,
			ValidFrom: optionalCalendarDate(schedule.ValidFrom), ValidUntil: optionalCalendarDate(schedule.ValidUntil),
		})
	}
	return result, nil
}

func optionalCalendarDate(value *string) *calendar.Date {
	if value == nil {
		return nil
	}
	date := calendar.Date(*value)
	return &date
}

func (t CareOfferingCatalogTimetable) Timeframes(ctx context.Context) ([]careplan.LinkedTimeframe, error) {
	timeframes, err := t.Timetable.ListTimeframes(ctx, timetable.TimeframeFilter{})
	if err != nil {
		return nil, err
	}
	result := make([]careplan.LinkedTimeframe, 0, len(timeframes))
	for _, timeframe := range timeframes {
		start, err := time.Parse("15:04:05", timeframe.StartTime)
		if err != nil {
			return nil, fmt.Errorf("parse timeframe start time: %w", err)
		}
		end, err := optionalCatalogClock(timeframe.EndTime)
		if err != nil {
			return nil, fmt.Errorf("parse timeframe end time: %w", err)
		}
		result = append(result, careplan.LinkedTimeframe{ID: timeframe.ID, StartTime: start, EndTime: end})
	}
	return result, nil
}

func (t CareOfferingCatalogTimetable) ExceptionsBetween(ctx context.Context, from, until calendar.Date) ([]careplan.LinkedException, error) {
	fromText, untilText := from.String(), until.String()
	return t.exceptions(ctx, timetable.ActivityExceptionFilter{FromDate: &fromText, ToDate: &untilText, OrderByDate: true})
}

func (t CareOfferingCatalogTimetable) GroupExceptions(ctx context.Context, groupID int64) ([]careplan.LinkedException, error) {
	return t.exceptions(ctx, timetable.ActivityExceptionFilter{ActivityGroupID: &groupID, OrderByDate: true})
}

func (t CareOfferingCatalogTimetable) exceptions(ctx context.Context, filter timetable.ActivityExceptionFilter) ([]careplan.LinkedException, error) {
	exceptions, err := t.Timetable.ListActivityExceptions(ctx, filter)
	if err != nil {
		return nil, err
	}
	result := make([]careplan.LinkedException, 0, len(exceptions))
	for _, exception := range exceptions {
		linked, err := linkedException(exception)
		if err != nil {
			return nil, err
		}
		result = append(result, linked)
	}
	return result, nil
}

func linkedException(exception timetable.ActivityException) (careplan.LinkedException, error) {
	date, err := calendar.ParseDate(exception.ExceptionDate)
	if err != nil {
		return careplan.LinkedException{}, fmt.Errorf("parse activity exception date: %w", err)
	}
	start, err := optionalCatalogClock(exception.StartTime)
	if err != nil {
		return careplan.LinkedException{}, fmt.Errorf("parse activity exception start time: %w", err)
	}
	end, err := optionalCatalogClock(exception.EndTime)
	if err != nil {
		return careplan.LinkedException{}, fmt.Errorf("parse activity exception end time: %w", err)
	}
	return careplan.LinkedException{
		GroupID: exception.ActivityGroupID, Date: date,
		Cancelled: exception.ExceptionType == timetable.ActivityExceptionCancelled,
		Modified:  exception.ExceptionType == timetable.ActivityExceptionModified,
		RoomID:    exception.RoomID, StartTime: start, EndTime: end,
	}, nil
}

func optionalCatalogClock(value *string) (*time.Time, error) {
	if value == nil {
		return nil, nil
	}
	parsed, err := time.Parse("15:04:05", *value)
	return &parsed, err
}

type CareOfferingCalendarReads interface {
	FindCalendarPeriod(context.Context, int64) (schoolcalendar.CalendarPeriod, error)
}

// careOfferingCatalogCalendar reads planning periods and asks the School
// Calendar's single A/B-week engine.
type CareOfferingCatalogCalendar struct{ Calendar CareOfferingCalendarReads }

func (c CareOfferingCatalogCalendar) FindPeriod(ctx context.Context, id int64) (careplan.LinkedPeriod, error) {
	period, err := c.Calendar.FindCalendarPeriod(ctx, id)
	if err != nil {
		if errors.Is(err, schoolcalendar.ErrCalendarPeriodNotFound) {
			return careplan.LinkedPeriod{}, careplanCompose.CatalogRowNotFound(err)
		}
		return careplan.LinkedPeriod{}, err
	}
	return careplan.LinkedPeriod{
		ID: period.ID, StartDate: calendar.Date(period.StartDate), EndDate: calendar.Date(period.EndDate),
		IsActive: period.IsActive, WeekCycleLength: period.WeekCycleLength, WeekCycleAnchor: period.WeekCycleAnchor,
	}, nil
}

func (CareOfferingCatalogCalendar) WeekPatternApplies(weekPattern int, date calendar.Date, period careplan.LinkedPeriod) bool {
	return schoolcalendar.WeekPatternApplies(weekPattern, date.String(), schoolcalendar.WeekCycle{
		Length: period.WeekCycleLength, Anchor: period.WeekCycleAnchor,
	})
}

type CareOfferingSettingsReads interface {
	ResolveInt(context.Context, string) (int, error)
}

type CareOfferingCatalogSettings struct{ Settings CareOfferingSettingsReads }

func (s CareOfferingCatalogSettings) GradeLevelMax(ctx context.Context) (int, error) {
	return s.Settings.ResolveInt(ctx, configModels.KeyEnrollmentGradeLevelMax)
}

// careOfferingCatalogTranslations interprets the translation document of an
// offering's name, description and selection group with Enrollment's rules
// (#3377).
type CareOfferingCatalogTranslations struct{}

func (CareOfferingCatalogTranslations) NormalizeTranslations(raw json.RawMessage) (json.RawMessage, bool, error) {
	return enrollmentOwner.NormalizeOfferingTranslations(raw)
}

// careOfferingSourceRules is the Timetable owner's offering-source contract.
type CareOfferingSourceRules struct{}

func (CareOfferingSourceRules) MaxSourcesPerTemplate() int {
	return timetable.MaxOfferingSourcesPerTemplate
}

func (CareOfferingSourceRules) Reject(reason string) error {
	return fmt.Errorf("%w: %s", timetable.ErrOfferingSourceInvalid, reason)
}

// offeringSourceRefusalCodes maps each refusal the editor words itself to
// its registered wire code (#2516).
var offeringSourceRefusalCodes = map[careplanCompose.OfferingSourceRefusalKind]string{
	careplanCompose.OfferingSourceTooMany:       timetable.CodeOfferingSourceTooMany,
	careplanCompose.OfferingSourceNotFound:      timetable.CodeOfferingSourceNotFound,
	careplanCompose.OfferingSourceInactive:      timetable.CodeOfferingSourceInactive,
	careplanCompose.OfferingSourceMixedPhases:   timetable.CodeOfferingSourceMixedPhases,
	careplanCompose.OfferingSourceOutsidePeriod: timetable.CodeOfferingSourceOutsidePeriod,
}

func (CareOfferingSourceRules) RejectWith(refusal careplanCompose.OfferingSourceRefusal) error {
	code, ok := offeringSourceRefusalCodes[refusal.Kind]
	if !ok {
		code = timetable.CodeOfferingSourceInvalid
	}
	values := timetable.RefusalValues{Max: refusal.Max, Given: refusal.Given, Offering: refusal.Offering}
	return timetable.WithCode(fmt.Errorf("%w: %s", timetable.ErrOfferingSourceInvalid, refusal.Reason), code, values)
}

func (CareOfferingSourceRules) IsRejection(err error) bool {
	return errors.Is(err, timetable.ErrOfferingSourceInvalid)
}
