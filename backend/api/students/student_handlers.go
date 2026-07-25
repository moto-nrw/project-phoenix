package students

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	guardiansAPI "github.com/moto-nrw/project-phoenix/api/guardians"
	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/internal/strutil"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/realtime"
	activeService "github.com/moto-nrw/project-phoenix/services/active"
	configService "github.com/moto-nrw/project-phoenix/services/config"
	userService "github.com/moto-nrw/project-phoenix/services/users"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// parseAndGetStudent parses the student ID from the URL and fetches the student
// Returns the student and true if successful, or renders an error and returns nil, false
func (rs *Resource) parseAndGetStudent(w http.ResponseWriter, r *http.Request) (*users.Student, bool) {
	student, ok := rs.parseAndGetStudentIncludingAlumni(w, r)
	if !ok {
		return nil, false
	}

	// A graduated (alumnus) student is soft-deleted: invisible to every staff
	// list and export. GetStudentByID is unfiltered, so this shared per-student
	// gate is where a bookmarked ID or a direct API call to any update /
	// status-day / schedule / RFID / privacy / delete route is rejected — the
	// same 404 those routes returned back when graduates were hard-deleted (#405).
	if student.Status == users.StudentStatusAlumnus {
		renderError(w, r, common.ErrorNotFound(errors.New("student not found")))
		return nil, false
	}

	return student, true
}

// parseAndGetStudentIncludingAlumni is parseAndGetStudent without the alumnus
// gate. It exists for the ONE operation that must still work on a departed
// child: releasing the RFID bracelet they are still holding. Graduation now
// clears the tag itself, so this only covers children graduated before that
// existed — for those, the gate would otherwise create a state the kiosk can
// detect but never resolve (#405 review). Every other route uses
// parseAndGetStudent; do not widen this one.
func (rs *Resource) parseAndGetStudentIncludingAlumni(w http.ResponseWriter, r *http.Request) (*users.Student, bool) {
	id, err := common.ParseID(r)
	if err != nil {
		renderError(w, r, common.ErrorInvalidRequest(errors.New(common.MsgInvalidStudentID)))
		return nil, false
	}

	student, err := rs.PersonService.GetStudentByID(r.Context(), id)
	if err != nil {
		renderError(w, r, common.ErrorNotFound(errors.New("student not found")))
		return nil, false
	}

	return student, true
}

// listStudents handles listing all students with staff-based filtering
func (rs *Resource) listStudents(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters and determine access
	params := parseStudentListParams(r)
	// Resolved BEFORE the fetch: the room/location pre-filters below query
	// today's live active.visits state, so a non-today planning request has to
	// be rejected before that query runs, not after it (#1939).
	planningDate, isToday, dateErr := resolvePlanningDate(params.date, rs.Now())
	if dateErr != nil {
		renderError(w, r, common.ErrorInvalidRequest(dateErr))
		return
	}
	if err := liveFilterError(activeLiveListFilters(params), planningDate, isToday); err != nil {
		renderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	accessCtx := rs.determineStudentAccess(r)

	// Fetch students based on parameters
	students, totalCount, err := rs.fetchStudentsForList(r, params)
	if err != nil {
		if errors.Is(err, ErrInvalidRequest) {
			renderError(w, r, common.ErrorInvalidRequest(err))
			return
		}
		renderError(w, r, common.ErrorInternalServer(err))
		return
	}

	// Bulk load all related data
	studentIDs, personIDs, groupIDs := collectIDsFromStudents(students)
	dataSnapshot, err := common.LoadStudentDataSnapshot(
		r.Context(),
		rs.PersonService,
		rs.EducationService,
		rs.ActiveService,
		studentIDs,
		personIDs,
		groupIDs,
	)
	if err != nil {
		slog.Default().Error("failed to load student data snapshot", slog.String("error", err.Error()))
		renderError(w, r, common.ErrorInternalServer(err))
		return
	}

	// Resolve once per request. populatePhotoFields runs per student.
	photosEnabled := configService.ResolveBoolOrDefault(r.Context(), rs.SettingsService, configModel.KeyStudentPhotosEnabled, false, rs.Logger)

	// Build and filter responses
	responses := rs.buildStudentResponses(r.Context(), students, params, accessCtx, dataSnapshot, photosEnabled)

	if !isToday {
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
	if err := rs.applyStatusDaysForDate(r.Context(), responses, planningDate.BerlinMidnight()); err != nil {
		slog.Default().Error("failed to apply student status days", slog.String("error", err.Error()))
		renderError(w, r, common.ErrorInternalServer(err))
		return
	}
	if err := rs.enrichWithDayPlanning(r.Context(), responses, planningDate, isToday, attendanceMapFromSnapshot(dataSnapshot)); err != nil {
		slog.Default().Error("failed to enrich student day planning", slog.String("error", err.Error()))
		renderError(w, r, common.ErrorInternalServer(err))
		return
	}
	responses = applyDayPlanningFilter(responses, params.dayStatus)
	// Administrative filters (#1492): bus / photo consent / pickup rule.
	// Applied here, before in-memory pagination, so server-side counts and
	// page boundaries reflect the filtered set (no client-side full-page
	// fetch needed).
	responses = applyAdministrativeFilters(responses, params.bus, params.photoConsent, params.pickupStatus)

	// Apply in-memory pagination if response-derived filters were used.
	if params.hasInMemoryFilters() {
		responses, totalCount = applyInMemoryPagination(responses, params.page, params.pageSize)
	}

	rs.enrichPaginatedPlanningTimes(r, responses, params, dataSnapshot, planningDate, isToday)

	// Companion ids ("läuft mit") for the day being SHOWN, not for today: the
	// grouping is per weekday, so a list rendered for another planning date must
	// resolve the links of that date. Fatal by design (see enrichWithCompanions)
	// — an empty grouping would be presented as a real departure arrangement.
	if err := rs.enrichWithCompanions(r.Context(), responses, params, planningDate.BerlinMidnight()); err != nil {
		renderError(w, r, common.ErrorInternalServer(err))
		return
	}

	common.RespondPaginated(w, r, http.StatusOK, responses, common.PaginationParams{Page: params.page, PageSize: params.pageSize, Total: totalCount}, "Students retrieved successfully")
}

// enrichPaginatedPlanningTimes layers the planning-date time data onto the final
// paginated slice: today's live check-in/out times (kept off any other day so
// current presence is never read as a plan), and, when requested, the effective
// pickup/arrival times for the date via a single bulk query per kind. Both skip
// redacted students — only rows the caller has full access to are enriched.
func (rs *Resource) enrichPaginatedPlanningTimes(r *http.Request, responses []StudentResponse, params *studentListParams, dataSnapshot *common.StudentDataSnapshot, planningDate timezone.Date, isToday bool) {
	if isToday {
		for i := range responses {
			if !responses[i].HasFullAccess {
				continue
			}
			applyActualTimesFromSnapshot(&responses[i], dataSnapshot)
		}
	}

	if !params.includePickupTimes && !params.includeArrivalTimes {
		return
	}
	fullAccessIDs := collectFullAccessStudentIDs(responses)
	if params.includePickupTimes {
		rs.enrichWithPickupTimes(r.Context(), responses, fullAccessIDs, planningDate.BerlinMidnight())
	}
	if params.includeArrivalTimes {
		rs.enrichWithArrivalTimes(r.Context(), responses, fullAccessIDs, planningDate.BerlinMidnight())
	}
}

// fetchStudentsForList fetches students based on the provided parameters. The
// location/room/group pre-filters each resolve a set of student IDs (or a fully
// materialized slice for the group-only fast path) before the standard query
// path applies school_class / guardian_name / pagination on top.
func (rs *Resource) fetchStudentsForList(r *http.Request, params *studentListParams) ([]*users.Student, int, error) {
	ctx := r.Context()

	switch {
	case params.locationState != "":
		nonEmpty, err := rs.resolveLocationStateFilter(ctx, params)
		if err != nil {
			return nil, 0, err
		}
		if !nonEmpty {
			return []*users.Student{}, 0, nil
		}
	case params.roomID > 0:
		nonEmpty, err := rs.resolveRoomFilter(ctx, params)
		if err != nil {
			return nil, 0, err
		}
		if !nonEmpty {
			return []*users.Student{}, 0, nil
		}
	case params.groupID > 0:
		students, totalCount, done, err := rs.resolveGroupFilter(ctx, params)
		if err != nil {
			return nil, 0, err
		}
		if done {
			return students, totalCount, nil
		}
	}

	return rs.runStandardStudentQuery(ctx, params)
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

	if params.groupID > 0 {
		ids, err = rs.filterStudentIDsByGroup(ctx, ids, params.groupID)
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
// path so school_class / guardian_name / pagination still apply. The visit join
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
	if params.groupID > 0 {
		filtered, err := rs.filterStudentIDsByGroup(ctx, ids, params.groupID)
		if err != nil {
			return false, err
		}
		if len(filtered) == 0 {
			return false, nil
		}
		ids = filtered
	}

	// params.groupID is intentionally NOT cleared even though buildBaseFilter
	// ignores it, because the room and group intersection was already computed
	// above, so re-applying group_id downstream would be redundant.
	params.studentIDs = ids
	return true, nil
}

// resolveGroupFilter handles the group-only path. When the request qualifies for
// the fast path it returns the materialized slice with done=true; otherwise it
// resolves params.studentIDs for the standard query and returns done=false.
// done=true with an empty slice signals a short-circuit empty page.
func (rs *Resource) resolveGroupFilter(ctx context.Context, params *studentListParams) ([]*users.Student, int, bool, error) {
	students, err := rs.PersonService.GetStudentsByGroupIDs(ctx, []int64{params.groupID})
	if err != nil {
		return nil, 0, false, err
	}

	// Fast path for true group-only requests keeps existing behavior.
	if params.canUseGroupOnlyShortcut() {
		return students, len(students), true, nil
	}

	if len(students) == 0 {
		return []*users.Student{}, 0, true, nil
	}

	ids := make([]int64, 0, len(students))
	for _, student := range students {
		if student != nil {
			ids = append(ids, student.ID)
		}
	}
	if len(ids) == 0 {
		return []*users.Student{}, 0, true, nil
	}

	params.studentIDs = ids
	return nil, 0, false, nil
}

// runStandardStudentQuery runs the SQL list/count path. buildBaseFilter picks up
// params.studentIDs (if set by a pre-filter above) and combines it with
// school_class / guardian_name and pagination.
func (rs *Resource) runStandardStudentQuery(ctx context.Context, params *studentListParams) ([]*users.Student, int, error) {
	queryOptions := params.buildQueryOptions()
	countOptions := params.buildCountOptions()

	totalCount, err := rs.StudentService.CountWithOptions(ctx, countOptions)
	if err != nil {
		return nil, 0, err
	}

	students, err := rs.StudentService.ListWithOptions(ctx, queryOptions)
	if err != nil {
		return nil, 0, err
	}

	return students, totalCount, nil
}

func (rs *Resource) listSchoolClasses(w http.ResponseWriter, r *http.Request) {
	classes, err := rs.StudentService.ListSchoolClasses(r.Context())
	if err != nil {
		renderError(w, r, common.ErrorInternalServer(err))
		return
	}
	common.Respond(w, r, http.StatusOK, classes, "School classes retrieved successfully")
}

// getStudent handles getting a student by ID
func (rs *Resource) getStudent(w http.ResponseWriter, r *http.Request) {
	student, ok := rs.parseAndGetStudent(w, r)
	if !ok {
		return
	}

	person, ok := rs.getPersonForStudent(w, r, student)
	if !ok {
		return
	}

	group := rs.getStudentGroup(r.Context(), student)
	hasFullAccess := rs.checkStudentReadAccess(r, student)
	hasWriteAccess := rs.checkStudentFullAccess(r, student)

	attendanceLogEnabled := configService.ResolveBoolOrDefault(r.Context(), rs.SettingsService, configModel.KeyAttendanceLogEnabled, false, rs.Logger)
	feedbackEnabled := configService.ResolveBoolOrDefault(r.Context(), rs.SettingsService, configModel.KeyFeedbackEnabled, false, rs.Logger)
	photosEnabled := configService.ResolveBoolOrDefault(r.Context(), rs.SettingsService, configModel.KeyStudentPhotosEnabled, false, rs.Logger)

	response := StudentDetailResponse{
		StudentResponse: newStudentResponseWithOpts(r.Context(), StudentResponseOpts{
			Student:       student,
			Person:        person,
			Group:         group,
			HasFullAccess: hasFullAccess,
			PhotosEnabled: photosEnabled,
		}, StudentResponseServices{
			ActiveService: rs.ActiveService,
			PersonService: rs.PersonService,
		}),
		HasFullAccess:        hasFullAccess,
		HasWriteAccess:       hasWriteAccess,
		AttendanceLogEnabled: attendanceLogEnabled,
		FeedbackEnabled:      feedbackEnabled,
	}
	now := rs.Now()
	rs.applyStatusDaysForDateToResponse(r.Context(), &response.StudentResponse, now)

	if hasFullAccess {
		attendanceStatus, err := rs.ActiveService.GetStudentAttendanceStatus(r.Context(), student.ID)
		if err != nil {
			rs.Logger.Warn("failed to resolve actual student arrival/pickup times",
				"student_id", student.ID,
				"error", err.Error(),
			)
		} else {
			applyActualTimesFromAttendance(&response.StudentResponse, attendanceStatus)
		}

		single := []StudentResponse{response.StudentResponse}
		if err := rs.enrichWithDayPlanning(r.Context(), single, timezone.DateFromTime(now), true, map[int64]*activeService.AttendanceStatus{
			student.ID: attendanceStatus,
		}); err != nil {
			renderError(w, r, common.ErrorInternalServer(err))
			return
		}
		response.StudentResponse = single[0]
	}

	// Add supervisor contacts for users without full access
	if !hasFullAccess && group != nil {
		response.GroupSupervisors = rs.buildSupervisorContacts(r.Context(), group.ID)
	}

	common.Respond(w, r, http.StatusOK, response, "Student retrieved successfully")
}

// createPersonFromStudentRequest creates a Person object from a StudentRequest
func createPersonFromStudentRequest(req *StudentRequest) (*users.Person, error) {
	person := &users.Person{
		FirstName: req.FirstName,
		LastName:  req.LastName,
	}

	// Set optional TagID if provided
	if req.TagID != "" {
		tagID := req.TagID
		person.TagID = &tagID
	}

	// Set optional Birthday if provided
	if req.Birthday != "" {
		parsedBirthday, err := timezone.ParseDate(req.Birthday)
		if err != nil {
			return nil, fmt.Errorf("invalid birthday format, expected YYYY-MM-DD: %w", err)
		}
		person.Birthday = &parsedBirthday
	}

	return person, nil
}

// createStudentFromRequest creates a Student object from a StudentRequest and personID
func createStudentFromRequest(req *StudentRequest, personID int64) *users.Student {
	student := &users.Student{
		PersonID:    personID,
		SchoolClass: req.SchoolClass,
	}

	// Set optional legacy guardian fields if provided
	if req.GuardianName != "" {
		name := req.GuardianName
		student.GuardianName = &name
	}
	if req.GuardianContact != "" {
		contact := req.GuardianContact
		student.GuardianContact = &contact
	}
	if req.GuardianEmail != "" {
		email := req.GuardianEmail
		student.GuardianEmail = &email
	}
	if req.GuardianPhone != "" {
		phone := req.GuardianPhone
		student.GuardianPhone = &phone
	}

	if req.GroupID != nil {
		student.GroupID = req.GroupID
	}
	if req.AddressStreet != "" {
		student.AddressStreet = strutil.TrimToNil(req.AddressStreet)
	}
	if req.AddressCity != "" {
		student.AddressCity = strutil.TrimToNil(req.AddressCity)
	}
	if req.AddressPostalCode != "" {
		student.AddressPostalCode = strutil.TrimToNil(req.AddressPostalCode)
	}
	if req.ExtraInfo != nil {
		student.ExtraInfo = req.ExtraInfo
	}
	if req.HealthInfo != nil {
		student.HealthInfo = req.HealthInfo
	}
	if req.SupervisorNotes != nil {
		student.SupervisorNotes = req.SupervisorNotes
	}
	if req.DepartureCompanionNote != nil {
		student.DepartureCompanionNote = req.DepartureCompanionNote
	}
	applyDeparturePlan(req.AllowedDepartureModes, req.DepartureDays, req.PickupStatus, req.PickupDays, req.Bus, req.BusDays, student)
	normalizeDepartureCompanionNote(student)

	return student
}

// normalizeDepartureCompanionNote drops the free-text "mit wem" note once the
// child's allowed departure modes no longer include the accompanied mode, so a
// note never outlives the "Mit anderem Kind" plan that justifies it (#1694).
// The UI hides the note input when no day is accompanied, so a stale value can
// otherwise sit in form state and be submitted unchanged.
func normalizeDepartureCompanionNote(student *users.Student) {
	if student.DepartureCompanionNote == nil {
		return
	}
	if !student.AllowedDepartureModes.HasMode(users.DepartureAccompanied) &&
		!student.DepartureDays.HasMode(users.DepartureAccompanied) {
		student.DepartureCompanionNote = nil
	}
}

// applyDeparturePlan sets how a child leaves each weekday from a create/update
// request. allowed_departure_modes is the rich source of truth when present.
// Legacy fields are passed through without rebuilding allowed_departure_modes,
// so stale older clients cannot collapse a stored multi-mode plan before the
// repository compares against current state.
func applyDeparturePlan(allowed *users.AllowedDepartureModes, departure *users.DepartureDays, status *string, pickupDays *users.PickupDays, legacyBus *bool, busDays *users.BusDays, student *users.Student) {
	if allowed == nil && departure == nil && status == nil && pickupDays == nil && legacyBus == nil && busDays == nil {
		return
	}
	if allowed != nil {
		modes := allowed.Normalize()
		student.AllowedDepartureModes = modes
		student.DepartureDays = modes.DepartureDays()
		student.BusDays = modes.BusDays()
		student.PickupDays = modes.PickupDays()
		// Full set, not the exclusive DepartureDays() projection: bus outranks
		// accompanied there, so a bus+accompanied day would bucket the child as a
		// self-goer. The repository re-derives this on persist; keep the handler
		// consistent so the in-memory student is never momentarily wrong (#1694).
		s := modes.LegacyPickupStatus()
		student.PickupStatus = &s
		return
	}
	if departure != nil {
		dd := departure.Normalize()
		student.DepartureDays = dd
		return
	}
	reconcilePickupFields(student, status, pickupDays)
	applyBusDays(legacyBus, busDays, student)
}

// applyBusDays sets the student's bus_days from a create/update request.
// bus_days is the single source of truth (#1582); the legacy bus boolean is
// accepted only as an alias (true => Mon–Fri, false => no days) and is ignored
// when bus_days is also supplied. The derived bus flag is no longer stored.
func applyBusDays(legacyBus *bool, days *users.BusDays, student *users.Student) {
	if days != nil {
		student.BusDays = *days
		return
	}
	if legacyBus == nil {
		return
	}
	switch {
	case !*legacyBus:
		// Explicitly off: clear all bus days.
		student.BusDays = users.BusDays{}
	case !student.BusDays.HasAny():
		// On with no existing per-day selection: default to all weekdays.
		student.BusDays = users.BusDaysFromLegacyFlag(true)
		// On with an existing per-day selection: preserve it (a legacy bus=true
		// must not flatten Mo/Fr into all weekdays).
	}
}

// reconcilePickupFields keeps student.PickupDays (the authoritative per-weekday
// map) and the legacy student.PickupStatus string in sync from a request that
// may carry either, both, or neither. When both are present the weekday map
// wins, since it is the granular source of truth.
//
// Contract for legacy (status-only) callers: pickup_status is treated as
// authoritative. A non-"Wird abgeholt" status therefore CLEARS any existing
// weekday map — a partial update that sends pickup_status without pickup_days
// will wipe previously stored pickup days. This is intentional (the legacy
// string has no per-day information to preserve), but it means new clients must
// always send pickup_days, never pickup_status alone, to mutate the map.
func reconcilePickupFields(student *users.Student, status *string, days *users.PickupDays) {
	if status != nil {
		student.PickupStatus = status
		if *status != users.PickupStatusPickedUp {
			student.PickupDays = users.PickupDays{}
		} else if !student.PickupDays.HasAny() {
			student.PickupDays = users.PickupDaysFromLegacyStatus(*status)
		}
	}
	if days != nil {
		student.PickupDays = *days
		s := student.PickupDays.LegacyPickupStatus()
		student.PickupStatus = &s
	}
}

// resolveScheduleStaffID resolves the acting staff for weekly schedules stamped
// with CreatedBy. It returns (0, nil) when the request carries no schedules so
// plain student creation is unaffected.
//
// Creating a student is governed by users:create, but attached weekly schedules
// are writes to the same Betreuungszeiten records edited by the standalone PUT
// endpoints. Keep that schedule write contract aligned: callers need
// users:update and must resolve to a staff record so schedule rows always carry
// a valid author. Both failure modes map to 403 at the call site.
func (rs *Resource) resolveScheduleStaffID(r *http.Request, req *StudentRequest) (int64, error) {
	if len(req.ArrivalSchedules) == 0 && len(req.PickupSchedules) == 0 {
		return 0, nil
	}
	if !authorize.HasPermission(permissions.UsersUpdate, jwt.PermissionsFromCtx(r.Context())) {
		return 0, errors.New("users:update permission required to create student schedules")
	}
	return rs.getStaffIDFromJWT(r)
}

// persistNewStudent writes the person, student, guardians, and weekly schedules
// atomically. Runs inside the caller's tenant transaction.
func (rs *Resource) persistNewStudent(ctx context.Context, person *users.Person, student *users.Student, guardians []userService.NewStudentGuardian, req *StudentRequest, staffID int64) error {
	// Validate guardians BEFORE writing the student. This route runs inside
	// TenantTxMiddleware, which only rolls back on 5xx; a guardian
	// ValidationError renders 400, so the middleware would otherwise commit
	// an already-created student. Validating first means a 400 commits an
	// empty transaction — no orphaned student/person rows.
	if len(guardians) > 0 {
		if err := rs.GuardianService.ValidateNewGuardians(ctx, guardians); err != nil {
			return err
		}
	}

	// Create person - validation occurs at the model layer
	if err := rs.PersonService.Create(ctx, person); err != nil {
		return err
	}

	// Create student with the person ID
	student.PersonID = person.ID
	if err := rs.StudentService.Create(ctx, student); err != nil {
		rs.cleanupPersonAfterStudentFailure(ctx, person.ID)
		return err
	}

	// Create any guardians supplied with the request inside the same
	// transaction so the student and its guardians are persisted
	// atomically — a guardian failure rolls back the whole student.
	if len(guardians) > 0 {
		if err := rs.GuardianService.AddGuardiansToStudent(ctx, student.ID, guardians); err != nil {
			return err
		}
	}

	return rs.persistNewStudentAttachments(ctx, student.ID, req, staffID)
}

// persistNewStudentAttachments writes the optional records that hang off a
// freshly created student: the weekly arrival/pickup schedules. They FK to the
// student, so they run in the same transaction as the create (mirrors the
// guardian handling above).
//
// Companion links ("läuft mit") are deliberately NOT accepted here: a link is
// only legal on a day BOTH children's departure plans allow it, and resolving
// that against a child that does not exist yet buys nothing. Linking is a
// follow-up action on the child's card.
func (rs *Resource) persistNewStudentAttachments(ctx context.Context, studentID int64, req *StudentRequest, staffID int64) error {
	if len(req.ArrivalSchedules) > 0 {
		arrivals := toArrivalScheduleModels(req.ArrivalSchedules, studentID, staffID)
		if err := rs.ArrivalScheduleService.UpsertBulkStudentArrivalSchedules(ctx, studentID, arrivals); err != nil {
			return err
		}
	}
	if len(req.PickupSchedules) > 0 {
		pickups := toPickupScheduleModels(req.PickupSchedules, studentID, staffID)
		if err := rs.PickupScheduleService.UpsertBulkStudentPickupSchedules(ctx, studentID, pickups); err != nil {
			return err
		}
	}
	return nil
}

// respondCreatedStudent writes the 201 response for a freshly created student.
func (rs *Resource) respondCreatedStudent(w http.ResponseWriter, r *http.Request, student *users.Student, person *users.Person) {
	// Get group data if student has a group
	group := rs.fetchStudentGroup(r.Context(), student.GroupID)

	// Admin users creating students can see full data including detailed location
	userPermissions := jwt.PermissionsFromCtx(r.Context())
	hasFullAccess := authorize.HasAdminWildcard(userPermissions)

	photosEnabled := configService.ResolveBoolOrDefault(r.Context(), rs.SettingsService, configModel.KeyStudentPhotosEnabled, false, rs.Logger)
	common.Respond(w, r, http.StatusCreated, newStudentResponseWithOpts(r.Context(), StudentResponseOpts{
		Student:       student,
		Person:        person,
		Group:         group,
		HasFullAccess: hasFullAccess,
		PhotosEnabled: photosEnabled,
	}, StudentResponseServices{
		ActiveService: rs.ActiveService,
		PersonService: rs.PersonService,
	}), "Student created successfully")
}

// createStudent handles creating a new student with their person record
func (rs *Resource) createStudent(w http.ResponseWriter, r *http.Request) {
	// Parse request
	req := &StudentRequest{}
	if err := render.Bind(r, req); err != nil {
		renderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	// Create person from request
	person, err := createPersonFromStudentRequest(req)
	if err != nil {
		renderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	// Create person and student in tenant transaction
	student := createStudentFromRequest(req, 0) // personID set after create
	guardians := guardiansAPI.ToNewStudentGuardians(req.Guardians)

	staffID, err := rs.resolveScheduleStaffID(r, req)
	if err != nil {
		renderError(w, r, common.ErrorForbidden(err))
		return
	}

	tenantID := tenant.FromContext(r.Context())
	if err := tenant.WithTenantTx(r.Context(), rs.DB, tenantID, func(ctx context.Context, _ bun.Tx) error {
		return rs.persistNewStudent(ctx, person, student, guardians, req, staffID)
	}); err != nil {
		// Bad guardian input (e.g. invalid email) is a client error: the
		// transaction has already rolled back, so no partial data survives.
		var validationErr *userService.ValidationError
		if errors.As(err, &validationErr) {
			renderError(w, r, common.ErrorInvalidRequest(validationErr))
			return
		}
		renderError(w, r, common.ErrorInternalServer(err))
		return
	}

	rs.respondCreatedStudent(w, r, student, person)
}

// cleanupPersonAfterStudentFailure removes the person record if student creation fails
func (rs *Resource) cleanupPersonAfterStudentFailure(ctx context.Context, personID int64) {
	if err := rs.PersonService.Delete(ctx, personID); err != nil {
		slog.Default().Error("failed to cleanup person after failed student creation",
			slog.Int64("person_id", personID),
			slog.String("error", err.Error()))
	}
}

// personUpdateResult contains the result of updating person fields
type personUpdateResult struct {
	updated bool
	err     error
}

// applyPersonUpdates applies person field changes from the request
// Returns whether any fields were updated and any error encountered
func applyPersonUpdates(req *UpdateStudentRequest, person *users.Person) personUpdateResult {
	result := personUpdateResult{}

	if req.FirstName != nil {
		person.FirstName = *req.FirstName
		result.updated = true
	}
	if req.LastName != nil {
		person.LastName = *req.LastName
		result.updated = true
	}
	if req.Birthday != nil {
		if *req.Birthday != "" {
			parsedBirthday, err := timezone.ParseDate(*req.Birthday)
			if err != nil {
				result.err = fmt.Errorf("invalid birthday format, expected YYYY-MM-DD: %w", err)
				return result
			}
			person.Birthday = &parsedBirthday
		} else {
			person.Birthday = nil
		}
		result.updated = true
	}
	if req.TagID != nil {
		if *req.TagID != "" {
			person.TagID = req.TagID
		} else {
			person.TagID = nil
		}
		result.updated = true
	}

	return result
}

// applyStudentFieldUpdates applies student field changes from the request
func applyStudentFieldUpdates(req *UpdateStudentRequest, student *users.Student) {
	if req.SchoolClass != nil {
		student.SchoolClass = *req.SchoolClass
	}
	applyGuardianUpdates(req, student)
	applyOptionalStudentFields(req, student)
	applySickStatus(req, student)
	applyExcusedStatus(req, student)
}

func reconcilePhotoConsentRequest(requested *bool, snapshot, fresh *users.Student) *bool {
	if requested == nil {
		return nil
	}
	snapshotHadConsent := snapshot != nil && snapshot.PhotoConsentGivenAt != nil
	freshHasConsent := fresh != nil && fresh.PhotoConsentGivenAt != nil

	// Treat values that merely echo the pre-transaction snapshot as no-ops.
	// Old clients used to serialize photo_consent_given on every PUT; if another
	// tab changed consent between the snapshot read and this row lock, replaying
	// that stale unchanged boolean would re-grant withdrawn consent or withdraw a
	// newly granted consent/photo.
	if *requested == snapshotHadConsent {
		return nil
	}

	// If a concurrent request already completed the intended transition, do not
	// re-stamp audit metadata or schedule duplicate photo cleanup.
	if *requested == freshHasConsent {
		return nil
	}

	return requested
}

// applyGuardianUpdates handles legacy guardian field updates
func applyGuardianUpdates(req *UpdateStudentRequest, student *users.Student) {
	if req.GuardianName != nil {
		trimmed := strings.TrimSpace(*req.GuardianName)
		if trimmed == "" {
			student.GuardianName = nil
		} else {
			student.GuardianName = &trimmed
		}
	}
	if req.GuardianContact != nil {
		trimmed := strings.TrimSpace(*req.GuardianContact)
		if trimmed == "" {
			student.GuardianContact = nil
		} else {
			student.GuardianContact = &trimmed
		}
	}
	if req.GuardianEmail != nil {
		student.GuardianEmail = req.GuardianEmail
	}
	if req.GuardianPhone != nil {
		student.GuardianPhone = req.GuardianPhone
	}
}

// applyOptionalStudentFields applies optional fields like GroupID, ExtraInfo, etc.
func applyOptionalStudentFields(req *UpdateStudentRequest, student *users.Student) {
	if req.GroupID != nil {
		student.GroupID = req.GroupID
	}
	if req.AddressStreet != nil {
		student.AddressStreet = strutil.TrimToNil(*req.AddressStreet)
	}
	if req.AddressCity != nil {
		student.AddressCity = strutil.TrimToNil(*req.AddressCity)
	}
	if req.AddressPostalCode != nil {
		student.AddressPostalCode = strutil.TrimToNil(*req.AddressPostalCode)
	}
	if req.ExtraInfo != nil {
		student.ExtraInfo = req.ExtraInfo
	}
	if req.HealthInfo != nil {
		student.HealthInfo = req.HealthInfo
	}
	if req.SupervisorNotes != nil {
		student.SupervisorNotes = req.SupervisorNotes
	}
	if req.DepartureCompanionNote != nil {
		student.DepartureCompanionNote = req.DepartureCompanionNote
	}
	applyDeparturePlan(req.AllowedDepartureModes, req.DepartureDays, req.PickupStatus, req.PickupDays, req.Bus, req.BusDays, student)
	// Only normalize when this request actually set the departure plan, so the
	// freshly applied modes are authoritative; an update that omits modes must
	// not clear a note against a possibly-unpopulated scanonly field.
	if req.AllowedDepartureModes != nil || req.DepartureDays != nil {
		normalizeDepartureCompanionNote(student)
	}
}

// applySickStatus handles sick status updates with SickSince timestamp logic
func applySickStatus(req *UpdateStudentRequest, student *users.Student) {
	if req.Sick == nil {
		return
	}
	student.Sick = req.Sick
	if *req.Sick {
		if student.SickSince == nil {
			now := time.Now()
			student.SickSince = &now
		}
	} else {
		student.SickSince = nil
	}
}

// applyExcusedStatus handles excused status updates with ExcusedSince timestamp logic
func applyExcusedStatus(req *UpdateStudentRequest, student *users.Student) {
	if req.Excused == nil {
		return
	}
	student.Excused = req.Excused
	if *req.Excused {
		if student.ExcusedSince == nil {
			now := time.Now()
			student.ExcusedSince = &now
		}
	} else {
		student.ExcusedSince = nil
	}
}

// checkSickExcusedConflict returns an error if the incoming update would
// result in both sick and excused being true simultaneously. Callers with a
// conflict should prompt the user to switch states rather than hold both.
func checkSickExcusedConflict(req *UpdateStudentRequest, student *users.Student) error {
	sickFinal := student.Sick != nil && *student.Sick
	if req.Sick != nil {
		sickFinal = *req.Sick
	}
	excusedFinal := student.Excused != nil && *student.Excused
	if req.Excused != nil {
		excusedFinal = *req.Excused
	}
	if sickFinal && excusedFinal {
		return errors.New("a student cannot be both sick and excused at the same time")
	}
	return nil
}

// broadcastStudentUpdated emits an SSE student_updated event to the
// AFFECTED tenant's connected clients only. use-global-sse.ts treats
// that event as "invalidate every student / room / dashboard cache in
// this tab", so a global fan-out (BroadcastToAll) would force tabs in
// schools B to N to refetch unrelated data whenever school A edits one
// student or one avatar. Routing via BroadcastToTenant, the helper
// already added in this branch for tenant_settings_changed, keeps the
// invalidation scoped to the school that actually changed.
//
// Callers MUST pass tenantID from request context (tenant.FromContext)
// or from a captured value inside an after-commit hook. Passing zero
// no-ops the broadcast rather than fanning out, since
// BroadcastToTenant rejects zero tenant IDs by definition (no Client
// has TenantID == 0).
func (rs *Resource) broadcastStudentUpdated(tenantID, studentID int64) {
	if rs.Broadcaster == nil {
		return
	}
	if tenantID <= 0 {
		// Defensive: a missing tenant context means we don't know which
		// school's clients should invalidate. Logging the case so a
		// future caller that forgets to thread tenantID through gets a
		// breadcrumb instead of silent loss.
		if rs.Logger != nil {
			rs.Logger.Warn(
				"skipping student_updated broadcast, no tenant context",
				"student_id", studentID,
			)
		}
		return
	}

	source := "manual"
	event := realtime.NewEvent(realtime.EventStudentUpdated, "", realtime.EventData{
		Source: &source,
	})

	if err := rs.Broadcaster.BroadcastToTenant(tenantID, event); err != nil && rs.Logger != nil {
		rs.Logger.Warn(
			"failed to broadcast student update",
			"tenant_id", tenantID,
			"student_id", studentID,
			"error", err.Error(),
		)
	}
}

// broadcastStudentCompanionsChanged tells the tenant's clients that a child's
// Laufgemeinschaft may have changed, so every mounted "läuft mit" view refetches
// (and an in-progress edit stops before it overwrites the change).
//
// Separate from student_updated on purpose: the links are symmetric, so a save
// on one child changes another child's card, and an editing form has to react by
// discarding or blocking its draft. Reacting that way to every student write —
// a photo, a name, a sick flag — would cost users their work for changes that
// never touched the links. Callers pass tenantID like broadcastStudentUpdated,
// and the fan-out is best-effort: a lost event costs a stale card, never data.
func (rs *Resource) broadcastStudentCompanionsChanged(tenantID, studentID int64) {
	if rs.Broadcaster == nil || tenantID <= 0 {
		return
	}

	source := "manual"
	event := realtime.NewEvent(realtime.EventStudentCompanionsChanged, "", realtime.EventData{
		Source: &source,
	})

	if err := rs.Broadcaster.BroadcastToTenant(tenantID, event); err != nil && rs.Logger != nil {
		rs.Logger.Warn(
			"failed to broadcast student companions change",
			"tenant_id", tenantID,
			"student_id", studentID,
			"error", err.Error(),
		)
	}
}

// wakeChildGuardians fans a message-INDEPENDENT parent_child_updated SSE event
// out to every guardian of the child, so an open parents-app tab refetches the
// child's care state live after a STAFF-side write (status day, pickup/arrival
// override, or weekly-plan change). Without it the parents portal — which only
// receives guardian-targeted events on its own SSE stream, never the tenant-wide
// student_updated / arrival_schedule_changed staff broadcasts — keeps showing a
// stale pickup time or presence until the parent refocuses or reloads (#1725).
//
// Schedule this from an after-commit hook (or otherwise only after the write has
// committed) so a woken client never reads the pre-commit snapshot. tenantID must
// come from tenant.FromContext (captured before the hook runs); a nil emitter or a
// non-positive tenant/student id is a safe no-op — the fan-out is best-effort,
// mirroring the existing broadcastStudentUpdated / excused-request pattern.
func (rs *Resource) wakeChildGuardians(tenantID, studentID int64) {
	if rs.ParentEventEmitter == nil {
		return
	}
	if tenantID <= 0 {
		if rs.Logger != nil {
			rs.Logger.Warn(
				"skipping guardian child-update wake, no tenant context",
				"student_id", studentID,
			)
		}
		return
	}
	rs.ParentEventEmitter.BroadcastChildUpdateToGuardians(tenantID, studentID)
}

// scheduleStudentUpdateWakes registers the after-commit SSE fan-out for a
// student update. It always broadcasts the tenant-wide student_updated staff
// event; it additionally wakes the child's guardians when the request actually
// touched a status field (sick/excused), because a sick/excused edit writes
// TODAY's status day — the exact signal the parent pickup tile resolves
// today_absent from — while student_updated never reaches the parents stream. A
// plain name/notes edit changes nothing parent-visible, so it wakes no one
// (#1725). Runs after the OUTER tx commits so a woken client never reads the
// pre-commit snapshot; tenantID is captured before the hook fires.
func (rs *Resource) scheduleStudentUpdateWakes(ctx context.Context, tenantID, studentID int64, req *UpdateStudentRequest, companionsChanged bool) {
	statusChanged := req.Sick != nil || req.Excused != nil
	tenant.RegisterAfterCommit(ctx, func() {
		rs.broadcastStudentUpdated(tenantID, studentID)
		// Only when the write actually changed the links (or a linked child's
		// departure plan) — see applyCompanionUpdate. An open Laufgemeinschaft
		// form reacts to this event by discarding or blocking the user's draft,
		// so firing it for a resubmitted, unchanged plan would cost somebody
		// their unsaved work for a change that never touched them.
		if companionsChanged {
			rs.broadcastStudentCompanionsChanged(tenantID, studentID)
		}
		if statusChanged {
			rs.wakeChildGuardians(tenantID, studentID)
		}
	})
}

// In-tx sentinel: a concurrent partial update committed between the pre-tx
// conflict check and the locked-row re-check produced the forbidden sick &&
// excused state. Mapped to 409 + SICK_EXCUSED_CONFLICT in the outer error
// switch so the frontend can prompt the user the same way it does for a
// synchronous conflict. errors.Is keeps the dispatch resilient to wrapping.
var errSickExcusedConflict = errors.New("sick and excused conflict on locked row")

// In-tx sentinel: a concurrent delete removed the student row between
// parseAndGetStudent and the locked re-read. The non-racy path returns 404;
// bare sql.ErrNoRows would otherwise fall through to a 500 for a legitimate
// concurrent delete.
var errStudentNotFoundUnderLock = errors.New("student deleted between snapshot and lock")

// In-tx sentinel: the pre-tx canUpdateStudent check ran on student.GroupID from
// the snapshot. canUpdateStudent decides off group membership, so a concurrent
// admin moving the student into a different group between snapshot and lock can
// leave the caller without write authority on the locked row. Re-checking
// against fresh closes that window. Mapped to 403 in the outer switch so the
// response status matches what the pre-tx gate would emit.
var errStudentReassigned = errors.New("student reassigned out of caller's scope mid-update")

// lockStudentForUpdate takes the row lock and re-validates every precondition
// against the LOCKED row rather than the pre-transaction snapshot, so a
// concurrent edit cannot slip a stale decision past. It writes nothing.
func (rs *Resource) lockStudentForUpdate(ctx context.Context, student *users.Student, req *UpdateStudentRequest, userPermissions []string) (*users.Student, error) {
	fresh, err := rs.StudentService.GetByIDForUpdate(ctx, student.ID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errStudentNotFoundUnderLock
		}
		return nil, err
	}

	// Re-check authorisation against the LOCKED row. A concurrent admin
	// reassignment could otherwise let a non-supervisor mutate the row.
	if ok, _ := canUpdateStudent(ctx, userPermissions, fresh, rs.UserContextService); !ok {
		return nil, errStudentReassigned
	}

	// Re-validate sick/excused on fresh: two concurrent partial updates
	// against a stale snapshot can otherwise merge into the forbidden
	// sick && excused state. TenantTxMiddleware only rolls back on 5xx,
	// so this 409-mapped sentinel must fire before any write below.
	if err := checkSickExcusedConflict(req, fresh); err != nil {
		return nil, errSickExcusedConflict
	}

	return fresh, nil
}

// applyStudentUpdate performs the locked-row student patch inside the caller's
// tenant transaction. Patch is applied to a freshly FOR-UPDATE-locked row so a
// concurrent photo upload can't have its photo_path clobbered by a stale
// snapshot write. It returns one of the package sentinel errors for the racy
// paths the outer switch maps to specific status codes.
// companionsChanged reports what the caller must forward to the client: whether
// this write actually changed the Laufgemeinschaft.
func (rs *Resource) applyStudentUpdate(ctx context.Context, tenantID int64, student *users.Student, person *users.Person, req *UpdateStudentRequest, userPermissions []string, personUpdated bool, statusHistoryNow time.Time) (bool, error) {
	// The photo-feature advisory lock comes FIRST, before any student row lock,
	// and only when consent is actually toggling (name/notes edits must not
	// queue behind a feature disable/purge). The photo upload and delete
	// transactions acquire LockPhotoFeature and THEN their row lock
	// (FindByIDForUpdate); taking a row lock here first would invert that order
	// and let a consent update and a concurrent upload deadlock each other.
	consentChanging := req.PhotoConsentGiven != nil &&
		(*req.PhotoConsentGiven) != (student.PhotoConsentGivenAt != nil)
	if consentChanging {
		if err := rs.StudentService.LockPhotoFeature(ctx); err != nil {
			return false, err
		}
	}

	// Before ANY row lock of this request: a companion update writes the linked
	// child too (a confirmed extension widens their departure plan), so subject
	// and companions have to be locked in one deterministic order or two
	// requests linking the same pair deadlock each other. Locking them here also
	// freezes the rows the companion authorization below judges, so the check
	// pass and the write pass cannot disagree. A request that cannot touch a
	// link takes none of those locks (see lockCompanionRows).
	if err := rs.lockCompanionRows(ctx, student, req); err != nil {
		return false, err
	}

	fresh, err := rs.lockStudentForUpdate(ctx, student, req, userPermissions)
	if err != nil {
		return false, err
	}

	// Read pre-update flags off fresh, not the snapshot, so status
	// history reflects the actual transition the commit will perform.
	// MUST happen before applyStudentFieldUpdates overwrites them.
	wasSick := boolPtrValue(fresh.Sick)
	wasExcused := boolPtrValue(fresh.Excused)

	// Apply the request to the locked row in memory FIRST. Nothing is written
	// yet, which is what lets the companion check below refuse the whole update
	// before any of it lands. updateStudent additionally marks the surrounding
	// tenant transaction for rollback on every error, so a late refusal cannot
	// leave a partial write behind either — checking first keeps the refusal
	// cheap, the rollback makes it safe.
	applyStudentFieldUpdates(req, fresh)
	authorizedExtensions, err := rs.checkCompanionConflicts(ctx, fresh, req, userPermissions)
	if err != nil {
		return false, err
	}

	if personUpdated {
		if err := rs.PersonService.Update(ctx, person); err != nil {
			return false, err
		}
	}

	effectiveConsent := reconcilePhotoConsentRequest(req.PhotoConsentGiven, student, fresh)
	rs.StudentPhotos.ApplyConsentTransition(ctx, effectiveConsent, fresh)

	if err := rs.persistStudentStatusHistory(ctx, fresh, wasSick, wasExcused, statusHistoryNow, strutil.TrimPtrToNil(req.SickReason)); err != nil {
		rs.logStatusHistoryError(student.ID, err)
		return false, err
	}
	// Companions BEFORE the student write: the link satisfies the
	// accompanied-requires-a-note invariant that Update validates, and a
	// conflict must abort the whole transaction before anything is persisted.
	companionsChanged, err := rs.applyCompanionUpdate(ctx, fresh, req, authorizedExtensions)
	if err != nil {
		return false, err
	}
	if err := rs.StudentService.Update(ctx, fresh); err != nil {
		return false, err
	}

	// Broadcast after the OUTER tx commits. Broadcasting now would race
	// subscribers into refetching the still-pre-commit row.
	rs.scheduleStudentUpdateWakes(ctx, tenantID, student.ID, req, companionsChanged)
	return companionsChanged, nil
}

// companionConflictRenderer returns the 409 payload when the transaction failed
// on a companion departure-plan mismatch, and nil otherwise.
func companionConflictRenderer(err error) render.Renderer {
	var conflictErr *companionConflictError
	if !errors.As(err, &conflictErr) {
		return nil
	}
	return &CompanionConflictResponse{
		Conflicts: conflictErr.Conflicts,
		Message:   "Der Heimweg des verknüpften Kindes erlaubt diese Tage noch nicht.",
	}
}

// updateStudentTxErrorRenderer maps the applyStudentUpdate transaction error to
// the wire response. The racy-path sentinels keep their specific status codes;
// everything else is a 500.
func updateStudentTxErrorRenderer(err error) render.Renderer {
	switch {
	case errors.Is(err, errSickExcusedConflict):
		return common.ErrorConflictWithCode(
			errors.New("a student cannot be both sick and excused at the same time"),
			ErrCodeSickExcusedConflict,
		)
	case errors.Is(err, errStudentReassigned):
		return common.ErrorForbidden(errors.New("you can only update students in groups you supervise"))
	case errors.Is(err, errStudentNotFoundUnderLock):
		return common.ErrorNotFound(errors.New("student not found"))
	// The merged plan (request modes applied onto the stored row) can violate
	// the accompanied-requires-note invariant — e.g. a caller sets a "Mit
	// anderem Kind" day on a child with no stored note. That is client input,
	// so surface it as a 400 rather than the model error leaking as a 500
	// (#1694). The binder cannot catch this on update: only here is the
	// stored note visible to fall back on.
	case errors.Is(err, users.ErrDepartureCompanionNoteRequired):
		return common.ErrorInvalidRequest(err)
	// A companion whose own departure plan does not allow the requested days.
	// Nothing was written (the transaction rolled back); the client asks the
	// user and may resend with extend_companion_plans.
	case companionConflictRenderer(err) != nil:
		return companionConflictRenderer(err)
	// The confirmation would widen a companion's own departure plan, which the
	// caller is not allowed to change. Refused before any write.
	case errors.Is(err, errCompanionExtendForbidden):
		return common.ErrorForbidden(err)
	// Companion input the client should not have sent: a day the child's own
	// plan does not allow, a duplicate, a self-link, an unknown child. All 4xx,
	// with the German sentinel text going straight to the UI.
	case errors.Is(err, userService.ErrCompanionNotFound):
		return common.ErrorNotFound(err)
	case errors.Is(err, userService.ErrCompanionDayNotAllowed),
		errors.Is(err, userService.ErrDuplicateCompanion),
		errors.Is(err, userService.ErrCompanionWeekdayRequired),
		errors.Is(err, userService.ErrTooManyCompanions),
		errors.Is(err, userService.ErrCompanionAtLimit),
		errors.Is(err, users.ErrCompanionSelfLink),
		errors.Is(err, users.ErrCompanionStudentIDRequired),
		errors.Is(err, users.ErrCompanionInvalidWeekday):
		return common.ErrorInvalidRequest(err)
	// The two sentinels every departure-plan write shares (stranded companion,
	// locked companion row) — classified once, in companionPlanErrorRenderer.
	case companionPlanErrorRenderer(err) != nil:
		return companionPlanErrorRenderer(err)
	default:
		return common.ErrorInternalServer(err)
	}
}

// respondUpdatedStudent re-reads the student and writes the 200 response,
// stamping the write's companion verdict onto it (see
// StudentResponse.CompanionsChanged).
func (rs *Resource) respondUpdatedStudent(w http.ResponseWriter, r *http.Request, studentID int64, person *users.Person, hasFullAccess, companionsChanged bool) {
	updatedStudent, err := rs.PersonService.GetStudentByID(r.Context(), studentID)
	if err != nil {
		renderError(w, r, common.ErrorInternalServer(err))
		return
	}

	group := rs.getStudentGroup(r.Context(), updatedStudent)

	photosEnabled := configService.ResolveBoolOrDefault(r.Context(), rs.SettingsService, configModel.KeyStudentPhotosEnabled, false, rs.Logger)
	response := newStudentResponseWithOpts(r.Context(), StudentResponseOpts{
		Student:       updatedStudent,
		Person:        person,
		Group:         group,
		HasFullAccess: hasFullAccess,
		PhotosEnabled: photosEnabled,
	}, StudentResponseServices{
		ActiveService: rs.ActiveService,
		PersonService: rs.PersonService,
	})
	response.CompanionsChanged = &companionsChanged
	common.Respond(w, r, http.StatusOK, response, "Student updated successfully")
}

// updateStudent handles updating an existing student
func (rs *Resource) updateStudent(w http.ResponseWriter, r *http.Request) {
	// Parse ID and get student
	student, ok := rs.parseAndGetStudent(w, r)
	if !ok {
		return
	}

	// Parse request
	req := &UpdateStudentRequest{}
	if err := render.Bind(r, req); err != nil {
		renderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	// Get existing person
	person, ok := rs.getPersonForStudent(w, r, student)
	if !ok {
		return
	}

	// Centralized permission check for updating student data
	userPermissions := jwt.PermissionsFromCtx(r.Context())
	authorized, authErr := canUpdateStudent(r.Context(), userPermissions, student, rs.UserContextService)
	if !authorized {
		renderError(w, r, common.ErrorForbidden(authErr))
		return
	}

	// Track whether the user is admin or group supervisor
	isAdmin := authorize.HasAdminWildcard(userPermissions)
	isGroupSupervisor := !isAdmin // If not admin but authorized, must be group supervisor

	// Update person fields using helper function
	personResult := applyPersonUpdates(req, person)
	if personResult.err != nil {
		renderError(w, r, common.ErrorInvalidRequest(personResult.err))
		return
	}

	// Reject updates that would leave the student in both sick and excused
	// states simultaneously. The frontend uses the SICK_EXCUSED_CONFLICT code
	// to prompt the user to switch states rather than hold both.
	if err := checkSickExcusedConflict(req, student); err != nil {
		renderError(w, r, common.ErrorConflictWithCode(err, ErrCodeSickExcusedConflict))
		return
	}

	statusHistoryNow := time.Now()

	tenantID := tenant.FromContext(r.Context())
	// The client needs the write's own verdict on the links (see
	// StudentResponse.CompanionsChanged); it is produced inside the transaction
	// and read after it, when the update is known to have succeeded.
	companionsChanged := false
	if err := tenant.WithTenantTx(r.Context(), rs.DB, tenantID, func(ctx context.Context, _ bun.Tx) error {
		changed, err := rs.applyStudentUpdate(ctx, tenantID, student, person, req, userPermissions, personResult.updated, statusHistoryNow)
		companionsChanged = changed
		return err
	}); err != nil {
		// This handler runs inside TenantTxMiddleware's transaction, and the
		// WithTenantTx above only REUSES it (tenant/tx.go) — returning an error
		// from the closure rolls nothing back, and the middleware commits on
		// every non-5xx response. applyStudentUpdate writes before its last
		// validation (companions, person, status history all land before
		// StudentService.Update can raise ErrDepartureCompanionNoteRequired), so
		// without this a refused PUT would answer 400/409 and still keep those
		// writes. A rejected update must leave nothing behind.
		tenant.MarkRollback(r.Context())
		renderError(w, r, updateStudentTxErrorRenderer(err))
		return
	}

	// Admin users and group supervisors can see full data including detailed location
	hasFullAccess := isAdmin || isGroupSupervisor
	rs.respondUpdatedStudent(w, r, student.ID, person, hasFullAccess, companionsChanged)
}

// deleteStudent handles deleting a student and their associated person record
func (rs *Resource) deleteStudent(w http.ResponseWriter, r *http.Request) {
	// Parse ID and get student
	student, ok := rs.parseAndGetStudent(w, r)
	if !ok {
		return
	}

	// Check if user has permission to delete this student
	userPermissions := jwt.PermissionsFromCtx(r.Context())
	authorized, authErr := canDeleteStudent(r.Context(), userPermissions, student, rs.UserContextService)
	if !authorized {
		renderError(w, r, common.ErrorForbidden(authErr))
		return
	}

	tenantID := tenant.FromContext(r.Context())
	if err := tenant.WithTenantTx(r.Context(), rs.DB, tenantID, func(ctx context.Context, _ bun.Tx) error {
		return rs.deleteStudentTx(ctx, tenantID, student)
	}); err != nil {
		// Same reason as updateStudent: the surrounding transaction belongs to
		// the middleware and commits on every non-5xx response, so the 409 paths
		// below have to request the rollback themselves.
		tenant.MarkRollback(r.Context())
		renderError(w, r, deleteStudentTxErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, nil, "Student deleted successfully")
}

// In-tx sentinel: the child graduated between parseAndGetStudent's alumnus gate
// and the locked re-read below. Mapped to the same 404 that gate returns, so the
// racy path and the ordinary one answer identically.
var errStudentGraduatedUnderLock = errors.New("student graduated between snapshot and lock")

// deleteStudentTx performs the locked student delete inside the caller's
// tenant transaction.
//
// Deleting the row also deletes every "läuft mit" edge via ON DELETE CASCADE —
// a removal that reaches into OTHER children's records like any list edit
// does. So it joins the shared lock protocol first (subject plus every linked
// companion, ascending), then refuses when a surviving child would be left
// with an accompanied plan and no "mit wem" detail; otherwise the cascade
// would silently bypass the removal protection and every later edit of that
// child would fail on the note invariant.
//
// The photo path is captured from the locked row (not the pre-tx snapshot) so
// a concurrent upload can't orphan a new file on disk; the unlink itself runs
// after the OUTER tenant tx commits.
func (rs *Resource) deleteStudentTx(ctx context.Context, tenantID int64, student *users.Student) error {
	if err := rs.lockStudentCompanionGraph(ctx, student.ID, nil); err != nil {
		return err
	}
	if err := rs.StudentService.CheckCompanionTrim(ctx, student.ID, nil); err != nil {
		return err
	}

	// Read under the graph lock, before the row goes: every edge of this child is
	// a row on ANOTHER child's card, and ON DELETE CASCADE takes it with the
	// student. Without the announcement below the surviving children's companion
	// cards keep listing a child that no longer exists, and an open editor keeps
	// working from a snapshot the delete already invalidated.
	companionIDs, err := rs.StudentService.ListCompanionIDs(ctx, student.ID)
	if err != nil {
		return err
	}

	// FOR UPDATE row-locks against any in-flight upload tx. We either
	// observe its committed photo_path or it sees our deleted row and
	// aborts.
	fresh, err := rs.StudentService.GetByIDForUpdate(ctx, student.ID)
	if err != nil {
		return err
	}

	// The pre-transaction alumnus gate (parseAndGetStudent) ran on a snapshot. A
	// grade transition that commits between that read and this lock turns the
	// subject into a soft-deleted graduate, and a delete that proceeds anyway
	// HARD-deletes the very row graduation preserved: the student and person rows
	// are gone, so a transition revert can never bring that child back and their
	// history is lost for good. Under the row lock the two serialize — either we
	// hold the row and the transition waits for a child that no longer exists, or
	// it committed first and we see the alumnus status and refuse (#405 review).
	if fresh.IsAlumnus() {
		return errStudentGraduatedUnderLock
	}

	var photoToRemove string
	if fresh.PhotoPath != nil {
		photoToRemove = *fresh.PhotoPath
	}

	if err := rs.StudentService.Delete(ctx, student.ID); err != nil {
		return err
	}

	// Person delete failure must not fail the request. The student row
	// is already gone, leaving the person orphaned is recoverable.
	if err := rs.PersonService.Delete(ctx, student.PersonID); err != nil {
		slog.Default().Error("failed to delete associated person record",
			slog.Int64("person_id", student.PersonID),
			slog.String("error", err.Error()))
	}

	rs.StudentPhotos.ScheduleUnlinkAfterCommit(ctx, photoToRemove)

	// After the OUTER tx commits, like every other companion broadcast: a
	// subscriber woken earlier would refetch the still-present row.
	if len(companionIDs) > 0 {
		studentID := student.ID
		tenant.RegisterAfterCommit(ctx, func() {
			rs.broadcastStudentCompanionsChanged(tenantID, studentID)
		})
	}
	return nil
}

// deleteStudentTxErrorRenderer maps the deleteStudentTx transaction error to
// the wire response. The caller has requested the rollback (tenant.MarkRollback),
// so nothing this transaction touched is committed.
func deleteStudentTxErrorRenderer(err error) render.Renderer {
	switch {
	// The child graduated while this request was in flight. Answered with the
	// same 404 the shared alumnus gate returns, so a delete never depends on
	// which of the two transactions won the race.
	case errors.Is(err, errStudentGraduatedUnderLock):
		return common.ErrorNotFound(errors.New("student not found"))
	// A linked child would be stranded (accompanied plan, no note, no other
	// link). The German sentinel text tells the user which precondition to
	// fix first.
	case errors.Is(err, userService.ErrCompanionWouldLoseDeparture):
		return common.ErrorConflictMessage(err.Error())
	// A linked child was locked by a concurrent edit this transaction could not
	// safely wait for. Retriable, so it must not read as a server error.
	case errors.Is(err, userService.ErrCompanionLockBusy):
		return common.ErrorConflictMessage(err.Error())
	case common.IsConstraintViolation(err):
		return common.ErrorConflictMessage("Kind kann nicht gelöscht werden: Kind hat aktive Besuche, Einschreibungen oder andere verknüpfte Daten")
	default:
		return common.ErrorInternalServer(err)
	}
}
