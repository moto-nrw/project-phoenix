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

	// UNIQUE (tenant_id, tag_id) on users.persons, created by migration
	// 1.14.3. RestoreStudentTag matches 23505 against this name to tell the
	// "somebody else holds the tag" race apart from a real failure.
	personsTenantTagUniqueIndex = "idx_persons_tenant_tag"
	// Savepoint guarding that single UPDATE. Reverts restore tags one student
	// at a time and each savepoint is released before the next is taken, so
	// reusing one name is safe.
	restoreStudentTagSavepoint = "restore_student_tag"
)

// GradeTransitionRepository implements education.GradeTransitionRepository interface
type GradeTransitionRepository struct {
	*base.Repository[*education.GradeTransition]
	db *bun.DB
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
			Err: err,
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
			Err: err,
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
			Err: err,
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
			Err: err,
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
			Err: err,
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
			Err: err,
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
			Err: err,
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
			Err: err,
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
			Err: err,
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
			Err: err,
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
			Err: err,
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
			Err: err,
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
			Err: err,
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
			Err: err,
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
			Err: err,
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
			Err: err,
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
			Err: err,
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
			Err: err,
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
			Err: err,
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
			Err: err,
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
			Err: err,
		}
	}

	return history, nil
}

// GetDistinctClasses retrieves all distinct school_class values from students
func (r *GradeTransitionRepository) GetDistinctClasses(ctx context.Context) ([]string, error) {
	var classes []string
	query := base.GetDB(ctx, r.db).NewSelect().
		TableExpr(`users.students AS student`).
		ColumnExpr(`DISTINCT student.school_class`).
		Where(`student.school_class IS NOT NULL AND student.school_class != ''`).
		Where(`student.status <> ?`, string(users.StudentStatusAlumnus)).
		Order(`student.school_class ASC`)

	query = base.WithTenantFilter(ctx, query, "student")

	err := query.Scan(ctx, &classes)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "get distinct classes",
			Err: err,
		}
	}

	return classes, nil
}

// GetStudentCountByClass returns the number of students in a class
func (r *GradeTransitionRepository) GetStudentCountByClass(ctx context.Context, className string) (int, error) {
	query := base.GetDB(ctx, r.db).NewSelect().
		TableExpr(`users.students AS student`).
		Where(`student.school_class = ?`, className).
		Where(`student.status <> ?`, string(users.StudentStatusAlumnus))

	query = base.WithTenantFilter(ctx, query, "student")

	count, err := query.Count(ctx)

	if err != nil {
		return 0, &modelBase.DatabaseError{
			Op:  "get student count by class",
			Err: err,
		}
	}

	return count, nil
}

// GetStudentsByClasses retrieves students in the given classes with their names.
// Note: Using unquoted aliases (s, p) here because this is raw SQL via TableExpr/ColumnExpr,
// not ModelTableExpr. The CLAUDE.md quoting rule applies to ModelTableExpr where BUN maps
// results to struct fields via the alias. Here we use explicit column aliases (student_id,
// person_id, etc.) which BUN scans by name directly.
func (r *GradeTransitionRepository) GetStudentsByClasses(ctx context.Context, classes []string) ([]*education.StudentClassInfo, error) {
	if len(classes) == 0 {
		return []*education.StudentClassInfo{}, nil
	}

	var students []*education.StudentClassInfo
	query := base.GetDB(ctx, r.db).NewSelect().
		ColumnExpr(`s.id AS student_id`).
		ColumnExpr(`s.person_id`).
		ColumnExpr(`CONCAT(p.first_name, ' ', p.last_name) AS person_name`).
		ColumnExpr(`s.school_class`).
		ColumnExpr(`s.status`).
		TableExpr(`users.students AS s`).
		Join(`INNER JOIN users.persons AS p ON p.id = s.person_id`).
		Where(`s.school_class IN (?)`, bun.List(classes)).
		Where(`s.status <> ?`, string(users.StudentStatusAlumnus)).
		Order(`s.school_class ASC, p.last_name ASC, p.first_name ASC`)

	query = base.WithTenantFilter(ctx, query, "s")

	err := query.Scan(ctx, &students)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "get students by classes",
			Err: err,
		}
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

	updQuery := base.GetDB(ctx, r.db).NewUpdate().
		Model((*users.Student)(nil)).
		ModelTableExpr(`users.students AS "student"`).
		Set("school_class = ?", toClass).
		Set("updated_at = NOW()").
		Where(`"student".id IN (?)`, bun.List(studentIDs)).
		Where(`"student".school_class = ?`, fromClass).
		Where(`"student".status <> ?`, string(users.StudentStatusAlumnus))

	updQuery = base.WithTenantFilter(ctx, updQuery, "student")

	result, err := updQuery.Exec(ctx)
	if err != nil {
		return 0, &modelBase.DatabaseError{
			Op:  "promote students by ids",
			Err: err,
		}
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return 0, &modelBase.DatabaseError{
			Op:  "get rows affected",
			Err: err,
		}
	}

	return affected, nil
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
			Err: err,
		}
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return 0, &modelBase.DatabaseError{
			Op:  "get rows affected",
			Err: err,
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
	updQuery := base.GetDB(ctx, r.db).NewUpdate().
		Model((*users.Student)(nil)).
		ModelTableExpr(`users.students AS "student"`).
		Set("school_class = ?", fromClass).
		Set("updated_at = NOW()").
		Where(`"student".id = ?`, studentID).
		Where(`"student".school_class = ?`, toClass)

	updQuery = base.WithTenantFilter(ctx, updQuery, "student")

	result, err := updQuery.Exec(ctx)
	if err != nil {
		return 0, &modelBase.DatabaseError{
			Op:  "revert student class",
			Err: err,
		}
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return 0, &modelBase.DatabaseError{
			Op:  "get rows affected",
			Err: err,
		}
	}

	return affected, nil
}

// GraduateStudentsByClasses soft-deletes graduating students: their rows are
// kept but status flips to "alumnus", which removes them from every
// staff-facing read path (see the alumnus filters in the student repository).
// A transition revert restores them via ReactivateStudentsByIDs.
func (r *GradeTransitionRepository) GraduateStudentsByClasses(ctx context.Context, classes []string) (int64, error) {
	if len(classes) == 0 {
		return 0, nil
	}

	updQuery := base.GetDB(ctx, r.db).NewUpdate().
		Model((*struct{})(nil)).
		ModelTableExpr(`users.students AS "student"`).
		Set("status = ?", string(users.StudentStatusAlumnus)).
		Set("updated_at = NOW()").
		Where(`"student".school_class IN (?)`, bun.List(classes)).
		Where(`"student".status <> ?`, string(users.StudentStatusAlumnus))

	updQuery = base.WithTenantFilter(ctx, updQuery, "student")

	result, err := updQuery.Exec(ctx)

	if err != nil {
		return 0, &modelBase.DatabaseError{
			Op:  "graduate students by classes",
			Err: err,
		}
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return 0, &modelBase.DatabaseError{
			Op:  "get rows affected",
			Err: err,
		}
	}

	return affected, nil
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

	updQuery := base.GetDB(ctx, r.db).NewUpdate().
		Model((*struct{})(nil)).
		ModelTableExpr(`users.students AS "student"`).
		Set("status = ?", string(users.StudentStatusAlumnus)).
		Set("updated_at = NOW()").
		Where(`"student".id IN (?)`, bun.List(studentIDs)).
		Where(`"student".status <> ?`, string(users.StudentStatusAlumnus))

	updQuery = base.WithTenantFilter(ctx, updQuery, "student")

	result, err := updQuery.Exec(ctx)
	if err != nil {
		return 0, &modelBase.DatabaseError{
			Op:  "graduate students by ids",
			Err: err,
		}
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return 0, &modelBase.DatabaseError{
			Op:  "get rows affected",
			Err: err,
		}
	}

	return affected, nil
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

	type reactivatedRow struct {
		ID int64 `bun:"id"`
	}
	var rows []reactivatedRow

	updQuery := base.GetDB(ctx, r.db).NewUpdate().
		Model((*struct{})(nil)).
		ModelTableExpr(`users.students AS "student"`).
		Set("status = ?", targetStatus).
		Set("updated_at = NOW()").
		Where(`"student".id IN (?)`, bun.List(studentIDs)).
		Where(`"student".status = ?`, string(users.StudentStatusAlumnus)).
		Returning(`"student".id`)

	updQuery = base.WithTenantFilter(ctx, updQuery, "student")

	if _, err := updQuery.Exec(ctx, &rows); err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "reactivate students to status",
			Err: err,
		}
	}

	reactivated := make([]int64, 0, len(rows))
	for _, row := range rows {
		reactivated = append(reactivated, row.ID)
	}

	return reactivated, nil
}

// ReleaseStudentTagsByIDs clears users.persons.tag_id for the given students and
// returns the tag each of them was holding, keyed by student id. Students
// without a tag are absent from the map.
//
// Graduation is a soft delete, so without this the physical bracelet stays bound
// to a departed child: every staff-facing student route 404s on an alumnus, the
// kiosk's dedicated unassign call goes through the same gate, and the tag can
// never be handed to a current child through the normal flow. Clearing it at
// apply time (and ledgering the value in grade_transition_history.rfid_tag)
// frees the bracelet without losing what to restore on a revert (#405 review).
//
// The rows are read FOR UPDATE in the same statement-order the caller already
// locked the student rows in, so a concurrent bracelet assignment either
// committed before this read (and is the value ledgered) or waits for the
// apply's transaction.
func (r *GradeTransitionRepository) ReleaseStudentTagsByIDs(ctx context.Context, studentIDs []int64) (map[int64]string, error) {
	if len(studentIDs) == 0 {
		return nil, nil
	}

	type heldTag struct {
		StudentID int64  `bun:"student_id"`
		PersonID  int64  `bun:"person_id"`
		TagID     string `bun:"tag_id"`
	}
	var held []heldTag

	tenantID := tenant.FromContext(ctx)

	selQuery := base.GetDB(ctx, r.db).NewSelect().
		Model((*struct{})(nil)).
		ModelTableExpr(`users.students AS "student"`).
		ColumnExpr(`"student".id AS student_id`).
		ColumnExpr(`"person".id AS person_id`).
		ColumnExpr(`"person".tag_id AS tag_id`).
		Join(`JOIN users.persons AS "person" ON "person".id = "student".person_id AND "person".tenant_id = "student".tenant_id`).
		Where(`"student".id IN (?)`, bun.List(studentIDs)).
		Where(`"person".tag_id IS NOT NULL`).
		Where(`"person".deleted_at IS NULL`).
		OrderExpr(`"person".id ASC`).
		For("UPDATE OF \"person\"")

	selQuery = base.WithTenantFilter(ctx, selQuery, "student")

	if err := selQuery.Scan(ctx, &held); err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "read student rfid tags",
			Err: err,
		}
	}
	if len(held) == 0 {
		return nil, nil
	}

	personIDs := make([]int64, 0, len(held))
	released := make(map[int64]string, len(held))
	for _, row := range held {
		personIDs = append(personIDs, row.PersonID)
		released[row.StudentID] = row.TagID
	}

	updQuery := base.GetDB(ctx, r.db).NewUpdate().
		Model((*struct{})(nil)).
		ModelTableExpr(`users.persons AS "person"`).
		Set("tag_id = NULL").
		Set("updated_at = NOW()").
		Where(`"person".id IN (?)`, bun.List(personIDs)).
		Where(`"person".tenant_id = ?`, tenantID)

	if _, err := updQuery.Exec(ctx); err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "release student rfid tags",
			Err: err,
		}
	}

	return released, nil
}

// RestoreStudentTag re-links a tag this transition released back to the
// student's person row. Returns false without touching anything when the child
// has since been given another tag, or when the released tag now belongs to
// somebody else — the bracelet was reissued while they were gone, and the
// current holder wins (users.persons carries UNIQUE (tenant_id, tag_id), so
// overwriting is not even an option).
//
// The NOT EXISTS holder check reads the statement snapshot, so a tag handed out
// after that read but before this UPDATE reaches the unique index arrives as a
// 23505 rather than a zero-row result. That is the same "current holder wins"
// outcome and must not fail the revert — but a raw 23505 aborts the surrounding
// transaction, so the write runs inside a savepoint that is rolled back on
// exactly that violation (#405 review).
func (r *GradeTransitionRepository) RestoreStudentTag(ctx context.Context, studentID int64, tagID string) (bool, error) {
	if tagID == "" {
		return false, nil
	}

	tenantID := tenant.FromContext(ctx)

	tx, inTx := modelBase.TxFromContext(ctx)
	if inTx {
		if _, err := tx.ExecContext(ctx, "SAVEPOINT "+restoreStudentTagSavepoint); err != nil {
			return false, &modelBase.DatabaseError{
				Op:  "restore student rfid tag savepoint",
				Err: err,
			}
		}
	}

	updQuery := base.GetDB(ctx, r.db).NewUpdate().
		Model((*struct{})(nil)).
		ModelTableExpr(`users.persons AS "person"`).
		Set("tag_id = ?", tagID).
		Set("updated_at = NOW()").
		Where(`"person".tenant_id = ?`, tenantID).
		Where(`"person".tag_id IS NULL`).
		Where(`"person".deleted_at IS NULL`).
		Where(`"person".id = (SELECT "student".person_id FROM users.students AS "student" WHERE "student".id = ? AND "student".tenant_id = ?)`,
			studentID, tenantID).
		Where(`NOT EXISTS (SELECT 1 FROM users.persons AS "holder" WHERE "holder".tenant_id = ? AND "holder".tag_id = ?)`,
			tenantID, tagID)

	result, err := updQuery.Exec(ctx)
	if err != nil {
		if modelBase.IsUniqueViolationOn(err, personsTenantTagUniqueIndex) {
			// Somebody claimed the tag in the gap. Undo the failed statement so
			// the revert transaction stays usable, and report "not re-linked".
			if inTx {
				if _, rbErr := tx.ExecContext(ctx, "ROLLBACK TO SAVEPOINT "+restoreStudentTagSavepoint); rbErr != nil {
					return false, &modelBase.DatabaseError{
						Op:  "restore student rfid tag rollback to savepoint",
						Err: rbErr,
					}
				}
			}
			return false, nil
		}
		return false, &modelBase.DatabaseError{
			Op:  "restore student rfid tag",
			Err: err,
		}
	}

	if inTx {
		if _, err := tx.ExecContext(ctx, "RELEASE SAVEPOINT "+restoreStudentTagSavepoint); err != nil {
			return false, &modelBase.DatabaseError{
				Op:  "restore student rfid tag release savepoint",
				Err: err,
			}
		}
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return false, &modelBase.DatabaseError{
			Op:  "get rows affected",
			Err: err,
		}
	}

	return affected > 0, nil
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

	type stateRow struct {
		ID     int64  `bun:"id"`
		Status string `bun:"status"`
	}
	var rows []stateRow

	query := base.GetDB(ctx, r.db).NewSelect().
		Model((*struct{})(nil)).
		ModelTableExpr(`users.students AS "student"`).
		Column("id", "status").
		Where(`"student".id IN (?)`, bun.List(studentIDs))

	query = base.WithTenantFilter(ctx, query, "student")

	if err := query.Scan(ctx, &rows); err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find student states by IDs",
			Err: err,
		}
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
			Err: err,
		}
	}
	return nil
}
