package students

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/departure"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/internal/strutil"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/legacy/jwt"
	peopleModule "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// createPersonFromStudentRequest creates a Person object from a StudentRequest
func createPersonFromStudentRequest(req *StudentRequest) (*peopleModule.Person, error) {
	person := &peopleModule.Person{
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
		person.Birthday = parsedBirthday.String()
	}

	return person, nil
}

// createStudentFromRequest creates a Student object from a StudentRequest and personID
func createStudentFromRequest(req *StudentRequest, personID int64) *Student {
	student := &Student{
		PersonID:    personID,
		SchoolClass: req.SchoolClass,
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
func normalizeDepartureCompanionNote(student *Student) {
	if student.DepartureCompanionNote == nil {
		return
	}
	if !student.AllowedDepartureModes.HasMode(departure.DepartureAccompanied) &&
		!student.DepartureDays.HasMode(departure.DepartureAccompanied) {
		student.DepartureCompanionNote = nil
	}
}

// applyDeparturePlan sets how a child leaves each weekday from a create/update
// request. allowed_departure_modes is the rich source of truth when present.
// Legacy fields are passed through without rebuilding allowed_departure_modes,
// so stale older clients cannot collapse a stored multi-mode plan before the
// repository compares against current state.
func applyDeparturePlan(allowed *departure.AllowedDepartureModes, departure *departure.DepartureDays, status *string, pickupDays *departure.PickupDays, legacyBus *bool, busDays *departure.BusDays, student *Student) {
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
func applyBusDays(legacyBus *bool, days *departure.BusDays, student *Student) {
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
		student.BusDays = departure.BusDays{}
	case !student.BusDays.HasAny():
		// On with no existing per-day selection: default to all weekdays.
		student.BusDays = departure.BusDaysFromLegacyFlag(true)
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
func reconcilePickupFields(student *Student, status *string, days *departure.PickupDays) {
	if status != nil {
		student.PickupStatus = status
		if *status != departure.PickupStatusPickedUp {
			student.PickupDays = departure.PickupDays{}
		} else if !student.PickupDays.HasAny() {
			student.PickupDays = departure.PickupDaysFromLegacyStatus(*status)
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
	if !securityruntime.HasPermission(permissions.UsersUpdate, jwt.PermissionsFromCtx(r.Context())) {
		return 0, errors.New("users:update permission required to create student schedules")
	}
	return rs.getStaffIDFromJWT(r)
}

// persistNewStudent writes the person, student, guardians, and weekly schedules
// atomically. Runs inside the caller's tenant transaction.
func (rs *Resource) persistNewStudent(ctx context.Context, person *peopleModule.Person, student *Student, guardians []peopleModule.NewStudentGuardian, req *StudentRequest, staffID int64) error {
	// Validate guardians BEFORE writing the student. This route runs inside
	// TenantTxMiddleware, which only rolls back on 5xx; a guardian
	// ValidationError renders 400, so the middleware would otherwise commit
	// an already-created student. Validating first means a 400 commits an
	// empty transaction — no orphaned student/person rows.
	if len(guardians) > 0 {
		if err := rs.PeopleDirectory.ValidateNewGuardians(ctx, guardians); err != nil {
			return err
		}
	}

	// Create person - the retained person service validates it
	created, err := rs.Persons.CreatePerson(ctx, peopleModule.CreatePerson{
		FirstName: person.FirstName, LastName: person.LastName,
		Birthday: person.Birthday, TagID: person.TagID, AccountID: person.AccountID,
	})
	if err != nil {
		return err
	}
	*person = created

	// Create student with the person ID
	student.PersonID = person.ID
	if err := rs.saveStudent(ctx, student, true); err != nil {
		rs.cleanupPersonAfterStudentFailure(ctx, person.ID)
		return err
	}

	// Create any guardians supplied with the request inside the same
	// transaction so the student and its guardians are persisted
	// atomically — a guardian failure rolls back the whole student.
	if len(guardians) > 0 {
		if err := rs.PeopleDirectory.AddGuardiansToStudent(ctx, student.ID, guardians); err != nil {
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
func (rs *Resource) respondCreatedStudent(w http.ResponseWriter, r *http.Request, student *Student, person *peopleModule.Person) {
	// Get group data if student has a group
	group := rs.fetchStudentGroup(r.Context(), student.GroupID)

	// Admin users creating students can see full data including detailed location
	userPermissions := jwt.PermissionsFromCtx(r.Context())
	hasFullAccess := securityruntime.HasAdminWildcard(userPermissions)

	photosEnabled := resolveBoolSetting(r.Context(), rs.SettingsService, settingStudentPhotosEnabled, false, rs.Logger)
	response, err := newStudentResponseWithOpts(r.Context(), StudentResponseOpts{
		Student:       student,
		Person:        person,
		Group:         group,
		HasFullAccess: hasFullAccess,
		PhotosEnabled: photosEnabled,
	}, StudentResponseServices{
		ActiveService: rs.ActiveService,
	})
	if err != nil {
		renderError(w, r, common.ErrorInternalServer(err))
		return
	}
	common.Respond(w, r, http.StatusCreated, response, "Student created successfully")
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
	guardians := req.Guardians

	staffID, err := rs.resolveScheduleStaffID(r, req)
	if err != nil {
		renderError(w, r, common.ErrorForbidden(err))
		return
	}

	tenantID := tenant.FromContext(r.Context())
	if err := withinTenant(r.Context(), tenantID, func(ctx context.Context) error {
		return rs.persistNewStudent(ctx, person, student, guardians, req, staffID)
	}); err != nil {
		// Bad guardian input (e.g. invalid email) is a client error: the
		// transaction has already rolled back, so no partial data survives.
		if errors.Is(err, peopleModule.ErrInvalidGuardian) {
			renderError(w, r, common.ErrorInvalidRequest(err))
			return
		}
		// A full Kinderkontingent (#3567) is a business rejection with
		// its own code; the form keeps its input and shows the numbers.
		if common.IsBusinessRejection(err) {
			renderError(w, r, common.ErrorBusinessRejection(err))
			return
		}
		renderError(w, r, common.ErrorInternalServer(err))
		return
	}

	rs.respondCreatedStudent(w, r, student, person)
}

// cleanupPersonAfterStudentFailure removes the person record if student creation fails
func (rs *Resource) cleanupPersonAfterStudentFailure(ctx context.Context, personID int64) {
	if err := rs.Persons.DeletePerson(ctx, personID); err != nil {
		slog.Default().Error("failed to cleanup person after failed student creation",
			slog.Int64("person_id", personID),
			slog.String("error", err.Error()))
	}
}
