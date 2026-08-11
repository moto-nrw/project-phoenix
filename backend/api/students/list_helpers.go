package students

import (
	"fmt"
	"maps"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/internal/schoolclass"
	"github.com/moto-nrw/project-phoenix/internal/strutil"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/users"
)

// studentListParams holds all query parameters for student listing
type studentListParams struct {
	// schoolClasses filters by exact class names. Several may be selected at
	// once (#2218: two groups supervised together need "3a AND 4b" in one
	// list); an empty slice means every class.
	schoolClasses []string
	guardianName  string
	firstName     string
	lastName      string
	location      string
	locationState string
	// groupIDs filters by educational group. Several may be selected at once
	// (#2218); an empty slice means every group.
	groupIDs            []int64
	roomID              int64
	search              string
	page                int
	pageSize            int
	includePickupTimes  bool
	includeCompanions   bool
	includeArrivalTimes bool
	dayStatus           string
	// date is the optional planning day (YYYY-MM-DD) the day-planning fields,
	// status days, and planned arrival/pickup times are evaluated for (#1939).
	// Empty means the school-local today. Parsed and validated in the handler
	// via resolvePlanningDate so an invalid value is a 400, not a silent today.
	date string
	// gradeLevels filters by the first numeric run in school_class (issue #1838,
	// Zielgruppe "Jahrgang"). Resolved in-memory via schoolclass.GradePrefix —
	// a SQL LIKE 'N%' would incorrectly match e.g. grade 1 against "13a".
	// Several levels may be selected at once (#2218); empty = off.
	gradeLevels []int
	// Administrative filters (#1492). Resolved against the enriched response
	// objects in the same in-memory pass as dayStatus so pagination and counts
	// stay correct. bus/photoConsent are "yes"/"no"; pickupStatus is one of the
	// pickupStatusKind buckets ("self"/"pickedUp"/"none"). Empty or "all" = off.
	bus          string
	photoConsent string
	pickupStatus string
	// studentIDs is an optional pre-filter populated by upstream resolution
	// (e.g., room_id → active visits) before the SQL list query runs. When
	// set, buildBaseFilter adds `student.id IN (...)` so the standard
	// school_class / guardian_name / pagination pipeline still applies.
	studentIDs []int64
	// fetchAll disables SQL pagination so the query returns every row matching
	// the SQL-level filters. Exports set this: the birthday-month and search
	// filters run in memory after the fetch, so a paginated page would hide
	// matching children past the page boundary and silently shorten the list.
	fetchAll bool
	// slimView selects the Kindersuche wire projection (#2097). Purely a
	// marshalling choice made after filtering and pagination — see
	// list_projection.go.
	slimView bool
}

// studentAccessContext is an alias for the shared common.StudentAccessContext
// so existing call sites in this package keep working unchanged.
type studentAccessContext = common.StudentAccessContext

// parseStudentListView resolves the `view` query parameter. Empty and "full"
// keep the historical projection; only "slim" opts into the Kindersuche shape
// (#2097). An unknown value is rejected instead of silently serving the full
// payload, so a typo in a caller surfaces immediately.
func parseStudentListView(value string) (bool, error) {
	switch value {
	case "", StudentListViewFull:
		return false, nil
	case StudentListViewSlim:
		return true, nil
	default:
		return false, fmt.Errorf("invalid view %q: must be %q or %q", value, StudentListViewFull, StudentListViewSlim)
	}
}

// parseMultiValueParam splits a filter parameter that may name several values
// at once (#2218). Values travel comma-separated (`school_class=3a,4b`);
// repeated parameters (`school_class=3a&school_class=4b`) are accepted too so a
// hand-built URL behaves the same. Blanks are dropped and duplicates collapsed,
// preserving the caller's order.
func parseMultiValueParam(raw []string) []string {
	values := make([]string, 0, len(raw))
	seen := make(map[string]struct{}, len(raw))
	for _, entry := range raw {
		for _, value := range strings.Split(entry, ",") {
			value = strings.TrimSpace(value)
			if value == "" {
				continue
			}
			if _, duplicate := seen[value]; duplicate {
				continue
			}
			seen[value] = struct{}{}
			values = append(values, value)
		}
	}
	return values
}

// parseGroupIDList turns a possibly repeated, comma-separated group_id
// parameter into positive group ids. Non-numeric and non-positive entries are
// dropped, which keeps the historical single-value contract: an unusable
// group_id has always meant "no group restriction" rather than a 400.
func parseGroupIDList(raw []string) []int64 {
	values := parseMultiValueParam(raw)
	groupIDs := make([]int64, 0, len(values))
	for _, value := range values {
		if groupID, err := strconv.ParseInt(value, 10, 64); err == nil && groupID > 0 {
			groupIDs = append(groupIDs, groupID)
		}
	}
	return groupIDs
}

// parseGradeLevelList turns a possibly repeated, comma-separated grade_level
// parameter into positive grade levels, dropping unusable entries for the same
// reason as parseGroupIDList.
func parseGradeLevelList(raw []string) []int {
	values := parseMultiValueParam(raw)
	levels := make([]int, 0, len(values))
	for _, value := range values {
		if level, err := strconv.Atoi(value); err == nil && level > 0 {
			levels = append(levels, level)
		}
	}
	return levels
}

// parseStudentListParams extracts query parameters from the request
func parseStudentListParams(r *http.Request) *studentListParams {
	params := &studentListParams{
		schoolClasses: parseMultiValueParam(r.URL.Query()["school_class"]),
		guardianName:  r.URL.Query().Get("guardian_name"),
		firstName:     r.URL.Query().Get("first_name"),
		lastName:      r.URL.Query().Get("last_name"),
		location:      r.URL.Query().Get("location"),
		locationState: r.URL.Query().Get("location_state"),
		search:        r.URL.Query().Get("search"),
	}

	// Parse group IDs if provided. Unparseable entries are skipped rather than
	// rejected, matching the historical single-value behavior.
	params.groupIDs = parseGroupIDList(r.URL.Query()["group_id"])

	// Parse room ID if provided. Filters the list to students currently
	// checked-in to any active group taking place in this room (joins via
	// active.visits → active.groups). Used by the "In Kindersuche öffnen"
	// link from the room detail page (#1323).
	if roomIDStr := r.URL.Query().Get("room_id"); roomIDStr != "" {
		if roomID, err := strconv.ParseInt(roomIDStr, 10, 64); err == nil {
			params.roomID = roomID
		}
	}

	// Parse grade levels if provided (issue #1838, Zielgruppe "Jahrgang").
	params.gradeLevels = parseGradeLevelList(r.URL.Query()["grade_level"])

	// Parse optional includes
	params.includePickupTimes = r.URL.Query().Get("include_pickup_times") == "true"
	params.includeCompanions = r.URL.Query().Get("include_companions") == "true"
	params.includeArrivalTimes = r.URL.Query().Get("include_arrival_times") == "true"
	params.dayStatus = parseDayStatusParam(r.URL.Query().Get("day_status"))
	params.date = r.URL.Query().Get("date")

	// Administrative filters (#1492). Applied in-memory against the enriched
	// responses, mirroring the student list export filter semantics.
	params.bus = r.URL.Query().Get("bus")
	params.photoConsent = r.URL.Query().Get("photo_consent")
	params.pickupStatus = r.URL.Query().Get("pickup_status")

	// Parse pagination
	params.page, params.pageSize = common.ParsePagination(r)

	return params
}

// hasPersonFilters returns true if any person-based filters are active
func (p *studentListParams) hasPersonFilters() bool {
	return p.search != "" || p.firstName != "" || p.lastName != "" || p.location != ""
}

func (p *studentListParams) hasInMemoryFilters() bool {
	return p.hasPersonFilters() ||
		p.dayStatus != "" && p.dayStatus != DayPlanningStatusAll ||
		len(p.gradeLevels) > 0 ||
		p.hasAdministrativeFilters()
}

// hasAdministrativeFilters reports whether any of the #1492 administrative
// filters (bus / photo consent / pickup rule) is active.
func (p *studentListParams) hasAdministrativeFilters() bool {
	return isActiveFilterValue(p.bus) ||
		isActiveFilterValue(p.photoConsent) ||
		isActiveFilterValue(p.pickupStatus)
}

func (p *studentListParams) canUseGroupOnlyShortcut() bool {
	return len(p.schoolClasses) == 0 &&
		p.guardianName == "" &&
		p.roomID == 0 &&
		len(p.studentIDs) == 0 &&
		!p.hasInMemoryFilters()
}

// isActiveFilterValue treats both empty and the neutral "all" sentinel as "off".
func isActiveFilterValue(value string) bool {
	return value != "" && value != "all"
}

func parseDayStatusParam(value string) string {
	switch value {
	case DayPlanningStatusComesToday, DayPlanningStatusNotComingToday:
		return value
	default:
		return DayPlanningStatusAll
	}
}

// buildBaseFilter creates the shared filter for school_class and guardian_name.
// school_class is an exact class selector for class rosters; free text class
// search still belongs in the broader `search` parameter.
func (p *studentListParams) buildBaseFilter() *base.Filter {
	filter := base.NewFilter()
	// Alumni (graduated via grade transition, soft-deleted) are invisible to
	// every staff list and export.
	filter.NotIn("status", string(users.StudentStatusAlumnus))
	// Several classes may be selected at once (#2218); TrimIn collapses to the
	// single-value TrimEqual when exactly one is requested.
	if len(p.schoolClasses) > 0 {
		filter.TrimIn("school_class", p.schoolClasses...)
	}
	if p.guardianName != "" {
		filter.ILike("guardian_name", "%"+p.guardianName+"%")
	}
	if len(p.studentIDs) > 0 {
		ids := make([]interface{}, len(p.studentIDs))
		for i, id := range p.studentIDs {
			ids[i] = id
		}
		filter.In("id", ids...)
	}
	return filter
}

// buildQueryOptions creates query options from parameters
func (p *studentListParams) buildQueryOptions() *base.QueryOptions {
	queryOptions := base.NewQueryOptions()
	queryOptions.Filter = p.buildBaseFilter()

	// Add pagination only if no person-based filters and the caller wants a
	// page. Exports (fetchAll) take every row so their in-memory filters see
	// the whole set.
	if !p.fetchAll && !p.hasInMemoryFilters() {
		queryOptions.WithPagination(p.page, p.pageSize)
	}

	return queryOptions
}

// buildCountOptions creates query options for counting records
func (p *studentListParams) buildCountOptions() *base.QueryOptions {
	countOptions := base.NewQueryOptions()
	countOptions.Filter = p.buildBaseFilter()
	return countOptions
}

// determineStudentAccess resolves the access context for the current request.
// Thin wrapper that injects this Resource's services into the shared common
// helper so per-student access checks stay cheap when iterating a list.
func (rs *Resource) determineStudentAccess(r *http.Request) *studentAccessContext {
	return common.DetermineStudentAccess(r, rs.UserContextService, rs.SettingsService, rs.Logger)
}

// collectIDsFromStudents extracts IDs needed for bulk loading
func collectIDsFromStudents(students []*users.Student) (studentIDs, personIDs, groupIDs []int64) {
	studentIDs = make([]int64, 0, len(students))
	personIDs = make([]int64, 0, len(students))
	groupIDSet := make(map[int64]struct{})

	for _, student := range students {
		studentIDs = append(studentIDs, student.ID)
		personIDs = append(personIDs, student.PersonID)
		if student.GroupID != nil {
			groupIDSet[*student.GroupID] = struct{}{}
		}
	}

	groupIDs = slices.Collect(maps.Keys(groupIDSet))

	return studentIDs, personIDs, groupIDs
}

// matchesSearchFilter checks if a student matches the search term
func matchesSearchFilter(person *users.Person, studentID int64, search string) bool {
	if search == "" {
		return true
	}

	studentIDStr := strconv.FormatInt(studentID, 10)
	fullName := person.FirstName + " " + person.LastName

	return strutil.ContainsFold(person.FirstName, search) ||
		strutil.ContainsFold(person.LastName, search) ||
		strutil.ContainsFold(studentIDStr, search) ||
		strutil.ContainsFold(fullName, search)
}

// matchesNameFilters checks if a student matches the name filters
func matchesNameFilters(person *users.Person, firstName, lastName string) bool {
	if firstName != "" && !strutil.ContainsFold(person.FirstName, firstName) {
		return false
	}
	if lastName != "" && !strutil.ContainsFold(person.LastName, lastName) {
		return false
	}
	return true
}

// matchesLocationFilter checks if a student matches the location filter
func matchesLocationFilter(location, studentLocation string, hasFullAccess bool) bool {
	if location == "" {
		return true
	}
	if !hasFullAccess {
		return true
	}
	if location == "Unknown" {
		return true
	}
	return studentLocation == location
}

// matchesGradeLevel reports whether schoolClass's first numeric run equals any
// of gradeLevels. An empty slice means the filter is off (matches everything).
// Uses schoolclass.GradePrefix rather than a naive string-prefix/LIKE check
// so e.g. grade 1 does not also match "13a".
func matchesGradeLevel(schoolClass string, gradeLevels []int) bool {
	if len(gradeLevels) == 0 {
		return true
	}
	prefix := schoolclass.GradePrefix(schoolClass)
	for _, gradeLevel := range gradeLevels {
		if gradeLevel > 0 && prefix == strconv.Itoa(gradeLevel) {
			return true
		}
	}
	return false
}

// applyInMemoryPagination applies pagination to an already-filtered slice
func applyInMemoryPagination(responses []StudentResponse, page, pageSize int) ([]StudentResponse, int) {
	totalCount := len(responses)

	startIndex := (page - 1) * pageSize
	endIndex := startIndex + pageSize

	// Ensure bounds are valid
	if startIndex > len(responses) {
		startIndex = len(responses)
	}
	if endIndex > len(responses) {
		endIndex = len(responses)
	}

	return responses[startIndex:endIndex], totalCount
}

func collectFullAccessStudentIDs(responses []StudentResponse) []int64 {
	fullAccessIDs := make([]int64, 0, len(responses))
	for i := range responses {
		if responses[i].HasFullAccess {
			fullAccessIDs = append(fullAccessIDs, responses[i].ID)
		}
	}
	return fullAccessIDs
}
