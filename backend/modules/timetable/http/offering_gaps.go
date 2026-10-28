// Package timetablehttp — offerings without a matching Regeltermin (#5140):
//
//	GET /api/timetable/offering-gaps?period_id=…
//	PUT /api/timetable/offering-gaps/{offeringId}/acknowledgement
package timetablehttp

import (
	"context"
	"errors"
	"net/http"
	"slices"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

type offeringGapResponse struct {
	Offering            templateRosterMaintenanceOfferingResponse `json:"offering"`
	StudentCount        int                                       `json:"student_count"`
	SchoolClasses       []string                                  `json:"school_classes"`
	Weekdays            []int                                     `json:"weekdays"`
	Acknowledged        bool                                      `json:"acknowledged"`
	AcknowledgedClasses []string                                  `json:"acknowledged_classes"`
}

type offeringGapsResponse struct {
	Gaps []offeringGapResponse `json:"gaps"`
}

// errInactivePlanningPeriod refuses a period the Betreuungsplan does not
// plan in: unknown, inactive or of another school. Only an active period
// shows hints and takes decisions, so no decision can stem from a period
// nobody plans in.
var errInactivePlanningPeriod = errors.New("period_id must name an active planning period")

// listOfferingGaps handles GET /offering-gaps: per offering of the period,
// the booked children no Regeltermin takes over, with the school's decision
// for this period. An unknown or inactive period shows no hints. Every read
// is batched over the period's templates and bookings.
func (rs *Resource) listOfferingGaps(w http.ResponseWriter, r *http.Request) {
	if rs.TimetableData == nil || rs.CalendarPeriods == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("timetable offering gaps not wired")))
		return
	}
	periodID, ok := templatePeriodIDFromRequest(w, r)
	if !ok {
		return
	}
	if periodID == nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("period_id is required")))
		return
	}
	gaps, err := rs.offeringGaps(r.Context(), *periodID)
	if errors.Is(err, errInactivePlanningPeriod) {
		gaps, err = []timetable.OfferingGap{}, nil
	}
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServerWrap("list offering gaps failed", err))
		return
	}
	result := make([]offeringGapResponse, 0, len(gaps))
	for _, gap := range gaps {
		result = append(result, offeringGapResponseFrom(gap))
	}
	common.Respond(w, r, http.StatusOK, offeringGapsResponse{Gaps: result}, "Offering gaps retrieved")
}

// offeringGaps compares the bookings of an active period from today on with
// the Regeltermine the offerings feed, date by date.
// errInactivePlanningPeriod for an unknown or inactive period. Without the
// offering side there is nothing to compare.
func (rs *Resource) offeringGaps(ctx context.Context, periodID int64) ([]timetable.OfferingGap, error) {
	period, err := rs.activePlanningPeriod(ctx, periodID)
	if err != nil || rs.OfferingSourceOptions == nil {
		return []timetable.OfferingGap{}, err
	}
	scope := timetable.OfferingGapScope{From: max(rs.todayDate(), calendar.Date(period.StartDate)), Until: calendar.Date(period.EndDate)}
	if scope.From.After(scope.Until) {
		return []timetable.OfferingGap{}, nil
	}
	bookings, err := rs.OfferingSourceOptions.OfferingWeekdayBookings(ctx, &periodID, scope.From)
	if err != nil || len(bookings) == 0 {
		return []timetable.OfferingGap{}, err
	}
	templates, err := rs.offeringGapTemplates(ctx, period, scope)
	if err != nil {
		return nil, err
	}
	return timetable.DetectOfferingGaps(scope, templates, bookings), nil
}

// activePlanningPeriod loads the period through the tenant-scoped School
// Calendar; another school's id is not found.
func (rs *Resource) activePlanningPeriod(ctx context.Context, periodID int64) (schoolcalendar.CalendarPeriod, error) {
	period, err := rs.CalendarPeriods.FindCalendarPeriod(ctx, periodID)
	if errors.Is(err, schoolcalendar.ErrCalendarPeriodNotFound) || (err == nil && !period.IsActive) {
		return schoolcalendar.CalendarPeriod{}, errInactivePlanningPeriod
	}
	return period, err
}

// offeringGapTemplates reads the period's Regeltermine with the offerings
// that feed them. Unlike the template list, a failed offering read fails
// the request: without it every booking would look uncovered.
func (rs *Resource) offeringGapTemplates(ctx context.Context, period schoolcalendar.CalendarPeriod, scope timetable.OfferingGapScope) ([]timetable.OfferingGapTemplate, error) {
	ratio := rs.childrenPerStaffRatio(ctx)
	rows, err := rs.TimetableData.ListTemplateEntriesForPeriod(ctx, &period.ID, ratio, &timetable.TemplateScheduleWindow{From: scope.From, Until: scope.Until})
	if err != nil || len(rows) == 0 {
		return []timetable.OfferingGapTemplate{}, err
	}
	templates := mapTemplateRows(rows, ratio, nil)
	queries := make([]timetable.TemplateRosterMaintenanceQuery, 0, len(templates))
	for _, template := range templates {
		queries = append(queries, timetable.TemplateRosterMaintenanceQuery{
			TemplateID:            template.ID,
			CalendarPeriodID:      rosterMaintenanceCalendarPeriodID(template, &period.ID),
			SourceCareOfferingIDs: template.SourceCareOfferingIDs,
			SourceGradeLevels:     template.SourceGradeLevels,
			SourceSchoolClasses:   template.SourceSchoolClasses,
		})
	}
	feeds, err := rs.OfferingSourceOptions.TemplateRosterMaintenance(ctx, queries)
	if err != nil {
		return nil, err
	}
	result := make([]timetable.OfferingGapTemplate, 0, len(templates))
	for _, template := range templates {
		if template.Type == timetable.GroupTypeDuty {
			continue
		}
		result = append(result, offeringGapTemplate(template, feeds[template.ID], period))
	}
	return result, nil
}

// offeringGapTemplate splits the feeding offerings into the source rule,
// which keeps its Jahrgang or Klassen filter, and the offerings' own links,
// which feed every booked child.
func offeringGapTemplate(template templateResponse, feeds timetable.TemplateRosterMaintenance, period schoolcalendar.CalendarPeriod) timetable.OfferingGapTemplate {
	gapTemplate := timetable.OfferingGapTemplate{
		Runs:              scheduleRuns(template, period),
		LinkedOfferingIDs: append([]int64(nil), feeds.LinkedOfferingIDs...),
		Sources: timetable.OfferingSourceFilter{
			GradeLevels: feeds.GradeLevels, SchoolClasses: feeds.SchoolClasses,
		},
	}
	for _, offering := range feeds.Offerings {
		if slices.Contains(template.SourceCareOfferingIDs, offering.ID) {
			gapTemplate.Sources.OfferingIDs = append(gapTemplate.Sources.OfferingIDs, offering.ID)
		}
	}
	return gapTemplate
}

// scheduleRuns dates each schedule of the period from its first to its last
// occurrence, within the period, the split bounds (valid_until exclusive)
// and the series end, with the A/B pattern deciding which weeks occur. A
// schedule without an occurrence gives no run.
func scheduleRuns(template templateResponse, period schoolcalendar.CalendarPeriod) []timetable.OfferingGapRun {
	periodFirst, periodLast := calendar.Date(period.StartDate), calendar.Date(period.EndDate)
	if template.EndDate != nil {
		periodLast = min(periodLast, calendar.Date(*template.EndDate))
	}
	cycle := schoolcalendar.WeekCycle{Length: period.WeekCycleLength, Anchor: period.WeekCycleAnchor}
	runs := make([]timetable.OfferingGapRun, 0, len(template.Schedules))
	for _, schedule := range template.Schedules {
		if schedule.CalendarPeriodID != nil && *schedule.CalendarPeriodID != period.ID {
			continue
		}
		first := max(periodFirst, calendar.Date(schedule.ValidFrom))
		last := periodLast
		if schedule.ValidUntil != "" {
			last = min(last, calendar.Date(schedule.ValidUntil).AddDays(-1))
		}
		occurs := func(date calendar.Date) bool {
			return schoolcalendar.WeekPatternApplies(schedule.WeekPattern, date.String(), cycle)
		}
		from, ok := firstOccurrence(timetable.NextWeekdayOnOrAfter(first, schedule.Weekday), last, 7, occurs)
		if !ok {
			continue
		}
		until, _ := firstOccurrence(timetable.PreviousWeekdayOnOrBefore(last, schedule.Weekday), from, -7, occurs)
		runs = append(runs, timetable.OfferingGapRun{Weekday: schedule.Weekday, From: from, Until: until})
	}
	return runs
}

// firstOccurrence walks from start towards bound in steps of days and
// returns the first date the pattern lets occur.
func firstOccurrence(start, bound calendar.Date, days int, occurs func(calendar.Date) bool) (calendar.Date, bool) {
	for date := start; (days > 0 && !date.After(bound)) || (days < 0 && !date.Before(bound)); date = date.AddDays(days) {
		if occurs(date) {
			return date, true
		}
	}
	return calendar.Date(""), false
}

func offeringGapResponseFrom(gap timetable.OfferingGap) offeringGapResponse {
	return offeringGapResponse{
		Offering:            templateRosterMaintenanceOfferingResponse{ID: gap.OfferingID, Name: gap.OfferingName},
		StudentCount:        gap.StudentCount,
		SchoolClasses:       gap.SchoolClasses,
		Weekdays:            gap.Weekdays,
		Acknowledged:        gap.Acknowledged,
		AcknowledgedClasses: gap.AcknowledgedClasses,
	}
}

type offeringGapAckRequest struct {
	PeriodID     int64 `json:"period_id"`
	Acknowledged *bool `json:"acknowledged"`
}

func (req *offeringGapAckRequest) Bind(*http.Request) error {
	if req.Acknowledged == nil {
		return errors.New("acknowledged is required")
	}
	if req.PeriodID <= 0 {
		return errors.New("period_id is required")
	}
	return nil
}

type offeringGapAckResponse struct {
	Acknowledged        bool     `json:"acknowledged"`
	AcknowledgedClasses []string `json:"acknowledged_classes"`
}

// setOfferingGapAck handles PUT /offering-gaps/{offeringId}/acknowledgement:
// "Kein eigener Regeltermin nötig" for the classes the hint names right now,
// or its undo. The classes come from the server's own comparison, never from
// the client, so the decision covers exactly what the hint showed, and it
// holds for this planning period only. Both directions need an active period
// and the offering's hint in it, decided or not.
func (rs *Resource) setOfferingGapAck(w http.ResponseWriter, r *http.Request) {
	offeringID, ok := common.ParsePositiveInt64IDWithError(w, r, "offeringId", "invalid offering id")
	if !ok {
		return
	}
	if rs.TimetableData == nil || rs.CalendarPeriods == nil || rs.OfferingSourceOptions == nil || rs.OfferingGapDecisions == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("timetable offering gaps not wired")))
		return
	}
	req := &offeringGapAckRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, bindErrorRenderer(err))
		return
	}
	classes, err := rs.openOfferingGapClasses(r.Context(), req.PeriodID, offeringID)
	if err != nil {
		renderOfferingGapError(w, r, err)
		return
	}
	if !*req.Acknowledged {
		classes = []string{}
	}
	if err := rs.OfferingGapDecisions.DecideOfferingTemplateGap(r.Context(), offeringID, req.PeriodID, classes); err != nil {
		renderOfferingGapError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, offeringGapAckResponse{
		Acknowledged: *req.Acknowledged, AcknowledgedClasses: classes,
	}, "Offering gap decision saved")
}

// openOfferingGapClasses returns the classes of the offering's current gap;
// ErrOfferingGapNotFound when every booked child has a Regeltermin.
func (rs *Resource) openOfferingGapClasses(ctx context.Context, periodID, offeringID int64) ([]string, error) {
	gaps, err := rs.offeringGaps(ctx, periodID)
	if err != nil {
		return nil, err
	}
	for _, gap := range gaps {
		if gap.OfferingID == offeringID {
			return gap.SchoolClasses, nil
		}
	}
	return nil, timetable.ErrOfferingGapNotFound
}

// renderOfferingGapError answers an unknown or inactive period with 400, an
// offering without a hint, or one Care Plan no longer knows, with 404 and
// everything else with 500.
func renderOfferingGapError(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, errInactivePlanningPeriod) {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	if errors.Is(err, timetable.ErrOfferingGapNotFound) || errors.Is(err, careplan.ErrCareOfferingNotFound) {
		common.RenderError(w, r, common.ErrorNotFound(err))
		return
	}
	common.RenderError(w, r, common.ErrorInternalServerWrap("offering gap decision failed", err))
}
