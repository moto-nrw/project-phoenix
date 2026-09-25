package students

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/departure"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/internal/strutil"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/legacy/jwt"
	peopleModule "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// personUpdateResult contains the result of updating person fields
type personUpdateResult struct {
	updated bool
	err     error
}

// applyPersonUpdates applies person field changes from the request
// Returns whether any fields were updated and any error encountered
func applyPersonUpdates(req *UpdateStudentRequest, person *peopleModule.Person) personUpdateResult {
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
			person.Birthday = parsedBirthday.String()
		} else {
			person.Birthday = ""
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
func applyStudentFieldUpdates(req *UpdateStudentRequest, student *Student) {
	if req.SchoolClass != nil {
		student.SchoolClass = *req.SchoolClass
	}
	applyOptionalStudentFields(req, student)
	applySickStatus(req, student)
	applyExcusedStatus(req, student)
}

func reconcilePhotoConsentRequest(requested *bool, snapshot, fresh *Student) *bool {
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

// applyOptionalStudentFields applies optional fields like GroupID, ExtraInfo, etc.
func applyOptionalStudentFields(req *UpdateStudentRequest, student *Student) {
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
func applySickStatus(req *UpdateStudentRequest, student *Student) {
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
func applyExcusedStatus(req *UpdateStudentRequest, student *Student) {
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
func checkSickExcusedConflict(req *UpdateStudentRequest, student *Student) error {
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

// In-tx sentinel: the pre-tx authorizeStudentUpdate check ran on the snapshot
// row. Re-checking against the locked row keeps the payload-aware permission
// decision (users:update vs absence-only, #2232) honest under concurrency.
// Mapped to 403 in the outer switch so the response status matches what the
// pre-tx gate would emit.
var errStudentReassigned = errors.New("caller lost write authority on this student mid-update")

// lockStudentForUpdate takes the row lock and re-validates every precondition
// against the LOCKED row rather than the pre-transaction snapshot, so a
// concurrent edit cannot slip a stale decision past. It writes nothing.
func (rs *Resource) lockStudentForUpdate(ctx context.Context, student *Student, req *UpdateStudentRequest, userPermissions []string) (*Student, error) {
	fresh, err := rs.lockStudent(ctx, student.ID)
	if err != nil {
		if isMissingStudent(err) {
			return nil, errStudentNotFoundUnderLock
		}
		return nil, err
	}

	// Re-check authorisation against the LOCKED row. A concurrent admin
	// reassignment could otherwise let a non-supervisor mutate the row. Uses the
	// same payload-aware gate as the pre-tx check so an absence-only write that
	// was authorized by the open-care absence gate is not rejected here (#2232).
	if _, ok, _ := rs.authorizeStudentUpdate(ctx, userPermissions, fresh, req); !ok {
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

// acquirePreRowLockGates takes the advisory locks that must precede ANY
// student row lock of this request. It returns whether a class-change resync
// may be owed after the write (see resyncSourcedTemplatesOnClassChange).
//
// Photo feature: the lock is taken only when consent is actually toggling
// (name/notes edits must not queue behind a feature disable/purge). The photo
// upload and delete transactions acquire LockPhotoFeature and THEN their row
// lock (FindByIDForUpdate); taking a row lock first would invert that order
// and let a consent update and a concurrent upload deadlock each other.
//
// Recurrence gate: a school_class edit moves the child between Jahrgängen, so
// the Jahrgang-filtered offering-sourced Regeltermine must be re-reconciled in
// the same transaction, exactly like a grade transition (#2147 review round
// 10). The gates are taken BEFORE any student row lock, in the project-wide
// grade-transition order (shared class-writes gate first, recurrence gate
// second, row locks last) — any other order can deadlock against a
// concurrently applying transition. They are taken whenever the request
// carries a class at all, because whether the class actually changes is only
// known once the row is locked.
func (rs *Resource) acquirePreRowLockGates(ctx context.Context, student *Student, req *UpdateStudentRequest) (classChangeRequested bool, err error) {
	consentChanging := req.PhotoConsentGiven != nil &&
		(*req.PhotoConsentGiven) != (student.PhotoConsentGivenAt != nil)
	if consentChanging {
		if err := rs.PeopleDirectory.LockStudentPhotoFeature(ctx); err != nil {
			return false, err
		}
	}

	if req.SchoolClass == nil || rs.OfferingSourceResyncer == nil {
		return false, nil
	}
	if rs.LockTemplateRecurrence == nil {
		return false, errors.New("template recurrence lock is not configured")
	}
	// Shared class-writes gate BEFORE the recurrence gate: a grade transition
	// takes the class-writes gate exclusively FIRST and then the recurrence
	// gate (lockRecurrenceThenTransitions). Taking recurrence here and the
	// shared class gate only later (implicitly, inside FindByIDForUpdate)
	// would let the two transactions wait on each other cyclically until
	// PostgreSQL aborts one (#2147 review round 12). Re-entrant, so the
	// row-lock methods' implicit shared acquisition stays a no-op.
	if err := rs.PeopleDirectory.LockEnrollmentClassWrites(ctx); err != nil {
		return false, err
	}
	if err := rs.LockTemplateRecurrence(ctx); err != nil {
		return false, err
	}
	return true, nil
}

// resyncSourcedTemplatesOnClassChange follows the child into the new Jahrgang
// on every offering-sourced template once the locked row's class actually
// changed. Runs under the recurrence gate acquirePreRowLockGates took; a
// resync failure aborts the whole update instead of committing a class edit
// whose sourced rosters are stale (#2147 review round 10).
func (rs *Resource) resyncSourcedTemplatesOnClassChange(ctx context.Context, classChangeRequested bool, previousSchoolClass, currentSchoolClass string) error {
	if !classChangeRequested || previousSchoolClass == currentSchoolClass {
		return nil
	}
	return rs.OfferingSourceResyncer.ResyncOfferingSourcedTemplates(ctx, rs.todayDate())
}

// studentUpdate is one locked-row student patch in flight: the request, the
// row as locked and patched in memory, and what the write steps need to know
// about the row before the patch.
type studentUpdate struct {
	tenantID             int64
	snapshot             *Student
	fresh                *Student
	before               Student
	person               *peopleModule.Person
	req                  *UpdateStudentRequest
	personUpdated        bool
	now                  time.Time
	wasSick              bool
	wasExcused           bool
	previousSchoolClass  string
	classChangeRequested bool
	authorizedExtensions map[int64]map[string]bool
}

// applyStudentUpdate performs the locked-row student patch inside the caller's
// tenant transaction. Patch is applied to a freshly FOR-UPDATE-locked row so a
// concurrent photo upload can't have its photo_path clobbered by a stale
// snapshot write. It returns one of the package sentinel errors for the racy
// paths the outer switch maps to specific status codes.
// companionsChanged reports what the caller must forward to the client: whether
// this write actually changed the Laufgemeinschaft.
func (rs *Resource) applyStudentUpdate(ctx context.Context, tenantID int64, student *Student, person *peopleModule.Person, req *UpdateStudentRequest, userPermissions []string, personUpdated bool, statusHistoryNow time.Time) (bool, error) {
	update := &studentUpdate{
		tenantID: tenantID, snapshot: student, person: person, req: req,
		personUpdated: personUpdated, now: statusHistoryNow,
	}
	if err := rs.lockStudentUpdateRows(ctx, update, userPermissions); err != nil {
		return false, err
	}
	fresh := update.fresh

	// Read pre-update flags off fresh, not the snapshot, so status
	// history reflects the actual transition the commit will perform.
	// MUST happen before applyStudentFieldUpdates overwrites them.
	update.wasSick = boolPtrValue(fresh.Sick)
	update.wasExcused = boolPtrValue(fresh.Excused)
	update.previousSchoolClass = fresh.SchoolClass

	// Snapshot the tracked profile fields before applying the patch so the
	// audit diff (#1455) compares the locked pre-update row with the persisted
	// result. The tracked pointer/map fields are replaced, not mutated in place,
	// so a shallow copy is a safe before-image. Normalize only the copy: legacy
	// rows may carry their effective plan solely in bus_days/pickup_days, while
	// mutating fresh before applyStudentFieldUpdates could change persistence
	// precedence.
	update.before = studentAuditBeforeImage(fresh, req.hasDeparturePlanUpdate())

	// Apply the request to the locked row in memory FIRST. Nothing is written
	// yet, which is what lets the companion check below refuse the whole update
	// before any of it lands. updateStudent additionally marks the surrounding
	// tenant transaction for rollback on every error, so a late refusal cannot
	// leave a partial write behind either — checking first keeps the refusal
	// cheap, the rollback makes it safe.
	applyStudentFieldUpdates(req, fresh)
	reportedStatus := newlyReportedAbsenceStatus(fresh, update.wasSick, update.wasExcused)
	authorizedExtensions, err := rs.checkCompanionConflicts(ctx, fresh, req, userPermissions)
	if err != nil {
		return false, err
	}
	update.authorizedExtensions = authorizedExtensions

	companionsChanged, err := rs.writeStudentUpdate(ctx, update)
	if err != nil {
		return false, err
	}

	// Broadcast after the OUTER tx commits. Broadcasting now would race
	// subscribers into refetching the still-pre-commit row.
	if err := rs.scheduleStudentUpdateWakes(ctx, tenantID, student.ID, req, companionsChanged, reportedStatus, timezone.DateFromTime(statusHistoryNow)); err != nil {
		return false, err
	}
	return companionsChanged, nil
}

// lockStudentUpdateRows takes every lock the update writes under, in the
// project-wide order, and reads the locked row.
func (rs *Resource) lockStudentUpdateRows(ctx context.Context, update *studentUpdate, userPermissions []string) error {
	// Advisory locks that must precede ANY student row lock of this request —
	// see the helper for the two ordering rationales.
	classChangeRequested, err := rs.acquirePreRowLockGates(ctx, update.snapshot, update.req)
	if err != nil {
		return err
	}
	update.classChangeRequested = classChangeRequested

	// Before ANY row lock of this request: a companion update writes the linked
	// child too (a confirmed extension widens their departure plan), so subject
	// and companions have to be locked in one deterministic order or two
	// requests linking the same pair deadlock each other. Locking them here also
	// freezes the rows the companion authorization below judges, so the check
	// pass and the write pass cannot disagree. A request that cannot touch a
	// link takes none of those locks (see lockCompanionRows).
	if err := rs.lockCompanionRows(ctx, update.snapshot, update.req); err != nil {
		return err
	}

	fresh, err := rs.lockStudentForUpdate(ctx, update.snapshot, update.req, userPermissions)
	if err != nil {
		return err
	}
	update.fresh = fresh
	return nil
}

// writeStudentUpdate persists the patched row and everything that has to land
// with it in the same transaction: the person, the photo consent, the status
// history, the companions, the consent trail, the class resync and the change
// history.
func (rs *Resource) writeStudentUpdate(ctx context.Context, update *studentUpdate) (bool, error) {
	fresh, req := update.fresh, update.req
	if update.personUpdated {
		person := update.person
		updated, err := rs.Persons.UpdatePerson(ctx, peopleModule.UpdatePerson{
			ID: person.ID, FirstName: person.FirstName, LastName: person.LastName,
			Birthday: person.Birthday, TagID: person.TagID, AccountID: person.AccountID,
		})
		if err != nil {
			return false, err
		}
		*person = updated
	}

	effectiveConsent := reconcilePhotoConsentRequest(req.PhotoConsentGiven, update.snapshot, fresh)
	rs.applyPhotoConsent(ctx, effectiveConsent, fresh)

	if err := rs.persistStudentStatusHistory(ctx, fresh, update.wasSick, update.wasExcused, update.now, strutil.TrimPtrToNil(req.SickReason)); err != nil {
		rs.logStatusHistoryError(update.snapshot.ID, err)
		return false, err
	}
	// Companions BEFORE the student write: the link satisfies the
	// accompanied-requires-a-note invariant that Update validates, and a
	// conflict must abort the whole transaction before anything is persisted.
	companionsChanged, err := rs.applyCompanionUpdate(ctx, fresh, req, update.authorizedExtensions)
	if err != nil {
		return false, err
	}
	if err := rs.saveStudent(ctx, fresh, false); err != nil {
		return false, err
	}
	if err := rs.recordConsentTransition(ctx, effectiveConsent, &update.before, fresh, update.now); err != nil {
		return false, err
	}

	if err := rs.resyncSourcedTemplatesOnClassChange(ctx, update.classChangeRequested, update.previousSchoolClass, fresh.SchoolClass); err != nil {
		return false, err
	}

	if req.hasDeparturePlanUpdate() {
		// Audit the effective unified plan even when a legacy client changed
		// only bus_days/pickup_days. The concrete repository currently writes
		// its resolved plan back into fresh, but the service contract does not
		// promise that mutation.
		normalizeDeparturePlanForAudit(fresh)
	}

	// Keep the audit rows atomic with the student write. A failed audit insert
	// aborts the surrounding tenant transaction instead of committing an
	// unlogged profile edit.
	if err := rs.recordStudentChanges(ctx, &update.before, fresh); err != nil {
		return false, err
	}
	return companionsChanged, nil
}

func (rs *Resource) recordConsentTransition(ctx context.Context, effectiveConsent *bool, before, after *Student, changedAt time.Time) error {
	if effectiveConsent == nil || rs.StudentConsents == nil {
		return nil
	}
	return rs.StudentConsents.RecordStudentConsentTransitions(
		ctx,
		before.consentSnapshot(),
		after.consentSnapshot(),
		consentSourceTenantPortal,
		jwt.ActorAccountIDFromCtx(ctx),
		changedAt,
	)
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
		return common.ErrorForbidden(errors.New("insufficient permissions to update this student's data"))
	case errors.Is(err, errStudentNotFoundUnderLock):
		return common.ErrorNotFound(errors.New("student not found"))
	case errors.Is(err, studentpresence.ErrStudentStatusDayPartialAbsenceConflict):
		return common.ErrorConflictWithCode(err, "partial_absence_conflict")
	// The merged plan (request modes applied onto the stored row) can violate
	// the accompanied-requires-note invariant — e.g. a caller sets a "Mit
	// anderem Kind" day on a child with no stored note. That is client input,
	// so surface it as a 400 rather than the model error leaking as a 500
	// (#1694). The binder cannot catch this on update: only here is the
	// stored note visible to fall back on.
	case errors.Is(err, departure.ErrDepartureCompanionNoteRequired):
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
	case errors.Is(err, careplan.ErrCompanionNotFound):
		return common.ErrorNotFound(err)
	case errors.Is(err, careplan.ErrCompanionDayNotAllowed),
		errors.Is(err, careplan.ErrDuplicateCompanion),
		errors.Is(err, careplan.ErrCompanionWeekdayRequired),
		errors.Is(err, careplan.ErrTooManyCompanions),
		errors.Is(err, careplan.ErrCompanionAtLimit),
		errors.Is(err, careplan.ErrCompanionSelfLink), errors.Is(err, departure.ErrCompanionSelfLink),
		errors.Is(err, careplan.ErrCompanionStudentIDRequired), errors.Is(err, departure.ErrCompanionStudentIDRequired),
		errors.Is(err, careplan.ErrCompanionInvalidWeekday), errors.Is(err, departure.ErrCompanionInvalidWeekday):
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
func (rs *Resource) respondUpdatedStudent(w http.ResponseWriter, r *http.Request, studentID int64, person *peopleModule.Person, hasFullAccess, companionsChanged bool) {
	updatedStudent, err := rs.findStudent(r.Context(), studentID)
	if err != nil {
		renderError(w, r, common.ErrorInternalServer(err))
		return
	}

	group := rs.getStudentGroup(r.Context(), updatedStudent)

	photosEnabled := resolveBoolSetting(r.Context(), rs.SettingsService, settingStudentPhotosEnabled, false, rs.Logger)
	response, err := newStudentResponseWithOpts(r.Context(), StudentResponseOpts{
		Student:       updatedStudent,
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

	userPermissions := jwt.PermissionsFromCtx(r.Context())
	hasFullWriteAccess, personUpdated, renderer := rs.stageStudentUpdate(r.Context(), userPermissions, student, person, req)
	if renderer != nil {
		renderError(w, r, renderer)
		return
	}

	// The client needs the write's own verdict on the links (see
	// StudentResponse.CompanionsChanged); it is produced inside the transaction
	// and read after it, when the update is known to have succeeded.
	companionsChanged, err := rs.runStudentUpdate(r.Context(), student, person, req, userPermissions, personUpdated)
	if err != nil {
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

	// Admin users and group supervisors can see full data including detailed
	// location; an absence-only writer under open care cannot.
	rs.respondUpdatedStudent(w, r, student.ID, person, hasFullWriteAccess, companionsChanged)
}

// stageStudentUpdate runs the checks an update passes before its transaction
// and applies the person fields in memory. It reports the caller's full write
// verdict and whether the person changed, or the refusal to render.
func (rs *Resource) stageStudentUpdate(ctx context.Context, userPermissions []string, student *Student, person *peopleModule.Person, req *UpdateStudentRequest) (hasFullWriteAccess, personUpdated bool, refusal render.Renderer) {
	// Centralized permission check for updating student data. An absence-only
	// payload may additionally pass through the action-scoped absence gate
	// (open care, #2232) — hasFullWriteAccess stays false in that case, so the
	// response is built for a caller who may NOT read the child's full record.
	hasFullWriteAccess, authorized, authErr := rs.authorizeStudentUpdate(ctx, userPermissions, student, req)
	if !authorized {
		return false, false, common.ErrorForbidden(authErr)
	}

	// Update person fields using helper function
	personResult := applyPersonUpdates(req, person)
	if personResult.err != nil {
		return false, false, common.ErrorInvalidRequest(personResult.err)
	}

	// Reject updates that would leave the student in both sick and excused
	// states simultaneously. The frontend uses the SICK_EXCUSED_CONFLICT code
	// to prompt the user to switch states rather than hold both.
	if err := checkSickExcusedConflict(req, student); err != nil {
		return false, false, common.ErrorConflictWithCode(err, ErrCodeSickExcusedConflict)
	}
	return hasFullWriteAccess, personResult.updated, nil
}

// runStudentUpdate runs the locked-row patch in the request's tenant
// transaction and reports whether it changed the Laufgemeinschaft.
func (rs *Resource) runStudentUpdate(ctx context.Context, student *Student, person *peopleModule.Person, req *UpdateStudentRequest, userPermissions []string, personUpdated bool) (bool, error) {
	statusHistoryNow := time.Now()
	tenantID := tenant.FromContext(ctx)
	companionsChanged := false
	err := withinTenant(ctx, tenantID, func(ctx context.Context) error {
		changed, err := rs.applyStudentUpdate(ctx, tenantID, student, person, req, userPermissions, personUpdated, statusHistoryNow)
		companionsChanged = changed
		return err
	})
	return companionsChanged, err
}

// errStudentUpdatePermissionRequired is the 403 for a caller who reached
// PUT /students/{id} through the users:absence branch of the route gate and
// then sent something other than a pure absence payload.
var errStudentUpdatePermissionRequired = errors.New("the users:update permission is required to edit this student's data")

// authorizeStudentUpdate is the PUT /students/{id} gate. It reports the full
// write verdict separately from the effective one because the two diverge for
// an absence-only payload: the caller may then be authorized by the
// action-scoped absence gate (canManageStudentAbsence) without holding write
// authority on the child's record at all, and the response must not be built as
// if they did.
//
// The absence fallback is only ever consulted for a payload that carries
// NOTHING but sick/excused (see UpdateStudentRequest.isAbsenceOnly), so it can
// never let a Stammdaten field through.
//
// The full path additionally requires users:update. The route gate admits
// users:update OR users:absence, but canUpdateStudent decides supervision only
// and never re-checks the permission — it was written when users:update was
// the route's sole gate. Without this check a users:absence holder who happens
// to supervise the child's group would inherit the entire Stammdaten write
// (address, class, notes) from that supervision, which is exactly the
// separation the dedicated permission exists to keep.
func (rs *Resource) authorizeStudentUpdate(
	ctx context.Context,
	userPermissions []string,
	student *Student,
	req *UpdateStudentRequest,
) (hasFullWriteAccess, authorized bool, authErr error) {
	if req.Sick != nil || req.Excused != nil || req.SickReason != nil {
		if ok, err := rs.canManageStudentStatus(ctx, userPermissions, student, "sick"); !ok {
			return false, false, err
		}
	}
	if !securityruntime.HasPermission(permissions.UsersUpdate, userPermissions) {
		if !req.isAbsenceOnly() {
			return false, false, errStudentUpdatePermissionRequired
		}
		absenceOK, absenceErr := rs.canManageStudentAbsence(ctx, userPermissions, student)
		return false, absenceOK, absenceErr
	}

	fullOK, fullErr := canUpdateStudent(ctx, userPermissions, student, rs.UserContextService)
	if fullOK || !req.isAbsenceOnly() {
		return fullOK, fullOK, fullErr
	}
	// On denial the absence gate reports the supervisor gate's own reason
	// wherever the tenant is not running open care, so the familiar 403 text
	// survives unchanged.
	absenceOK, absenceErr := rs.canManageStudentAbsence(ctx, userPermissions, student)
	return false, absenceOK, absenceErr
}
