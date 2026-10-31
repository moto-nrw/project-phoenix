// Package timetablehttp — roster review of hand-kept Regeltermine (#5103):
//
//	GET  /api/timetable/templates/roster-deviations?period_id=…
//	GET  /api/timetable/templates/{id}/offering-preview?offering_ids=…
//	POST /api/timetable/templates/{id}/offering-connection
//	PUT  /api/timetable/templates/{id}/manual-roster
package timetablehttp

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

type rosterDeviationResponse struct {
	TemplateID int64 `json:"template_id"`
	// Offerings is the suggested source; several offerings combine (#5139).
	Offerings     []templateRosterMaintenanceOfferingResponse `json:"offerings"`
	GradeLevels   []int                                       `json:"grade_levels"`
	SchoolClasses []string                                    `json:"school_classes"`
	UnbookedCount int                                         `json:"unbooked_count"`
	MissingCount  int                                         `json:"missing_count"`
	Candidates    []templateRosterMaintenanceOfferingResponse `json:"candidates"`
	// Combinations are the equal choices when Ambiguous, else empty.
	Combinations [][]templateRosterMaintenanceOfferingResponse `json:"combinations"`
	// Ambiguous: other offerings fit as well as suggested ones; the school
	// picks.
	Ambiguous bool `json:"ambiguous"`
}

type rosterDeviationsResponse struct {
	Deviations []rosterDeviationResponse `json:"deviations"`
}

// listTemplateRosterDeviations handles GET /templates/roster-deviations: the
// Regeltermine of the period that are not maintained automatically, not
// confirmed as hand-kept and whose children obviously follow an offering's
// bookings. Every read is batched over the period's templates.
func (rs *Resource) listTemplateRosterDeviations(w http.ResponseWriter, r *http.Request) {
	if rs.TimetableData == nil || rs.RosterReview == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("timetable roster review not wired")))
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
	deviations, err := rs.templateRosterDeviations(r.Context(), *periodID)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServerWrap("list roster deviations failed", err))
		return
	}
	common.Respond(w, r, http.StatusOK, rosterDeviationsResponse{Deviations: deviations}, "Roster deviations retrieved")
}

func (rs *Resource) templateRosterDeviations(ctx context.Context, periodID int64) ([]rosterDeviationResponse, error) {
	result := []rosterDeviationResponse{}
	if rs.OfferingSourceOptions == nil {
		return result, nil
	}
	ratio := rs.childrenPerStaffRatio(ctx)
	rows, err := rs.TimetableData.ListTemplateEntriesForPeriod(ctx, &periodID, ratio, nil)
	if err != nil {
		return nil, err
	}
	templates := mapTemplateRows(rows, ratio, nil)
	rs.attachRosterMaintenance(ctx, templates, &periodID)
	reviewed := reviewableTemplateIDs(templates)
	if len(reviewed) == 0 {
		return result, nil
	}
	today := rs.todayDate()
	bookings, err := rs.OfferingSourceOptions.OfferingWeekdayBookings(ctx, &periodID, today)
	if err != nil || len(bookings) == 0 {
		return result, err
	}
	rosters, err := rs.RosterReview.FutureRosters(ctx, reviewed, today)
	if err != nil {
		return nil, err
	}
	kinds := make(map[int64]timetable.RosterOfferingKind, len(templates))
	for _, template := range templates {
		kinds[template.ID] = timetable.RosterOfferingKindFor(template.Type, template.ListKind)
	}
	for _, templateID := range reviewed {
		roster, found := rosters[templateID]
		if !found {
			continue
		}
		roster.OfferingKind = kinds[templateID]
		if deviation := timetable.DetectRosterDeviation(roster, bookings); deviation != nil {
			result = append(result, rosterDeviationResponseFrom(templateID, deviation))
		}
	}
	return result, nil
}

// reviewableTemplateIDs keeps the Regeltermine with children whose upkeep is
// not automatic and that the school has not confirmed as hand-kept. Without
// the indicator nothing is reviewed: the hint must not contradict it.
func reviewableTemplateIDs(templates []templateResponse) []int64 {
	ids := make([]int64, 0, len(templates))
	for _, template := range templates {
		if template.Type == timetable.GroupTypeDuty || template.ManualRosterConfirmed ||
			template.RosterMaintenance == nil || template.RosterMaintenance.Mode == "automatic" {
			continue
		}
		ids = append(ids, template.ID)
	}
	return ids
}

func rosterDeviationResponseFrom(templateID int64, deviation *timetable.RosterDeviation) rosterDeviationResponse {
	gradeLevels := deviation.GradeLevels
	if gradeLevels == nil {
		gradeLevels = []int{}
	}
	schoolClasses := deviation.SchoolClasses
	if schoolClasses == nil {
		schoolClasses = []string{}
	}
	return rosterDeviationResponse{
		TemplateID:    templateID,
		Offerings:     rosterMaintenanceOfferings(deviation.Offerings),
		GradeLevels:   gradeLevels,
		SchoolClasses: schoolClasses,
		UnbookedCount: deviation.UnbookedCount,
		MissingCount:  deviation.MissingCount,
		Candidates:    rosterMaintenanceOfferings(deviation.Candidates),
		Combinations:  rosterMaintenanceCombinations(deviation.Combinations),
		Ambiguous:     deviation.Ambiguous,
	}
}

func rosterMaintenanceCombinations(combinations [][]timetable.OfferingRef) [][]templateRosterMaintenanceOfferingResponse {
	result := make([][]templateRosterMaintenanceOfferingResponse, 0, len(combinations))
	for _, combination := range combinations {
		result = append(result, rosterMaintenanceOfferings(combination))
	}
	return result
}

type offeringPreviewStudentResponse struct {
	StudentID int64  `json:"student_id"`
	Name      string `json:"name"`
}

type offeringPreviewWeekdayResponse struct {
	Weekday   int                              `json:"weekday"`
	Added     []offeringPreviewStudentResponse `json:"added"`
	Removed   []offeringPreviewStudentResponse `json:"removed"`
	KeptCount int                              `json:"kept_count"`
}

type offeringPreviewSegmentResponse struct {
	TemplateID int64   `json:"template_id"`
	ValidFrom  *string `json:"valid_from"`
	ValidUntil *string `json:"valid_until"`
}

type offeringPreviewResponse struct {
	From     string                           `json:"from"`
	Weekdays []offeringPreviewWeekdayResponse `json:"weekdays"`
	Segments []offeringPreviewSegmentResponse `json:"segments"`
}

// previewTemplateOfferingSource handles GET /templates/{id}/offering-preview:
// per weekday, who joins and who leaves from the next occurrence on when the
// offerings become the series' source.
func (rs *Resource) previewTemplateOfferingSource(w http.ResponseWriter, r *http.Request) {
	id, ok := templateIDFromRequest(w, r)
	if !ok {
		return
	}
	if rs.RosterReview == nil || rs.OfferingSourceOptions == nil || rs.People == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("timetable roster review not wired")))
		return
	}
	filter, err := offeringSourceFilterFromQuery(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, common.CodeTimetableTemplateInvalid))
		return
	}
	from, err := rs.offeringPreviewFrom(r, id)
	if err != nil {
		renderRosterReviewError(w, r, err)
		return
	}
	preview, err := rs.offeringSourcePreview(r.Context(), id, filter, from)
	if err != nil {
		renderRosterReviewError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, preview, "Offering preview retrieved")
}

// offeringPreviewFrom is the explicit from, which must not lie in the past,
// else the series' next planned occurrence, else today.
func (rs *Resource) offeringPreviewFrom(r *http.Request, templateID int64) (calendar.Date, error) {
	today := rs.todayDate()
	if raw := r.URL.Query().Get("from"); raw != "" {
		from, err := calendar.ParseDate(raw)
		if err != nil || from.Before(today) {
			return calendar.Date(""), errors.Join(timetable.ErrRosterReviewInvalid, errors.New("from must be a date from today on"))
		}
		return from, nil
	}
	next, err := rs.RosterReview.NextSeriesOccurrence(r.Context(), templateID, today)
	if err != nil {
		return calendar.Date(""), err
	}
	if next == nil {
		return today, nil
	}
	return *next, nil
}

func (rs *Resource) offeringSourcePreview(ctx context.Context, templateID int64, filter timetable.OfferingSourceFilter, from calendar.Date) (offeringPreviewResponse, error) {
	segments, err := rs.RosterReview.SeriesSegments(ctx, templateID, from)
	if err != nil {
		return offeringPreviewResponse{}, err
	}
	if len(segments) == 0 {
		return offeringPreviewResponse{}, timetable.ErrTemplateSegmentNotEditable
	}
	living := segments[len(segments)-1]
	if _, err := rs.OfferingSourceOptions.CombinedOfferingSourceCounts(ctx, filter.OfferingIDs, living.CalendarPeriodID); err != nil {
		return offeringPreviewResponse{}, err
	}
	bookings, err := rs.OfferingSourceOptions.OfferingWeekdayBookings(ctx, living.CalendarPeriodID, from)
	if err != nil {
		return offeringPreviewResponse{}, err
	}
	rosters, err := rs.RosterReview.FutureRosters(ctx, []int64{living.TemplateID}, from)
	if err != nil {
		return offeringPreviewResponse{}, err
	}
	changes := timetable.PreviewOfferingSource(rosters[living.TemplateID], filter, bookings)
	names, err := rs.previewStudentNames(ctx, changes, from)
	if err != nil {
		return offeringPreviewResponse{}, err
	}
	return offeringPreviewResponse{
		From:     from.String(),
		Weekdays: offeringPreviewWeekdays(changes, names),
		Segments: offeringPreviewSegments(segments),
	}, nil
}

// previewStudentNames resolves the names of every child the preview names in
// two reads.
func (rs *Resource) previewStudentNames(ctx context.Context, changes []timetable.OfferingSourceWeekdayChange, day calendar.Date) (map[int64]string, error) {
	studentIDs := make([]int64, 0)
	for _, change := range changes {
		studentIDs = append(studentIDs, change.Added...)
		studentIDs = append(studentIDs, change.Removed...)
	}
	slices.Sort(studentIDs)
	studentIDs = slices.Compact(studentIDs)
	names := make(map[int64]string, len(studentIDs))
	if len(studentIDs) == 0 {
		return names, nil
	}
	persons, err := rs.People.AttendingStudentPersons(ctx, studentIDs, day)
	if err != nil {
		return nil, err
	}
	personIDs := make([]int64, 0, len(persons))
	for _, personID := range persons {
		personIDs = append(personIDs, personID)
	}
	byPerson, err := rs.People.PersonNames(ctx, personIDs)
	if err != nil {
		return nil, err
	}
	for studentID, personID := range persons {
		names[studentID] = byPerson[personID]
	}
	return names, nil
}

func offeringPreviewWeekdays(changes []timetable.OfferingSourceWeekdayChange, names map[int64]string) []offeringPreviewWeekdayResponse {
	weekdays := make([]offeringPreviewWeekdayResponse, 0, len(changes))
	for _, change := range changes {
		weekdays = append(weekdays, offeringPreviewWeekdayResponse{
			Weekday:   change.Weekday,
			Added:     namedPreviewStudents(change.Added, names),
			Removed:   namedPreviewStudents(change.Removed, names),
			KeptCount: change.KeptCount,
		})
	}
	return weekdays
}

// namedPreviewStudents keeps the children who still attend, sorted by name.
func namedPreviewStudents(studentIDs []int64, names map[int64]string) []offeringPreviewStudentResponse {
	students := make([]offeringPreviewStudentResponse, 0, len(studentIDs))
	for _, studentID := range studentIDs {
		if name, found := names[studentID]; found {
			students = append(students, offeringPreviewStudentResponse{StudentID: studentID, Name: name})
		}
	}
	slices.SortFunc(students, func(left, right offeringPreviewStudentResponse) int {
		return strings.Compare(strings.ToLower(left.Name), strings.ToLower(right.Name))
	})
	return students
}

func offeringPreviewSegments(segments []timetable.TemplateSeriesSegment) []offeringPreviewSegmentResponse {
	result := make([]offeringPreviewSegmentResponse, 0, len(segments))
	for _, segment := range segments {
		result = append(result, offeringPreviewSegmentResponse{
			TemplateID: segment.TemplateID,
			ValidFrom:  optionalDateString(segment.ValidFrom),
			ValidUntil: optionalDateString(segment.ValidUntil),
		})
	}
	return result
}

func optionalDateString(date *calendar.Date) *string {
	if date == nil {
		return nil
	}
	value := date.String()
	return &value
}

// offeringSourceFilterFromQuery reads offering_ids, grade_levels and
// school_classes as comma-separated lists.
func offeringSourceFilterFromQuery(r *http.Request) (timetable.OfferingSourceFilter, error) {
	query := r.URL.Query()
	offeringIDs, err := commaSeparatedInts(query.Get("offering_ids"))
	if err != nil || len(offeringIDs) == 0 {
		return timetable.OfferingSourceFilter{}, errors.New("offering_ids must list at least one offering id")
	}
	grades, err := commaSeparatedInts(query.Get("grade_levels"))
	if err != nil {
		return timetable.OfferingSourceFilter{}, errors.New("grade_levels must list whole numbers")
	}
	filter := timetable.OfferingSourceFilter{OfferingIDs: offeringIDs}
	for _, grade := range grades {
		filter.GradeLevels = append(filter.GradeLevels, int(grade))
	}
	for _, class := range strings.Split(query.Get("school_classes"), ",") {
		if class = strings.TrimSpace(class); class != "" {
			filter.SchoolClasses = append(filter.SchoolClasses, class)
		}
	}
	if len(filter.GradeLevels) > 0 && len(filter.SchoolClasses) > 0 {
		return timetable.OfferingSourceFilter{}, errors.New("grade_levels and school_classes exclude each other")
	}
	return filter, nil
}

func commaSeparatedInts(raw string) ([]int64, error) {
	values := make([]int64, 0)
	for _, part := range strings.Split(raw, ",") {
		if part = strings.TrimSpace(part); part == "" {
			continue
		}
		value, err := strconv.ParseInt(part, 10, 64)
		if err != nil || value <= 0 {
			return nil, errors.New("invalid number")
		}
		values = append(values, value)
	}
	return values, nil
}

type offeringConnectionRequest struct {
	OfferingIDs   []int64  `json:"offering_ids"`
	GradeLevels   []int    `json:"grade_levels"`
	SchoolClasses []string `json:"school_classes"`
	EffectiveDate string   `json:"effective_date"`
}

func (*offeringConnectionRequest) Bind(*http.Request) error { return nil }

type offeringConnectionResponse struct {
	TemplateIDs   []int64 `json:"template_ids"`
	EffectiveDate string  `json:"effective_date"`
}

// connectTemplateOfferingSource handles POST
// /templates/{id}/offering-connection: every series segment from the
// effective date on takes its children from the offerings.
func (rs *Resource) connectTemplateOfferingSource(w http.ResponseWriter, r *http.Request) {
	id, ok := templateIDFromRequest(w, r)
	if !ok {
		return
	}
	if rs.RosterReview == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("timetable roster review not wired")))
		return
	}
	req := &offeringConnectionRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, bindErrorRenderer(err))
		return
	}
	effectiveDate, err := calendar.ParseDate(req.EffectiveDate)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidOnField(errors.New("effective_date must be YYYY-MM-DD"), common.CodeTimetableTemplateInvalid, "effective_date"))
		return
	}
	result, err := rs.RosterReview.ConnectOfferingSource(r.Context(), timetable.ConnectOfferingSourceCommand{
		TemplateID: id, OfferingIDs: req.OfferingIDs, GradeLevels: req.GradeLevels,
		SchoolClasses: req.SchoolClasses, EffectiveDate: effectiveDate,
	})
	if err != nil {
		renderRosterReviewError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, offeringConnectionResponse{
		TemplateIDs: result.TemplateIDs, EffectiveDate: result.EffectiveDate.String(),
	}, "Template connected to offering")
}

type manualRosterRequest struct {
	Confirmed *bool `json:"confirmed"`
}

func (req *manualRosterRequest) Bind(*http.Request) error {
	if req.Confirmed == nil {
		return errors.New("confirmed is required")
	}
	return nil
}

type manualRosterResponse struct {
	ManualRosterConfirmed bool `json:"manual_roster_confirmed"`
}

// setTemplateManualRoster handles PUT /templates/{id}/manual-roster: "Bleibt
// von Hand gepflegt" for the whole series, or its undo.
func (rs *Resource) setTemplateManualRoster(w http.ResponseWriter, r *http.Request) {
	id, ok := templateIDFromRequest(w, r)
	if !ok {
		return
	}
	if rs.RosterReview == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("timetable roster review not wired")))
		return
	}
	req := &manualRosterRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, bindErrorRenderer(err))
		return
	}
	if err := rs.RosterReview.SetManualRosterConfirmed(r.Context(), id, *req.Confirmed); err != nil {
		renderRosterReviewError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, manualRosterResponse{ManualRosterConfirmed: *req.Confirmed}, "Manual roster updated")
}

// renderRosterReviewError maps the roster review's refusals: an unknown or
// archived series is the template 404, an invalid request or offering
// selection a 400, an incompatible care offering a 409.
func renderRosterReviewError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, timetable.ErrTemplateSegmentNotEditable):
		renderTemplateNotFound(w, r)
	case errors.Is(err, timetable.ErrRosterReviewInvalid):
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, common.CodeTimetableTemplateInvalid))
	case renderTemplateCareOfferingConflict(w, r, err):
	case templateRefusalRenderer(err) != nil:
		common.RenderError(w, r, templateRefusalRenderer(err))
	default:
		common.RenderError(w, r, common.ErrorInternalServerWrap("roster review failed", err))
	}
}
