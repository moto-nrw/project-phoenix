package students

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"sort"
	"strings"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/internal/collation"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	peopleModule "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
)

func (rs *Resource) prefetchListSettings(ctx context.Context) (context.Context, error) {
	return common.PrefetchSettingsOrError(ctx, rs.SettingsService,
		settingEnrollmentBookingsAuthoritative,
		settingPresenceMode,
		settingSessionEndTime,
		settingStudentPhotosEnabled,
	)
}

// listStudents handles listing all students with staff-based filtering
func (rs *Resource) listStudents(w http.ResponseWriter, r *http.Request) {
	plan, renderer := rs.parseStudentListPlan(r)
	if renderer != nil {
		renderError(w, r, renderer)
		return
	}
	accessCtx := rs.determineStudentAccess(r)
	settingsCtx, err := rs.prefetchListSettings(r.Context())
	if err != nil {
		renderError(w, r, common.ErrorInternalServer(err))
		return
	}
	r = r.WithContext(settingsCtx)

	// Fetch students based on parameters
	students, totalCount, err := rs.fetchStudentsForList(r, plan.params)
	if errors.Is(err, ErrInvalidRequest) {
		renderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	if err != nil {
		renderError(w, r, common.ErrorInternalServer(err))
		return
	}

	page, err := rs.buildStudentListPage(r.Context(), students, plan, accessCtx)
	if err != nil {
		renderError(w, r, common.ErrorInternalServer(err))
		return
	}
	responses, totalCount, err := rs.finishStudentListPage(r.Context(), page, plan, totalCount)
	if err != nil {
		renderError(w, r, common.ErrorInternalServer(err))
		return
	}

	params := plan.params
	pagination := common.PaginationParams{Page: params.page, PageSize: params.pageSize, Total: totalCount}
	// Projection happens last, after every filter, sort and pagination step has
	// run on the full responses — the two views differ on the wire only (#2097).
	if params.slimView {
		common.RespondPaginated(w, r, http.StatusOK, slimStudentResponses(responses, plan.planningDate), pagination, "Students retrieved successfully")
		return
	}
	common.RespondPaginated(w, r, http.StatusOK, responses, pagination, "Students retrieved successfully")
}

// studentListPlan is a parsed list request: its filters and the day the list
// is rendered for.
type studentListPlan struct {
	params       *studentListParams
	planningDate timezone.Date
	isToday      bool
}

// parseStudentListPlan reads the list request. The planning date is resolved
// BEFORE the fetch: the room/location pre-filters query today's live
// active.visits state, so a non-today planning request has to be rejected
// before that query runs, not after it (#1939).
func (rs *Resource) parseStudentListPlan(r *http.Request) (studentListPlan, render.Renderer) {
	params := parseStudentListParams(r)
	slimView, viewErr := parseStudentListView(r.URL.Query().Get("view"))
	if viewErr != nil {
		return studentListPlan{}, common.ErrorInvalidRequest(viewErr)
	}
	params.slimView = slimView
	careStatus, careErr := careStatusFromRequest(r)
	if careErr != nil {
		return studentListPlan{}, careErr
	}
	params.careStatus = careStatus
	now := rs.Now()
	planningDate, isToday, dateErr := resolvePlanningDate(params.date, now)
	if dateErr != nil {
		return studentListPlan{}, common.ErrorInvalidRequest(dateErr)
	}
	if err := liveFilterError(activeLiveListFilters(params), planningDate, isToday); err != nil {
		return studentListPlan{}, common.ErrorInvalidRequest(err)
	}
	params.careStatusOn = planningDate
	params.careStatusToday = timezone.DateFromTime(now)
	return studentListPlan{params: params, planningDate: planningDate, isToday: isToday}, nil
}

// studentListPage is the list before its response-derived filters: every
// fetched child's response and the bulk data the later steps reuse.
type studentListPage struct {
	responses     []StudentResponse
	dataSnapshot  *studentDataSnapshot
	planningTimes dayPlanningTimes
}

// buildStudentListPage bulk-loads what the rows render besides themselves and
// builds their responses for the planning date.
func (rs *Resource) buildStudentListPage(ctx context.Context, students []*Student, plan studentListPlan, accessCtx *studentAccessContext) (studentListPage, error) {
	dataSnapshot, groups, err := rs.loadStudentListData(ctx, students)
	if err != nil {
		return studentListPage{}, err
	}
	// Resolve once per request. populatePhotoFields runs per student.
	photosEnabled := resolveBoolSetting(ctx, rs.SettingsService, settingStudentPhotosEnabled, false, rs.Logger)

	responses := rs.buildStudentResponses(ctx, students, plan.params, accessCtx, dataSnapshot, groups, photosEnabled)

	if !plan.isToday {
		// The row-seeded Sick/Excused flags describe today; a non-today view
		// must start clean and only carry the requested date's status days.
		resetScheduledStatusFlags(responses)
		// Same for the live-location snapshot: a list labelled for another day
		// must not ship today's whereabouts. The page already renders the
		// planned expectation instead of the location badge for a non-today
		// date; stripping the fields keeps a direct API consumer from reading
		// them as the plan (#1939).
		resetLiveLocationFields(responses)
	}
	if err := rs.applyStatusDaysForDate(ctx, responses, plan.planningDate.BerlinMidnight()); err != nil {
		slog.Default().Error("failed to apply student status days", slog.String("error", err.Error()))
		return studentListPage{}, err
	}
	planningTimes, err := rs.enrichWithDayPlanning(ctx, responses, plan.planningDate, plan.isToday, attendanceMapFromSnapshot(dataSnapshot))
	if err != nil {
		slog.Default().Error("failed to enrich student day planning", slog.String("error", err.Error()))
		return studentListPage{}, err
	}
	return studentListPage{responses: responses, dataSnapshot: dataSnapshot, planningTimes: planningTimes}, nil
}

// finishStudentListPage applies the response-derived filters, the in-memory
// page window and the per-page enrichments.
func (rs *Resource) finishStudentListPage(ctx context.Context, page studentListPage, plan studentListPlan, totalCount int) ([]StudentResponse, int, error) {
	params := plan.params
	responses := applyLocationFilter(page.responses, params.location)
	responses = applyDayPlanningFilter(responses, params.dayStatus)
	// Administrative filters (#1492): bus / photo consent / pickup rule.
	// Applied here, before in-memory pagination, so server-side counts and
	// page boundaries reflect the filtered set (no client-side full-page
	// fetch needed).
	responses = applyAdministrativeFilters(responses, params.bus, params.photoConsent, params.pickupStatus, plan.planningDate)

	// Apply in-memory pagination if response-derived filters were used.
	if params.hasInMemoryFilters() {
		responses, totalCount = applyInMemoryPagination(responses, params.page, params.pageSize)
	}

	enrichPaginatedPlanningTimes(responses, params, page.dataSnapshot, page.planningTimes, plan.isToday)

	// After pagination: only the page that is actually returned needs to know
	// which of its children carry a recorded exit (#2487).
	if err := rs.enrichWithCareExitFlag(ctx, responses); err != nil {
		return nil, 0, err
	}

	// Companion ids ("läuft mit") for the day being SHOWN, not for today: the
	// grouping is per weekday, so a list rendered for another planning date must
	// resolve the links of that date. Fatal by design (see enrichWithCompanions)
	// — an empty grouping would be presented as a real departure arrangement.
	if err := rs.enrichWithCompanions(ctx, responses, params, plan.planningDate.BerlinMidnight()); err != nil {
		return nil, 0, err
	}
	return responses, totalCount, nil
}

// enrichPaginatedPlanningTimes layers the planning-date time data onto the final
// paginated slice: today's live check-in/out times (kept off any other day so
// current presence is never read as a plan), and, when requested, the effective
// pickup/arrival times from the maps enrichWithDayPlanning already bulk-loaded
// for the pre-pagination superset (#2098 — no second round of the same
// queries). Both skip redacted students — only rows the caller has full access
// to are enriched.
func enrichPaginatedPlanningTimes(responses []StudentResponse, params *studentListParams, dataSnapshot *studentDataSnapshot, planningTimes dayPlanningTimes, isToday bool) {
	if isToday {
		for i := range responses {
			if !responses[i].HasFullAccess {
				continue
			}
			applyActualTimesFromSnapshot(&responses[i], dataSnapshot)
		}
	}

	if params.includePickupTimes {
		applyPickupTimesFromMap(responses, planningTimes.pickups)
	}
	if params.includeArrivalTimes {
		applyArrivalTimesFromMap(responses, planningTimes.arrivals)
	}
}

// fetchStudentsForList fetches students based on the provided parameters. The
// location/room/group pre-filters each resolve a set of student IDs (or a fully
// materialized slice for the group-only fast path) before the standard query
// path applies school_class / pagination on top.
func (rs *Resource) fetchStudentsForList(r *http.Request, params *studentListParams) ([]*Student, int, error) {
	ctx := r.Context()

	switch {
	case params.locationState != "":
		nonEmpty, err := rs.resolveLocationStateFilter(ctx, params)
		if err != nil {
			return nil, 0, err
		}
		if !nonEmpty {
			return []*Student{}, 0, nil
		}
	case params.roomID > 0:
		nonEmpty, err := rs.resolveRoomFilter(ctx, params)
		if err != nil {
			return nil, 0, err
		}
		if !nonEmpty {
			return []*Student{}, 0, nil
		}
	case len(params.groupIDs) > 0:
		students, totalCount, done, err := rs.resolveGroupFilter(ctx, params)
		if err != nil {
			return nil, 0, err
		}
		if done {
			return students, totalCount, nil
		}
	}
	if nonEmpty, err := rs.resolveOperationalParticipantFilter(ctx, params); err != nil {
		return nil, 0, err
	} else if !nonEmpty {
		return []*Student{}, 0, nil
	}

	return rs.runStandardStudentQuery(ctx, params)
}

func (rs *Resource) resolveOperationalParticipantFilter(
	ctx context.Context, params *studentListParams,
) (bool, error) {
	if params.careStatus != CareStatusRunning {
		return true, nil
	}
	if rs.CareLifecycleService == nil {
		return false, errors.New("student list: care lifecycle service is not configured")
	}
	resolution, err := rs.CareLifecycleService.ResolveListParticipation(
		ctx, params.studentIDs, params.careStatusOn, params.careStatusToday, params.includePendingWithdrawals,
	)
	if err != nil {
		return false, err
	}
	params.actuallyPresentIDs = retainedStudentIDs(resolution.CandidateIDs, resolution.ActuallyPresentIDs)
	params.studentIDs = retainedStudentIDs(resolution.CandidateIDs, resolution.ParticipatingIDs)
	return len(params.studentIDs) > 0, nil
}

func retainedStudentIDs(candidates []int64, participating map[int64]bool) []int64 {
	retained := make([]int64, 0, len(candidates))
	for _, id := range candidates {
		if participating[id] {
			retained = append(retained, id)
		}
	}
	return retained
}

// resolveLocationStateFilter resolves the present/transit pre-filter into
// params.studentIDs. It reports nonEmpty=false when the resolved set is empty so
// the caller can short-circuit with an empty page.
func (rs *Resource) resolveLocationStateFilter(ctx context.Context, params *studentListParams) (bool, error) {
	if params.locationState != "transit" && params.locationState != "present" {
		return false, ErrInvalidRequest
	}
	if params.roomID > 0 {
		return false, ErrInvalidRequest
	}

	var ids []int64
	var err error
	if params.locationState == "present" {
		ids, err = rs.ActiveService.ListStudentsPresentToday(ctx)
	} else {
		ids, err = rs.ActiveService.ListStudentsInTransit(ctx)
	}
	if err != nil {
		return false, err
	}
	if len(ids) == 0 {
		return false, nil
	}

	if len(params.groupIDs) > 0 {
		ids, err = rs.filterStudentIDsByGroups(ctx, ids, params.groupIDs)
		if err != nil {
			return false, err
		}
		if len(ids) == 0 {
			return false, nil
		}
	}

	params.studentIDs = ids
	return true, nil
}

// resolveRoomFilter resolves the room_id pre-filter (#1323): students currently
// checked-in to any active group in the room, pushed through the standard query
// path so school_class / pagination still apply. The visit join
// lives in the active service (rule 11: services own queries, not handlers). It
// reports nonEmpty=false when the resolved set is empty.
func (rs *Resource) resolveRoomFilter(ctx context.Context, params *studentListParams) (bool, error) {
	ids, err := rs.ActiveService.ListStudentsPresentInRoom(ctx, params.roomID)
	if err != nil {
		return false, err
	}
	if len(ids) == 0 {
		return false, nil
	}

	// When both room_id and group_id are supplied, intersect with the student's
	// group_id so the response stays consistent with the active-group chip in the
	// search UI. group_id lives on the Student row, so a single bulk lookup is
	// enough.
	if len(params.groupIDs) > 0 {
		filtered, err := rs.filterStudentIDsByGroups(ctx, ids, params.groupIDs)
		if err != nil {
			return false, err
		}
		if len(filtered) == 0 {
			return false, nil
		}
		ids = filtered
	}

	// params.groupIDs is intentionally NOT cleared even though buildBaseFilter
	// ignores it, because the room and group intersection was already computed
	// above, so re-applying group_id downstream would be redundant.
	params.studentIDs = ids
	return true, nil
}

// resolveGroupFilter handles the group-only path. When the request qualifies for
// the fast path it returns the materialized slice with done=true; otherwise it
// resolves params.studentIDs for the standard query and returns done=false.
// done=true with an empty slice signals a short-circuit empty page.
func (rs *Resource) resolveGroupFilter(ctx context.Context, params *studentListParams) ([]*Student, int, bool, error) {
	var (
		students []*Student
		err      error
	)
	if params.canUseGroupOnlyShortcut() {
		students, err = rs.studentsByGroups(ctx, params.groupIDs, peopleModule.StudentScopeEnrolled)
	} else {
		students, err = rs.studentsByGroups(ctx, params.groupIDs, peopleModule.StudentScopeAll)
	}
	if err != nil {
		return nil, 0, false, err
	}

	// Fast path for true group-only requests: no SQL round trip for the page,
	// but the requested window still has to be honored, because this path never
	// reaches the query's LIMIT/OFFSET (#2218 review). The total stays the whole
	// selection, so the reported count keeps naming every child of the groups.
	if params.canUseGroupOnlyShortcut() {
		return params.pageOfGroupStudents(students), len(students), true, nil
	}

	if len(students) == 0 {
		return []*Student{}, 0, true, nil
	}

	ids := make([]int64, 0, len(students))
	for _, student := range students {
		if student != nil {
			ids = append(ids, student.ID)
		}
	}
	if len(ids) == 0 {
		return []*Student{}, 0, true, nil
	}

	params.studentIDs = ids
	return nil, 0, false, nil
}

// runStandardStudentQuery runs the SQL list/count path. buildBaseFilter picks up
// params.studentIDs (if set by a pre-filter above) and combines it with
// school_class and pagination.
func (rs *Resource) runStandardStudentQuery(ctx context.Context, params *studentListParams) ([]*Student, int, error) {
	totalCount, err := rs.PeopleDirectory.CountStudentDirectory(ctx, params.buildDirectoryFilter())
	if err != nil {
		return nil, 0, err
	}

	records, err := rs.PeopleDirectory.ListStudentDirectory(ctx, params.buildPagedDirectoryFilter())
	if err != nil {
		return nil, 0, err
	}
	if len(records) == 0 {
		return nil, totalCount, nil
	}

	return studentsFromRecords(records), totalCount, nil
}

func (rs *Resource) listSchoolClasses(w http.ResponseWriter, r *http.Request) {
	classes, err := rs.PeopleDirectory.ListSchoolClasses(r.Context())
	if err != nil {
		renderError(w, r, common.ErrorInternalServer(err))
		return
	}
	classes, err = rs.appendClassListEntryClasses(r.Context(), classes)
	if err != nil {
		renderError(w, r, common.ErrorInternalServer(err))
		return
	}
	common.Respond(w, r, http.StatusOK, classes, "School classes retrieved successfully")
}

// appendClassListEntryClasses unions the classes that exist only through
// class-list-only entries (#2382) into the dropdown list: a class whose
// children are all list entries must still be selectable for class lists.
// Dedupe uses the LOWER(TRIM(...)) identity every class comparison uses.
func (rs *Resource) appendClassListEntryClasses(ctx context.Context, classes []string) ([]string, error) {
	if rs.ClassListEntries == nil {
		return classes, nil
	}
	entries, err := rs.ClassListEntries.ListClassListEntriesInDisplayOrder(ctx)
	if err != nil {
		return nil, err
	}
	seen := make(map[string]bool, len(classes))
	for _, class := range classes {
		seen[strings.ToLower(strings.TrimSpace(class))] = true
	}
	added := false
	for _, entry := range entries {
		key := strings.ToLower(strings.TrimSpace(entry.SchoolClass))
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		classes = append(classes, strings.TrimSpace(entry.SchoolClass))
		added = true
	}
	if added {
		sort.SliceStable(classes, func(i, j int) bool {
			return collation.CompareSchoolClasses(classes[i], classes[j]) < 0
		})
	}
	return classes, nil
}
