package education

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/education"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// Table and query constants
const (
	tableGradeTransitions        = "education.grade_transitions"
	tableGradeTransitionMappings = "education.grade_transition_mappings"
	tableGradeTransitionHistory  = "education.grade_transition_history"
	orderByCreatedAtDesc         = "created_at DESC"
	whereTransitionID            = "transition_id = ?"
)

// GradeTransitionRepository implements education.GradeTransitionRepository interface
type GradeTransitionRepository struct {
	*base.Repository[*education.GradeTransition]
	db       *bun.DB
	students StudentDirectory
}

// BindStudentDirectory installs the People Directory every student read
// and write of the transition goes through (#2662).
func (r *GradeTransitionRepository) BindStudentDirectory(students StudentDirectory) {
	r.students = students
}

// NewGradeTransitionRepository creates a new GradeTransitionRepository
func NewGradeTransitionRepository(db *bun.DB) education.GradeTransitionRepository {
	repo := base.NewRepository[*education.GradeTransition](db, tableGradeTransitions, "GradeTransition")
	repo.TenantScoped = true
	return &GradeTransitionRepository{
		Repository: repo,
		db:         db,
	}
}

// Create creates a new grade transition
func (r *GradeTransitionRepository) Create(ctx context.Context, t *education.GradeTransition) error {
	if t == nil {
		return fmt.Errorf("grade transition cannot be nil")
	}

	if err := t.Validate(); err != nil {
		return err
	}

	base.EnsureTenantID(ctx, t)

	_, err := base.GetDB(ctx, r.db).NewInsert().
		Model(t).
		ModelTableExpr(tableGradeTransitions).
		Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "create grade transition",
			Err: base.TranslateNotFound(err),
		}
	}

	return nil
}

// FindByID retrieves a grade transition by ID
func (r *GradeTransitionRepository) FindByID(ctx context.Context, id int64) (*education.GradeTransition, error) {
	t := new(education.GradeTransition)
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(t).
		ModelTableExpr(tableGradeTransitions+` AS "grade_transition"`).
		Where(`"grade_transition".id = ?`, id)

	query = base.WithTenantFilter(ctx, query, "grade_transition")

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find grade transition by id",
			Err: base.TranslateNotFound(err),
		}
	}

	return t, nil
}

// FindByIDWithMappings retrieves a grade transition with its mappings
func (r *GradeTransitionRepository) FindByIDWithMappings(ctx context.Context, id int64) (*education.GradeTransition, error) {
	t := new(education.GradeTransition)
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(t).
		ModelTableExpr(tableGradeTransitions+` AS "grade_transition"`).
		Where(`"grade_transition".id = ?`, id)

	query = base.WithTenantFilter(ctx, query, "grade_transition")

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find grade transition by id",
			Err: base.TranslateNotFound(err),
		}
	}

	// Load mappings separately
	mappings, err := r.GetMappings(ctx, id)
	if err != nil {
		return nil, err
	}
	t.Mappings = mappings

	return t, nil
}

// Update updates a grade transition
func (r *GradeTransitionRepository) Update(ctx context.Context, t *education.GradeTransition) error {
	if t == nil {
		return fmt.Errorf("grade transition cannot be nil")
	}

	if err := t.Validate(); err != nil {
		return err
	}

	query := base.GetDB(ctx, r.db).NewUpdate().
		Model(t).
		ModelTableExpr(tableGradeTransitions + ` AS "grade_transition"`).
		WherePK()

	query = base.WithTenantFilter(ctx, query, "grade_transition")

	result, err := query.Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "update grade transition",
			Err: base.TranslateNotFound(err),
		}
	}

	return base.AssertRowsAffected(result, 1, "update grade_transition")
}

// Delete deletes a grade transition
func (r *GradeTransitionRepository) Delete(ctx context.Context, id int64) error {
	query := base.GetDB(ctx, r.db).NewDelete().
		Model((*education.GradeTransition)(nil)).
		ModelTableExpr(tableGradeTransitions+` AS "grade_transition"`).
		Where(`"grade_transition".id = ?`, id)

	query = base.WithTenantFilter(ctx, query, "grade_transition")

	_, err := query.Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "delete grade transition",
			Err: base.TranslateNotFound(err),
		}
	}

	return nil
}

// List retrieves grade transitions with pagination
func (r *GradeTransitionRepository) List(ctx context.Context, options *modelBase.QueryOptions) ([]*education.GradeTransition, int, error) {
	count, err := r.CountWithOptions(ctx, options)
	if err != nil {
		return nil, 0, &modelBase.DatabaseError{Op: "count grade transitions", Err: base.DatabaseErrorCause(err)}
	}

	listOptions := &modelBase.QueryOptions{}
	if options != nil {
		*listOptions = *options
	}
	fields := make([]modelBase.SortField, 0, 1)
	if options != nil && options.Sorting != nil {
		fields = append(fields, options.Sorting.Fields...)
	}
	fields = append(fields, modelBase.SortField{Field: "created_at", Direction: modelBase.SortDesc})
	listOptions.Sorting = &modelBase.Sorting{Fields: fields}

	transitions, err := r.ListWithOptions(ctx, listOptions)
	if err != nil {
		return nil, 0, &modelBase.DatabaseError{Op: "list grade transitions", Err: base.DatabaseErrorCause(err)}
	}
	if len(transitions) == 0 {
		return nil, count, nil
	}

	return transitions, count, nil
}

// FindByAcademicYear retrieves grade transitions for a specific academic year
func (r *GradeTransitionRepository) FindByAcademicYear(ctx context.Context, year string) ([]*education.GradeTransition, error) {
	var transitions []*education.GradeTransition
	query := base.GetDB(ctx, r.db).NewSelect().
		TableExpr(tableGradeTransitions+` AS "grade_transition"`).
		ColumnExpr(`"grade_transition".*`).
		Where(`"grade_transition".academic_year = ?`, year).
		Order(orderByCreatedAtDesc)

	query = base.WithTenantFilter(ctx, query, "grade_transition")

	err := query.Scan(ctx, &transitions)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find grade transitions by academic year",
			Err: base.TranslateNotFound(err),
		}
	}

	return transitions, nil
}

// LockLatestApplied returns the most recently applied transition with a
// FOR UPDATE row lock, or (nil, nil) when no transition is currently applied.
// Ordering matches the frontend's latest-revertable gate: applied_at DESC
// (NULLS LAST for legacy rows without a timestamp), id DESC as a stable
// tiebreaker. The lock is held for the caller's transaction so two concurrent
// reverts of the same row serialize instead of both replaying history.
func (r *GradeTransitionRepository) LockLatestApplied(ctx context.Context) (*education.GradeTransition, error) {
	t := new(education.GradeTransition)
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(t).
		ModelTableExpr(tableGradeTransitions+` AS "grade_transition"`).
		Where(`"grade_transition".status = ?`, education.TransitionStatusApplied).
		OrderExpr(`"grade_transition".applied_at DESC NULLS LAST, "grade_transition".id DESC`).
		Limit(1).
		For("UPDATE")

	query = base.WithTenantFilter(ctx, query, "grade_transition")

	err := query.Scan(ctx)
	if err != nil {
		err = base.TranslateNotFound(err)
		if modelBase.IsNoRows(err) {
			return nil, nil
		}
		return nil, &modelBase.DatabaseError{
			Op:  "lock latest applied grade transition",
			Err: err,
		}
	}

	return t, nil
}

// LockTenantTransitions takes a tenant-wide EXCLUSIVE transaction-scoped
// advisory lock. Both Apply and Revert acquire it first, so the two operations
// serialize against one another instead of interleaving (an apply snapshotting
// classes a concurrent revert then changes underneath it). The key is scoped by
// tenant so different schools never block each other; the lock releases
// automatically at COMMIT/ROLLBACK of the caller's transaction (#405 review).
//
// The timetable materializer takes the SAME key — see
// education.TenantTransitionsLockKey for why a graduation and a materialization
// pass must not interleave.
func (r *GradeTransitionRepository) LockTenantTransitions(ctx context.Context) error {
	key := education.TenantTransitionsLockKey(tenant.FromContext(ctx))
	if err := base.AcquireXactLock(ctx, r.db, key); err != nil {
		return &modelBase.DatabaseError{
			Op:  "lock tenant grade transitions",
			Err: base.TranslateNotFound(err),
		}
	}
	return nil
}

// LockTenantRecurrenceWrites takes the tenant-wide recurrence gate that guards
// re-planning and materialization, using the SAME key services/schedule uses.
// Apply and Revert take it before LockTenantTransitions because they mutate
// instance_students — recurrence-derived state a concurrent re-plan may already
// hold row locks on while it waits for the transition gate. Acquiring the two
// gates in the opposite order is a textbook deadlock (#405 review).
func (r *GradeTransitionRepository) LockTenantRecurrenceWrites(ctx context.Context) error {
	key := scheduleModels.TenantRecurrenceLockKey(tenant.FromContext(ctx))
	if err := base.AcquireXactLock(ctx, r.db, key); err != nil {
		return &modelBase.DatabaseError{
			Op:  "lock tenant recurrence writes",
			Err: base.TranslateNotFound(err),
		}
	}
	return nil
}

// FindByStatus retrieves grade transitions with a specific status
func (r *GradeTransitionRepository) FindByStatus(ctx context.Context, status string) ([]*education.GradeTransition, error) {
	var transitions []*education.GradeTransition
	query := base.GetDB(ctx, r.db).NewSelect().
		TableExpr(tableGradeTransitions+` AS "grade_transition"`).
		ColumnExpr(`"grade_transition".*`).
		Where(`"grade_transition".status = ?`, status).
		Order(orderByCreatedAtDesc)

	query = base.WithTenantFilter(ctx, query, "grade_transition")

	err := query.Scan(ctx, &transitions)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find grade transitions by status",
			Err: base.TranslateNotFound(err),
		}
	}

	return transitions, nil
}

// CreateMapping creates a new mapping
func (r *GradeTransitionRepository) CreateMapping(ctx context.Context, m *education.GradeTransitionMapping) error {
	if m == nil {
		return fmt.Errorf("mapping cannot be nil")
	}

	if err := m.Validate(); err != nil {
		return err
	}

	base.EnsureTenantID(ctx, m)

	_, err := base.GetDB(ctx, r.db).NewInsert().
		Model(m).
		ModelTableExpr(tableGradeTransitionMappings).
		Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "create grade transition mapping",
			Err: base.TranslateNotFound(err),
		}
	}

	return nil
}

// CreateMappings creates multiple mappings in a batch
func (r *GradeTransitionRepository) CreateMappings(ctx context.Context, mappings []*education.GradeTransitionMapping) error {
	if len(mappings) == 0 {
		return nil
	}

	// Validate all mappings
	for _, m := range mappings {
		if err := m.Validate(); err != nil {
			return err
		}
		base.EnsureTenantID(ctx, m)
	}

	_, err := base.GetDB(ctx, r.db).NewInsert().
		Model(&mappings).
		ModelTableExpr(tableGradeTransitionMappings).
		Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "create grade transition mappings batch",
			Err: base.TranslateNotFound(err),
		}
	}

	return nil
}

// DeleteMappings deletes all mappings for a transition
func (r *GradeTransitionRepository) DeleteMappings(ctx context.Context, transitionID int64) error {
	delQuery := base.GetDB(ctx, r.db).NewDelete().
		TableExpr(tableGradeTransitionMappings+` AS "grade_transition_mapping"`).
		Where(`"grade_transition_mapping".`+whereTransitionID, transitionID)

	delQuery = base.WithTenantFilter(ctx, delQuery, "grade_transition_mapping")

	_, err := delQuery.Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "delete grade transition mappings",
			Err: base.TranslateNotFound(err),
		}
	}

	return nil
}

// GetMappings retrieves all mappings for a transition
func (r *GradeTransitionRepository) GetMappings(ctx context.Context, transitionID int64) ([]*education.GradeTransitionMapping, error) {
	var mappings []*education.GradeTransitionMapping
	query := base.GetDB(ctx, r.db).NewSelect().
		TableExpr(tableGradeTransitionMappings+` AS "grade_transition_mapping"`).
		ColumnExpr(`"grade_transition_mapping".*`).
		Where(`"grade_transition_mapping".`+whereTransitionID, transitionID).
		Order("from_class ASC")

	query = base.WithTenantFilter(ctx, query, "grade_transition_mapping")

	err := query.Scan(ctx, &mappings)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "get grade transition mappings",
			Err: base.TranslateNotFound(err),
		}
	}

	return mappings, nil
}

// GetMappingsByTransitionIDs retrieves all mappings for several transitions in
// a single query, grouped by transition_id. Used to hydrate the list response
// so every draft shows its mappings (and a correct can_apply) without an N+1.
func (r *GradeTransitionRepository) GetMappingsByTransitionIDs(ctx context.Context, transitionIDs []int64) (map[int64][]*education.GradeTransitionMapping, error) {
	result := make(map[int64][]*education.GradeTransitionMapping)
	if len(transitionIDs) == 0 {
		return result, nil
	}

	var mappings []*education.GradeTransitionMapping
	query := base.GetDB(ctx, r.db).NewSelect().
		TableExpr(tableGradeTransitionMappings+` AS "grade_transition_mapping"`).
		ColumnExpr(`"grade_transition_mapping".*`).
		Where(`"grade_transition_mapping".transition_id IN (?)`, bun.List(transitionIDs)).
		Order("transition_id ASC", "from_class ASC")

	query = base.WithTenantFilter(ctx, query, "grade_transition_mapping")

	if err := query.Scan(ctx, &mappings); err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "get grade transition mappings by transition ids",
			Err: base.TranslateNotFound(err),
		}
	}

	for _, m := range mappings {
		result[m.TransitionID] = append(result[m.TransitionID], m)
	}
	return result, nil
}

// CreateHistory creates a new history record
func (r *GradeTransitionRepository) CreateHistory(ctx context.Context, h *education.GradeTransitionHistory) error {
	if h == nil {
		return fmt.Errorf("history cannot be nil")
	}

	if err := h.Validate(); err != nil {
		return err
	}

	base.EnsureTenantID(ctx, h)

	_, err := base.GetDB(ctx, r.db).NewInsert().
		Model(h).
		ModelTableExpr(tableGradeTransitionHistory).
		Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "create grade transition history",
			Err: base.TranslateNotFound(err),
		}
	}

	return nil
}

// CreateHistoryBatch creates multiple history records in a batch
func (r *GradeTransitionRepository) CreateHistoryBatch(ctx context.Context, history []*education.GradeTransitionHistory) error {
	if len(history) == 0 {
		return nil
	}

	// Validate all history records
	for _, h := range history {
		if err := h.Validate(); err != nil {
			return err
		}
		base.EnsureTenantID(ctx, h)
	}

	_, err := base.GetDB(ctx, r.db).NewInsert().
		Model(&history).
		ModelTableExpr(tableGradeTransitionHistory).
		Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "create grade transition history batch",
			Err: base.TranslateNotFound(err),
		}
	}

	return nil
}

// GetHistory retrieves all history records for a transition
func (r *GradeTransitionRepository) GetHistory(ctx context.Context, transitionID int64) ([]*education.GradeTransitionHistory, error) {
	var history []*education.GradeTransitionHistory
	query := base.GetDB(ctx, r.db).NewSelect().
		TableExpr(tableGradeTransitionHistory+` AS "grade_transition_history"`).
		ColumnExpr(`"grade_transition_history".*`).
		Where(`"grade_transition_history".`+whereTransitionID, transitionID).
		Order("created_at ASC")

	query = base.WithTenantFilter(ctx, query, "grade_transition_history")

	err := query.Scan(ctx, &history)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "get grade transition history",
			Err: base.TranslateNotFound(err),
		}
	}

	return history, nil
}

// CreateClassTeacherHistoryBatch records what an apply did to
// education.class_teachers (#1772). One transition applies at most once
// (a reverted transition can never be re-applied), so entries are never
// cleared or rewritten.
func (r *GradeTransitionRepository) CreateClassTeacherHistoryBatch(ctx context.Context, history []*education.GradeTransitionClassTeacher) error {
	if len(history) == 0 {
		return nil
	}

	for _, h := range history {
		if err := h.Validate(); err != nil {
			return err
		}
		base.EnsureTenantID(ctx, h)
	}

	_, err := base.GetDB(ctx, r.db).NewInsert().
		Model(&history).
		ModelTableExpr(`education.grade_transition_class_teachers`).
		Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "create class teacher history batch",
			Err: base.TranslateNotFound(err),
		}
	}
	return nil
}

// GetClassTeacherHistory retrieves the class-teacher ledger of a transition.
func (r *GradeTransitionRepository) GetClassTeacherHistory(ctx context.Context, transitionID int64) ([]*education.GradeTransitionClassTeacher, error) {
	var history []*education.GradeTransitionClassTeacher
	query := base.GetDB(ctx, r.db).NewSelect().
		TableExpr(`education.grade_transition_class_teachers AS "gtct"`).
		ColumnExpr(`"gtct".*`).
		Where(`"gtct".`+whereTransitionID, transitionID).
		Order("created_at ASC")

	query = base.WithTenantFilter(ctx, query, "gtct")

	if err := query.Scan(ctx, &history); err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "get class teacher history",
			Err: base.TranslateNotFound(err),
		}
	}

	return history, nil
}

// CreateClassListEntryHistoryBatch records what an apply did to
// users.class_list_entries (#2382). One transition applies at most once
// (a reverted transition can never be re-applied), so entries are never
// cleared or rewritten.
func (r *GradeTransitionRepository) CreateClassListEntryHistoryBatch(ctx context.Context, history []*education.GradeTransitionClassListEntry) error {
	if len(history) == 0 {
		return nil
	}

	for _, h := range history {
		if err := h.Validate(); err != nil {
			return err
		}
		base.EnsureTenantID(ctx, h)
	}

	_, err := base.GetDB(ctx, r.db).NewInsert().
		Model(&history).
		ModelTableExpr(`education.grade_transition_class_list_entries`).
		Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "create class list entry history batch",
			Err: base.TranslateNotFound(err),
		}
	}
	return nil
}

// GetClassListEntryHistory retrieves the class-list-entry ledger of a
// transition.
func (r *GradeTransitionRepository) GetClassListEntryHistory(ctx context.Context, transitionID int64) ([]*education.GradeTransitionClassListEntry, error) {
	var history []*education.GradeTransitionClassListEntry
	query := base.GetDB(ctx, r.db).NewSelect().
		TableExpr(`education.grade_transition_class_list_entries AS "gtcle"`).
		ColumnExpr(`"gtcle".*`).
		Where(`"gtcle".`+whereTransitionID, transitionID).
		Order("created_at ASC")

	query = base.WithTenantFilter(ctx, query, "gtcle")

	if err := query.Scan(ctx, &history); err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "get class list entry history",
			Err: base.TranslateNotFound(err),
		}
	}

	return history, nil
}

// GetDistinctClasses retrieves all distinct school_class values of the
// tenant's non-alumni students through the People Directory (#2662).
func (r *GradeTransitionRepository) GetDistinctClasses(ctx context.Context) ([]string, error) {
	if r.students == nil {
		return nil, errStudentDirectoryRequired
	}
	return r.students.ListSchoolClasses(ctx)
}

// GetStudentCountByClass returns the number of non-alumni students in a
// class, read through the People Directory (#2662).
func (r *GradeTransitionRepository) GetStudentCountByClass(ctx context.Context, className string) (int, error) {
	if r.students == nil {
		return 0, errStudentDirectoryRequired
	}
	students, err := r.students.ListStudentsByClasses(ctx, []string{className})
	if err != nil {
		return 0, err
	}
	return len(students), nil
}

// GetStudentsByClasses retrieves the non-alumni students in the given
// classes through the People Directory (#2662), in class/id order. The
// person names and the name order are attached by the composition layer
// (#2661).
func (r *GradeTransitionRepository) GetStudentsByClasses(ctx context.Context, classes []string) ([]*education.StudentClassInfo, error) {
	if len(classes) == 0 {
		return []*education.StudentClassInfo{}, nil
	}
	if r.students == nil {
		return nil, errStudentDirectoryRequired
	}
	rows, err := r.students.ListStudentsByClasses(ctx, classes)
	if err != nil {
		return nil, err
	}
	students := make([]*education.StudentClassInfo, 0, len(rows))
	for _, row := range rows {
		students = append(students, &education.StudentClassInfo{
			StudentID:   row.ID,
			PersonID:    row.PersonID,
			SchoolClass: row.SchoolClass,
			Status:      row.Status,
		})
	}
	return students, nil
}

// PromoteStudentsByIDs moves exactly the given students from fromClass to
// toClass. Apply passes the same FOR UPDATE-locked IDs it recorded in history,
// so the promoted and recorded sets are identical: a student created or moved
// into a promoted class after the cohort was locked is not swept along without a
// history row (which a revert would then be unable to undo), and no history row
// describes a promotion that never happened (#405 review).
//
// The from-class equality guard and the alumnus exclusion are defensive — the
// caller holds a row lock on every id — and keep the statement idempotent.
func (r *GradeTransitionRepository) PromoteStudentsByIDs(
	ctx context.Context, studentIDs []int64, fromClass, toClass string,
) (int64, error) {
	if len(studentIDs) == 0 {
		return 0, nil
	}
	if r.students == nil {
		return 0, errStudentDirectoryRequired
	}
	return r.students.PromoteStudents(ctx, studentIDs, fromClass, toClass)
}

// UpdateStudentClasses updates student classes based on transition mappings
// This is a join-based UPDATE for efficiency.
//
// NOT used by Apply anymore: re-evaluating class membership at write time races
// the history snapshot in both directions, so promotions go through
// PromoteStudentsByIDs on the locked cohort instead (#405 review). Kept for the
// same reason as GraduateStudentsByClasses — a bulk, mapping-driven fallback.
func (r *GradeTransitionRepository) UpdateStudentClasses(ctx context.Context, transitionID int64) (int64, error) {
	// Build tenant-aware bulk UPDATE using JOIN on mappings
	rawSQL := `
		UPDATE users.students s
		SET school_class = m.to_class,
		    updated_at = NOW()
		FROM education.grade_transition_mappings m
		WHERE m.transition_id = ?
		  AND m.to_class IS NOT NULL
		  AND s.school_class = m.from_class
		  AND s.status <> ?`

	args := []interface{}{transitionID, string(users.StudentStatusAlumnus)}

	if tenantID := tenant.FromContext(ctx); tenantID > 0 {
		rawSQL += `
		  AND s.tenant_id = ?`
		args = append(args, tenantID)
	}

	result, err := base.GetDB(ctx, r.db).ExecContext(ctx, rawSQL, args...)

	if err != nil {
		return 0, &modelBase.DatabaseError{
			Op:  "update student classes",
			Err: base.TranslateNotFound(err),
		}
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return 0, &modelBase.DatabaseError{
			Op:  "get rows affected",
			Err: base.TranslateNotFound(err),
		}
	}

	return affected, nil
}

// RevertStudentClass moves a single promoted student back to fromClass, guarded
// on the student still being in toClass (the class this transition assigned).
// The equality guard is what preserves a post-transition correction: if an
// admin moved the child to a different class — or a later transition promoted
// them again — school_class no longer equals toClass, the WHERE matches nothing,
// and 0 rows are affected so the older revert cannot clobber the newer value.
func (r *GradeTransitionRepository) RevertStudentClass(ctx context.Context, studentID int64, fromClass, toClass string) (int64, error) {
	if r.students == nil {
		return 0, errStudentDirectoryRequired
	}
	return r.students.RevertStudentClass(ctx, studentID, fromClass, toClass)
}

// GraduateStudentsByClasses soft-deletes graduating students: their rows are
// kept but status flips to "alumnus", which removes them from every
// staff-facing read path (see the alumnus filters in the student repository).
// A transition revert restores them via ReactivateStudentsByIDs.
func (r *GradeTransitionRepository) GraduateStudentsByClasses(ctx context.Context, classes []string) (int64, error) {
	if len(classes) == 0 {
		return 0, nil
	}
	if r.students == nil {
		return 0, errStudentDirectoryRequired
	}
	return r.students.GraduateStudentsByClasses(ctx, classes)
}

// GraduateStudentsByIDs soft-deletes exactly the given students (status flips to
// alumnus). Only rows still non-alumnus are touched, so a re-run is idempotent.
// Apply passes the same FOR UPDATE-locked IDs it checked for open check-ins and
// wrote to history, so the checked / recorded / mutated sets are identical and a
// concurrently inserted student cannot be graduated without those guards (#405).
func (r *GradeTransitionRepository) GraduateStudentsByIDs(ctx context.Context, studentIDs []int64) (int64, error) {
	if len(studentIDs) == 0 {
		return 0, nil
	}
	if r.students == nil {
		return 0, errStudentDirectoryRequired
	}
	return r.students.GraduateStudents(ctx, studentIDs)
}

// ReactivateStudentsByIDs restores graduated (alumnus) students back to
// active. Convenience wrapper over ReactivateStudentsToStatus; only rows still
// in alumnus status are touched so a manually changed status is never
// clobbered.
func (r *GradeTransitionRepository) ReactivateStudentsByIDs(ctx context.Context, studentIDs []int64) (int64, error) {
	reactivated, err := r.ReactivateStudentsToStatus(ctx, studentIDs, string(users.StudentStatusActive))
	if err != nil {
		return 0, err
	}
	return int64(len(reactivated)), nil
}

// ReactivateStudentsToStatus restores graduated (alumnus) students to a
// specific lifecycle status — the one they held before the transition
// graduated them (see grade_transition_history.from_status). Only rows still in
// alumnus status are touched, so a status changed manually after graduation is
// never clobbered.
//
// It returns the ids it ACTUALLY restored, not just how many. A skipped child
// (deleted, or manually moved off alumnus since the graduation) is deliberately
// left alone, and the revert's roster reconciliation must skip them too — the
// caller cannot tell which ids those were from a count (#405 review).
func (r *GradeTransitionRepository) ReactivateStudentsToStatus(ctx context.Context, studentIDs []int64, targetStatus string) ([]int64, error) {
	if len(studentIDs) == 0 {
		return nil, nil
	}
	if r.students == nil {
		return nil, errStudentDirectoryRequired
	}
	return r.students.ReactivateStudents(ctx, studentIDs, targetStatus)
}

// PersonIDsByStudentIDs maps the given students to their person ids. The
// caller holds the student rows locked already (apply and revert lock the
// cohort first); the person rows are locked by the People Directory when
// it releases or restores the tags.
func (r *GradeTransitionRepository) PersonIDsByStudentIDs(ctx context.Context, studentIDs []int64) (map[int64]int64, error) {
	if len(studentIDs) == 0 {
		return map[int64]int64{}, nil
	}
	if r.students == nil {
		return nil, errStudentDirectoryRequired
	}
	rows, err := r.students.ListStudentsByID(ctx, studentIDs)
	if err != nil {
		return nil, err
	}
	result := make(map[int64]int64, len(rows))
	for _, row := range rows {
		result[row.ID] = row.PersonID
	}
	return result, nil
}

// ReleaseStudentTagsByIDs is served by the People Directory composition
// (#2661): the person rows belong to that owner. The repository only
// supplies PersonIDsByStudentIDs.
func (r *GradeTransitionRepository) ReleaseStudentTagsByIDs(context.Context, []int64) (map[int64]string, error) {
	return nil, fmt.Errorf("release student tags through the people directory composition")
}

// RestoreStudentTag is served by the People Directory composition (#2661);
// see ReleaseStudentTagsByIDs.
func (r *GradeTransitionRepository) RestoreStudentTag(context.Context, int64, string) (bool, error) {
	return false, fmt.Errorf("restore student tag through the people directory composition")
}

// PurgedStudentPlaceholder replaces a graduate's name in the ledger once the
// child has been hard-deleted. The row itself stays: it is what makes the
// applied transition's own count ("11 Abgänge") still add up afterwards, and
// the revert reads it to know which children it must NOT try to restore.
const PurgedStudentPlaceholder = "Gelöschtes Kind"

// FindStudentStatesByIDs maps each given student id to its current lifecycle
// status. Ids missing from the result no longer have a row at all — they were
// hard-deleted after graduation.
//
// Deliberately WITHOUT the alumnus filter every other student read carries:
// this is the one query whose entire purpose is to see graduates. It stays in
// the grade-transition repository rather than users.StudentRepository so the
// exception cannot be reached from an ordinary staff list by accident.
func (r *GradeTransitionRepository) FindStudentStatesByIDs(ctx context.Context, studentIDs []int64) (map[int64]string, error) {
	if len(studentIDs) == 0 {
		return map[int64]string{}, nil
	}
	if r.students == nil {
		return nil, errStudentDirectoryRequired
	}
	rows, err := r.students.ListStudentsByID(ctx, studentIDs)
	if err != nil {
		return nil, err
	}
	states := make(map[int64]string, len(rows))
	for _, row := range rows {
		states[row.ID] = row.Status
	}
	return states, nil
}

// AnonymizeHistoryForStudent replaces the stored name and clears the stored
// RFID identifier on every ledger row of a student that has just been
// hard-deleted.
//
// Without it the "endgültig löschen" the UI promises would be a half-truth:
// grade_transition_history.person_name is a denormalized copy of the child's
// name that carries no foreign key, so it survives the delete of both the
// student and the person row and would keep identifying the child indefinitely.
func (r *GradeTransitionRepository) AnonymizeHistoryForStudent(ctx context.Context, studentID int64) error {
	updQuery := base.GetDB(ctx, r.db).NewUpdate().
		Model((*struct{})(nil)).
		ModelTableExpr(`education.grade_transition_history AS "history"`).
		Set("person_name = ?", PurgedStudentPlaceholder).
		Set("rfid_tag = NULL").
		Set("updated_at = NOW()").
		Where(`"history".student_id = ?`, studentID).
		Where(`("history".person_name <> ? OR "history".rfid_tag IS NOT NULL)`, PurgedStudentPlaceholder)

	updQuery = base.WithTenantFilter(ctx, updQuery, "history")

	if _, err := updQuery.Exec(ctx); err != nil {
		return &modelBase.DatabaseError{
			Op:  "anonymize grade transition history for student",
			Err: base.TranslateNotFound(err),
		}
	}
	return nil
}
