// backend/database/repositories/users/student.go
package users

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/driver/pgdriver"
)

// studentPhotoFeatureLockClass is the pg_advisory_xact_lock class id used
// to serialize concurrent transactions that affect the student-photo
// feature for a tenant. The disable path (PurgeAllPhotos invoked from the
// OnValueSet hook) and the upload path (api/students/photo.go) both take
// this lock so they cannot interleave: the previous post-update recheck
// only narrowed the race, it didn't eliminate it — the disable's CTE
// only locks rows whose photo_path is currently non-null, so a brand-new
// upload's row (photo_path NULL at the moment the CTE evaluates) was
// never serialized against the disable, and an upload that committed
// after the CTE ran could leave a stored file the disable's post-commit
// purge never knew about.
//
// Class id is an arbitrary stable int32 ("phot" in ASCII). pg_advisory_
// xact_lock releases automatically at COMMIT/ROLLBACK of the surrounding
// tx, so callers must already be inside a tenant tx — there is no
// separate Unlock method to forget.
const studentPhotoFeatureLockClass int32 = 0x70686F74

// Table name constants (S1192 - avoid duplicate string literals)
const (
	tableUsersStudents              = "users.students"
	tableExprUsersStudentsAsStudent = "users.students AS student"
)

// StudentRepository implements users.StudentRepository interface
type StudentRepository struct {
	*base.Repository[*users.Student]
	db *bun.DB
	// companions backs the departure-plan reconciliation in Update: trimming a
	// plan has to trim the "läuft mit" edges that lose their basis, and that
	// must happen in the one write path EVERY caller passes through — the
	// student service's HTTP flow, but also enrollment approval, imports, and
	// any other direct repository writer (#1694).
	companions *StudentCompanionRepository
}

// NewStudentRepository creates a new StudentRepository
func NewStudentRepository(db *bun.DB) users.StudentRepository {
	repo := base.NewRepository[*users.Student](db, tableUsersStudents, "Student")
	repo.TenantScoped = true
	return &StudentRepository{
		Repository: repo,
		db:         db,
		companions: newStudentCompanionRepository(db),
	}
}

// FindByID retrieves a student by their ID.
func (r *StudentRepository) FindByID(ctx context.Context, id interface{}) (*users.Student, error) {
	student, err := r.Repository.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := r.hydrateBusDaysIfPresent(ctx, []*users.Student{student}); err != nil {
		return nil, err
	}
	return student, nil
}

// FindByPersonID retrieves a student by their person ID
func (r *StudentRepository) FindByPersonID(ctx context.Context, personID int64) (*users.Student, error) {
	student := new(users.Student)
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(student).
		ModelTableExpr(tableExprUsersStudentsAsStudent).
		Where("person_id = ?", personID)

	query = base.WithTenantFilter(ctx, query, "student")

	err := query.Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by person ID",
			Err: err,
		}
	}

	return student, nil
}

// FindByIDs retrieves multiple students by their IDs in a single query
func (r *StudentRepository) FindByIDs(ctx context.Context, ids []int64) (map[int64]*users.Student, error) {
	if len(ids) == 0 {
		return make(map[int64]*users.Student), nil
	}

	var students []*users.Student
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&students).
		ModelTableExpr(`users.students AS "student"`).
		Where(`"student".id IN (?)`, bun.List(ids))

	query = base.WithTenantFilter(ctx, query, "student")

	err := query.Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by IDs",
			Err: err,
		}
	}

	result := make(map[int64]*users.Student, len(students))
	for _, student := range students {
		result[student.ID] = student
	}
	if err := r.hydrateBusDaysIfPresent(ctx, students); err != nil {
		return nil, err
	}

	return result, nil
}

// FindReadScopeByIDs retrieves a lightweight projection of the given students —
// only id, group_id, person_id, and school_class — in a single primary-key
// IN-list query. Unlike FindByIDs it does NOT run hydrateBusDaysIfPresent, so it
// avoids the extra information_schema column probe and jsonb weekday-hydration
// round-trip. Callers that only gate read access and display a name (e.g. the
// reminders header, polled per browser every 60s) get just those small rows and
// nothing they never read. The returned *Student values have ONLY those four
// fields populated — do not use them where full student data is expected.
func (r *StudentRepository) FindReadScopeByIDs(ctx context.Context, ids []int64) (map[int64]*users.Student, error) {
	if len(ids) == 0 {
		return make(map[int64]*users.Student), nil
	}

	var students []*users.Student
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&students).
		ModelTableExpr(`users.students AS "student"`).
		Column("id", "group_id", "person_id", "school_class").
		Where(`"student".id IN (?)`, bun.List(ids))

	query = base.WithTenantFilter(ctx, query, "student")

	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find read scope by IDs",
			Err: err,
		}
	}

	result := make(map[int64]*users.Student, len(students))
	for _, student := range students {
		result[student.ID] = student
	}
	return result, nil
}

// FindByGroupID retrieves students by their group ID
func (r *StudentRepository) FindByGroupID(ctx context.Context, groupID int64) ([]*users.Student, error) {
	var students []*users.Student
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&students).
		ModelTableExpr(tableExprUsersStudentsAsStudent).
		Where("group_id = ?", groupID)

	query = base.WithTenantFilter(ctx, query, "student")

	err := query.Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by group ID",
			Err: err,
		}
	}

	if err := r.hydrateBusDaysForStudents(ctx, students); err != nil {
		return nil, err
	}

	return students, nil
}

// FindByGroupIDs retrieves students by multiple group IDs
func (r *StudentRepository) FindByGroupIDs(ctx context.Context, groupIDs []int64) ([]*users.Student, error) {
	if len(groupIDs) == 0 {
		return []*users.Student{}, nil
	}

	var students []*users.Student
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&students).
		ModelTableExpr(tableExprUsersStudentsAsStudent).
		Where("group_id IN (?)", bun.List(groupIDs))

	query = base.WithTenantFilter(ctx, query, "student")

	err := query.Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by group IDs",
			Err: err,
		}
	}

	if err := r.hydrateBusDaysForStudents(ctx, students); err != nil {
		return nil, err
	}

	return students, nil
}

// FindBySchoolClass retrieves students by their school class
func (r *StudentRepository) FindBySchoolClass(ctx context.Context, schoolClass string) ([]*users.Student, error) {
	var students []*users.Student
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&students).
		ModelTableExpr(tableExprUsersStudentsAsStudent).
		Where("LOWER(TRIM(school_class)) = LOWER(TRIM(?))", schoolClass)

	query = base.WithTenantFilter(ctx, query, "student")

	err := query.Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by school class",
			Err: err,
		}
	}

	if err := r.hydrateBusDaysForStudents(ctx, students); err != nil {
		return nil, err
	}

	return students, nil
}

// ExistsEnrolledByNameAndBirthday reports whether an already-enrolled
// student with the given (case-insensitive, trimmed) name and birthday
// exists in the tenant. Backs the enrollment new_students audience check
// (#1663). "Enrolled" spans both active and pending students: an
// enrollment approved before its service start date creates the resulting
// student as pending until the activation scheduler flips it to active
// (approvalActivationPlan), so pending children are already enrolled and
// must be treated as such — otherwise a just-approved child would slip
// through a new_students phase and create a duplicate record. The tenant
// filter is explicit (not RLS/context-based) because the parent submit
// path runs under an admin transaction. A zero birthday binds NULL and
// matches nothing — the safe outcome for incomplete input.
func (r *StudentRepository) ExistsEnrolledByNameAndBirthday(ctx context.Context, tenantID int64, firstName, lastName string, birthday timezone.Date) (bool, error) {
	count, err := base.GetDB(ctx, r.db).NewSelect().
		Model((*users.Student)(nil)).
		ModelTableExpr(tableExprUsersStudentsAsStudent).
		Join(`INNER JOIN users.persons AS "person" ON "person".id = "student".person_id`).
		Where(`"student".tenant_id = ?`, tenantID).
		Where(`"student".status IN (?)`, bun.List([]users.StudentStatus{users.StudentStatusActive, users.StudentStatusPending})).
		Where(`LOWER(TRIM("person".first_name)) = LOWER(TRIM(?))`, firstName).
		Where(`LOWER(TRIM("person".last_name)) = LOWER(TRIM(?))`, lastName).
		Where(`"person".birthday = ?`, birthday).
		Where(`"person".deleted_at IS NULL`).
		Count(ctx)
	if err != nil {
		return false, &modelBase.DatabaseError{
			Op:  "exists enrolled by name and birthday",
			Err: err,
		}
	}
	return count > 0, nil
}

// FindEnrolledStudentIDByNameAndBirthday resolves the single already-enrolled
// student matching the given (case-insensitive, trimmed) name and birthday in
// the tenant, backing the existing_students re-enrollment path (#1663). It
// returns the student ID ONLY when exactly one active/pending student matches:
// zero matches or an ambiguous multi-match both yield (nil, nil) so the caller
// stores no reference and approval falls back to the fresh-create path rather
// than renewing an arbitrary record. Same enrolled-scope and explicit tenant
// filter as ExistsEnrolledByNameAndBirthday (the parent submit path runs under
// an admin transaction, not RLS context). A zero birthday binds NULL and
// matches nothing.
func (r *StudentRepository) FindEnrolledStudentIDByNameAndBirthday(ctx context.Context, tenantID int64, firstName, lastName string, birthday timezone.Date) (*int64, error) {
	var ids []int64
	err := base.GetDB(ctx, r.db).NewSelect().
		Model((*users.Student)(nil)).
		ModelTableExpr(tableExprUsersStudentsAsStudent).
		Join(`INNER JOIN users.persons AS "person" ON "person".id = "student".person_id`).
		ColumnExpr(`"student".id`).
		Where(`"student".tenant_id = ?`, tenantID).
		Where(`"student".status IN (?)`, bun.List([]users.StudentStatus{users.StudentStatusActive, users.StudentStatusPending})).
		Where(`LOWER(TRIM("person".first_name)) = LOWER(TRIM(?))`, firstName).
		Where(`LOWER(TRIM("person".last_name)) = LOWER(TRIM(?))`, lastName).
		Where(`"person".birthday = ?`, birthday).
		Where(`"person".deleted_at IS NULL`).
		OrderExpr(`"student".id ASC`).
		Limit(2).
		Scan(ctx, &ids)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find enrolled student id by name and birthday",
			Err: err,
		}
	}
	if len(ids) != 1 {
		// Zero or ambiguous (>1): no unambiguous student to renew.
		return nil, nil
	}
	id := ids[0]
	return &id, nil
}

// ListSchoolClasses retrieves all distinct non-empty school_class values.
func (r *StudentRepository) ListSchoolClasses(ctx context.Context) ([]string, error) {
	var classes []string
	query := base.GetDB(ctx, r.db).NewSelect().
		TableExpr(`users.students AS "student"`).
		ColumnExpr(`DISTINCT TRIM("student".school_class)`).
		Where(`TRIM("student".school_class) != ''`).
		OrderExpr(`TRIM("student".school_class) ASC`)

	query = base.WithTenantFilter(ctx, query, "student")

	if err := query.Scan(ctx, &classes); err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "list school classes",
			Err: err,
		}
	}

	return classes, nil
}

// Create overrides the base Create method to handle validation
func (r *StudentRepository) Create(ctx context.Context, student *users.Student) error {
	if student == nil {
		return fmt.Errorf("student cannot be nil")
	}

	// Validate student
	if err := student.Validate(); err != nil {
		return err
	}

	if err := r.Repository.Create(ctx, student); err != nil {
		return err
	}
	return r.persistDepartureDays(ctx, student, nil)
}

// Update overrides the base Update method to handle validation
func (r *StudentRepository) Update(ctx context.Context, student *users.Student) error {
	if student == nil {
		return fmt.Errorf("student cannot be nil")
	}

	// Take the subject's row lock BEFORE reading its stored departure plan or its
	// edges, so both reads see the state this update actually overwrites.
	// Without it a caller that hydrated the student earlier (FindByID hydrates
	// the plan, so planTouched is true even for an unrelated write such as
	// autoClearStudentSickness) re-persists its cached plan: the row UPDATE
	// blocks on a concurrent companion transaction, commits the pre-change plan
	// on top of it, while planCompanionReconcile — reading before that
	// transaction committed — never saw the new edge and so left it in place.
	// The result is an edge the stored plan forbids. Locking first makes the two
	// reads agree with the write: the edge is either trimmed along with the plan
	// or the update is refused by the stranding check.
	if err := r.lockSubjectForDepartureWrite(ctx, student); err != nil {
		return err
	}

	currentDeparture, err := r.findCurrentDepartureState(ctx, student.ID)
	if err != nil {
		return err
	}

	// Move the plan fields the caller never touched onto the freshly locked
	// state, BEFORE anything interprets the difference between them as an
	// intentional change.
	rebaseUntouchedDeparturePlan(student, currentDeparture)

	// Align the in-memory departure plan to the plan that will actually be
	// persisted BEFORE validating, so Validate() checks the effective plan rather
	// than a transient mix of a stale hydrated allowed_departure_modes set and a
	// freshly-set legacy departure_days. Without this a legacy client that removes
	// the accompanied mode via departure_days while clearing the "mit wem" note is
	// rejected against the stale accompanied mode it never sent (#1694).
	r.alignDeparturePlanForValidation(student, currentDeparture)

	// Reconcile the "läuft mit" edges with the plan that is about to be
	// persisted. This is the shared write path every departure-plan writer
	// passes through — the HTTP student flow trims links itself before calling
	// Update (making this a no-op there), but enrollment approval, imports and
	// other direct repository callers replace the plan without knowing links
	// exist, and would otherwise leave edges the stored plan forbids (the
	// Kindersuche would keep grouping the child contrary to its Stammdaten).
	// Stranding a linked child refuses the whole update with
	// ErrCompanionWouldLoseDeparture, exactly like the service-level check.
	trim, err := r.planCompanionReconcile(ctx, student)
	if err != nil {
		return err
	}

	// A structured "läuft mit" link satisfies the accompanied-requires-a-note
	// invariant just like the free-text note does, but it lives in another table
	// and is not part of the model. Derive it HERE, the one layer every update
	// passes through, so a caller that knows nothing about companions (status
	// days, sick/excused auto-clear, care-request approval, imports) can still
	// save a child whose "mit wem" is answered by a link (#1694). When the
	// reconcile pass already loaded the edges, its survivor count is
	// authoritative — an EXISTS probe would still see the edges the trim is
	// about to drop. Never CLEAR a day the caller set: extendAccompaniedDays
	// asserts it for an edge that is written later in the same transaction, so
	// the stored edges legitimately don't show it yet. The cover is per weekday,
	// because a Monday link is no answer for an accompanied Tuesday.
	if trim != nil {
		for day, kept := range trim.keptDays {
			if kept {
				student.MarkDepartureCompanionDays(day)
			}
		}
	} else if err := r.applyCompanionLinkDays(ctx, student); err != nil {
		return err
	}

	// Validate student
	if err := student.Validate(); err != nil {
		return err
	}

	if err := r.Repository.Update(ctx, student); err != nil {
		return err
	}
	if err := r.persistDepartureDays(ctx, student, currentDeparture); err != nil {
		return err
	}
	// Drop the trimmed edges only after the plan write succeeded, so a
	// validation or persistence failure never leaves links deleted for a plan
	// that was never stored. Callers run inside the request's tenant
	// transaction, so plan and trim still commit or roll back together.
	if trim != nil {
		if err := r.companions.DeleteEdges(ctx, trim.dropIDs); err != nil {
			return err
		}
		// Tell a caller that does not write links itself whether THIS write
		// touched any — the only honest basis for announcing
		// student_companions_changed (see users.CompanionChangeRecorder).
		if len(trim.dropIDs) > 0 {
			users.RecordCompanionChange(ctx)
		}
	}
	return nil
}

// departurePlanTouched reports whether this update carries a departure plan at
// all. Every plan-resolving step keys off it: a caller that loaded no plan
// leaves all four fields nil and must not have the stored plan rewritten.
func departurePlanTouched(student *users.Student) bool {
	return student.AllowedDepartureModes != nil ||
		student.DepartureDays != nil ||
		student.BusDays != nil ||
		student.PickupDays != nil ||
		student.PickupStatus != nil
}

// lockSubjectForDepartureWrite takes the subject's row lock when this update
// will (re)write the departure columns — a plan was supplied, or a companion
// note was, which persistDepartureDays resolves against the STORED plan and
// therefore rewrites the same columns.
//
// It is the first lock this transaction takes, which is also what
// lockCompanionFarEnds assumes ("the subject's row is normally already locked
// by the caller"): every companion writer then walks the far ends in ascending
// id order from a held subject, and only downward acquisitions go NOWAIT.
// Callers that already hold the row (the HTTP path's lockStudentForUpdate)
// re-acquire it for free. A missing row is not an error here — the subsequent
// UPDATE simply matches nothing, exactly as before.
func (r *StudentRepository) lockSubjectForDepartureWrite(ctx context.Context, student *users.Student) error {
	if student.ID <= 0 || (!departurePlanTouched(student) && student.DepartureCompanionNote == nil) {
		return nil
	}
	if _, err := r.FindByIDForUpdate(ctx, student.ID); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	return nil
}

// rebaseUntouchedDeparturePlan replaces every departure-plan field that still
// carries exactly what the read hydrated with the state just read under the row
// lock.
//
// Taking the lock makes the reads agree with the write, but only for the STORED
// side: the in-memory student is whatever the caller loaded, possibly long
// before a concurrent companion edit committed. A direct caller that never
// touches the plan (autoClearStudentSickness, status days, imports) still
// carries all four hydrated fields, so departurePlanTouched is true and
// resolveAllowedDepartureModes would read the difference to the now-newer
// stored plan as an intentional change — reverting the committed edit, trimming
// its fresh edges, or refusing the unrelated update with
// ErrCompanionWouldLoseDeparture. Rebasing the untouched fields makes such an
// update a no-op re-persist of the current plan again.
//
// Fields the caller genuinely changed differ from the baseline and are left
// alone, so an intentional plan write still wins over the stored state (last
// writer wins, as before — this only stops a NON-writer from winning). Without
// a baseline (the caller built the student itself, or the read predates this
// snapshot) nothing is rebased and the supplied fields are taken at face value.
func rebaseUntouchedDeparturePlan(student *users.Student, current *studentDepartureState) {
	baseline := student.DepartureBaseline
	if baseline == nil || current == nil {
		return
	}
	if student.AllowedDepartureModes != nil &&
		allowedDepartureModesEqual(student.AllowedDepartureModes, baseline.AllowedDepartureModes) {
		student.AllowedDepartureModes = current.AllowedDepartureModes
	}
	if student.DepartureDays != nil &&
		departureDaysEqual(student.DepartureDays, baseline.DepartureDays) {
		student.DepartureDays = current.DepartureDays
	}
	if student.BusDays != nil && busDaysEqual(student.BusDays, baseline.BusDays) {
		student.BusDays = current.BusDays
	}
	if student.PickupDays != nil && pickupDaysEqual(student.PickupDays, baseline.PickupDays) {
		student.PickupDays = current.PickupDays
	}
	// PickupStatus needs no rebase: hydration always leaves PickupDays non-nil,
	// and resolvedPickupDays only falls back to the legacy status string when
	// PickupDays is nil.
}

// companionTrim is the outcome of planCompanionReconcile: the edge rows the new
// departure plan no longer allows, plus the weekdays on which the child keeps a
// link (its structured "mit wem" answer, per day).
type companionTrim struct {
	dropIDs  []int64
	keptDays map[string]bool
}

// planCompanionReconcile determines which "läuft mit" edges lose their basis
// under the departure plan this update is about to persist, and refuses the
// update when dropping one would strand the child at the FAR end (accompanied
// plan on that weekday, no note, no other link on that weekday) — the same
// rule services/users checkCompanionRemovals enforces for the HTTP path.
//
// Returns nil when it did not evaluate the edges (plan untouched, plan allows
// every weekday, or the table predates this schema); the caller then falls back
// to the EXISTS-based link flag probe.
func (r *StudentRepository) planCompanionReconcile(ctx context.Context, student *users.Student) (*companionTrim, error) {
	if student.ID <= 0 || !departurePlanTouched(student) {
		return nil, nil
	}

	// alignDeparturePlanForValidation already rewrote the in-memory plan to the
	// one persistDepartureDays will store, so this reads the effective plan.
	accompanied := users.AccompaniedWeekdays(student.AllowedDepartureModes, student.DepartureDays)
	if len(accompanied) == len(users.PickupDayOrder) {
		// Every weekday still allows "Anderes Kind" — no edge can lose its
		// basis, so skip the edge query on this common widening path.
		return nil, nil
	}

	// The table landed in 1.15.209 and the migration tests exercise historical
	// schemas with the current model; an absent table means no links.
	exists, err := r.hasCompanionTable(ctx)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, nil
	}

	edges, err := r.companions.ListForStudent(ctx, student.ID)
	if err != nil {
		return nil, err
	}
	if len(edges) == 0 {
		return &companionTrim{}, nil
	}

	trim := &companionTrim{keptDays: make(map[string]bool, len(users.PickupDayOrder))}
	removedDays := make(map[int64][]string, len(edges))
	removed := make([]int64, 0, len(edges))
	for _, edge := range edges {
		far, ok := edge.Other(student.ID)
		if !ok {
			continue
		}
		day := users.CompanionWeekdayKeys[edge.Weekday]
		if accompanied[day] {
			trim.keptDays[day] = true
			continue
		}
		trim.dropIDs = append(trim.dropIDs, edge.ID)
		if _, seen := removedDays[far]; !seen {
			removed = append(removed, far)
		}
		removedDays[far] = append(removedDays[far], day)
	}
	if len(trim.dropIDs) == 0 {
		return trim, nil
	}
	// Serialize against every other writer that could remove one of the far
	// child's OTHER links before checkCompanionStranding reads them.
	if err := r.lockCompanionFarEnds(ctx, student.ID, removed); err != nil {
		return nil, err
	}
	if err := r.checkCompanionStranding(ctx, student.ID, removed, removedDays); err != nil {
		return nil, err
	}
	return trim, nil
}

// lockCompanionFarEnds takes the row lock of every child at the far end of a
// link this update is about to drop.
//
// Without it the stranding check is a read that two transactions can pass on
// each other's soon-to-be-deleted data: with links A-B and C-B, where B has no
// note and depends on them, a writer narrowing A's plan and a writer narrowing
// C's plan each still SEE the other edge, both conclude B stays covered, and
// both commit — leaving B with an accompanied plan and no "mit wem" detail.
// api/students takes exactly these locks for the HTTP path (lockCompanionRows),
// but this repository method is also the write path of callers that never go
// through it (masterDataReviewService.applyDepartureChange,
// careScheduleRequestService.applyDepartureModeChanges, imports, enrollment
// approval), so the invariant has to be re-established here.
//
// Order: ascending by id, the order every companion writer uses. The subject's
// row is normally already locked by the caller, so a far end BELOW it can only
// be acquired against that order — those are taken with NOWAIT and surface as
// the retriable users.ErrCompanionLockBusy rather than blocking into a deadlock
// with a writer coming up from the other side.
func (r *StudentRepository) lockCompanionFarEnds(ctx context.Context, studentID int64, farEnds []int64) error {
	ordered := make([]int64, 0, len(farEnds))
	for _, id := range farEnds {
		if id > 0 && id != studentID {
			ordered = append(ordered, id)
		}
	}
	if len(ordered) == 0 {
		return nil
	}
	sort.Slice(ordered, func(i, j int) bool { return ordered[i] < ordered[j] })

	for _, id := range ordered {
		var err error
		if id < studentID {
			_, err = r.FindByIDForUpdateNoWait(ctx, id)
		} else {
			_, err = r.FindByIDForUpdate(ctx, id)
		}
		switch {
		case err == nil:
		case errors.Is(err, sql.ErrNoRows):
			// Deleted or another tenant — checkCompanionStranding skips it too.
		case modelBase.IsLockNotAvailable(err):
			return users.ErrCompanionLockBusy
		default:
			return err
		}
	}
	return nil
}

// checkCompanionStranding refuses when any removed edge would leave its far
// child with an accompanied departure plan and no remaining "mit wem" detail
// FOR THAT WEEKDAY. The cover is per day (Student.Validate): an edge the far
// child keeps on Monday — with this subject or anyone else — does not answer
// for an accompanied Tuesday whose edge is being dropped. Mirrors
// services/users checkCompanionRemovals; both return the shared
// users.ErrCompanionWouldLoseDeparture sentinel.
//
// Inside a coordinated multi-child write (an open users.CompanionStrandingBatch
// on the context) the verdict is DEFERRED rather than decided here: the far
// child may be another member of the same batch whose own plan change — the one
// that makes this removal legitimate — has not been applied yet. The batch's
// owner decides every deferred verdict against the final state via
// VerifyCompanionStrandingBatch before it commits.
func (r *StudentRepository) checkCompanionStranding(ctx context.Context, studentID int64, removed []int64, removedDays map[int64][]string) error {
	if len(removed) == 0 {
		return nil
	}
	if batch := users.CompanionStrandingBatchFromContext(ctx); batch != nil {
		for _, id := range removed {
			batch.Defer(id, removedDays[id])
		}
		return nil
	}
	return r.checkCompanionStrandingNow(ctx, studentID, removed, removedDays)
}

// checkCompanionStrandingNow is the verdict itself, evaluated against the state
// the database has right now. Edges of studentID are ignored as cover because
// the caller is about to delete them; pass 0 to count every stored edge.
func (r *StudentRepository) checkCompanionStrandingNow(ctx context.Context, studentID int64, removed []int64, removedDays map[int64][]string) error {
	covered, err := r.companions.CompanionDaysCoveredExcluding(ctx, removed, studentID)
	if err != nil {
		return err
	}
	companions, err := r.FindByIDs(ctx, removed)
	if err != nil {
		return err
	}
	for _, id := range removed {
		companion := companions[id]
		if companion == nil {
			continue // deleted or another tenant — nothing left to strand
		}
		if companion.DepartureCompanionNote != nil && strings.TrimSpace(*companion.DepartureCompanionNote) != "" {
			continue // the free-text note carries the detail for every day
		}
		accompanied := users.AccompaniedWeekdays(companion.AllowedDepartureModes, companion.DepartureDays)
		for _, day := range removedDays[id] {
			if !accompanied[day] {
				continue // their plan does not claim "Anderes Kind" on this day
			}
			if covered[id][day] {
				continue // another companion walks with them on this day
			}
			return users.ErrCompanionWouldLoseDeparture
		}
	}
	return nil
}

// VerifyCompanionStrandingBatch decides the stranding verdicts the writes of a
// coordinated multi-child edit deferred into the users.CompanionStrandingBatch
// carried by ctx (see checkCompanionStranding). Without an open batch — every
// single-child write — it is a no-op.
//
// It re-runs the very same check, but now against the state the whole batch
// leaves behind: every member's departure plan is written and every edge the
// batch trims is deleted, so a child whose accompanied day went away in the same
// edit passes, while a child genuinely left with an accompanied day, no note and
// no remaining link on that day still fails with
// users.ErrCompanionWouldLoseDeparture. Nothing is excluded from the coverage
// read here — unlike the per-write check, which has to ignore edges its own
// caller is about to delete, this runs after those deletions.
//
// The caller runs inside the request's tenant transaction, so a refusal rolls
// the coordinated edit back as a whole.
func (r *StudentRepository) VerifyCompanionStrandingBatch(ctx context.Context) error {
	batch := users.CompanionStrandingBatchFromContext(ctx)
	if batch == nil {
		return nil
	}
	removed, removedDays := batch.Pending()
	if len(removed) == 0 {
		return nil
	}
	// studentID 0 excludes nobody: every edge that still exists counts as cover.
	return r.checkCompanionStrandingNow(ctx, 0, removed, removedDays)
}

// alignDeparturePlanForValidation rewrites the in-memory departure plan to the
// plan persistDepartureDays will resolve and store, so Student.Validate() sees a
// self-consistent plan instead of a stale hydrated allowed-modes set alongside a
// freshly-set legacy field. The rewrite is idempotent: persistDepartureDays
// re-resolves from the same inputs and reaches the same plan (the rewritten
// per-weekday maps shadow any stale legacy pickup_status during that re-resolve).
// It only runs when the plan was actually touched, so an update that loads no
// plan stays the no-op persistDepartureDays expects. All four fields are
// scanonly, so the rewrite never leaks into the base Update's column set (#1694).
func (r *StudentRepository) alignDeparturePlanForValidation(student *users.Student, current *studentDepartureState) {
	if !departurePlanTouched(student) {
		return
	}
	allowed := resolveAllowedDepartureModes(student, current)
	student.AllowedDepartureModes = allowed
	student.DepartureDays = allowed.DepartureDays()
	student.BusDays = allowed.BusDays()
	student.PickupDays = allowed.PickupDays()
}

// persistDepartureDays writes the unified per-weekday departure mode AND its
// derived legacy mirrors (bus_days, pickup_days, pickup_status) in a single
// update, so the repository is the single source of truth (#1610): any caller
// that mutates the departure plan — via allowed modes, DepartureDays, or one of
// the legacy maps — leaves all columns consistent. Callers that touch unrelated
// student fields without loading the departure plan leave all four nil and this
// is a no-op, preserving the previous "don't clobber what wasn't provided"
// behavior — except that an orphan companion note is still cleared (see below).
func (r *StudentRepository) persistDepartureDays(ctx context.Context, student *users.Student, current *studentDepartureState) error {
	planTouched := departurePlanTouched(student)

	// Resolve the plan that will be in effect after this write: from the request
	// (merged with the stored state) when a plan field was provided, otherwise the
	// unchanged stored plan (current) — empty on create, where current is nil.
	var allowed users.AllowedDepartureModes
	switch {
	case planTouched:
		allowed = resolveAllowedDepartureModes(student, current)
	case current != nil:
		allowed = current.AllowedDepartureModes.Normalize()
	}

	// The companion note is scanonly (see the model), so the generic create/
	// update column set never references it and this is its single writer.
	// Resolve the value the column must hold after this write, honoring the
	// invariant that the free-text "mit wem" note must never outlive the
	// accompanied mode that justifies it (#1694):
	//   - resolved plan allows accompanied -> store the provided note (Validate
	//     guarantees a note is present on every accompanied plan)
	//   - resolved plan has no accompanied day -> store NULL, regardless of which
	//     fields (unified or legacy) drove the change, even on a note-only update
	//     that leaves the plan untouched
	// Only touch the column when the plan was (re)written or a note value was
	// supplied in-memory, so an unrelated update leaves it alone.
	noteTouched := planTouched || student.DepartureCompanionNote != nil
	var noteToStore *string
	if allowed.HasMode(users.DepartureAccompanied) {
		noteToStore = student.DepartureCompanionNote
	}

	if !noteTouched {
		return nil
	}

	query := base.GetDB(ctx, r.db).NewUpdate().
		TableExpr(`users.students AS "student"`).
		Where(`"student".id = ?`, student.ID)

	departure := allowed.DepartureDays()
	busDays := allowed.BusDays()
	pickupDays := allowed.PickupDays()
	// Derive pickup_status from the FULL non-exclusive set, not from the exclusive
	// `departure` projection above: the projection ranks bus over accompanied, so a
	// day allowing both would drop the accompanied signal and bucket the child as a
	// self-goer. This mirrors the clearNote check above, which already uses the full
	// set (#1694).
	pickupStatus := allowed.LegacyPickupStatus()

	// Resolve every optional departure column's existence in one query. Each
	// landed in its own migration (departure_days 1.15.120, bus_days 1.15.112,
	// pickup_days 1.15.116, allowed_departure_modes 1.15.130, the scanonly
	// departure_companion_note 1.15.138 which rollback drops again), and the
	// migration tests exercise historical schemas with the current model, so only
	// set columns that actually exist — batched, not one round-trip per column.
	cols, err := r.hasStudentColumns(ctx, "departure_days", "allowed_departure_modes", "bus_days", "pickup_days", "departure_companion_note")
	if err != nil {
		return err
	}

	if planTouched {
		query = query.Set(`pickup_status = ?`, pickupStatus)

		if cols["departure_days"] {
			query = query.Set(`departure_days = ?`, departure)
		}
		if cols["allowed_departure_modes"] {
			query = query.Set(`allowed_departure_modes = ?`, allowed)
		}
		if cols["bus_days"] {
			query = query.Set(`bus_days = ?`, busDays)
		}
		if cols["pickup_days"] {
			query = query.Set(`pickup_days = ?`, pickupDays)
		}
	}

	noteWritten := false
	if noteTouched && cols["departure_companion_note"] {
		query = query.Set(`departure_companion_note = ?`, noteToStore)
		noteWritten = true
	}

	// A note-only update on a schema that predates the companion-note column has
	// nothing left to write — bail before issuing a SET-less UPDATE.
	if !planTouched && !noteWritten {
		return nil
	}

	query = base.WithTenantFilter(ctx, query, "student")

	result, err := query.Exec(ctx)
	if err != nil {
		// Before 1.6.18.1 even pickup_status did not exist; on those ancient
		// schemas the legacy in-memory fields cover the gap.
		if isUndefinedColumnError(err) {
			return nil
		}
		return &modelBase.DatabaseError{
			Op:  "update student departure days",
			Err: err,
		}
	}

	if noteWritten {
		student.DepartureCompanionNote = noteToStore
	}
	if planTouched {
		student.DepartureDays = departure
		student.AllowedDepartureModes = allowed
		student.BusDays = busDays
		student.PickupDays = pickupDays
		student.PickupStatus = &pickupStatus
		// The in-memory plan now equals the stored one, so it is also the new
		// baseline: reusing the same instance for a second Update must not make
		// this write's own result look like a pending caller change.
		student.SnapshotDeparturePlan()
	}
	return base.AssertRowsAffected(result, 1, "update student departure days")
}

func resolveAllowedDepartureModes(student *users.Student, current *studentDepartureState) users.AllowedDepartureModes {
	if student.AllowedDepartureModes != nil {
		if shouldUseAllowedDepartureModes(student, current) {
			return student.AllowedDepartureModes.Normalize()
		}
	}
	if current == nil {
		if student.DepartureDays != nil {
			return users.AllowedDepartureModesFromDeparture(student.DepartureDays).Normalize()
		}
		return users.AllowedDepartureModesFromLegacy(student.BusDays, resolvedPickupDays(student)).Normalize()
	}

	pickup := resolvedPickupDays(student)
	busChanged := student.BusDays != nil && !busDaysEqual(student.BusDays, current.BusDays)
	pickupChanged := pickup != nil && !pickupDaysEqual(pickup, current.PickupDays)
	if busChanged || pickupChanged {
		return mergeLegacyDepartureModes(current.AllowedDepartureModes, student.BusDays, pickup, busChanged, pickupChanged)
	}

	if student.DepartureDays != nil && !departureDaysEqual(student.DepartureDays, current.DepartureDays) {
		return users.AllowedDepartureModesFromDeparture(student.DepartureDays).Normalize()
	}
	return current.AllowedDepartureModes.Normalize()
}

func shouldUseAllowedDepartureModes(student *users.Student, current *studentDepartureState) bool {
	if current == nil {
		return true
	}
	if !allowedDepartureModesEqual(student.AllowedDepartureModes, current.AllowedDepartureModes) {
		return true
	}
	pickup := resolvedPickupDays(student)
	departureChanged := student.DepartureDays != nil &&
		!departureDaysEqual(student.DepartureDays, current.DepartureDays)
	legacyChanged := (student.BusDays != nil && !busDaysEqual(student.BusDays, current.BusDays)) ||
		(pickup != nil && !pickupDaysEqual(pickup, current.PickupDays))
	return !departureChanged && !legacyChanged
}

func resolvedPickupDays(student *users.Student) users.PickupDays {
	if student.PickupDays != nil {
		return student.PickupDays
	}
	if student.PickupStatus != nil {
		return users.PickupDaysFromLegacyStatus(*student.PickupStatus)
	}
	return nil
}

func mergeLegacyDepartureModes(current users.AllowedDepartureModes, bus users.BusDays, pickup users.PickupDays, busChanged, pickupChanged bool) users.AllowedDepartureModes {
	current = current.Normalize()
	out := users.AllowedDepartureModes{}
	for _, day := range users.PickupDayOrder {
		modes := map[users.DepartureMode]bool{}
		for _, mode := range current[day] {
			modes[mode] = true
		}
		if busChanged {
			modes[users.DepartureBus] = bus[day]
		}
		if pickupChanged {
			modes[users.DeparturePickup] = pickup[day]
		}
		for _, mode := range []users.DepartureMode{users.DepartureAlone, users.DepartureBus, users.DeparturePickup, users.DepartureAccompanied} {
			if modes[mode] {
				out[day] = append(out[day], mode)
			}
		}
	}
	return out.Normalize()
}

type studentDepartureState struct {
	BusDays               users.BusDays
	PickupDays            users.PickupDays
	DepartureDays         users.DepartureDays
	AllowedDepartureModes users.AllowedDepartureModes
}

func (r *StudentRepository) findCurrentDepartureState(ctx context.Context, studentID int64) (*studentDepartureState, error) {
	if studentID == 0 {
		return nil, nil
	}
	student := &users.Student{}
	student.ID = studentID
	students := []*users.Student{student}
	if err := r.hydrateBusDaysForStudents(ctx, students); err != nil {
		return nil, err
	}
	return &studentDepartureState{
		BusDays:               student.BusDays.Normalize(),
		PickupDays:            student.PickupDays.Normalize(),
		DepartureDays:         student.DepartureDays.Normalize(),
		AllowedDepartureModes: student.AllowedDepartureModes.Normalize(),
	}, nil
}

func allowedDepartureModesEqual(a, b users.AllowedDepartureModes) bool {
	a = a.Normalize()
	b = b.Normalize()
	for _, day := range users.PickupDayOrder {
		am := a[day]
		bm := b[day]
		if len(am) != len(bm) {
			return false
		}
		for i := range am {
			if am[i] != bm[i] {
				return false
			}
		}
	}
	return true
}

func departureDaysEqual(a, b users.DepartureDays) bool {
	a = a.Normalize()
	b = b.Normalize()
	for _, day := range users.PickupDayOrder {
		if a.ModeFor(day) != b.ModeFor(day) {
			return false
		}
	}
	return true
}

func busDaysEqual(a, b users.BusDays) bool {
	a = a.Normalize()
	b = b.Normalize()
	for _, day := range users.PickupDayOrder {
		if a[day] != b[day] {
			return false
		}
	}
	return true
}

func pickupDaysEqual(a, b users.PickupDays) bool {
	a = a.Normalize()
	b = b.Normalize()
	for _, day := range users.PickupDayOrder {
		if a[day] != b[day] {
			return false
		}
	}
	return true
}

func isUndefinedColumnError(err error) bool {
	var pgErr pgdriver.Error
	return errors.As(err, &pgErr) && pgErr.Field('C') == "42703"
}

// Legacy method to maintain compatibility with old interface
func (r *StudentRepository) List(ctx context.Context, filters map[string]interface{}) ([]*users.Student, error) {
	options := modelBase.NewQueryOptions()
	filter := modelBase.NewFilter()

	for field, value := range filters {
		if value != nil {
			applyStudentFilter(filter, field, value)
		}
	}

	options.Filter = filter
	return r.ListWithOptions(ctx, options)
}

// applyStudentFilter applies a single filter based on field name
func applyStudentFilter(filter *modelBase.Filter, field string, value interface{}) {
	switch field {
	case "school_class_like":
		applyStudentStringLikeFilter(filter, "school_class", value)
	case "guardian_name_like":
		applyStudentStringLikeFilter(filter, "guardian_name", value)
	case "has_group":
		applyNullableFieldFilter(filter, "group_id", value)
	default:
		filter.Equal(field, value)
	}
}

// applyStudentStringLikeFilter applies LIKE filter for string fields
func applyStudentStringLikeFilter(filter *modelBase.Filter, column string, value interface{}) {
	if strValue, ok := value.(string); ok {
		filter.ILike(column, "%"+strValue+"%")
	}
}

// ListWithOptions provides a type-safe way to list students with query options
func (r *StudentRepository) ListWithOptions(ctx context.Context, options *modelBase.QueryOptions) ([]*users.Student, error) {
	var students []*users.Student
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&students).
		ModelTableExpr(tableExprUsersStudentsAsStudent)

	query = base.WithTenantFilter(ctx, query, "student")

	// Apply query options with table alias
	if options != nil {
		if options.Filter != nil {
			options.Filter.WithTableAlias("student")
		}
		query = options.ApplyToQuery(query)
	}

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "list with options",
			Err: err,
		}
	}

	if err := r.hydrateBusDaysForStudents(ctx, students); err != nil {
		return nil, err
	}

	return students, nil
}

// CountByGroupIDs counts students per group for multiple groups in a single query
func (r *StudentRepository) CountByGroupIDs(ctx context.Context, groupIDs []int64) (map[int64]int, error) {
	if len(groupIDs) == 0 {
		return make(map[int64]int), nil
	}

	type countResult struct {
		GroupID int64 `bun:"group_id"`
		Count   int   `bun:"count"`
	}

	var results []countResult
	query := base.GetDB(ctx, r.db).NewSelect().
		TableExpr(`users.students AS "student"`).
		ColumnExpr(`"student".group_id`).
		ColumnExpr("COUNT(*) AS count").
		Where(`"student".group_id IN (?)`, bun.List(groupIDs)).
		GroupExpr(`"student".group_id`)

	query = base.WithTenantFilter(ctx, query, "student")

	err := query.Scan(ctx, &results)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "count by group IDs",
			Err: err,
		}
	}

	counts := make(map[int64]int, len(results))
	for _, r := range results {
		counts[r.GroupID] = r.Count
	}
	return counts, nil
}

// FindByGuardianEmail finds students with a specific guardian email
func (r *StudentRepository) FindByGuardianEmail(ctx context.Context, email string) ([]*users.Student, error) {
	var students []*users.Student
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&students).
		ModelTableExpr(`users.students AS "student"`).
		Where(`LOWER("student".guardian_email) = LOWER(?)`, email)

	query = base.WithTenantFilter(ctx, query, "student")

	err := query.Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by guardian email",
			Err: err,
		}
	}

	return students, nil
}

// FindByGuardianPhone finds students with a specific guardian phone
func (r *StudentRepository) FindByGuardianPhone(ctx context.Context, phone string) ([]*users.Student, error) {
	var students []*users.Student
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&students).
		ModelTableExpr(`users.students AS "student"`).
		Where(`"student".guardian_phone = ?`, phone)

	query = base.WithTenantFilter(ctx, query, "student")

	err := query.Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by guardian phone",
			Err: err,
		}
	}

	return students, nil
}

// studentWithPersonAndGroup is the scan target for queries that join students, persons, and groups.
type studentWithPersonAndGroup struct {
	Student   *users.Student `bun:"student"`
	Person    *users.Person  `bun:"person"`
	GroupName string         `bun:"group_name"`
}

// newStudentWithGroupQuery returns a select query pre-configured with student+person column
// expressions and the person JOIN. Callers add group JOIN, WHERE, and ORDER as needed.
func (r *StudentRepository) newStudentWithGroupQuery(ctx context.Context, results *[]*studentWithPersonAndGroup) *bun.SelectQuery {
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(results).
		ModelTableExpr(`users.students AS "student"`).
		ColumnExpr(`"student".id AS "student__id", "student".created_at AS "student__created_at", "student".updated_at AS "student__updated_at"`).
		ColumnExpr(`"student".tenant_id AS "student__tenant_id"`).
		ColumnExpr(`"student".person_id AS "student__person_id", "student".school_class AS "student__school_class"`).
		ColumnExpr(`"student".guardian_name AS "student__guardian_name", "student".guardian_contact AS "student__guardian_contact"`).
		ColumnExpr(`"student".guardian_email AS "student__guardian_email", "student".guardian_phone AS "student__guardian_phone"`).
		ColumnExpr(`"student".group_id AS "student__group_id"`).
		ColumnExpr(`"student".extra_info AS "student__extra_info", "student".supervisor_notes AS "student__supervisor_notes"`).
		ColumnExpr(`"student".health_info AS "student__health_info", "student".pickup_status AS "student__pickup_status"`).
		// departure_companion_note is scanonly and hydrated via
		// hydrateBusDaysForStudents — never selected here (#1694).
		ColumnExpr(`"person".id AS "person__id", "person".created_at AS "person__created_at", "person".updated_at AS "person__updated_at"`).
		ColumnExpr(`"person".first_name AS "person__first_name", "person".last_name AS "person__last_name"`).
		ColumnExpr(`"person".tag_id AS "person__tag_id", "person".account_id AS "person__account_id"`).
		Join(`INNER JOIN users.persons AS "person" ON "person".id = "student".person_id`)

	query = base.WithTenantFilter(ctx, query, "student")

	return query
}

// mapStudentGroupResults converts raw scan results into StudentWithGroupInfo slices.
func mapStudentGroupResults(results []*studentWithPersonAndGroup) []*users.StudentWithGroupInfo {
	out := make([]*users.StudentWithGroupInfo, len(results))
	for i, result := range results {
		student := result.Student
		if result.Person != nil && result.Person.ID != 0 {
			student.Person = result.Person
		}
		out[i] = &users.StudentWithGroupInfo{
			Student:   student,
			GroupName: result.GroupName,
		}
	}
	return out
}

func (r *StudentRepository) hydrateBusDaysForGroupInfo(ctx context.Context, infos []*users.StudentWithGroupInfo) error {
	students := make([]*users.Student, 0, len(infos))
	for _, info := range infos {
		if info != nil && info.Student != nil {
			students = append(students, info.Student)
		}
	}
	return r.hydrateBusDaysForStudents(ctx, students)
}

func (r *StudentRepository) hydrateBusDaysForStudents(ctx context.Context, students []*users.Student) error {
	if len(students) == 0 {
		return nil
	}

	ids := make([]int64, 0, len(students))
	byID := make(map[int64]*users.Student, len(students))
	for _, student := range students {
		if student == nil || student.ID == 0 {
			continue
		}
		ids = append(ids, student.ID)
		byID[student.ID] = student
	}
	if len(ids) == 0 {
		return nil
	}

	// departure_days (1.15.120) is the source of truth, but pickup_days
	// (1.15.116) and departure_days each landed after bus_days (1.15.112), so
	// there are schema windows where a later column does not exist yet. Detect
	// each independently and only select present columns — otherwise a missing
	// column would fail the whole query and drop the bus_days hydration with it.
	// All four are optional columns that landed in their own migrations; resolve
	// their existence in one query so hydration stays batched (see
	// hasStudentColumns). departure_companion_note (1.15.138) is scanonly, so the
	// generic model select never fetches it: hydrate it here, behind the same
	// column-existence guard, so it survives the schema windows where the column
	// is absent.
	cols, err := r.hasStudentColumns(ctx, "pickup_days", "departure_days", "allowed_departure_modes", "departure_companion_note")
	if err != nil {
		return err
	}
	hasPickupDays := cols["pickup_days"]
	hasDepartureDays := cols["departure_days"]
	hasAllowedDepartureModes := cols["allowed_departure_modes"]
	hasCompanionNote := cols["departure_companion_note"]

	type weekdayDaysRow struct {
		ID                     int64                       `bun:"id"`
		BusDays                users.BusDays               `bun:"bus_days"`
		PickupDays             users.PickupDays            `bun:"pickup_days"`
		DepartureDays          users.DepartureDays         `bun:"departure_days"`
		AllowedDepartureModes  users.AllowedDepartureModes `bun:"allowed_departure_modes"`
		DepartureCompanionNote *string                     `bun:"departure_companion_note"`
	}
	var rows []weekdayDaysRow
	pickupCol := `, NULL::jsonb AS pickup_days`
	if hasPickupDays {
		pickupCol = `, "student".pickup_days`
	}
	departureCol := `, NULL::jsonb AS departure_days`
	if hasDepartureDays {
		departureCol = `, "student".departure_days`
	}
	allowedCol := `, NULL::jsonb AS allowed_departure_modes`
	if hasAllowedDepartureModes {
		allowedCol = `, "student".allowed_departure_modes`
	}
	noteCol := `, NULL::text AS departure_companion_note`
	if hasCompanionNote {
		noteCol = `, "student".departure_companion_note`
	}
	sql := `SELECT "student".id, "student".bus_days` + pickupCol + departureCol + allowedCol + noteCol +
		` FROM users.students AS "student" WHERE "student".id IN (?)`
	args := []any{bun.List(ids)}
	if tenantID := tenant.FromContext(ctx); tenantID > 0 {
		sql += ` AND "student".tenant_id = ?`
		args = append(args, tenantID)
	}

	if err := base.GetDB(ctx, r.db).NewRaw(sql, args...).Scan(ctx, &rows); err != nil {
		// Migration tests exercise historical schemas with the current model.
		// Before 1.15.112 the bus_days column legitimately does not exist;
		// callers can still rely on the legacy bus flag in those schema states.
		if strings.Contains(err.Error(), "bus_days") {
			return nil
		}
		return &modelBase.DatabaseError{
			Op:  "hydrate student weekday days",
			Err: err,
		}
	}

	for _, row := range rows {
		student := byID[row.ID]
		if student == nil {
			continue
		}
		// The companion note is independent of which departure projection wins
		// below, so set it before the branches (all of which `continue`).
		if hasCompanionNote {
			student.DepartureCompanionNote = row.DepartureCompanionNote
		}
		if allowed := row.AllowedDepartureModes.Normalize(); hasAllowedDepartureModes && allowed.HasAny() {
			student.AllowedDepartureModes = allowed
			student.DepartureDays = allowed.DepartureDays()
			student.BusDays = allowed.BusDays()
			student.PickupDays = allowed.PickupDays()
			student.SnapshotDeparturePlan()
			continue
		}
		// departure_days is authoritative when it carries any non-alone day:
		// derive the legacy per-day views from it. When it is empty we cannot
		// tell "genuinely all alone" from "not yet backfilled" (e.g. the
		// pre-1.15.120 schema window, or a row written straight to bus_days),
		// so we fall back to the stored legacy maps — which are empty too for a
		// truly all-alone child, giving the same result either way.
		if departure := row.DepartureDays.Normalize(); hasDepartureDays && departure.HasAny() {
			student.DepartureDays = departure
			student.AllowedDepartureModes = users.AllowedDepartureModesFromDeparture(departure)
			student.BusDays = departure.BusDays()
			student.PickupDays = departure.PickupDays()
			student.SnapshotDeparturePlan()
			continue
		}
		student.BusDays = row.BusDays.Normalize()
		student.PickupDays = row.PickupDays.Normalize()
		student.DepartureDays = users.DepartureDaysFromLegacy(student.BusDays, student.PickupDays)
		student.AllowedDepartureModes = users.AllowedDepartureModesFromLegacy(student.BusDays, student.PickupDays)
		// The hydrated plan is the baseline Update rebases untouched fields
		// onto — see rebaseUntouchedDeparturePlan.
		student.SnapshotDeparturePlan()
	}
	return nil
}

// FindByTeacherIDWithGroups retrieves students with group names supervised by a teacher
func (r *StudentRepository) FindByTeacherIDWithGroups(ctx context.Context, teacherID int64) ([]*users.StudentWithGroupInfo, error) {
	var results []*studentWithPersonAndGroup
	err := r.newStudentWithGroupQuery(ctx, &results).
		ColumnExpr(`"group".name AS "group_name"`).
		Join(`INNER JOIN education.groups AS "group" ON "group".id = "student".group_id`).
		Join(`INNER JOIN education.group_teacher AS "gt" ON "gt".group_id = "group".id`).
		Where(`"gt".teacher_id = ? AND "student".group_id IS NOT NULL`, teacherID).
		Distinct().
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by teacher ID with groups",
			Err: err,
		}
	}

	infos := mapStudentGroupResults(results)
	if err := r.hydrateBusDaysForGroupInfo(ctx, infos); err != nil {
		return nil, err
	}
	return infos, nil
}

// FindAllWithGroups retrieves all students with their group names.
// Uses LEFT JOIN on groups so students without a group assignment are included.
func (r *StudentRepository) FindAllWithGroups(ctx context.Context) ([]*users.StudentWithGroupInfo, error) {
	var results []*studentWithPersonAndGroup
	err := r.newStudentWithGroupQuery(ctx, &results).
		ColumnExpr(`COALESCE("group".name, '') AS "group_name"`).
		Join(`LEFT JOIN education.groups AS "group" ON "group".id = "student".group_id`).
		Distinct().
		OrderExpr(`"person".last_name, "person".first_name`).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find all with groups",
			Err: err,
		}
	}

	infos := mapStudentGroupResults(results)
	if err := r.hydrateBusDaysForGroupInfo(ctx, infos); err != nil {
		return nil, err
	}
	return infos, nil
}

// LockPhotoFeature acquires the per-tenant pg_advisory_xact_lock that
// serializes concurrent transactions affecting the student-photo feature.
// Both PurgeAllPhotos (called from the OnValueSet hook on a feature
// disable) and the upload handler in api/students/photo.go take this
// lock so they cannot interleave: without it, an upload tx could read
// "feature enabled" from a uncommitted-by-another-tx setting state and
// commit a fresh photo_path AFTER a concurrent disable's purge CTE had
// already evaluated, leaving a stored file the disable's post-commit
// purge never knew about.
//
// pg_advisory_xact_lock releases automatically at COMMIT/ROLLBACK of the
// surrounding tenant tx, so callers must already be inside one — there
// is no separate Unlock method to forget. Returns an error if no tenant
// context is set; lock keys must be tenant-scoped to avoid one tenant's
// disable serializing every other tenant's uploads.
func (r *StudentRepository) LockPhotoFeature(ctx context.Context) error {
	tenantID := tenant.FromContext(ctx)
	if tenantID <= 0 {
		return fmt.Errorf("LockPhotoFeature: tenant_id must be set")
	}
	if tenantID > 0x7fffffff {
		return fmt.Errorf("LockPhotoFeature: tenant_id %d exceeds advisory-lock obj id range", tenantID)
	}
	_, err := base.GetDB(ctx, r.db).
		NewRaw("SELECT pg_advisory_xact_lock(?, ?)", studentPhotoFeatureLockClass, int32(tenantID)).
		Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{Op: "lock_photo_feature", Err: err}
	}
	return nil
}

// FindByIDForUpdate fetches a student row with a SELECT … FOR UPDATE so
// the caller can re-validate state (consent, photo_path, …) under the
// same row lock the subsequent UPDATE will use. Used by the photo upload
// flow to close a lost-update race against concurrent consent
// withdrawals: a stale snapshot from before the withdrawal would
// otherwise re-write the cleared consent columns when the upload's
// full-row UPDATE commits.
//
// Returns sql.ErrNoRows wrapped in DatabaseError if the row doesn't
// exist. RLS / TenantWhere scopes visibility to the current tenant.
func (r *StudentRepository) FindByIDForUpdate(ctx context.Context, id int64) (*users.Student, error) {
	return r.findByIDForUpdate(ctx, id, false)
}

// FindByIDForUpdateNoWait is FindByIDForUpdate that never blocks: when another
// transaction already holds the row, PostgreSQL raises 55P03 immediately
// instead of waiting.
//
// It exists for the one situation where waiting is unsafe — taking a lock on an
// id BELOW an id this transaction already holds. Every companion writer acquires
// student rows in ascending id order, so a downward acquisition inverts that
// order and can deadlock against a writer coming the other way. The companion
// graph is not fully known before the first lock (it is read from the edge
// table, which a concurrent commit can grow), so downward acquisitions cannot
// be designed away — they are made non-blocking instead, and the caller turns
// the refusal into the retriable users.ErrCompanionLockBusy.
func (r *StudentRepository) FindByIDForUpdateNoWait(ctx context.Context, id int64) (*users.Student, error) {
	return r.findByIDForUpdate(ctx, id, true)
}

func (r *StudentRepository) findByIDForUpdate(ctx context.Context, id int64, noWait bool) (*users.Student, error) {
	lockClause := "UPDATE"
	op := "find_by_id_for_update"
	if noWait {
		lockClause = "UPDATE NOWAIT"
		op = "find_by_id_for_update_nowait"
	}

	student := new(users.Student)
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(student).
		ModelTableExpr(tableExprUsersStudentsAsStudent).
		Where(`"student".id = ?`, id).
		For(lockClause)

	query = base.WithTenantFilter(ctx, query, "student")

	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{Op: op, Err: err}
	}
	if err := r.hydrateBusDaysIfPresent(ctx, []*users.Student{student}); err != nil {
		return nil, err
	}
	return student, nil
}

func (r *StudentRepository) hydrateBusDaysIfPresent(ctx context.Context, students []*users.Student) error {
	hasBusDays, err := r.hasStudentColumn(ctx, "bus_days")
	if err != nil {
		return err
	}
	if !hasBusDays {
		return nil
	}
	return r.hydrateBusDaysForStudents(ctx, students)
}

// hasStudentColumns resolves the existence of several users.students columns in
// a SINGLE information_schema query, instead of one round-trip per column.
// Hydration checks four optional departure columns at once; folding them into
// one query keeps the per-request query budget batched (api/timetable guards
// this with a query-count ceiling) rather than N+1.
func (r *StudentRepository) hasStudentColumns(ctx context.Context, columns ...string) (map[string]bool, error) {
	present := make(map[string]bool, len(columns))
	if len(columns) == 0 {
		return present, nil
	}
	var existing []string
	err := base.GetDB(ctx, r.db).NewRaw(`
		SELECT column_name
		FROM information_schema.columns
		WHERE table_schema = 'users'
		  AND table_name = 'students'
		  AND column_name IN (?)
	`, bun.List(columns)).Scan(ctx, &existing)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "check students columns",
			Err: err,
		}
	}
	for _, col := range existing {
		present[col] = true
	}
	return present, nil
}

// applyCompanionLinkDays fills the non-persisted Student.DepartureCompanionDays
// from users.student_companions, so Student.Validate() accepts an accompanied
// weekday whose "mit wem" is answered by a link on THAT day instead of the
// free-text note.
//
// Only queried when the answer can change the outcome: an update that already
// carries a note, or whose accompanied days are already covered by what the
// caller marked, is untouched, so the common path pays nothing.
func (r *StudentRepository) applyCompanionLinkDays(ctx context.Context, student *users.Student) error {
	if student.ID <= 0 || student.DepartureCompanionNote != nil {
		return nil
	}
	uncovered := false
	for day, accompanied := range users.AccompaniedWeekdays(student.AllowedDepartureModes, student.DepartureDays) {
		if accompanied && !student.DepartureCompanionDays[day] {
			uncovered = true
			break
		}
	}
	if !uncovered {
		return nil
	}

	// The table landed in 1.15.209 and the migration tests exercise historical
	// schemas with the current model, so probe it like the optional departure
	// columns are probed. Absent table means no links, which keeps the note
	// requirement in force — failing closed.
	exists, err := r.hasCompanionTable(ctx)
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}

	var weekdays []int
	if err := base.GetDB(ctx, r.db).NewRaw(`
		SELECT DISTINCT weekday
		FROM users.student_companions
		WHERE student_low_id = ? OR student_high_id = ?
	`, student.ID, student.ID).Scan(ctx, &weekdays); err != nil {
		return &modelBase.DatabaseError{Op: "check student companion links", Err: err}
	}

	for _, weekday := range weekdays {
		if day, ok := users.CompanionWeekdayKeys[weekday]; ok {
			student.MarkDepartureCompanionDays(day)
		}
	}
	return nil
}

// hasCompanionTable reports whether users.student_companions exists. The table
// landed in 1.15.209 and the migration tests exercise historical schemas with
// the current model, so every companion read in this repository is guarded by
// this probe.
func (r *StudentRepository) hasCompanionTable(ctx context.Context) (bool, error) {
	var exists bool
	if err := base.GetDB(ctx, r.db).NewRaw(`
		SELECT EXISTS (
			SELECT 1
			FROM information_schema.tables
			WHERE table_schema = 'users' AND table_name = 'student_companions'
		)
	`).Scan(ctx, &exists); err != nil {
		return false, &modelBase.DatabaseError{Op: "check student companions table", Err: err}
	}
	return exists, nil
}

func (r *StudentRepository) hasStudentColumn(ctx context.Context, column string) (bool, error) {
	var exists bool
	err := base.GetDB(ctx, r.db).NewRaw(`
		SELECT EXISTS (
			SELECT 1
			FROM information_schema.columns
			WHERE table_schema = 'users'
			  AND table_name = 'students'
			  AND column_name = ?
		)
	`, column).Scan(ctx, &exists)
	if err != nil {
		return false, &modelBase.DatabaseError{
			Op:  "check students column",
			Err: err,
		}
	}
	return exists, nil
}

// PurgeAllPhotos clears photo_path for every row visible in the current
// tenant context (RLS scopes it) and returns the cleared URLs so the
// caller can unlink the files. Photo consent metadata is left intact —
// withdrawing parental consent is a separate audit event from a
// tenant-wide feature toggle.
//
// Acquires LockPhotoFeature first to serialize against concurrent
// uploads. Without the advisory lock, an upload that read "feature
// enabled" before a disable's SetValue committed could still commit a
// fresh photo_path AFTER the disable's purge CTE evaluated — the CTE
// only locks rows whose photo_path is currently non-null, so a row
// that's still NULL at the moment the CTE evaluates isn't serialized
// against the upload's UPDATE on that same row. The advisory lock
// closes that window: the upload waits behind the disable (or the
// disable waits behind the upload), and whichever runs second sees the
// other's committed state.
//
// Implemented as a single SQL statement (CTE + joined UPDATE … RETURNING)
// so the rows we identify and the rows we clear are exactly the same set.
// A previous two-step variant (SELECT then UPDATE) raced with concurrent
// uploads in a different way: an upload that committed a fresh photo_path
// between the SELECT and the UPDATE would be cleared by the UPDATE (its
// row was non-null at that moment) but its URL was missing from the
// SELECT's snapshot, so the post-commit unlink left the file orphaned.
// The single-statement form fixes that case; the advisory lock fixes the
// "still-NULL when CTE evaluates" case.
//
// Postgres returns the post-update value from a plain UPDATE … RETURNING
// (which would always be NULL here); the CTE join is the standard way to
// surface the OLD value in a single statement.
func (r *StudentRepository) PurgeAllPhotos(ctx context.Context) ([]string, error) {
	if err := r.LockPhotoFeature(ctx); err != nil {
		return nil, err
	}

	type photoRow struct {
		PhotoPath string `bun:"photo_path"`
	}

	const baseQuery = `
		WITH locked AS (
			SELECT id, photo_path
			FROM users.students
			WHERE photo_path IS NOT NULL%s
			FOR UPDATE
		)
		UPDATE users.students AS student
		SET photo_path = NULL
		FROM locked
		WHERE student.id = locked.id
		RETURNING locked.photo_path
	`

	var rows []photoRow
	var err error
	if where, val, ok := base.TenantWhere(ctx, "student"); ok {
		// Defense-in-depth tenant filter; the CTE alias is "student"-less so
		// substitute the qualified column directly. RLS already scopes the
		// query when the caller is inside a tenant tx (the standard path);
		// the explicit predicate guards against a future caller running this
		// outside the tenant middleware.
		_ = where // documented for parity with other repo methods
		err = base.GetDB(ctx, r.db).
			NewRaw(fmt.Sprintf(baseQuery, " AND tenant_id = ?"), val).
			Scan(ctx, &rows)
	} else {
		err = base.GetDB(ctx, r.db).
			NewRaw(fmt.Sprintf(baseQuery, "")).
			Scan(ctx, &rows)
	}
	if err != nil {
		return nil, &modelBase.DatabaseError{Op: "purge_all_photos", Err: err}
	}

	if len(rows) == 0 {
		return nil, nil
	}

	urls := make([]string, 0, len(rows))
	for _, row := range rows {
		if row.PhotoPath != "" {
			urls = append(urls, row.PhotoPath)
		}
	}
	return urls, nil
}

// FindByNameAndClass retrieves students by first name, last name, and school class (for import duplicate detection)
func (r *StudentRepository) FindByNameAndClass(ctx context.Context, firstName, lastName, schoolClass string) ([]*users.Student, error) {
	var students []*users.Student
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&students).
		ModelTableExpr(`users.students AS "student"`).
		Join(`INNER JOIN users.persons AS "person" ON "person".id = "student".person_id`).
		Where(`LOWER("person".first_name) = LOWER(?)`, firstName).
		Where(`LOWER("person".last_name) = LOWER(?)`, lastName).
		Where(`LOWER("student".school_class) = LOWER(?)`, schoolClass)

	query = base.WithTenantFilter(ctx, query, "student")

	err := query.Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by name and class",
			Err: err,
		}
	}

	return students, nil
}

// UpdateStatus changes the lifecycle status of a single student. Tenant-scoped
// via context. Returns an error if no row was affected (wrong tenant or
// missing student).
func (r *StudentRepository) UpdateStatus(ctx context.Context, studentID int64, newStatus users.StudentStatus) error {
	query := base.GetDB(ctx, r.db).NewUpdate().
		TableExpr(`users.students AS "student"`).
		Set("status = ?", string(newStatus)).
		Set("updated_at = NOW()").
		Where(`"student".id = ?`, studentID)

	query = base.WithTenantFilter(ctx, query, "student")

	result, err := query.Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "update student status",
			Err: err,
		}
	}

	return base.AssertRowsAffected(result, 1, "update student status")
}

// FindPendingDueForActivation returns students whose status='pending' and
// enrolled_from <= asOf within the current tenant context. Drives the
// pending→active half of the activate-students scheduler tick.
func (r *StudentRepository) FindPendingDueForActivation(ctx context.Context, asOf timezone.Date) ([]*users.Student, error) {
	var students []*users.Student
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&students).
		ModelTableExpr(tableExprUsersStudentsAsStudent).
		Where(`"student".status = ?`, string(users.StudentStatusPending)).
		Where(`"student".enrolled_from IS NOT NULL`).
		Where(`"student".enrolled_from <= ?`, asOf)

	query = base.WithTenantFilter(ctx, query, "student")

	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find pending students due for activation",
			Err: err,
		}
	}

	return students, nil
}

// FindActiveDueForDeactivation returns students whose status='active' and
// enrolled_until <= asOf within the current tenant context. Drives the
// active→inactive half of the activate-students scheduler tick.
func (r *StudentRepository) FindActiveDueForDeactivation(ctx context.Context, asOf timezone.Date) ([]*users.Student, error) {
	var students []*users.Student
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&students).
		ModelTableExpr(tableExprUsersStudentsAsStudent).
		Where(`"student".status = ?`, string(users.StudentStatusActive)).
		Where(`"student".enrolled_until IS NOT NULL`).
		Where(`"student".enrolled_until <= ?`, asOf)

	query = base.WithTenantFilter(ctx, query, "student")

	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find active students due for deactivation",
			Err: err,
		}
	}

	return students, nil
}
