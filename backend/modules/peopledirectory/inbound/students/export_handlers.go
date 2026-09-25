package students

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/documentrendering/lists"
)

// studentExportPageSize caps how many child rows a single export document may
// carry. The cap is applied to the FINAL, filtered result (see exportStudents),
// not to the raw school size — a narrow birthday or search list still exports at
// a large school. It also bounds the initial fetch page for callers that still
// paginate.
const studentExportPageSize = 5000

type studentExportRequest struct {
	Format  lists.Format         `json:"format"`
	Preset  lists.Preset         `json:"preset"`
	Title   string               `json:"title"`
	Filters studentExportFilters `json:"filters"`
	Columns []lists.ColumnID     `json:"columns"`
}

type studentExportFilters struct {
	Search       string `json:"search"`
	GroupID      string `json:"group_id"`
	RoomID       string `json:"room_id"`
	Year         string `json:"year"`
	SchoolClass  string `json:"school_class"`
	Status       string `json:"status"`
	Bus          string `json:"bus"`
	PhotoConsent string `json:"photo_consent"`
	PickupStatus string `json:"pickup_status"`
	DayStatus    string `json:"day_status"`
	// Date is the optional planning day (YYYY-MM-DD) the day-planning status,
	// status days, and planned arrival/pickup times are evaluated for (#1939).
	// Empty means the school-local today.
	Date string `json:"date"`
	// PickupTime accepts comma-separated selections as well as legacy single values.
	PickupTime   string `json:"pickup_time"`
	ArrivalTime  string `json:"arrival_time"`
	Sort         string `json:"sort"`
	GroupByClass bool   `json:"group_by_class"`
	// Months restricts a birthday list to the given birth months ("01".."12").
	// Empty means every month. A birthday recurs annually, so this matches on
	// month alone and never on the birth year.
	Months []string `json:"months"`
	// IncludeWithoutHealthInfo keeps children without a stored health note
	// on the Gesundheitsliste (#3323); they print "Nicht hinterlegt". Other
	// presets ignore it.
	IncludeWithoutHealthInfo bool `json:"include_without_health_info"`
}

type weeklySchedule struct {
	ArrivalByWeekday  map[int]string
	CareDaysByWeekday map[int]bool
	PickupByWeekday   map[int]string
}

func (rs *Resource) exportStudents(w http.ResponseWriter, r *http.Request) {
	if rs.ListExportService == nil {
		renderError(w, r, common.ErrorInternalServer(errors.New("list export service is not configured")))
		return
	}

	req, err := decodeStudentExportRequest(r)
	if err != nil {
		renderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	selection, errResp := rs.selectExportResponses(r, req)
	if errResp != nil {
		renderError(w, r, errResp)
		return
	}
	sources, columns, errResp := rs.buildExportSources(r, req, selection)
	if errResp != nil {
		renderError(w, r, errResp)
		return
	}
	planningDate, isToday := selection.planningDate, selection.isToday
	rows := buildExportRowSources(sources, req.Filters.GroupByClass)
	doc := lists.Document{
		Title:       exportTitle(req),
		Subtitle:    rs.exportSubtitle(r, len(sources)),
		GeneratedAt: time.Now(),
		Filters:     exportDocumentFilterLabels(req, planningDate, isToday),
		Columns:     columns,
		Rows:        rows,
	}

	file, err := rs.ListExportService.Render(doc, req.Format, doc.Title)
	if err != nil {
		renderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	w.Header().Set("Content-Type", file.ContentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, file.Filename))
	w.Header().Set("Content-Length", strconv.Itoa(len(file.Data)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(file.Data)
}

// enrichExportCompanions loads the structured "mit wem" names, but only for an
// export that actually renders them.
//
// Only the departure column reads DepartureCompanions, so any other column would
// pay for the links of every exported child, their far-end students and the
// authorization behind them and then throw the result away. At the export cap of
// 5.000 rows that is the most expensive lookup in this handler, so it is skipped
// outright rather than merely tolerated when it fails.
//
// Which is also why the error is fatal to the caller: it now only runs where a
// missing name makes the document wrong rather than merely less detailed — see
// enrichWithCompanionLinks.
func (rs *Resource) enrichExportCompanions(r *http.Request, responses []StudentResponse, columns []lists.Column, accessCtx *studentAccessContext) error {
	if !exportHasColumn(columns, lists.ColumnDeparture) {
		return nil
	}
	return rs.enrichWithCompanionLinks(r.Context(), responses, accessCtx)
}

// resolveExportPlanningDate resolves the export's planning day and rejects the
// live presence filters that cannot be answered for any day but today. Both
// checks run before the fetch so a dated export never reaches the live-state
// query path (#1939).
func resolveExportPlanningDate(filters studentExportFilters, now time.Time) (timezone.Date, bool, render.Renderer) {
	planningDate, isToday, dateErr := resolvePlanningDate(filters.Date, now)
	if dateErr != nil {
		return timezone.Date(""), false, common.ErrorInvalidRequest(dateErr)
	}
	if err := liveFilterError(activeLiveExportFilters(filters), planningDate, isToday); err != nil {
		return timezone.Date(""), false, common.ErrorInvalidRequest(err)
	}
	return planningDate, isToday, nil
}

// prepareDatedExportResponses layers the date-scoped view onto the already-built
// responses: today keeps live check-in/out times, any other day starts from the
// row's clean state and carries only that day's status days, plans, and
// effective arrival/pickup times. It returns a renderer when a lookup fails.
func (rs *Resource) prepareDatedExportResponses(r *http.Request, responses []StudentResponse, dataSnapshot *studentDataSnapshot, planningDate timezone.Date, isToday bool) render.Renderer {
	// Actual check-in/out times and the row-seeded Sick/Excused flags describe
	// today; a non-today planning export starts clean and only carries the
	// requested date's status days and plans.
	if isToday {
		applyFullAccessActualTimes(responses, dataSnapshot)
	} else {
		resetScheduledStatusFlags(responses)
		// The live-location snapshot describes today; strip it so a document
		// labelled for another day cannot leak the child's current whereabouts
		// through the current-location column or the momentary-status filter (#1939).
		resetLiveLocationFields(responses)
	}
	if err := rs.applyStatusDaysForDate(r.Context(), responses, planningDate.BerlinMidnight()); err != nil {
		return common.ErrorInternalServer(err)
	}
	planningTimes, err := rs.enrichWithDayPlanning(r.Context(), responses, planningDate, isToday, attendanceMapFromSnapshot(dataSnapshot))
	if err != nil {
		return common.ErrorInternalServer(err)
	}
	applyPickupTimesFromMap(responses, planningTimes.pickups)
	applyArrivalTimesFromMap(responses, planningTimes.arrivals)
	return nil
}

func decodeStudentExportRequest(r *http.Request) (studentExportRequest, error) {
	var req studentExportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return req, err
	}
	if req.Format == "" {
		req.Format = lists.FormatPDF
	}
	if req.Preset == "" {
		req.Preset = lists.PresetOGSWeekly
	}
	switch req.Format {
	case lists.FormatPDF, lists.FormatDOCX, lists.FormatXLSX:
	default:
		return req, fmt.Errorf("unsupported export format %q", req.Format)
	}
	if _, err := parseExportMonths(req.Filters.Months); err != nil {
		return req, err
	}
	return req, nil
}

// parseExportMonths turns the wire month filter ("01".."12") into a lookup set.
// An empty list means "every month" and yields a nil set. Unknown values are
// rejected rather than skipped: silently dropping a month would render a list
// that looks complete but quietly covers the wrong period.
func parseExportMonths(values []string) (map[time.Month]bool, error) {
	if len(values) == 0 {
		return nil, nil
	}
	months := make(map[time.Month]bool, len(values))
	for _, value := range values {
		number, err := strconv.Atoi(strings.TrimSpace(value))
		if err != nil || number < 1 || number > 12 {
			return nil, fmt.Errorf("invalid birthday month %q, expected \"01\" to \"12\"", value)
		}
		months[time.Month(number)] = true
	}
	return months, nil
}

// fetchStudentsForExport loads every student matching the SQL-level filters for
// an export. It returns a ready-to-render error rather than writing the response
// so the handler keeps a single error branch. params.fetchAll is set, so the
// query is unpaginated: the in-memory birthday-month and search filters that run
// afterward see the whole set, and the size cap is enforced on the FINAL,
// filtered result in exportStudents — not here on the raw school size.
func (rs *Resource) fetchStudentsForExport(r *http.Request, params *studentListParams) ([]*Student, render.Renderer) {
	students, _, err := rs.fetchStudentsForList(r, params)
	if err != nil {
		return nil, common.ErrorInternalServer(err)
	}
	return students, nil
}

// exportSelectionCapError returns a ready-to-render error when a filtered export
// exceeds what one document can carry, or nil when it fits. It is checked
// against the rows that actually land in the file, after every requested filter
// has run: a narrow birthday or search list still exports at a large school, and
// only a genuinely oversized result is refused. Because the fetch is complete
// (params.fetchAll), refusing here is never silent truncation.
func exportSelectionCapError(count int) render.Renderer {
	if exportSelectionTooLarge(count) {
		return common.ErrorInvalidRequest(errExportSelectionTooLarge(count))
	}
	return nil
}

// exportSelectionTooLarge reports whether a filtered export exceeds what a single
// document can carry (studentExportPageSize rows).
func exportSelectionTooLarge(total int) bool {
	return total > studentExportPageSize
}

// errExportSelectionTooLarge is the user-facing message shown when the selection
// is over the cap. Lowercase and unpunctuated to satisfy Go error-string linting
// while still reading as a full sentence in the frontend toast.
func errExportSelectionTooLarge(total int) error {
	return fmt.Errorf("die Auswahl umfasst %d Kinder, ein Export ist auf höchstens %d Kinder begrenzt, bitte die Auswahl eingrenzen (etwa nach Gruppe oder Klasse)", total, studentExportPageSize)
}

func exportRequestToListParams(req studentExportRequest, today timezone.Date) *studentListParams {
	params := &studentListParams{
		search:              strings.TrimSpace(req.Filters.Search),
		page:                1,
		pageSize:            studentExportPageSize,
		includePickupTimes:  true,
		includeArrivalTimes: true,
		dayStatus:           parseDayStatusParam(req.Filters.DayStatus),
		careStatus:          CareStatusRunning,
		careStatusOn:        today,
		careStatusToday:     today,
		// Class and group travel comma-separated so an export mirrors a
		// multi-selection made in the Kindersuche (#2218).
		schoolClasses: parseMultiValueParam([]string{req.Filters.SchoolClass}),
		// The birthday-month and search filters run in memory after the fetch,
		// so pull every SQL-matching row: a paginated page would drop matching
		// children past the boundary and silently shorten the list.
		fetchAll: true,
	}
	params.groupIDs = parseGroupIDList([]string{req.Filters.GroupID})
	if req.Filters.RoomID != "" {
		if roomID, err := strconv.ParseInt(req.Filters.RoomID, 10, 64); err == nil {
			params.roomID = roomID
		}
	}
	return params
}

// applyFullAccessActualTimes overlays the snapshot's actual arrival/pickup times
// onto every response the caller has full access to; redacted rows are left
// untouched so GDPR-scoped exports never carry times the caller may not see.
func applyFullAccessActualTimes(responses []StudentResponse, dataSnapshot *studentDataSnapshot) {
	for i := range responses {
		if responses[i].HasFullAccess {
			applyActualTimesFromSnapshot(&responses[i], dataSnapshot)
		}
	}
}

func exportNeedsPhotoConsentFilter(filters studentExportFilters) bool {
	return filters.PhotoConsent != "" && filters.PhotoConsent != "all"
}

func populateExportPhotoConsentFilterData(responses []StudentResponse, students []*Student) {
	consentByStudentID := make(map[int64]bool, len(students))
	for _, student := range students {
		if student == nil {
			continue
		}
		consentByStudentID[student.ID] = student.PhotoConsentGivenAt != nil
	}
	for i := range responses {
		consentGiven, ok := consentByStudentID[responses[i].ID]
		if !ok {
			continue
		}
		responses[i].PhotoConsentGiven = &consentGiven
	}
}

// birthdayExportMatch reports whether a child belongs on a birthday-filtered
// export. Children without a parseable birthday never match: a birthday list
// carrying rows with an empty date is noise, not data. An empty month set
// accepts every month.
func birthdayExportMatch(student StudentResponse, months map[time.Month]bool) bool {
	birthday, err := timezone.ParseDate(student.Birthday)
	if err != nil {
		return false
	}
	return len(months) == 0 || months[birthday.Month()]
}

// matchesTimeFilter reports whether a child's planned arrival/pickup time
// satisfies the requested filter. "" and "all" accept everyone; "none" keeps
// only children with no planned time and no exception for today; any other
// value is matched literally against the HH:MM time.
func matchesTimeFilter(planned *string, isException bool, filter string) bool {
	if filter == "" || filter == "all" {
		return true
	}
	if filter == "none" {
		return planned == nil && !isException
	}
	return planned != nil && *planned == filter
}

// matchesPickupTimeFilter applies OR within the pickup selection. Redacted
// times are unknown, not missing, so they cannot match a restricted selection.
func matchesPickupTimeFilter(student StudentResponse, raw string) bool {
	times := slices.DeleteFunc(parseMultiValueParam([]string{raw}), func(value string) bool {
		return value == "all"
	})
	if len(times) == 0 {
		return true
	}
	if !student.HasFullAccess {
		return false
	}
	return slices.ContainsFunc(times, func(value string) bool {
		return matchesTimeFilter(student.PickupTime, student.PickupIsException, value)
	})
}

// exportYearFilterValues resolves the school-year ("Stufe") export filter into
// the set of years an export is restricted to. Several years may be selected at
// once (#2218) and travel comma-separated; empty and the neutral "all" sentinel
// both mean "no restriction".
func exportYearFilterValues(raw string) []string {
	if !isActiveFilterValue(strings.TrimSpace(raw)) {
		return nil
	}
	return parseMultiValueParam([]string{raw})
}

// matchesExportYearFilter reports whether a child's class falls into any of the
// selected school years.
func matchesExportYearFilter(schoolClass, raw string) bool {
	years := exportYearFilterValues(raw)
	if len(years) == 0 {
		return true
	}
	return slices.Contains(years, schoolYear(schoolClass))
}

func applyExportFilters(students []StudentResponse, filters studentExportFilters, preset lists.Preset, planningDate timezone.Date) []StudentResponse {
	// Months were validated when the request was decoded.
	months, _ := parseExportMonths(filters.Months)
	// The birthday preset demands a birthday even without a month filter, so a
	// child with no stored date is dropped rather than printed as a blank row.
	byBirthday := preset == lists.PresetBirthdayList || len(months) > 0
	withHealthInfoOnly := preset == lists.PresetHealthList && !filters.IncludeWithoutHealthInfo
	filtered := make([]StudentResponse, 0, len(students))
	for _, student := range students {
		if withHealthInfoOnly && !hasHealthInfo(student) {
			continue
		}
		if exportStudentMatchesFilters(student, filters, byBirthday, months, planningDate) {
			filtered = append(filtered, student)
		}
	}
	return filtered
}

// exportStudentMatchesFilters reports whether one child survives every requested
// export filter. byBirthday and months are precomputed by applyExportFilters.
func exportStudentMatchesFilters(student StudentResponse, filters studentExportFilters, byBirthday bool, months map[time.Month]bool, planningDate timezone.Date) bool {
	if byBirthday && !birthdayExportMatch(student, months) {
		return false
	}
	if !matchesExportYearFilter(student.SchoolClass, filters.Year) {
		return false
	}
	if filters.Status != "" && filters.Status != "all" && exportStatus(student) != filters.Status {
		return false
	}
	if !matchesAdministrativeFilters(student, filters.Bus, filters.PhotoConsent, filters.PickupStatus, planningDate) {
		return false
	}
	if filters.DayStatus != "" && filters.DayStatus != DayPlanningStatusAll && student.DayPlanningStatus != filters.DayStatus {
		return false
	}
	if !matchesPickupTimeFilter(student, filters.PickupTime) {
		return false
	}
	if !matchesTimeFilter(student.ArrivalTime, student.ArrivalIsException, filters.ArrivalTime) {
		return false
	}
	return true
}

// exportSelection is the export's children after every requested filter,
// sorted as the document lists them, and the day the list describes.
type exportSelection struct {
	responses    []StudentResponse
	accessCtx    *studentAccessContext
	planningDate timezone.Date
	isToday      bool
}

// selectExportResponses fetches the export's children, builds their responses
// for the planning date, filters them and sorts them.
func (rs *Resource) selectExportResponses(r *http.Request, req studentExportRequest) (exportSelection, render.Renderer) {
	// Resolved before the fetch for the same reason as in listStudents: the
	// room pre-filter reads today's live active.visits state (#1939).
	now := rs.Now()
	planningDate, isToday, errResp := resolveExportPlanningDate(req.Filters, now)
	if errResp != nil {
		return exportSelection{}, errResp
	}

	params := exportRequestToListParams(req, timezone.DateFromTime(now))
	params.careStatusOn = planningDate
	students, errResp := rs.fetchStudentsForExport(r, params)
	if errResp != nil {
		return exportSelection{}, errResp
	}

	dataSnapshot, groups, err := rs.loadStudentListData(r.Context(), students)
	if err != nil {
		return exportSelection{}, common.ErrorInternalServer(err)
	}

	accessCtx := rs.determineStudentAccess(r)
	responses := rs.buildStudentResponses(r.Context(), students, params, accessCtx, dataSnapshot, groups, false)
	if exportNeedsPhotoConsentFilter(req.Filters) {
		populateExportPhotoConsentFilterData(responses, students)
	}

	if errResp := rs.prepareDatedExportResponses(r, responses, dataSnapshot, planningDate, isToday); errResp != nil {
		return exportSelection{}, errResp
	}

	responses = applyExportFilters(responses, req.Filters, req.Preset, planningDate)
	// The cap is applied to the rows that actually land in the document, after
	// every requested filter has run — so a narrow list still exports at a large
	// school and only a genuinely oversized result is refused.
	if errResp := exportSelectionCapError(len(responses)); errResp != nil {
		return exportSelection{}, errResp
	}
	sortExportResponses(responses, exportSortMode(req))
	if req.Filters.GroupByClass {
		groupExportResponsesByClass(responses)
	}
	return exportSelection{responses: responses, accessCtx: accessCtx, planningDate: planningDate, isToday: isToday}, nil
}

// buildExportSources loads what the requested columns render and merges the
// class-list-only entries into the document's rows.
func (rs *Resource) buildExportSources(r *http.Request, req studentExportRequest, selection exportSelection) ([]exportRowSource, []lists.Column, render.Renderer) {
	responses, planningDate, isToday := selection.responses, selection.planningDate, selection.isToday
	weekly, err := rs.loadWeeklySchedules(r, collectResponseIDs(responses), planningDate)
	if err != nil {
		return nil, nil, common.ErrorInternalServer(err)
	}
	columns := lists.ResolveColumns(req.Columns, req.Preset)
	if err := rs.enrichExportCompanions(r, responses, columns, selection.accessCtx); err != nil {
		return nil, nil, common.ErrorInternalServer(err)
	}
	enrollmentSummaries, err := rs.loadActiveEnrollmentSummaries(r, collectResponseIDs(responses), planningDate, columns)
	if err != nil {
		return nil, nil, common.ErrorInternalServer(err)
	}

	sources := responseRowSources(responses, weekly, enrollmentSummaries, planningDate, isToday)
	// Class-list-only entries (#2382) complete the Klassenverband of the
	// "Klassenliste" preset; filters on properties they don't have exclude
	// them (classListEntryExportEligible).
	sources, err = rs.mergeClassListEntrySources(r, req, sources)
	if err != nil {
		return nil, nil, common.ErrorInternalServer(err)
	}
	// Re-check the cap on the FINAL merged source set: the class-list entries
	// joined after the student-side check above, and the document limit is a
	// limit on rows in the file, not on students alone. The Gesundheitsliste
	// also fills and audits its health column here (finalizeExportSources).
	if errResp := rs.finalizeExportSources(r, req, selection.responses, sources); errResp != nil {
		return nil, nil, errResp
	}
	return sources, columns, nil
}
