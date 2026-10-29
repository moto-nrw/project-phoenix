// Package timetablehttp — offering suggestion for a Regeltermin that is still
// being planned (#5104):
//
//	GET /api/timetable/offering-sources/suggestion?period_id=…&roster=1:4,5,6
//	GET /api/timetable/offering-sources/preview?period_id=…&offering_ids=…&roster=…
//
// The dialog sends the children it would plan per weekday (`roster`, one
// value per weekday as "W:id,id"), so nothing has to be stored yet. Both
// reads apply the rule and the preview of the roster review (#5103).
package timetablehttp

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// maxDraftRosterChildren caps the child-weekdays a dialog may send; a whole
// school on five weekdays stays below it.
const maxDraftRosterChildren = 2000

type offeringSuggestionResponse struct {
	// Offerings is the suggested source; several offerings combine (#5139).
	Offerings     []templateRosterMaintenanceOfferingResponse   `json:"offerings"`
	GradeLevels   []int                                         `json:"grade_levels"`
	SchoolClasses []string                                      `json:"school_classes"`
	UnbookedCount int                                           `json:"unbooked_count"`
	MissingCount  int                                           `json:"missing_count"`
	Candidates    []templateRosterMaintenanceOfferingResponse   `json:"candidates"`
	Combinations  [][]templateRosterMaintenanceOfferingResponse `json:"combinations"`
	// Ambiguous: other offerings fit as well as suggested ones; the school
	// picks.
	Ambiguous bool `json:"ambiguous"`
}

type offeringSuggestionEnvelope struct {
	// Suggestion is null when no offering obviously stands behind the
	// children.
	Suggestion *offeringSuggestionResponse `json:"suggestion"`
}

// suggestDraftOfferingSource handles GET /offering-sources/suggestion: the
// offerings whose bookings the planned children obviously follow, with the
// Klassen- or Jahrgangsfilter of the planned targets.
func (rs *Resource) suggestDraftOfferingSource(w http.ResponseWriter, r *http.Request) {
	if rs.OfferingSourceOptions == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("offering sources not wired")))
		return
	}
	periodID, roster, ok := draftRosterFromRequest(w, r)
	if !ok {
		return
	}
	query := r.URL.Query()
	roster.TargetSchoolClasses = commaSeparatedStrings(query.Get("school_classes"))
	if len(roster.TargetSchoolClasses) == 0 {
		grades, err := commaSeparatedInts(query.Get("grade_levels"))
		if err != nil {
			common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("grade_levels must list whole numbers")))
			return
		}
		for _, grade := range grades {
			roster.TargetGradeLevels = append(roster.TargetGradeLevels, int(grade))
		}
	}
	listKind := query.Get("list_kind")
	roster.OfferingKind = timetable.RosterOfferingKindFor(query.Get("type"), &listKind)
	bookings, err := rs.OfferingSourceOptions.OfferingWeekdayBookings(r.Context(), &periodID, rs.todayDate())
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServerWrap("offering suggestion failed", err))
		return
	}
	envelope := offeringSuggestionEnvelope{}
	if suggestion := timetable.SuggestOfferingSource(roster, bookings); suggestion != nil {
		response := rosterDeviationResponseFrom(0, suggestion).withoutTemplate()
		response.Combinations = make([][]templateRosterMaintenanceOfferingResponse, 0, len(suggestion.Combinations))
		for _, combination := range suggestion.Combinations {
			response.Combinations = append(response.Combinations, rosterMaintenanceOfferings(combination))
		}
		envelope.Suggestion = &response
	}
	common.Respond(w, r, http.StatusOK, envelope, "Offering suggestion retrieved")
}

// withoutTemplate drops the template id a planned Regeltermin does not have.
func (d rosterDeviationResponse) withoutTemplate() offeringSuggestionResponse {
	return offeringSuggestionResponse{
		Offerings: d.Offerings, GradeLevels: d.GradeLevels, SchoolClasses: d.SchoolClasses,
		UnbookedCount: d.UnbookedCount, MissingCount: d.MissingCount, Candidates: d.Candidates,
		Ambiguous: d.Ambiguous,
	}
}

// previewDraftOfferingSource handles GET /offering-sources/preview: per
// weekday, who joins and who leaves when the planned Regeltermin takes its
// children from the offerings instead.
func (rs *Resource) previewDraftOfferingSource(w http.ResponseWriter, r *http.Request) {
	if rs.OfferingSourceOptions == nil || rs.People == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("offering sources not wired")))
		return
	}
	periodID, roster, ok := draftRosterFromRequest(w, r)
	if !ok {
		return
	}
	filter, err := offeringSourceFilterFromQuery(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, common.CodeTimetableTemplateInvalid))
		return
	}
	from := rs.todayDate()
	if raw := r.URL.Query().Get("from"); raw != "" {
		if from, err = calendar.ParseDate(raw); err != nil || from.Before(rs.todayDate()) {
			renderRosterReviewError(w, r, errors.Join(timetable.ErrRosterReviewInvalid, errors.New("from must be a date from today on")))
			return
		}
	}
	preview, err := rs.draftOfferingPreview(r.Context(), periodID, roster, filter, from)
	if err != nil {
		renderRosterReviewError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, preview, "Offering preview retrieved")
}

func (rs *Resource) draftOfferingPreview(
	ctx context.Context,
	periodID int64,
	roster timetable.TemplateFutureRoster,
	filter timetable.OfferingSourceFilter,
	from calendar.Date,
) (offeringPreviewResponse, error) {
	// Validates the selection like a save would (same phase, fitting period).
	if _, err := rs.OfferingSourceOptions.CombinedOfferingSourceCounts(ctx, filter.OfferingIDs, &periodID); err != nil {
		return offeringPreviewResponse{}, err
	}
	bookings, err := rs.OfferingSourceOptions.OfferingWeekdayBookings(ctx, &periodID, from)
	if err != nil {
		return offeringPreviewResponse{}, err
	}
	changes := timetable.PreviewOfferingSource(roster, filter, bookings)
	names, err := rs.previewStudentNames(ctx, changes, from)
	if err != nil {
		return offeringPreviewResponse{}, err
	}
	return offeringPreviewResponse{
		From:     from.String(),
		Weekdays: offeringPreviewWeekdays(changes, names),
		Segments: []offeringPreviewSegmentResponse{},
	}, nil
}

// draftRosterFromRequest reads the required period and the planned children
// per weekday; it renders the 400 itself.
func draftRosterFromRequest(w http.ResponseWriter, r *http.Request) (int64, timetable.TemplateFutureRoster, bool) {
	periodID, ok := templatePeriodIDFromRequest(w, r)
	if !ok {
		return 0, timetable.TemplateFutureRoster{}, false
	}
	if periodID == nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("period_id is required")))
		return 0, timetable.TemplateFutureRoster{}, false
	}
	weekdays, err := parseDraftRoster(r.URL.Query()["roster"])
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return 0, timetable.TemplateFutureRoster{}, false
	}
	return *periodID, timetable.TemplateFutureRoster{Weekdays: weekdays}, true
}

// parseDraftRoster reads `roster` values "W:id,id" into the children per ISO
// weekday. A weekday may stay empty ("3:"); the total is capped.
func parseDraftRoster(values []string) (map[int][]int64, error) {
	weekdays := make(map[int][]int64, len(values))
	total := 0
	for _, value := range values {
		day, ids, found := strings.Cut(value, ":")
		if !found {
			return nil, errors.New("roster must look like 1:12,13")
		}
		weekday, err := strconv.Atoi(strings.TrimSpace(day))
		if err != nil || weekday < 1 || weekday > 7 {
			return nil, errors.New("roster weekday must be 1 to 7")
		}
		students, err := commaSeparatedInts(ids)
		if err != nil {
			return nil, errors.New("roster children must be positive ids")
		}
		if total += len(students); total > maxDraftRosterChildren {
			return nil, errors.New("roster names too many children")
		}
		weekdays[weekday] = append(weekdays[weekday], students...)
	}
	for weekday, students := range weekdays {
		slices.Sort(students)
		weekdays[weekday] = slices.Compact(students)
	}
	return weekdays, nil
}

func commaSeparatedStrings(raw string) []string {
	values := make([]string, 0)
	for _, part := range strings.Split(raw, ",") {
		if part = strings.TrimSpace(part); part != "" {
			values = append(values, part)
		}
	}
	return values
}
