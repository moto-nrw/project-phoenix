package students

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/internal/collation"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activitiesModels "github.com/moto-nrw/project-phoenix/models/activities"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/services/listexport"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// studentExportPageSize caps how many child rows a single export document may
// carry. The cap is applied to the FINAL, filtered result (see exportStudents),
// not to the raw school size — a narrow birthday or search list still exports at
// a large school. It also bounds the initial fetch page for callers that still
// paginate.
const studentExportPageSize = 5000

type studentExportRequest struct {
	Format  listexport.Format     `json:"format"`
	Preset  listexport.Preset     `json:"preset"`
	Title   string                `json:"title"`
	Filters studentExportFilters  `json:"filters"`
	Columns []listexport.ColumnID `json:"columns"`
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
	Date         string `json:"date"`
	PickupTime   string `json:"pickup_time"`
	ArrivalTime  string `json:"arrival_time"`
	Sort         string `json:"sort"`
	GroupByClass bool   `json:"group_by_class"`
	// Months restricts a birthday list to the given birth months ("01".."12").
	// Empty means every month. A birthday recurs annually, so this matches on
	// month alone and never on the birth year.
	Months []string `json:"months"`
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

	// Resolved before the fetch for the same reason as in listStudents: the
	// room pre-filter reads today's live active.visits state (#1939).
	now := rs.Now()
	planningDate, isToday, errResp := resolveExportPlanningDate(req.Filters, now)
	if errResp != nil {
		renderError(w, r, errResp)
		return
	}

	params := exportRequestToListParams(req, timezone.DateFromTime(now))
	params.careStatusOn = planningDate
	students, errResp := rs.fetchStudentsForExport(r, params)
	if errResp != nil {
		renderError(w, r, errResp)
		return
	}

	studentIDs, personIDs, groupIDs := collectIDsFromStudents(students)
	dataSnapshot := common.LoadStudentDataSnapshot(r.Context(), rs.PersonService, rs.EducationService, rs.ActiveService, studentIDs, personIDs, groupIDs)

	accessCtx := rs.determineStudentAccess(r)
	responses := rs.buildStudentResponses(r.Context(), students, params, accessCtx, dataSnapshot, false)
	if exportNeedsPhotoConsentFilter(req.Filters) {
		populateExportPhotoConsentFilterData(responses, students)
	}

	if errResp := rs.prepareDatedExportResponses(r, responses, dataSnapshot, planningDate, isToday); errResp != nil {
		renderError(w, r, errResp)
		return
	}

	responses = applyExportFilters(responses, req.Filters, req.Preset, planningDate)
	// The cap is applied to the rows that actually land in the document, after
	// every requested filter has run — so a narrow list still exports at a large
	// school and only a genuinely oversized result is refused.
	if errResp := exportSelectionCapError(len(responses)); errResp != nil {
		renderError(w, r, errResp)
		return
	}
	sortExportResponses(responses, exportSortMode(req))
	if req.Filters.GroupByClass {
		groupExportResponsesByClass(responses)
	}

	weekly, err := rs.loadWeeklySchedules(r, collectResponseIDs(responses), planningDate)
	if err != nil {
		renderError(w, r, common.ErrorInternalServer(err))
		return
	}
	columns := listexport.ResolveColumns(req.Columns, req.Preset)
	if err := rs.enrichExportCompanions(r, responses, columns, accessCtx); err != nil {
		renderError(w, r, common.ErrorInternalServer(err))
		return
	}
	enrollmentSummaries, err := rs.loadActiveEnrollmentSummaries(r, collectResponseIDs(responses), planningDate, columns)
	if err != nil {
		renderError(w, r, common.ErrorInternalServer(err))
		return
	}

	sources := responseRowSources(responses, weekly, enrollmentSummaries, planningDate, isToday)
	// Class-list-only entries (#2382) complete the Klassenverband of the
	// "Klassenliste" preset; filters on properties they don't have exclude
	// them (classListEntryExportEligible).
	sources, err = rs.mergeClassListEntrySources(r, req, sources)
	if err != nil {
		renderError(w, r, common.ErrorInternalServer(err))
		return
	}
	// Re-check the cap on the FINAL merged source set: the class-list entries
	// joined after the student-side check above, and the document limit is a
	// limit on rows in the file, not on students alone.
	if errResp := exportSelectionCapError(len(sources)); errResp != nil {
		renderError(w, r, errResp)
		return
	}
	rows := buildExportRowSources(sources, req.Filters.GroupByClass)
	doc := listexport.Document{
		Title:       exportTitle(req),
		Subtitle:    rs.exportSubtitle(r, len(sources)),
		GeneratedAt: time.Now(),
		Filters:     exportFilterLabelsForDate(req.Filters, planningDate, isToday),
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
func (rs *Resource) enrichExportCompanions(r *http.Request, responses []StudentResponse, columns []listexport.Column, accessCtx *studentAccessContext) error {
	if !exportHasColumn(columns, listexport.ColumnDeparture) {
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
		return timezone.Date{}, false, common.ErrorInvalidRequest(dateErr)
	}
	if err := liveFilterError(activeLiveExportFilters(filters), planningDate, isToday); err != nil {
		return timezone.Date{}, false, common.ErrorInvalidRequest(err)
	}
	return planningDate, isToday, nil
}

// prepareDatedExportResponses layers the date-scoped view onto the already-built
// responses: today keeps live check-in/out times, any other day starts from the
// row's clean state and carries only that day's status days, plans, and
// effective arrival/pickup times. It returns a renderer when a lookup fails.
func (rs *Resource) prepareDatedExportResponses(r *http.Request, responses []StudentResponse, dataSnapshot *common.StudentDataSnapshot, planningDate timezone.Date, isToday bool) render.Renderer {
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
		req.Format = listexport.FormatPDF
	}
	if req.Preset == "" {
		req.Preset = listexport.PresetOGSWeekly
	}
	switch req.Format {
	case listexport.FormatPDF, listexport.FormatDOCX, listexport.FormatXLSX:
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
func (rs *Resource) fetchStudentsForExport(r *http.Request, params *studentListParams) ([]*users.Student, render.Renderer) {
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
func applyFullAccessActualTimes(responses []StudentResponse, dataSnapshot *common.StudentDataSnapshot) {
	for i := range responses {
		if responses[i].HasFullAccess {
			applyActualTimesFromSnapshot(&responses[i], dataSnapshot)
		}
	}
}

func exportNeedsPhotoConsentFilter(filters studentExportFilters) bool {
	return filters.PhotoConsent != "" && filters.PhotoConsent != "all"
}

func populateExportPhotoConsentFilterData(responses []StudentResponse, students []*users.Student) {
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
	return len(months) == 0 || months[birthday.Month]
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

func applyExportFilters(students []StudentResponse, filters studentExportFilters, preset listexport.Preset, planningDate timezone.Date) []StudentResponse {
	// Months were validated when the request was decoded.
	months, _ := parseExportMonths(filters.Months)
	// The birthday preset demands a birthday even without a month filter, so a
	// child with no stored date is dropped rather than printed as a blank row.
	byBirthday := preset == listexport.PresetBirthdayList || len(months) > 0
	filtered := make([]StudentResponse, 0, len(students))
	for _, student := range students {
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
	if !matchesTimeFilter(student.PickupTime, student.PickupIsException, filters.PickupTime) {
		return false
	}
	if !matchesTimeFilter(student.ArrivalTime, student.ArrivalIsException, filters.ArrivalTime) {
		return false
	}
	return true
}

// exportSortMode resolves the ordering a request asks for. Calendar order is
// what makes a birthday list a birthday list, so the preset implies it rather
// than depending on a caller to pass the matching sort; an explicit sort still
// wins for the rare pickup/arrival view of the same list.
func exportSortMode(req studentExportRequest) string {
	if req.Filters.Sort == "" && req.Preset == listexport.PresetBirthdayList {
		return "birthday"
	}
	return req.Filters.Sort
}

func sortExportResponses(students []StudentResponse, sortMode string) {
	sort.SliceStable(students, func(i, j int) bool {
		a := students[i]
		b := students[j]
		if sortMode == "pickup" {
			return timeValue(a.PickupTime) < timeValue(b.PickupTime)
		}
		if sortMode == "arrival" {
			return timeValue(a.ArrivalTime) < timeValue(b.ArrivalTime)
		}
		// Same-day children fall through to name collation below.
		if sortMode == "birthday" {
			if ka, kb := birthdaySortKey(a.Birthday), birthdaySortKey(b.Birthday); ka != kb {
				return ka < kb
			}
		}
		return collation.CompareGermanNames(a.LastName, a.FirstName, b.LastName, b.FirstName) < 0
	})
}

// birthdaySortKey orders by the annually recurring day ("MM-DD") rather than
// the birth year, so a birthday list reads as a calendar instead of an age
// ranking. Children without a birthday sort last.
func birthdaySortKey(birthday string) string {
	date, err := timezone.ParseDate(birthday)
	if err != nil {
		return "99-99"
	}
	return fmt.Sprintf("%02d-%02d", int(date.Month), date.Day)
}

func (rs *Resource) loadWeeklySchedules(r *http.Request, studentIDs []int64, planningDate timezone.Date) (map[int64]weeklySchedule, error) {
	result := make(map[int64]weeklySchedule, len(studentIDs))
	if len(studentIDs) == 0 {
		return result, nil
	}
	if rs.ArrivalScheduleService == nil || rs.PickupScheduleService == nil {
		return nil, errors.New("student schedule repositories are not configured")
	}
	for _, studentID := range studentIDs {
		result[studentID] = weeklySchedule{
			ArrivalByWeekday:  make(map[int]string),
			CareDaysByWeekday: make(map[int]bool),
			PickupByWeekday:   make(map[int]string),
		}
	}
	pickups, err := rs.PickupScheduleService.GetWeeklySchedulesByStudentIDsForDate(r.Context(), studentIDs, planningDate)
	if err != nil {
		return nil, err
	}
	for _, pickup := range pickups {
		weekly := result[pickup.StudentID]
		weekly.PickupByWeekday[pickup.Weekday] = formatWallClock(pickup.PickupTime)
		result[pickup.StudentID] = weekly
	}
	arrivals, err := rs.ArrivalScheduleService.GetWeeklySchedulesByStudentIDsForDate(r.Context(), studentIDs, planningDate)
	if err != nil {
		return nil, err
	}
	for _, arrival := range arrivals {
		weekly := result[arrival.StudentID]
		weekly.CareDaysByWeekday[arrival.Weekday] = true
		if !arrival.ExpectedArrival.IsZero() {
			weekly.ArrivalByWeekday[arrival.Weekday] = formatWallClock(arrival.ExpectedArrival)
		}
		result[arrival.StudentID] = weekly
	}
	return result, nil
}

func (rs *Resource) loadActiveEnrollmentSummaries(r *http.Request, studentIDs []int64, onDate timezone.Date, columns []listexport.Column) (map[int64]string, error) {
	if !columnsContain(columns, listexport.ColumnEnrollmentSummary) {
		return map[int64]string{}, nil
	}
	if rs.ActivityService == nil {
		return nil, errors.New("activity service is not configured")
	}
	groupsByStudent, err := rs.ActivityService.GetActiveStudentEnrollmentsByStudentIDs(r.Context(), studentIDs, onDate)
	if err != nil {
		return nil, err
	}
	summaries := make(map[int64]string, len(studentIDs))
	for _, studentID := range studentIDs {
		summaries[studentID] = enrollmentSummaryLabel(groupsByStudent[studentID])
	}
	return summaries, nil
}

// enrollmentSummaryLabel renders the export's "angemeldet" cell for one child:
// the deduplicated, alphabetically sorted list of active activity-group names,
// or "Keine Anmeldung" when the child has no active enrollment.
func enrollmentSummaryLabel(groups []*activitiesModels.Group) string {
	if len(groups) == 0 {
		return "Keine Anmeldung"
	}
	names := make([]string, 0, len(groups))
	seen := make(map[string]bool, len(groups))
	for _, group := range groups {
		if group == nil {
			continue
		}
		name := strings.TrimSpace(group.Name)
		if name == "" {
			name = "Gruppe #" + strconv.FormatInt(group.ID, 10)
		}
		if seen[name] {
			continue
		}
		seen[name] = true
		names = append(names, name)
	}
	sort.Strings(names)
	if len(names) == 0 {
		return "Keine Anmeldung"
	}
	return "Angemeldet: " + strings.Join(names, ", ")
}

func columnsContain(columns []listexport.Column, id listexport.ColumnID) bool {
	for _, column := range columns {
		if column.ID == id {
			return true
		}
	}
	return false
}

// groupExportResponsesByClass stably re-sorts by class only, preserving the
// prior within-class ordering (name collation or pickup/arrival sort mode).
func groupExportResponsesByClass(students []StudentResponse) {
	sort.SliceStable(students, func(i, j int) bool {
		return collation.CompareSchoolClasses(students[i].SchoolClass, students[j].SchoolClass) < 0
	})
}

// responseRowSources renders every student response into its future document
// row plus the sort keys the class-list-entry merge (#2382) needs. The order
// of `students` (whatever sort mode produced it) is preserved.
func responseRowSources(students []StudentResponse, weekly map[int64]weeklySchedule, enrollmentSummaries map[int64]string, onDate timezone.Date, isToday bool) []exportRowSource {
	sources := make([]exportRowSource, 0, len(students))
	for _, student := range students {
		sources = append(sources, exportRowSource{
			schoolClass: student.SchoolClass,
			lastName:    student.LastName,
			firstName:   student.FirstName,
			row:         buildExportRow(student, weekly[student.ID], enrollmentSummaries, onDate, isToday),
		})
	}
	return sources
}

// birthdayExportCell renders the birth date German-style ("02.09.2018").
// Children without a stored birthday render empty rather than a fake date.
func birthdayExportCell(birthday string) string {
	date, err := timezone.ParseDate(birthday)
	if err != nil {
		return ""
	}
	return date.Format("02.01.2006")
}

// ageExportCell renders the age in completed years as of onDate. A birthday
// later this year has not happened yet, so that year is not counted.
func ageExportCell(birthday string, onDate timezone.Date) string {
	date, err := timezone.ParseDate(birthday)
	if err != nil {
		return ""
	}
	years := onDate.Year - date.Year
	if onDate.Month < date.Month || (onDate.Month == date.Month && onDate.Day < date.Day) {
		years--
	}
	if years < 0 {
		return ""
	}
	return strconv.Itoa(years)
}

// buildExportRow renders one child into the generic list document.
//
// It deliberately carries NO health note: whether a child's allergies land on
// paper is decided by operations.emergency_list_health_info, and that switch is
// asked once, by the Notfallliste (#2609). The generic export never resolves
// ColumnHealthInfo (see listexport.ColumnCatalog), so a value here could only
// reach a document past that switch.
func buildExportRow(student StudentResponse, plan weeklySchedule, enrollmentSummaries map[int64]string, onDate timezone.Date, isToday bool) listexport.Row {
	return listexport.Row{Values: map[listexport.ColumnID]string{
		listexport.ColumnName:              strings.TrimSpace(student.FirstName + " " + student.LastName),
		listexport.ColumnSchoolClass:       student.SchoolClass,
		listexport.ColumnGroup:             student.GroupName,
		listexport.ColumnEnrollmentSummary: enrollmentSummaries[student.ID],
		listexport.ColumnCareDays:          careDays(plan),
		listexport.ColumnWeeklyMonday:      weeklyCell(plan, schedule.WeekdayMonday),
		listexport.ColumnWeeklyTuesday:     weeklyCell(plan, schedule.WeekdayTuesday),
		listexport.ColumnWeeklyWednesday:   weeklyCell(plan, schedule.WeekdayWednesday),
		listexport.ColumnWeeklyThursday:    weeklyCell(plan, schedule.WeekdayThursday),
		listexport.ColumnWeeklyFriday:      weeklyCell(plan, schedule.WeekdayFriday),
		listexport.ColumnDailyStatus:       dailyStatusExportCell(student, isToday),
		listexport.ColumnPlannedArrival:    base.Deref(student.ArrivalTime),
		listexport.ColumnPlannedPickup:     base.Deref(student.PickupTime),
		listexport.ColumnDeparture:         departureExportCell(student),
		listexport.ColumnDailyNotes:        dailyNotes(student),
		listexport.ColumnCurrentLocation:   student.Location,
		listexport.ColumnBirthday:          birthdayExportCell(student.Birthday),
		listexport.ColumnAge:               ageExportCell(student.Birthday, onDate),
	}}
}

func dailyStatusExportCell(student StudentResponse, isToday bool) string {
	switch student.DayPlanningStatus {
	case DayPlanningStatusComesToday:
		return dayLabel("Kommt heute", "Wird erwartet", isToday)
	case DayPlanningStatusNotComingToday:
		switch student.DayPlanningReason {
		case dayPlanningReasonSick:
			return "Krank"
		case dayPlanningReasonExcused:
			return "Entschuldigt"
		case dayPlanningReasonClassTrip:
			return "Klassenfahrt"
		}
		if student.DayPlanningLabel != "" {
			return sentenceCase(student.DayPlanningLabel)
		}
		return dayLabel("Kommt heute nicht", "Wird nicht erwartet", isToday)
	}

	if student.Sick {
		return "Krank"
	}
	if student.ClassTrip {
		return "Klassenfahrt"
	}
	if student.Excused {
		return "Entschuldigt"
	}
	return ""
}

func sentenceCase(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	r, size := utf8.DecodeRuneInString(value)
	if r == utf8.RuneError && size == 0 {
		return ""
	}
	return string(unicode.ToUpper(r)) + value[size:]
}

// exportHasColumn reports whether the resolved column set carries the given
// column.
func exportHasColumn(columns []listexport.Column, id listexport.ColumnID) bool {
	for _, column := range columns {
		if column.ID == id {
			return true
		}
	}
	return false
}

// departureExportCell renders the per-weekday departure plan and appends the
// coupled "mit wem" detail whenever the plan allows the accompanied ("Mit
// anderem Kind") mode, so offline pickup/weekly lists carry the actionable
// "with whom" information staff need to act on (#1694).
//
// Both sources of that detail are rendered, structured links first: since links
// satisfy the accompanied-requires-a-note rule per weekday, a child that walks
// in a Laufgemeinschaft legitimately has NO note at all, and a cell built from
// the note alone would print "Mit anderem Kind" and leave the paper list — the
// one staff use when the app is not at hand — without a single name.
func departureExportCell(student StudentResponse) string {
	summary := departureSummary(student.AllowedDepartureModes, student.DepartureDays)
	details := make([]string, 0, 2)
	if companions := users.FormatCompanionLinks(student.DepartureCompanions); companions != "" {
		details = append(details, companions)
	}
	if student.DepartureCompanionNote != "" {
		details = append(details, student.DepartureCompanionNote)
	}
	if len(details) == 0 {
		return summary
	}
	allowed := student.AllowedDepartureModes.Normalize()
	if !allowed.HasAny() {
		allowed = users.AllowedDepartureModesFromDeparture(student.DepartureDays)
	}
	if !allowed.HasMode(users.DepartureAccompanied) {
		return summary
	}
	return summary + " (mit: " + strings.Join(details, "; ") + ")"
}

// departureSummary renders the per-weekday departure plan for the export, e.g.
// "Mo: Bus, Mi: Abholung". Alone/unset days are omitted; an all-alone plan
// renders "Geht alleine" (#1610).
func departureSummary(allowed users.AllowedDepartureModes, fallback users.DepartureDays) string {
	modeLabels := map[users.DepartureMode]string{
		users.DepartureAlone:       "zu Fuß",
		users.DepartureBus:         "Bus",
		users.DeparturePickup:      "Abholung",
		users.DepartureAccompanied: "Mit anderem Kind",
	}
	shortDay := map[string]string{
		users.PickupDayMonday:    "Mo",
		users.PickupDayTuesday:   "Di",
		users.PickupDayWednesday: "Mi",
		users.PickupDayThursday:  "Do",
		users.PickupDayFriday:    "Fr",
	}
	allowed = allowed.Normalize()
	if !allowed.HasAny() {
		allowed = users.AllowedDepartureModesFromDeparture(fallback)
	}
	parts := make([]string, 0, len(users.PickupDayOrder))
	for _, day := range users.PickupDayOrder {
		modes := allowed[day]
		if len(modes) == 0 {
			continue
		}
		labels := make([]string, 0, len(modes))
		for _, mode := range modes {
			labels = append(labels, modeLabels[mode])
		}
		parts = append(parts, shortDay[day]+": "+strings.Join(labels, ", "))
	}
	if len(parts) == 0 {
		return "Geht alleine"
	}
	return strings.Join(parts, ", ")
}

func weeklyCell(plan weeklySchedule, weekday int) string {
	arrival := plan.ArrivalByWeekday[weekday]
	pickup := plan.PickupByWeekday[weekday]
	if arrival == "" && pickup == "" && !plan.CareDaysByWeekday[weekday] {
		return "nein"
	}
	if arrival != "" && pickup != "" {
		return "Ankunft: " + arrival + ", Abholung: " + pickup
	}
	if arrival != "" {
		return "Ankunft: " + arrival
	}
	if pickup == "" {
		return "Ankunft: keine Zeit"
	}
	return "Abholung: " + pickup
}

func careDays(plan weeklySchedule) string {
	labels := []string{}
	for _, day := range []struct {
		weekday int
		label   string
	}{
		{schedule.WeekdayMonday, "Mo"},
		{schedule.WeekdayTuesday, "Di"},
		{schedule.WeekdayWednesday, "Mi"},
		{schedule.WeekdayThursday, "Do"},
		{schedule.WeekdayFriday, "Fr"},
	} {
		if plan.CareDaysByWeekday[day.weekday] || plan.ArrivalByWeekday[day.weekday] != "" || plan.PickupByWeekday[day.weekday] != "" {
			labels = append(labels, day.label)
		}
	}
	if len(labels) == 0 {
		return "keine"
	}
	return strings.Join(labels, ", ")
}

func dailyNotes(student StudentResponse) string {
	notes := []string{}
	if student.ArrivalNotes != "" {
		notes = append(notes, "Ankunft: "+student.ArrivalNotes)
	}
	if student.PickupNotes != "" {
		notes = append(notes, "Abholung: "+student.PickupNotes)
	}
	return strings.Join(notes, "; ")
}

func collectResponseIDs(students []StudentResponse) []int64 {
	ids := make([]int64, 0, len(students))
	for _, student := range students {
		if student.HasFullAccess {
			ids = append(ids, student.ID)
		}
	}
	return ids
}

func (rs *Resource) exportSubtitle(r *http.Request, count int) string {
	name := "Alle Kinder"
	if tenantID := tenant.FromContext(r.Context()); tenantID > 0 && rs.SchoolService != nil {
		if school, err := rs.SchoolService.GetSchoolByID(r.Context(), tenantID); err == nil && school != nil && school.Name != "" {
			name = school.Name
		}
	}
	return fmt.Sprintf("%s - %d Kinder", name, count)
}

func exportTitle(req studentExportRequest) string {
	title := strings.TrimSpace(req.Title)
	if title != "" {
		return title
	}
	switch req.Preset {
	case listexport.PresetOGSCompact:
		return "OGS Kompaktliste"
	case listexport.PresetClassRoster:
		return "Klassenliste"
	case listexport.PresetDailyPlanning:
		// "Tagesplanung", nicht "Tagesliste": der Name kollidierte mit den
		// slot-basierten Tageslisten aus dem Betreuungsplan (#1565).
		return "Tagesplanung"
	case listexport.PresetAttendanceSnapshot:
		return "Anwesenheitsliste"
	case listexport.PresetPickupList:
		return "Abholliste"
	case listexport.PresetBlankChecklist:
		return "Checkliste"
	case listexport.PresetBirthdayList:
		return "Geburtstagsliste"
	default:
		return "OGS Wochenliste"
	}
}

func exportFilterLabelsForDate(filters studentExportFilters, planningDate timezone.Date, isToday bool) []string {
	labels := exportIdentityFilterLabels(filters, planningDate, isToday)
	return append(labels, exportAttributeFilterLabels(filters, isToday)...)
}

// exportIdentityFilterLabels names the "who / which" filters for the printed
// header: the planning date, free-text search, group, school year, class, and
// the momentary status snapshot.
func exportIdentityFilterLabels(filters studentExportFilters, planningDate timezone.Date, isToday bool) []string {
	labels := []string{}
	if !isToday {
		labels = append(labels, "Datum: "+planningDate.Format("02.01.2006"))
	}
	if filters.Search != "" {
		labels = append(labels, "Suche: "+filters.Search)
	}
	if filters.GroupID != "" {
		labels = append(labels, "Gruppe gefiltert")
	}
	if years := exportYearFilterValues(filters.Year); len(years) > 0 {
		labels = append(labels, "Stufe: "+strings.Join(years, ", "))
	}
	if classes := parseMultiValueParam([]string{filters.SchoolClass}); len(classes) > 0 {
		labels = append(labels, "Klasse: "+strings.Join(classes, ", "))
	}
	if filters.Status != "" && filters.Status != "all" {
		// Only the location-derived buckets are a snapshot of right now; on a
		// dated export the remaining ones (krank/klassenfahrt/entschuldigt) come
		// from that day's status days, so calling them a Momentaufnahme would
		// mislabel a plan (#1939).
		labels = append(labels, dayLabel("Momentaufnahme: ", "Geplanter Status: ", isToday)+exportStatusLabel(filters.Status))
	}
	return labels
}

// exportAttributeFilterLabels names the per-child attribute filters for the
// printed header: bus, photo consent, pickup rule, day planning, class grouping,
// and birthday months.
func exportAttributeFilterLabels(filters studentExportFilters, isToday bool) []string {
	labels := []string{}
	if label := binaryFilterLabel(filters.Bus, "Buskind", "Kein Buskind"); label != "" {
		labels = append(labels, label)
	}
	if label := binaryFilterLabel(filters.PhotoConsent, "Fotoerlaubnis liegt vor", "Keine Fotoerlaubnis"); label != "" {
		labels = append(labels, label)
	}
	if filters.PickupStatus != "" && filters.PickupStatus != "all" {
		labels = append(labels, "Abholregelung: "+exportPickupStatusLabel(filters.PickupStatus))
	}
	if filters.DayStatus != "" && filters.DayStatus != DayPlanningStatusAll {
		labels = append(labels, "Tagesplanung: "+dayStatusExportLabel(filters.DayStatus, isToday))
	}
	if filters.GroupByClass {
		labels = append(labels, "Nach Klassen getrennt")
	}
	if label := birthdayMonthFilterLabel(filters.Months); label != "" {
		labels = append(labels, label)
	}
	return labels
}

// binaryFilterLabel names a yes/no filter for the printed header, or "" when
// the filter is inactive.
func binaryFilterLabel(value, yesLabel, noLabel string) string {
	if value == "" || value == "all" {
		return ""
	}
	if value == "yes" {
		return yesLabel
	}
	return noLabel
}

var germanMonthNames = [12]string{
	"Januar", "Februar", "März", "April", "Mai", "Juni",
	"Juli", "August", "September", "Oktober", "November", "Dezember",
}

// birthdayMonthFilterLabel names the selected birth months chronologically,
// independent of the order they arrived in, so the printed header matches the
// order of the list below it.
func birthdayMonthFilterLabel(values []string) string {
	months, err := parseExportMonths(values)
	if err != nil || len(months) == 0 {
		return ""
	}
	names := make([]string, 0, len(months))
	for month := time.January; month <= time.December; month++ {
		if months[month] {
			names = append(names, germanMonthNames[month-1])
		}
	}
	if len(names) == 1 {
		return "Geburtsmonat: " + names[0]
	}
	return "Geburtsmonate: " + strings.Join(names, ", ")
}

func exportPickupStatusLabel(status string) string {
	switch status {
	case "self":
		return "Geht alleine nach Hause"
	case "pickedUp":
		return "Wird abgeholt"
	case "none":
		return "Keine Abholregelung"
	default:
		return "Sonstige"
	}
}

func exportStatusLabel(status string) string {
	switch status {
	case "krank":
		return "Krank"
	case "klassenfahrt":
		return "Klassenfahrt"
	case "entschuldigt":
		return "Entschuldigt"
	case "abwesend":
		return "Abwesend"
	case "unterwegs":
		return "Unterwegs"
	case "schulhof":
		return "Schulhof"
	case "anwesend":
		return "Anwesend"
	default:
		return status
	}
}

func dayStatusExportLabel(status string, isToday bool) string {
	switch status {
	case DayPlanningStatusComesToday:
		return dayLabel("Kommt heute", "Wird erwartet", isToday)
	case DayPlanningStatusNotComingToday:
		return dayLabel("Kommt heute nicht", "Wird nicht erwartet", isToday)
	default:
		return status
	}
}

func schoolYear(schoolClass string) string {
	for _, r := range schoolClass {
		if r >= '1' && r <= '9' {
			return string(r)
		}
	}
	return ""
}

func exportStatus(student StudentResponse) string {
	if student.ClassTrip {
		return "klassenfahrt"
	}
	if student.Sick {
		return "krank"
	}
	if student.Excused {
		return "entschuldigt"
	}
	switch student.Location {
	case "Zuhause", "":
		return "abwesend"
	case "Unterwegs":
		return "unterwegs"
	case "Schulhof":
		return "schulhof"
	default:
		return "anwesend"
	}
}

func timeValue(value *string) string {
	if value == nil || *value == "" {
		return "99:99"
	}
	return *value
}

func formatWallClock(value time.Time) string {
	return timezone.NormalizeWallClock(value).Format("15:04")
}
