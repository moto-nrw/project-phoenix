package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	configModels "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	careplanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	enrollmentOwner "github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/services/enrollment"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The Care Plan care-offering catalog (#3559) reads the Timetable, the School
// Calendar and Enrollment through its own ports. The adapters below bind
// them over the owners' public capabilities and translate their rows into
// the catalog's vocabulary; they add no read of their own.

// careOfferingCatalogInputs are the owners the catalog is composed over.
type careOfferingCatalogInputs struct {
	Records   careplanCompose.CatalogRecords
	Phases    careOfferingPhaseReads
	Bookings  careOfferingBookingReads
	Timetable careOfferingTimetableReads
	Calendar  careOfferingCalendarReads
	Settings  careOfferingSettingsReads
	// SourcedTemplates and Pickup resolve Enrollment's decision service,
	// which is composed after the catalog; they return nil until then.
	SourcedTemplates func() careplanCompose.SourcedTemplateResyncer
	Pickup           func() careplanCompose.PickupResyncer
	LockRecurrence   func(context.Context) error
	Today            func() calendar.Date
	Logger           *slog.Logger
}

// careOfferingResync is the decision service's side of an offering edit.
type careOfferingResync interface {
	careplanCompose.SourcedTemplateResyncer
	careplanCompose.PickupResyncer
}

// lateCareOfferingResync resolves the decision service bound after the
// catalog: both halves read the same variable at call time.
func lateCareOfferingResync(resync *careOfferingResync) (func() careplanCompose.SourcedTemplateResyncer, func() careplanCompose.PickupResyncer) {
	sourced := func() careplanCompose.SourcedTemplateResyncer {
		if *resync == nil {
			return nil
		}
		return *resync
	}
	pickup := func() careplanCompose.PickupResyncer {
		if *resync == nil {
			return nil
		}
		return *resync
	}
	return sourced, pickup
}

func newCareOfferingCatalog(inputs careOfferingCatalogInputs) (careplan.CareOfferingCatalogCapability, error) {
	if inputs.Phases == nil || inputs.Bookings == nil || inputs.Timetable == nil || inputs.Calendar == nil || inputs.Settings == nil {
		return nil, errors.New("care offering catalog: phases, bookings, timetable, calendar and settings are required")
	}
	return careplanCompose.NewCareOfferingCatalog(careplanCompose.CareOfferingCatalogDependencies{
		Records:                inputs.Records,
		Phases:                 careOfferingCatalogPhases{phases: inputs.Phases},
		Timetable:              careOfferingCatalogTimetable{timetable: inputs.Timetable},
		Calendar:               careOfferingCatalogCalendar{calendar: inputs.Calendar},
		Bookings:               careOfferingCatalogBookings{bookings: inputs.Bookings},
		Settings:               careOfferingCatalogSettings{settings: inputs.Settings},
		Translations:           careOfferingCatalogTranslations{},
		SourceRules:            careOfferingSourceRules{},
		SourcedTemplates:       inputs.SourcedTemplates,
		Pickup:                 inputs.Pickup,
		LockTemplateRecurrence: inputs.LockRecurrence,
		Today:                  inputs.Today,
		Logger:                 inputs.Logger,
	})
}

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

type careOfferingPhaseReads interface {
	Phase(ctx context.Context, id int64) (*enrollmentOwner.Phase, error)
}

// careOfferingCatalogPhases serves the phase service windows. The Enrollment
// owner reports a missing phase as a plain error, which the catalog has
// always treated as a failure rather than as a correctable input.
type careOfferingCatalogPhases struct{ phases careOfferingPhaseReads }

func (p careOfferingCatalogPhases) Phase(ctx context.Context, id int64) (careplan.OfferingPhase, error) {
	phase, err := p.phases.Phase(ctx, id)
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

type careOfferingBookingReads interface {
	OfferingGradeCounts(context.Context, []int64, enrollmentOwner.Date, enrollmentOwner.Date) ([]*enrollmentOwner.OfferingGradeCount, error)
	OfferingCapacityPeaks(context.Context, []int64, enrollmentOwner.Date, enrollmentOwner.Date) (map[int64]int, error)
	MaterializableOfferingCount(context.Context, int64, enrollmentOwner.Date) (int, error)
}

type careOfferingCatalogBookings struct{ bookings careOfferingBookingReads }

func (b careOfferingCatalogBookings) OfferingGradeCounts(ctx context.Context, offeringIDs []int64, from, until calendar.Date) ([]careplanCompose.OfferingGradeCount, error) {
	rows, err := b.bookings.OfferingGradeCounts(ctx, offeringIDs, enrollmentOwner.Date(from), enrollmentOwner.Date(until))
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

func (b careOfferingCatalogBookings) OfferingCapacityPeaks(ctx context.Context, offeringIDs []int64, from, until calendar.Date) (map[int64]int, error) {
	return b.bookings.OfferingCapacityPeaks(ctx, offeringIDs, enrollmentOwner.Date(from), enrollmentOwner.Date(until))
}

func (b careOfferingCatalogBookings) MaterializableOfferingCount(ctx context.Context, offeringID int64, today calendar.Date) (int, error) {
	return b.bookings.MaterializableOfferingCount(ctx, offeringID, enrollmentOwner.Date(today))
}

type careOfferingTimetableReads interface {
	FindGroup(context.Context, int64) (timetable.Group, error)
	ListGroups(context.Context, timetable.GroupFilter) ([]timetable.Group, error)
	ListSchedules(context.Context, timetable.ScheduleFilter) ([]timetable.Schedule, error)
	ListTimeframes(context.Context, timetable.TimeframeFilter) ([]timetable.Timeframe, error)
	ListActivityExceptions(context.Context, timetable.ActivityExceptionFilter) ([]timetable.ActivityException, error)
}

// careOfferingCatalogTimetable reads the Timetable rows a linked offering
// materializes from, with the filters the retained repositories used.
type careOfferingCatalogTimetable struct{ timetable careOfferingTimetableReads }

func (t careOfferingCatalogTimetable) FindGroup(ctx context.Context, id int64) (careplan.LinkedGroup, error) {
	group, err := t.timetable.FindGroup(ctx, id)
	if err != nil {
		if errors.Is(err, timetable.ErrGroupNotFound) || errors.Is(err, timetable.ErrInvalidGroupQuery) {
			return careplan.LinkedGroup{}, careplanCompose.CatalogRowNotFound(err)
		}
		return careplan.LinkedGroup{}, err
	}
	return linkedGroup(group), nil
}

func (t careOfferingCatalogTimetable) TemplateSeries(ctx context.Context, groupID int64) ([]careplan.LinkedGroup, error) {
	isTemplate := true
	groups, err := t.timetable.ListGroups(ctx, timetable.GroupFilter{
		IsTemplate: &isTemplate, ActiveOnly: true, SeriesForGroupID: &groupID, OrderByID: true,
	})
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

func (t careOfferingCatalogTimetable) GroupSchedules(ctx context.Context, groupIDs []int64) ([]careplan.LinkedSchedule, error) {
	if len(groupIDs) == 0 {
		return []careplan.LinkedSchedule{}, nil
	}
	schedules, err := t.timetable.ListSchedules(ctx, timetable.ScheduleFilter{GroupIDs: groupIDs})
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

func (t careOfferingCatalogTimetable) Timeframes(ctx context.Context) ([]careplan.LinkedTimeframe, error) {
	timeframes, err := t.timetable.ListTimeframes(ctx, timetable.TimeframeFilter{})
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

func (t careOfferingCatalogTimetable) ExceptionsBetween(ctx context.Context, from, until calendar.Date) ([]careplan.LinkedException, error) {
	fromText, untilText := from.String(), until.String()
	return t.exceptions(ctx, timetable.ActivityExceptionFilter{FromDate: &fromText, ToDate: &untilText, OrderByDate: true})
}

func (t careOfferingCatalogTimetable) GroupExceptions(ctx context.Context, groupID int64) ([]careplan.LinkedException, error) {
	return t.exceptions(ctx, timetable.ActivityExceptionFilter{ActivityGroupID: &groupID, OrderByDate: true})
}

func (t careOfferingCatalogTimetable) exceptions(ctx context.Context, filter timetable.ActivityExceptionFilter) ([]careplan.LinkedException, error) {
	exceptions, err := t.timetable.ListActivityExceptions(ctx, filter)
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

type careOfferingCalendarReads interface {
	FindCalendarPeriod(context.Context, int64) (schoolcalendar.CalendarPeriod, error)
}

// careOfferingCatalogCalendar reads planning periods and asks the School
// Calendar's single A/B-week engine.
type careOfferingCatalogCalendar struct{ calendar careOfferingCalendarReads }

func (c careOfferingCatalogCalendar) FindPeriod(ctx context.Context, id int64) (careplan.LinkedPeriod, error) {
	period, err := c.calendar.FindCalendarPeriod(ctx, id)
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

func (careOfferingCatalogCalendar) WeekPatternApplies(weekPattern int, date calendar.Date, period careplan.LinkedPeriod) bool {
	return schoolcalendar.WeekPatternApplies(weekPattern, date.String(), schoolcalendar.WeekCycle{
		Length: period.WeekCycleLength, Anchor: period.WeekCycleAnchor,
	})
}

type careOfferingSettingsReads interface {
	ResolveInt(context.Context, string) (int, error)
}

type careOfferingCatalogSettings struct{ settings careOfferingSettingsReads }

func (s careOfferingCatalogSettings) GradeLevelMax(ctx context.Context) (int, error) {
	return s.settings.ResolveInt(ctx, configModels.KeyEnrollmentGradeLevelMax)
}

// careOfferingCatalogTranslations interprets the translation document of an
// offering's name, description and selection group with Enrollment's rules
// (#3377).
type careOfferingCatalogTranslations struct{}

func (careOfferingCatalogTranslations) NormalizeTranslations(raw json.RawMessage) (json.RawMessage, bool, error) {
	var translations enrollmentOwner.Translations
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &translations); err != nil {
			return nil, false, nil
		}
	}
	normalized, err := translations.Normalize(
		enrollmentOwner.TranslationAttrName,
		enrollmentOwner.TranslationAttrDescription,
		enrollmentOwner.TranslationAttrSelectionGroup,
	)
	if err != nil {
		return nil, true, err
	}
	if normalized == nil {
		return nil, true, nil
	}
	encoded, err := json.Marshal(normalized)
	if err != nil {
		return nil, true, fmt.Errorf("encode care offering translations: %w", err)
	}
	return encoded, true, nil
}

// careOfferingSourceRules is the Timetable owner's offering-source contract.
type careOfferingSourceRules struct{}

func (careOfferingSourceRules) MaxSourcesPerTemplate() int {
	return timetable.MaxOfferingSourcesPerTemplate
}

func (careOfferingSourceRules) Reject(reason string) error {
	return fmt.Errorf("%w: %s", timetable.ErrOfferingSourceInvalid, reason)
}

func (careOfferingSourceRules) IsRejection(err error) bool {
	return errors.Is(err, timetable.ErrOfferingSourceInvalid)
}

// EnrollmentCareOfferingRows serves the Care Plan catalog to the enrollment
// routes in the enrollment rows they still render.
func (f *Factory) EnrollmentCareOfferingRows() enrollment.CareOfferingRows {
	return enrollment.NewCareOfferingRows(f.EnrollmentCareOffering)
}
