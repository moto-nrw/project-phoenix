// backend/database/repositories/users/student.go
package users

import (
	"context"
	"errors"
	"fmt"
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

	if where, val, ok := base.TenantWhere(ctx, "student"); ok {
		query = query.Where(where, val)
	}

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

	if where, val, ok := base.TenantWhere(ctx, "student"); ok {
		query = query.Where(where, val)
	}

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

// FindByGroupID retrieves students by their group ID
func (r *StudentRepository) FindByGroupID(ctx context.Context, groupID int64) ([]*users.Student, error) {
	var students []*users.Student
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&students).
		ModelTableExpr(tableExprUsersStudentsAsStudent).
		Where("group_id = ?", groupID)

	if where, val, ok := base.TenantWhere(ctx, "student"); ok {
		query = query.Where(where, val)
	}

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

	if where, val, ok := base.TenantWhere(ctx, "student"); ok {
		query = query.Where(where, val)
	}

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
		Where("LOWER(school_class) = LOWER(?)", schoolClass)

	if where, val, ok := base.TenantWhere(ctx, "student"); ok {
		query = query.Where(where, val)
	}

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

// AssignToGroup assigns a student to a group
func (r *StudentRepository) AssignToGroup(ctx context.Context, studentID int64, groupID int64) error {
	query := base.GetDB(ctx, r.db).NewUpdate().
		Model((*users.Student)(nil)).
		ModelTableExpr(`users.students AS "student"`).
		Set("group_id = ?", groupID).
		Where(`"student".id = ?`, studentID)

	if where, val, ok := base.TenantWhere(ctx, "student"); ok {
		query = query.Where(where, val)
	}

	result, err := query.Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "assign to group",
			Err: err,
		}
	}

	return base.AssertRowsAffected(result, 1, "assign to group")
}

// RemoveFromGroup removes a student from their group
func (r *StudentRepository) RemoveFromGroup(ctx context.Context, studentID int64) error {
	query := base.GetDB(ctx, r.db).NewUpdate().
		Model((*users.Student)(nil)).
		ModelTableExpr(`users.students AS "student"`).
		Set("group_id = NULL").
		Where(`"student".id = ?`, studentID)

	if where, val, ok := base.TenantWhere(ctx, "student"); ok {
		query = query.Where(where, val)
	}

	result, err := query.Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "remove from group",
			Err: err,
		}
	}

	return base.AssertRowsAffected(result, 1, "remove from group")
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

	currentDeparture, err := r.findCurrentDepartureState(ctx, student.ID)
	if err != nil {
		return err
	}

	// Align the in-memory departure plan to the plan that will actually be
	// persisted BEFORE validating, so Validate() checks the effective plan rather
	// than a transient mix of a stale hydrated allowed_departure_modes set and a
	// freshly-set legacy departure_days. Without this a legacy client that removes
	// the accompanied mode via departure_days while clearing the "mit wem" note is
	// rejected against the stale accompanied mode it never sent (#1694).
	r.alignDeparturePlanForValidation(student, currentDeparture)

	// Validate student
	if err := student.Validate(); err != nil {
		return err
	}

	if err := r.Repository.Update(ctx, student); err != nil {
		return err
	}
	return r.persistDepartureDays(ctx, student, currentDeparture)
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
	planTouched := student.AllowedDepartureModes != nil ||
		student.DepartureDays != nil ||
		student.BusDays != nil ||
		student.PickupDays != nil ||
		student.PickupStatus != nil
	if !planTouched {
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
	planTouched := student.AllowedDepartureModes != nil ||
		student.DepartureDays != nil ||
		student.BusDays != nil ||
		student.PickupDays != nil ||
		student.PickupStatus != nil

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

	if where, val, ok := base.TenantWhere(ctx, "student"); ok {
		query = query.Where(where, val)
	}

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

	if where, val, ok := base.TenantWhere(ctx, "student"); ok {
		query = query.Where(where, val)
	}

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

	if where, val, ok := base.TenantWhere(ctx, "student"); ok {
		query = query.Where(where, val)
	}

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

// FindWithPerson retrieves a student with their associated person data
func (r *StudentRepository) FindWithPerson(ctx context.Context, id int64) (*users.Student, error) {
	student := new(users.Student)
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(student).
		ModelTableExpr(`users.students AS "student"`).
		Relation("Person").
		Where(`"student".id = ?`, id)

	if where, val, ok := base.TenantWhere(ctx, "student"); ok {
		query = query.Where(where, val)
	}

	err := query.Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find with person",
			Err: err,
		}
	}

	return student, nil
}

// FindByGuardianEmail finds students with a specific guardian email
func (r *StudentRepository) FindByGuardianEmail(ctx context.Context, email string) ([]*users.Student, error) {
	var students []*users.Student
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&students).
		ModelTableExpr(`users.students AS "student"`).
		Where(`LOWER("student".guardian_email) = LOWER(?)`, email)

	if where, val, ok := base.TenantWhere(ctx, "student"); ok {
		query = query.Where(where, val)
	}

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

	if where, val, ok := base.TenantWhere(ctx, "student"); ok {
		query = query.Where(where, val)
	}

	err := query.Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by guardian phone",
			Err: err,
		}
	}

	return students, nil
}

// FindByTeacherID retrieves students supervised by a teacher (through group assignments)
func (r *StudentRepository) FindByTeacherID(ctx context.Context, teacherID int64) ([]*users.Student, error) {
	// Define a result struct to handle the complex JOIN and mapping
	type studentWithPersonAndGroup struct {
		Student   *users.Student `bun:"student"`
		Person    *users.Person  `bun:"person"`
		GroupName string         `bun:"group_name"`
	}

	var results []*studentWithPersonAndGroup
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&results).
		ModelTableExpr(`users.students AS "student"`).
		// Student columns with proper aliasing
		ColumnExpr(`"student".id AS "student__id", "student".created_at AS "student__created_at", "student".updated_at AS "student__updated_at"`).
		ColumnExpr(`"student".tenant_id AS "student__tenant_id"`).
		ColumnExpr(`"student".person_id AS "student__person_id", "student".school_class AS "student__school_class"`).
		ColumnExpr(`"student".guardian_name AS "student__guardian_name", "student".guardian_contact AS "student__guardian_contact"`).
		ColumnExpr(`"student".guardian_email AS "student__guardian_email", "student".guardian_phone AS "student__guardian_phone"`).
		ColumnExpr(`"student".group_id AS "student__group_id"`).
		ColumnExpr(`"student".extra_info AS "student__extra_info", "student".supervisor_notes AS "student__supervisor_notes"`).
		ColumnExpr(`"student".health_info AS "student__health_info", "student".pickup_status AS "student__pickup_status"`).
		// departure_companion_note is scanonly and hydrated below via
		// hydrateBusDaysForStudents — never selected here, so the query stays
		// valid on schemas where the column is absent (#1694).
		// Person columns with proper aliasing
		ColumnExpr(`"person".id AS "person__id", "person".created_at AS "person__created_at", "person".updated_at AS "person__updated_at"`).
		ColumnExpr(`"person".first_name AS "person__first_name", "person".last_name AS "person__last_name"`).
		ColumnExpr(`"person".tag_id AS "person__tag_id", "person".account_id AS "person__account_id"`).
		// Group name for reference
		ColumnExpr(`"group".name AS "group_name"`).
		// JOINs to traverse the relationship chain
		Join(`INNER JOIN users.persons AS "person" ON "person".id = "student".person_id`).
		Join(`INNER JOIN education.groups AS "group" ON "group".id = "student".group_id`).
		Join(`INNER JOIN education.group_teacher AS "gt" ON "gt".group_id = "group".id`).
		// Filter by teacher ID and ensure student has a group assignment
		Where(`"gt".teacher_id = ? AND "student".group_id IS NOT NULL`, teacherID).
		// Use DISTINCT to avoid duplicates if a teacher supervises multiple groups with same student
		Distinct()

	if where, val, ok := base.TenantWhere(ctx, "student"); ok {
		query = query.Where(where, val)
	}

	err := query.Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by teacher ID",
			Err: err,
		}
	}

	// Extract students from results and map the person relationship
	students := make([]*users.Student, len(results))
	for i, result := range results {
		student := result.Student
		if result.Person != nil && result.Person.ID != 0 {
			student.Person = result.Person
		}
		students[i] = student
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

	if where, val, ok := base.TenantWhere(ctx, "student"); ok {
		query = query.Where(where, val)
	}

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
			continue
		}
		student.BusDays = row.BusDays.Normalize()
		student.PickupDays = row.PickupDays.Normalize()
		student.DepartureDays = users.DepartureDaysFromLegacy(student.BusDays, student.PickupDays)
		student.AllowedDepartureModes = users.AllowedDepartureModesFromLegacy(student.BusDays, student.PickupDays)
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
	student := new(users.Student)
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(student).
		ModelTableExpr(tableExprUsersStudentsAsStudent).
		Where(`"student".id = ?`, id).
		For("UPDATE")

	if where, val, ok := base.TenantWhere(ctx, "student"); ok {
		query = query.Where(where, val)
	}

	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{Op: "find_by_id_for_update", Err: err}
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

	if where, val, ok := base.TenantWhere(ctx, "student"); ok {
		query = query.Where(where, val)
	}

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

	if where, val, ok := base.TenantWhere(ctx, "student"); ok {
		query = query.Where(where, val)
	}

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

	if where, val, ok := base.TenantWhere(ctx, "student"); ok {
		query = query.Where(where, val)
	}

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

	if where, val, ok := base.TenantWhere(ctx, "student"); ok {
		query = query.Where(where, val)
	}

	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find active students due for deactivation",
			Err: err,
		}
	}

	return students, nil
}
