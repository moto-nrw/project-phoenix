// backend/database/repositories/users/student.go
package users

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// studentClassWritesLockClass is the pg_advisory_xact_lock class id ("clas" in
// ASCII) that serializes a grade transition apply/revert against every write
// which could put a child INTO one of the classes it is transitioning.
//
// Row locks cannot express that: a student created in a mapped class — or moved
// in from an unmapped one — has no row the transition could have locked. The
// apply's post-lock re-read (services/education.ensureNoLateArrivals) therefore
// only sees arrivals that COMMITTED before it ran; one committing after that
// statement but before the apply commits was silently left behind in a class the
// transition had just emptied, while the transition reported success and wrote
// no history row a revert could undo it with (#405 review).
//
// Writers take the gate SHARED — shared holders never conflict with each other,
// so the normal path costs one in-memory advisory-lock round-trip and never
// blocks. Apply/revert take it EXCLUSIVE for their whole transaction, which is
// the only time a writer waits. Both forms are transaction-scoped: they release
// at COMMIT/ROLLBACK (no Unlock to forget), and re-acquiring a lock the same
// transaction already holds never waits — the transition's own student reads and
// writes below pass straight through its exclusive hold.
//
// LOCK ORDER (must stay acyclic):
//
//  1. this gate, BEFORE the tenant recurrence gate and the grade-transition gate
//     (services/education.lockRecurrenceThenTransitions takes all three in that
//     order), and
//  2. BEFORE any users.students row lock — every acquisition below sits in front
//     of the row lock taken by the same method, so no transaction can hold a
//     student row and then queue for the gate while the gate holder waits for
//     that row.
const studentClassWritesLockClass int32 = 0x636C6173

// Table name constants (S1192 - avoid duplicate string literals)
const (
	tableUsersStudents              = "users.students"
	tableExprUsersStudentsAsStudent = "users.students AS student"
)

// StudentRepository implements users.StudentRepository interface
type StudentRepository struct {
	*base.Repository[*users.Student]
	db *bun.DB
	// teacherGroupIDs resolves education.group_teacher through composition. This
	// Postgres adapter stays independent of the School Membership owner and can
	// resolve several teachers without one owner call per teacher.
	teacherGroupIDs      func(context.Context, int64) ([]int64, error)
	teacherStaffGroupIDs func(context.Context, []int64) ([]int64, error)
}

// NewStudentRepository creates a new StudentRepository
func NewStudentRepository(db *bun.DB) users.StudentRepository {
	repo := base.NewRepository[*users.Student](db, tableUsersStudents, "Student")
	repo.TenantScoped = true
	return &StudentRepository{
		Repository: repo,
		db:         db,
	}
}

func (r *StudentRepository) BindTeacherGroupIDs(query func(context.Context, int64) ([]int64, error)) {
	if query == nil {
		panic("student repository: school membership is required")
	}
	r.teacherGroupIDs = query
}

func (r *StudentRepository) BindTeacherStaffGroupIDs(query func(context.Context, []int64) ([]int64, error)) {
	if query == nil {
		panic("student repository: school membership is required")
	}
	r.teacherStaffGroupIDs = query
}

// FindByID retrieves a student by their ID.
func (r *StudentRepository) FindByID(ctx context.Context, id interface{}) (*users.Student, error) {
	student, err := r.Repository.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := r.hydrateBusDaysForStudents(ctx, []*users.Student{student}); err != nil {
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
			Err: base.TranslateNotFound(err),
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
			Err: base.TranslateNotFound(err),
		}
	}

	result := make(map[int64]*users.Student, len(students))
	for _, student := range students {
		result[student.ID] = student
	}
	if err := r.hydrateBusDaysForStudents(ctx, students); err != nil {
		return nil, err
	}

	return result, nil
}

// FindReadScopeByIDs retrieves a lightweight projection of the given students —
// only id, group_id, person_id, and school_class — in a single primary-key
// IN-list query. Unlike FindByIDs it does NOT run hydrateBusDaysForStudents, so
// it avoids the extra jsonb weekday-hydration round-trip. Callers that only
// gate read access and display a name (e.g. the
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
			Err: base.TranslateNotFound(err),
		}
	}

	result := make(map[int64]*users.Student, len(students))
	for _, student := range students {
		result[student.ID] = student
	}
	return result, nil
}

// FindByGroupID retrieves students by their group ID. Alumni (graduated,
// soft-deleted) are excluded — their rows only exist for transition reverts.
func (r *StudentRepository) FindByGroupID(ctx context.Context, groupID int64) ([]*users.Student, error) {
	var students []*users.Student
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&students).
		ModelTableExpr(tableExprUsersStudentsAsStudent).
		Where("group_id = ?", groupID).
		Where(`"student".status <> ?`, string(users.StudentStatusAlumnus))

	query = base.WithTenantFilter(ctx, query, "student")

	err := query.Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by group ID",
			Err: base.TranslateNotFound(err),
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
		Where("group_id IN (?)", bun.List(groupIDs)).
		Where(`"student".status <> ?`, string(users.StudentStatusAlumnus))

	query = base.WithTenantFilter(ctx, query, "student")

	err := query.Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by group IDs",
			Err: base.TranslateNotFound(err),
		}
	}

	if err := r.hydrateBusDaysForStudents(ctx, students); err != nil {
		return nil, err
	}

	return students, nil
}

// FindBySchoolClass retrieves students by their school class. Alumni
// (graduated, soft-deleted) are excluded — staff-facing callers (arrival-plan
// bulk upsert, enrollment reports, calendar targeting) must never write to or
// count a graduate. Their rows survive only for transition reverts, which use
// the education repository's by-ID paths, not this lookup.
func (r *StudentRepository) FindBySchoolClass(ctx context.Context, schoolClass string) ([]*users.Student, error) {
	var students []*users.Student
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&students).
		ModelTableExpr(tableExprUsersStudentsAsStudent).
		Where("LOWER(TRIM(school_class)) = LOWER(TRIM(?))", schoolClass).
		Where(`"student".status <> ?`, string(users.StudentStatusAlumnus))

	query = base.WithTenantFilter(ctx, query, "student")

	err := query.Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by school class",
			Err: base.TranslateNotFound(err),
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
			Err: base.TranslateNotFound(err),
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
			Err: base.TranslateNotFound(err),
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
		Where(`"student".status <> ?`, string(users.StudentStatusAlumnus)).
		OrderExpr(`TRIM("student".school_class) ASC`)

	query = base.WithTenantFilter(ctx, query, "student")

	if err := query.Scan(ctx, &classes); err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "list school classes",
			Err: base.TranslateNotFound(err),
		}
	}

	return classes, nil
}

func (r *StudentRepository) ListIDs(ctx context.Context) ([]int64, error) {
	ids := make([]int64, 0)
	query := base.GetDB(ctx, r.db).NewSelect().
		TableExpr(`users.students AS "student"`).
		ColumnExpr(`"student".id`).
		OrderExpr(`"student".id`)
	query = base.WithTenantFilter(ctx, query, "student")
	if err := query.Scan(ctx, &ids); err != nil {
		return nil, &modelBase.DatabaseError{Op: "list student ids", Err: base.TranslateNotFound(err)}
	}
	return ids, nil
}

// applyEffectiveDeparturePlan resolves the stored projections into the plan
// that is in effect and records it as the baseline a later Update rebases
// untouched fields onto (see rebaseUntouchedDeparturePlan). The precedence
// itself belongs to the owner; this only moves the result onto the row.
func applyEffectiveDeparturePlan(student *users.Student, stored users.DeparturePlan) {
	effective := stored.Effective()
	student.AllowedDepartureModes = effective.AllowedDepartureModes
	student.DepartureDays = effective.DepartureDays
	student.BusDays = effective.BusDays
	student.PickupDays = effective.PickupDays
	student.SnapshotDeparturePlan()
}

// The child's own lifecycle moved to People Directory in #3349: the gates, the
// row lock order, the departure-plan resolution, the companion reconcile and
// the stranding refusal all live in modules/peopledirectory now. The four write
// entry points stay on this type because the retained StudentRepository
// interface still declares them, and the composition root routes them to the
// owner (database/repositories.bindStudentWrites).
//
// Reaching one of them here means a graph was built without the owner behind
// it. That is a configuration error, and saying so is better than writing the
// row through a path that no longer enforces any of the above.
var errStudentWritesMoved = errors.New(
	"student writes moved to People Directory: bind the directory before writing a child")

func (r *StudentRepository) Create(context.Context, *users.Student) error {
	return errStudentWritesMoved
}

func (r *StudentRepository) Update(context.Context, *users.Student) error {
	return errStudentWritesMoved
}

func (r *StudentRepository) Delete(context.Context, any) error {
	return errStudentWritesMoved
}

func (r *StudentRepository) VerifyCompanionStrandingBatch(context.Context) error {
	return errStudentWritesMoved
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

	// Exclude soft-deleted alumni by default so unscoped reads (e.g. database
	// statistics counting Student.List(ctx, nil)) never count graduates. A
	// caller that filters on status explicitly (pending / active / alumnus) is
	// respected and gets exactly what it asked for.
	if _, ok := filters["status"]; !ok {
		filter.NotIn("status", string(users.StudentStatusAlumnus))
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
	// Without an ORDER BY, PostgreSQL is free to return the same rows in a
	// different order for every execution, so two LIMIT/OFFSET requests over the
	// same selection can hand back the same child twice and never mention
	// another one at all (#2218 review). Anything walking this list page by page
	// — the Kindersuche does, once a selection exceeds one page — depends on a
	// total order, so fall back to the primary key when the caller did not ask
	// for a specific one. An explicit Sorting wins: it is then the caller's job
	// to make it total.
	listOptions := &modelBase.QueryOptions{}
	if options != nil {
		*listOptions = *options
	}
	if options == nil || options.Sorting == nil {
		listOptions.Sorting = &modelBase.Sorting{Fields: []modelBase.SortField{{
			Field:     "id",
			Direction: modelBase.SortAsc,
		}}}
	}

	students, err := r.Repository.ListWithOptions(ctx, listOptions)
	if err != nil {
		return nil, err
	}
	if len(students) == 0 {
		return nil, nil
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
		Where(`"student".status <> ?`, string(users.StudentStatusAlumnus)).
		GroupExpr(`"student".group_id`)

	query = base.WithTenantFilter(ctx, query, "student")

	err := query.Scan(ctx, &results)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "count by group IDs",
			Err: base.TranslateNotFound(err),
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
			Err: base.TranslateNotFound(err),
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
			Err: base.TranslateNotFound(err),
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
// Alumni are excluded for every caller — see the filter below.
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
		ColumnExpr(`"student".enrolled_from AS "student__enrolled_from", "student".enrolled_until AS "student__enrolled_until"`).
		// status carries the immediate-activation signal of users.EnrolledOn:
		// without it the scanned zero value drops an active child whose
		// enrolled_from still lies ahead, although the WHERE clause of
		// FindOverlappingWithGroups and the statistics room aggregate both
		// admit them (#2606).
		ColumnExpr(`"student".status AS "student__status"`).
		ColumnExpr(`"student".extra_info AS "student__extra_info", "student".supervisor_notes AS "student__supervisor_notes"`).
		ColumnExpr(`"student".health_info AS "student__health_info", "student".pickup_status AS "student__pickup_status"`).
		// departure_companion_note is scanonly and hydrated via
		// hydrateBusDaysForStudents — never selected here (#1694).
		ColumnExpr(`"person".id AS "person__id", "person".created_at AS "person__created_at", "person".updated_at AS "person__updated_at"`).
		ColumnExpr(`"person".first_name AS "person__first_name", "person".last_name AS "person__last_name"`).
		ColumnExpr(`"person".tag_id AS "person__tag_id", "person".account_id AS "person__account_id"`).
		Join(`INNER JOIN users.persons AS "person" ON "person".id = "student".person_id`)

	query = base.WithTenantFilter(ctx, query, "student")

	// Alumni (graduated students) are soft-deleted and must be invisible to
	// every staff-facing and kiosk roster. This shared builder backs
	// FindAllWithGroups and FindByTeacherIDWithGroups, which feed the IoT
	// student roster (GET /api/iot/students) and the calendar student picker;
	// without this filter graduates stayed visible on the tablet teacher list
	// and bracelet-assignment despite the documented promise (#405).
	// FindOverlappingWithGroups (statistics) shares the filter so its child
	// numbers cover the same population as the room aggregate (#2606).
	query = query.Where(`"student".status <> ?`, string(users.StudentStatusAlumnus))

	return query
}

// activeRosterEnrollmentFilter keeps a child whose care has ended from the
// current rosters. Historical readers use their own interval-overlap filter.
func activeRosterEnrollmentFilter(query *bun.SelectQuery) *bun.SelectQuery {
	// A child whose care has ended disappears from the same rosters for the
	// same reason (#2487): the tablet list, the bracelet assignment and the
	// calendar student picker all answer "which children does this school care
	// for", and from the day after their last care day they do not. Filtered
	// on the enrollment interval rather than the lifecycle status, because the
	// status only follows once the scheduler ticks.
	return query.Where(
		`("student".enrolled_until IS NULL OR "student".enrolled_until >= ?)`,
		timezone.TodayDate(),
	)
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

	// All departure columns (bus_days 1.15.112, pickup_days 1.15.116,
	// departure_days 1.15.120, allowed_departure_modes 1.15.130, the scanonly
	// departure_companion_note 1.15.138) are guaranteed present: the server
	// only starts against a fully migrated schema (VerifyStudentSchema at
	// boot). Hydration therefore selects them with one static query — the
	// per-request information_schema probes are gone (#2059).
	type weekdayDaysRow struct {
		ID                     int64                       `bun:"id"`
		BusDays                users.BusDays               `bun:"bus_days"`
		PickupDays             users.PickupDays            `bun:"pickup_days"`
		DepartureDays          users.DepartureDays         `bun:"departure_days"`
		AllowedDepartureModes  users.AllowedDepartureModes `bun:"allowed_departure_modes"`
		DepartureCompanionNote *string                     `bun:"departure_companion_note"`
	}
	var rows []weekdayDaysRow
	query := base.GetDB(ctx, r.db).NewSelect().
		TableExpr(`users.students AS "student"`).
		ColumnExpr(`"student".id, "student".bus_days, "student".pickup_days, "student".departure_days`).
		ColumnExpr(`"student".allowed_departure_modes, "student".departure_companion_note`).
		Where(`"student".id IN (?)`, bun.List(ids))
	query = base.WithTenantFilter(ctx, query, "student")

	if err := query.Scan(ctx, &rows); err != nil {
		return &modelBase.DatabaseError{
			Op:  "hydrate student weekday days",
			Err: base.TranslateNotFound(err),
		}
	}

	for _, row := range rows {
		student := byID[row.ID]
		if student == nil {
			continue
		}
		// The companion note is independent of which departure projection wins,
		// so it is set outside the resolution.
		student.DepartureCompanionNote = row.DepartureCompanionNote
		applyEffectiveDeparturePlan(student, users.DeparturePlan{
			AllowedDepartureModes: row.AllowedDepartureModes,
			DepartureDays:         row.DepartureDays,
			BusDays:               row.BusDays,
			PickupDays:            row.PickupDays,
		})
	}
	return nil
}

// FindByTeacherIDWithGroups retrieves students with group names supervised by a teacher
func (r *StudentRepository) FindByTeacherIDWithGroups(ctx context.Context, teacherID int64) ([]*users.StudentWithGroupInfo, error) {
	if r.teacherGroupIDs == nil {
		return nil, errors.New("student repository resolves teacher assignments through School Membership")
	}
	groupIDs, err := r.teacherGroupIDs(ctx, teacherID)
	if err != nil {
		return nil, &modelBase.DatabaseError{Op: "find by teacher ID with groups", Err: err}
	}
	if len(groupIDs) == 0 {
		return []*users.StudentWithGroupInfo{}, nil
	}
	var results []*studentWithPersonAndGroup
	err = activeRosterEnrollmentFilter(r.newStudentWithGroupQuery(ctx, &results)).
		Where(`"student".group_id IN (?)`, bun.List(groupIDs)).
		Distinct().
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by teacher ID with groups",
			Err: base.TranslateNotFound(err),
		}
	}

	infos := mapStudentGroupResults(results)
	if err := r.hydrateBusDaysForGroupInfo(ctx, infos); err != nil {
		return nil, err
	}
	return infos, nil
}

// FindByTeacherStaffIDsWithGroups retrieves the union of students supervised by
// teachers belonging to any requested staff ID. A shared child appears once.
func (r *StudentRepository) FindByTeacherStaffIDsWithGroups(ctx context.Context, staffIDs []int64) ([]*users.StudentWithGroupInfo, error) {
	if len(staffIDs) == 0 {
		return []*users.StudentWithGroupInfo{}, nil
	}
	if r.teacherStaffGroupIDs == nil {
		return nil, errors.New("student repository resolves teacher assignments through School Membership")
	}
	groupIDs, err := r.teacherStaffGroupIDs(ctx, staffIDs)
	if err != nil {
		return nil, &modelBase.DatabaseError{Op: "find by teacher IDs with groups", Err: err}
	}
	if len(groupIDs) == 0 {
		return []*users.StudentWithGroupInfo{}, nil
	}
	var results []*studentWithPersonAndGroup
	if err := activeRosterEnrollmentFilter(r.newStudentWithGroupQuery(ctx, &results)).
		Where(`"student".group_id IN (?)`, bun.List(groupIDs)).
		Distinct().
		Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{Op: "find by teacher IDs with groups", Err: base.TranslateNotFound(err)}
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
	err := activeRosterEnrollmentFilter(r.newStudentWithGroupQuery(ctx, &results)).
		Distinct().
		OrderExpr(`"person".last_name, "person".first_name`).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find all with groups",
			Err: base.TranslateNotFound(err),
		}
	}

	infos := mapStudentGroupResults(results)
	if err := r.hydrateBusDaysForGroupInfo(ctx, infos); err != nil {
		return nil, err
	}
	return infos, nil
}

// FindOverlappingWithGroups retrieves the non-alumni children who are enrolled
// on at least one day of [from, to], with their current group. Unlike the live
// roster this keeps children whose care has since ended and omits later
// enrollments.
//
// The membership rule is users.EnrolledOn, expressed here as an interval
// overlap so one query answers it for the whole window:
//
//   - enrolled_until must not lie before the window
//   - enrolled_from must not lie after it — unless immediate activation
//     applies, which lifts that bound to today for an active child, and today
//     is inside the window (callers pass today; statistics windows never end
//     after it, so this is the last day at most)
//   - a row with neither bound carries no interval, so an inactive status is
//     the only remaining signal and means "no longer enrolled"
//
// Alumni stay out: they are soft-deleted and invisible everywhere else, and
// the statistics room aggregate excludes them too — counting them here would
// give the child table a different population than the room table (#2606).
func (r *StudentRepository) FindOverlappingWithGroups(ctx context.Context, from, to, today timezone.Date) ([]*users.StudentWithGroupInfo, error) {
	var results []*studentWithPersonAndGroup
	query := r.newStudentWithGroupQuery(ctx, &results).
		Where(`("student".enrolled_until IS NULL OR "student".enrolled_until >= ?)`, from).
		Where(`NOT ("student".enrolled_from IS NULL AND "student".enrolled_until IS NULL AND "student".status = ?)`,
			string(users.StudentStatusInactive))

	if today.After(to) {
		query = query.Where(`("student".enrolled_from IS NULL OR "student".enrolled_from <= ?)`, to)
	} else {
		// Today lies inside the window, so an active child whose enrollment
		// has not formally started is nonetheless enrolled on that day.
		query = query.Where(
			`("student".enrolled_from IS NULL OR "student".enrolled_from <= ? OR "student".status = ?)`,
			to, string(users.StudentStatusActive),
		)
	}

	err := query.
		Distinct().
		OrderExpr(`"person".last_name, "person".first_name`).
		Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{Op: "find students overlapping range with groups", Err: base.TranslateNotFound(err)}
	}
	return mapStudentGroupResults(results), nil
}

// FindOverlappingWithGroupsOnDate translates the meal-plan composition
// boundary into the repository's calendar-date types. now is an instant so
// the immediate-activation rule uses the actual current Berlin date rather
// than applying the requested list date retroactively.
func (r *StudentRepository) FindOverlappingWithGroupsOnDate(ctx context.Context, value string, now time.Time) ([]*users.StudentWithGroupInfo, error) {
	date, err := timezone.ParseDate(value)
	if err != nil {
		return nil, fmt.Errorf("find students overlapping date: %w", err)
	}
	return r.FindOverlappingWithGroups(ctx, date, date, timezone.DateFromTime(now))
}

// lockClassWritesShared takes the SHARED per-tenant class-writes gate. Called at
// the top of every repository method that inserts a student, updates one, or
// takes a student row lock the caller will update under — always BEFORE that
// method's own row lock, which is what keeps the acquisition order acyclic.
func (r *StudentRepository) lockClassWritesShared(ctx context.Context) error {
	return r.lockClassWrites(ctx, true)
}

func (r *StudentRepository) lockClassWrites(ctx context.Context, shared bool) error {
	tenantID := tenant.FromContext(ctx)
	if tenantID <= 0 {
		// No tenant in context: the CLI / migration / seeding paths that run
		// outside the tenant transaction as superuser. There is no per-tenant
		// gate to take there, and a grade transition (a tenant-scoped HTTP
		// request) can never be one of those callers.
		return nil
	}
	if tenantID > 0x7fffffff {
		return fmt.Errorf("lockClassWrites: tenant_id %d exceeds advisory-lock obj id range", tenantID)
	}

	db := base.GetDB(ctx, r.db)
	var err error
	op := "lock_student_class_writes"
	if shared {
		op = "lock_student_class_writes_shared"
		_, err = db.NewRaw("SELECT pg_advisory_xact_lock_shared(?, ?)", studentClassWritesLockClass, int32(tenantID)).Exec(ctx)
	} else {
		_, err = db.NewRaw("SELECT pg_advisory_xact_lock(?, ?)", studentClassWritesLockClass, int32(tenantID)).Exec(ctx)
	}
	if err != nil {
		return &modelBase.DatabaseError{Op: op, Err: base.TranslateNotFound(err)}
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
	// Callers of this method lock the row in order to update it, so the gate has
	// to be taken here rather than only in Update — otherwise such a caller would
	// hold a student row and THEN queue behind a grade transition that is waiting
	// for exactly that row. noWait keeps its meaning for the ROW lock (55P03
	// instead of waiting, which is what the companion lock protocol relies on);
	// the gate itself is uncontended except while an apply/revert runs.
	if err := r.lockClassWritesShared(ctx); err != nil {
		return nil, err
	}

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
		return nil, &modelBase.DatabaseError{Op: op, Err: base.TranslateNotFound(err)}
	}
	if err := r.hydrateBusDaysForStudents(ctx, []*users.Student{student}); err != nil {
		return nil, err
	}
	return student, nil
}

// FindByIDsForUpdate fetches and locks the given student rows in one
// SELECT … ORDER BY id FOR UPDATE. Ascending id order is the project-wide
// student lock convention (see FindByIDForUpdateNoWait), so overlapping
// batch callers serialize on their first shared row instead of deadlocking.
// The class-writes shared gate is taken once for the whole batch, exactly
// like the single-row lock does per row. Unknown or foreign ids are simply
// absent from the returned map — callers re-validate per id.
func (r *StudentRepository) FindByIDsForUpdate(ctx context.Context, ids []int64) (map[int64]*users.Student, error) {
	result := make(map[int64]*users.Student, len(ids))
	if len(ids) == 0 {
		return result, nil
	}
	if err := r.lockClassWritesShared(ctx); err != nil {
		return nil, err
	}

	var students []*users.Student
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&students).
		ModelTableExpr(tableExprUsersStudentsAsStudent).
		Where(`"student".id IN (?)`, bun.List(ids)).
		OrderExpr(`"student".id ASC`).
		For("UPDATE")

	query = base.WithTenantFilter(ctx, query, "student")

	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{Op: "find_by_ids_for_update", Err: base.TranslateNotFound(err)}
	}
	if err := r.hydrateBusDaysForStudents(ctx, students); err != nil {
		return nil, err
	}
	for _, student := range students {
		result[student.ID] = student
	}
	return result, nil
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
			Err: base.TranslateNotFound(err),
		}
	}

	return base.AssertRowsAffected(result, 1, "update student status")
}

// TransitionStatus changes a student's lifecycle status only when the stored
// status still matches expected. It returns false without error when another
// writer changed or removed the row after the caller selected it.
//
// Background lifecycle work (the activate-students tick) selects due rows, then
// updates them one by one. An unconditional update by id resurrects a student
// whose status changed in that window: a grade transition graduating the child
// commits `alumnus`, the pending update waits on the same row lock, and then
// replaces it with `active` or `inactive` — putting a departed child back into
// every staff list, roster and export, past all the alumnus read filters and
// without any of apply's guards. Comparing against the status the caller
// actually saw makes that update a no-op instead (#405 review).
func (r *StudentRepository) TransitionStatus(
	ctx context.Context,
	studentID int64,
	expected users.StudentStatus,
	next users.StudentStatus,
) (bool, error) {
	query := base.GetDB(ctx, r.db).NewUpdate().
		TableExpr(`users.students AS "student"`).
		Set("status = ?", string(next)).
		Set("updated_at = NOW()").
		Where(`"student".id = ?`, studentID).
		Where(`"student".status = ?`, string(expected))

	query = base.WithTenantFilter(ctx, query, "student")

	result, err := query.Exec(ctx)
	if err != nil {
		return false, &modelBase.DatabaseError{
			Op:  "transition student status",
			Err: base.TranslateNotFound(err),
		}
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, &modelBase.DatabaseError{
			Op:  "transition student status",
			Err: base.TranslateNotFound(err),
		}
	}
	return affected == 1, nil
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
			Err: base.TranslateNotFound(err),
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
			Err: base.TranslateNotFound(err),
		}
	}

	return students, nil
}

// SetEnrolledUntilByIDs writes the enrollment interval's inclusive upper bound
// for a batch of children in one statement (#2487).
func (r *StudentRepository) SetEnrolledUntilByIDs(
	ctx context.Context, ids []int64, until *timezone.Date,
) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	query := base.GetDB(ctx, r.db).NewUpdate().
		Model((*users.Student)(nil)).
		ModelTableExpr(tableExprUsersStudentsAsStudent).
		Set("enrolled_until = ?", until).
		Set("updated_at = NOW()").
		Where(`"student".id IN (?)`, bun.List(ids))
	query = base.WithTenantFilter(ctx, query, "student")

	result, err := query.Exec(ctx)
	if err != nil {
		return 0, &modelBase.DatabaseError{Op: "set enrolled_until by ids", Err: base.TranslateNotFound(err)}
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return 0, &modelBase.DatabaseError{Op: "set enrolled_until by ids", Err: base.TranslateNotFound(err)}
	}
	return affected, nil
}

// SetEnrollmentWindowByID reopens one child's care: a new start day, no end
// day, and the lifecycle status the caller derived for today (#2487).
func (r *StudentRepository) SetEnrollmentWindowByID(
	ctx context.Context, id int64, from timezone.Date, status users.StudentStatus,
) error {
	query := base.GetDB(ctx, r.db).NewUpdate().
		Model((*users.Student)(nil)).
		ModelTableExpr(tableExprUsersStudentsAsStudent).
		Set("enrolled_from = ?", from).
		Set("enrolled_until = NULL").
		Set("status = ?", string(status)).
		Set("updated_at = NOW()").
		Where(`"student".id = ?`, id)
	query = base.WithTenantFilter(ctx, query, "student")

	result, err := query.Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{Op: "set enrollment window", Err: base.TranslateNotFound(err)}
	}
	return base.AssertRowsAffected(result, 1, "set enrollment window")
}

// FindCareBoundsByIDs projects the enrollment interval's upper bound for the
// given children (#2487).
func (r *StudentRepository) FindCareBoundsByIDs(
	ctx context.Context, ids []int64,
) (map[int64]timezone.Date, error) {
	bounds := make(map[int64]timezone.Date, len(ids))
	if len(ids) == 0 {
		return bounds, nil
	}
	var rows []struct {
		ID            int64         `bun:"id"`
		EnrolledUntil timezone.Date `bun:"enrolled_until"`
	}
	query := base.GetDB(ctx, r.db).NewSelect().
		Model((*users.Student)(nil)).
		ModelTableExpr(tableExprUsersStudentsAsStudent).
		ColumnExpr(`"student".id AS id`).
		ColumnExpr(`"student".enrolled_until AS enrolled_until`).
		Where(`"student".id IN (?)`, bun.List(ids)).
		Where(`"student".enrolled_until IS NOT NULL`)
	query = base.WithTenantFilter(ctx, query, "student")
	if err := query.Scan(ctx, &rows); err != nil {
		return nil, &modelBase.DatabaseError{Op: "find care bounds by ids", Err: base.TranslateNotFound(err)}
	}
	for _, row := range rows {
		bounds[row.ID] = row.EnrolledUntil
	}
	return bounds, nil
}
