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

// The lifecycle status and the care window are the owner's too (#3349). These
// stay only as the interface the retained StudentRepository declares; the
// composition root routes them to People Directory.
func (r *StudentRepository) UpdateStatus(context.Context, int64, users.StudentStatus) error {
	return errStudentWritesMoved
}

func (r *StudentRepository) TransitionStatus(
	context.Context, int64, users.StudentStatus, users.StudentStatus,
) (bool, error) {
	return false, errStudentWritesMoved
}

func (r *StudentRepository) SetEnrolledUntilByIDs(context.Context, []int64, *timezone.Date) (int64, error) {
	return 0, errStudentWritesMoved
}

func (r *StudentRepository) SetEnrollmentWindowByID(
	context.Context, int64, timezone.Date, users.StudentStatus,
) error {
	return errStudentWritesMoved
}

func (r *StudentRepository) FindCareBoundsByIDs(context.Context, []int64) (map[int64]timezone.Date, error) {
	return nil, errStudentWritesMoved
}

// These reads moved to People Directory in #3349. They stay on this type
// because the retained StudentRepository interface still declares them, and the
// composition root routes them to the owner (database/repositories.NewStudentReads).
func (r *StudentRepository) FindByID(context.Context, any) (*users.Student, error) {
	return nil, errStudentWritesMoved
}

func (r *StudentRepository) FindByPersonID(context.Context, int64) (*users.Student, error) {
	return nil, errStudentWritesMoved
}

func (r *StudentRepository) FindByIDs(context.Context, []int64) (map[int64]*users.Student, error) {
	return nil, errStudentWritesMoved
}

func (r *StudentRepository) FindReadScopeByIDs(context.Context, []int64) (map[int64]*users.Student, error) {
	return nil, errStudentWritesMoved
}

func (r *StudentRepository) FindByGroupID(context.Context, int64) ([]*users.Student, error) {
	return nil, errStudentWritesMoved
}

func (r *StudentRepository) FindByGroupIDs(context.Context, []int64) ([]*users.Student, error) {
	return nil, errStudentWritesMoved
}

func (r *StudentRepository) FindBySchoolClass(context.Context, string) ([]*users.Student, error) {
	return nil, errStudentWritesMoved
}

func (r *StudentRepository) ExistsEnrolledByNameAndBirthday(
	context.Context, int64, string, string, timezone.Date,
) (bool, error) {
	return false, errStudentWritesMoved
}

func (r *StudentRepository) FindEnrolledStudentIDByNameAndBirthday(
	context.Context, int64, string, string, timezone.Date,
) (*int64, error) {
	return nil, errStudentWritesMoved
}

func (r *StudentRepository) ListSchoolClasses(context.Context) ([]string, error) {
	return nil, errStudentWritesMoved
}

func (r *StudentRepository) ListIDs(context.Context) ([]int64, error) {
	return nil, errStudentWritesMoved
}

func (r *StudentRepository) CountByGroupIDs(context.Context, []int64) (map[int64]int, error) {
	return nil, errStudentWritesMoved
}

func (r *StudentRepository) FindByGuardianEmail(context.Context, string) ([]*users.Student, error) {
	return nil, errStudentWritesMoved
}

func (r *StudentRepository) FindByGuardianPhone(context.Context, string) ([]*users.Student, error) {
	return nil, errStudentWritesMoved
}

func (r *StudentRepository) FindPendingDueForActivation(context.Context, timezone.Date) ([]*users.Student, error) {
	return nil, errStudentWritesMoved
}

func (r *StudentRepository) FindActiveDueForDeactivation(context.Context, timezone.Date) ([]*users.Student, error) {
	return nil, errStudentWritesMoved
}

func (r *StudentRepository) FindByIDsForUpdate(context.Context, []int64) (map[int64]*users.Student, error) {
	return nil, errStudentWritesMoved
}
