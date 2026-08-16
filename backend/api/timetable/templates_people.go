package timetable

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
)

// errTemplateStartDateOutsidePeriod marks a create request whose start_date
// lies outside its pinned calendar period (#2135); rendered as a 400.
var errTemplateStartDateOutsidePeriod = errors.New("start_date must lie within the calendar period")

// templateRosterValidFrom resolves the anchor date the initial roster becomes
// valid from. An explicit series start (#2135) wins; otherwise the pinned
// period's start date, or today when no period is pinned. With a pinned period
// the start date must lie within it.
func (rs *Resource) templateRosterValidFrom(
	ctx context.Context,
	calendarPeriodID *int64,
	startDate *timezone.Date,
) (timezone.Date, error) {
	if calendarPeriodID == nil {
		if startDate != nil {
			return *startDate, nil
		}
		return timezone.TodayDate(), nil
	}
	if rs.CalendarPeriodService == nil {
		return timezone.Date{}, errors.New("calendar period service not wired")
	}
	period, err := rs.CalendarPeriodService.GetPeriodByID(ctx, *calendarPeriodID)
	if err != nil {
		return timezone.Date{}, err
	}
	if startDate != nil {
		if startDate.Before(period.StartDate) || startDate.After(period.EndDate) {
			return timezone.Date{}, fmt.Errorf("%w (%s to %s)",
				errTemplateStartDateOutsidePeriod,
				period.StartDate.String(), period.EndDate.String())
		}
		return *startDate, nil
	}
	return period.StartDate, nil
}

func renderTemplatePeriodLookupError(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, sql.ErrNoRows) {
		common.RenderError(w, r, common.ErrorNotFound(errors.New("calendar period not found")))
		return
	}
	if errors.Is(err, errTemplateStartDateOutsidePeriod) {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	common.RenderError(w, r, common.ErrorInternalServerWrap("load calendar period failed", err))
}

// templateWritePreflight resolves the tenant-scoped inputs both the create and
// update write paths need before touching the database: the grade-level cap and
// the roster valid_from anchor. Create passes its optional series start
// (#2135); update passes nil (the stored validity envelope is preserved there).
// It renders the appropriate error and returns ok=false on failure.
func (rs *Resource) templateWritePreflight(
	w http.ResponseWriter,
	r *http.Request,
	calendarPeriodID *int64,
	startDate *timezone.Date,
) (gradeLevelMax int, rosterValidFrom timezone.Date, ok bool) {
	ctx := r.Context()
	gradeLevelMax, err := rs.resolveTemplateGradeLevelMax(ctx)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServerWrap(
			"resolve template grade level limit failed", err))
		return 0, timezone.Date{}, false
	}
	rosterValidFrom, err = rs.templateRosterValidFrom(ctx, calendarPeriodID, startDate)
	if err != nil {
		renderTemplatePeriodLookupError(w, r, err)
		return 0, timezone.Date{}, false
	}
	return gradeLevelMax, rosterValidFrom, true
}

// renderTemplateEducationGroupError maps an education_group_id precheck failure
// to a 400, preserving the precise message. Returns false for other errors so
// callers can fall through to their next classification.
func renderTemplateEducationGroupError(w http.ResponseWriter, r *http.Request, err error) bool {
	var egErr *scheduleSvc.TemplateEducationGroupError
	if !errors.As(err, &egErr) {
		return false
	}
	common.RenderError(w, r, common.ErrorInvalidRequest(egErr))
	return true
}

// parsedTemplateTiming holds the clock window and defaulted numeric fields
// shared by the create and update request shapes.
type parsedTemplateTiming struct {
	startTime       time.Time
	endTime         time.Time
	weekPattern     int
	maxParticipants int
}

// parseTemplateTiming validates the clock window and applies the week-pattern
// and max-participants defaults shared by create and update. Format errors
// render precise 400 messages and return ok=false.
func parseTemplateTiming(
	w http.ResponseWriter,
	r *http.Request,
	startStr, endStr string,
	weekPatternPtr *int,
	maxParticipantsPtr *int,
) (parsedTemplateTiming, bool) {
	startTime, err := parseClockTime(startStr)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(
			errors.New("invalid start_time format, expected HH:MM")))
		return parsedTemplateTiming{}, false
	}
	endTime, err := parseClockTime(endStr)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(
			errors.New("invalid end_time format, expected HH:MM")))
		return parsedTemplateTiming{}, false
	}
	if !endTime.After(startTime) {
		common.RenderError(w, r, common.ErrorInvalidRequest(
			errors.New("end_time must be after start_time")))
		return parsedTemplateTiming{}, false
	}
	weekPattern := 0
	if weekPatternPtr != nil {
		weekPattern = *weekPatternPtr
	}
	if weekPattern < 0 || weekPattern > 2 {
		common.RenderError(w, r, common.ErrorInvalidRequest(
			errors.New("week_pattern must be 0 (every), 1 (A), or 2 (B)")))
		return parsedTemplateTiming{}, false
	}
	maxParticipants := 0
	if maxParticipantsPtr != nil {
		if *maxParticipantsPtr <= 0 {
			common.RenderError(w, r, common.ErrorInvalidRequest(
				errors.New("max_participants must be greater than zero when set")))
			return parsedTemplateTiming{}, false
		}
		maxParticipants = *maxParticipantsPtr
	}
	return parsedTemplateTiming{
		startTime:       startTime,
		endTime:         endTime,
		weekPattern:     weekPattern,
		maxParticipants: maxParticipants,
	}, true
}
