package education

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/education"
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
	db *bun.DB
}

// NewGradeTransitionRepository creates a new GradeTransitionRepository
func NewGradeTransitionRepository(db *bun.DB) education.GradeTransitionRepository {
	return &GradeTransitionRepository{
		Repository: base.NewRepository[*education.GradeTransition](db, tableGradeTransitions, "GradeTransition"),
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

	// Use transaction from context if available
	var db bun.IDB = r.db
	if tx, ok := modelBase.TxFromContext(ctx); ok && tx != nil {
		db = tx
	}

	_, err := db.NewInsert().
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
	err := r.db.NewSelect().
		Model(t).
		ModelTableExpr(tableGradeTransitions + ` AS "grade_transition"`).
		Where(`"grade_transition".id = ?`, id).
		Scan(ctx)

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
	err := r.db.NewSelect().
		Model(t).
		ModelTableExpr(tableGradeTransitions + ` AS "grade_transition"`).
		Where(`"grade_transition".id = ?`, id).
		Scan(ctx)

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

	// Use transaction from context if available
	var db bun.IDB = r.db
	if tx, ok := modelBase.TxFromContext(ctx); ok && tx != nil {
		db = tx
	}

	_, err := db.NewUpdate().
		Model(t).
		ModelTableExpr(tableGradeTransitions + ` AS "grade_transition"`).
		WherePK().
		Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "update grade transition",
			Err: err,
		}
	}

	return nil
}

// Delete deletes a grade transition
func (r *GradeTransitionRepository) Delete(ctx context.Context, id int64) error {
	// Use transaction from context if available
	var db bun.IDB = r.db
	if tx, ok := modelBase.TxFromContext(ctx); ok && tx != nil {
		db = tx
	}

	_, err := db.NewDelete().
		Model((*education.GradeTransition)(nil)).
		ModelTableExpr(tableGradeTransitions + ` AS "grade_transition"`).
		Where(`"grade_transition".id = ?`, id).
		Exec(ctx)
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
	var transitions []*education.GradeTransition

	query := r.db.NewSelect().
		TableExpr(tableGradeTransitions).
		ColumnExpr("*")

	// Build count query with same filters (but without pagination)
	countQuery := r.db.NewSelect().
		TableExpr(tableGradeTransitions)

	// Apply query options (filters + pagination to data query, filters only to count query)
	if options != nil {
		query = options.ApplyToQuery(query)
		// Apply only filters to count query (not pagination)
		if options.Filter != nil {
			countQuery = options.Filter.ApplyToQuery(countQuery)
		}
	}

	// Get total count with filters applied
	count, err := countQuery.Count(ctx)
	if err != nil {
		return nil, 0, &modelBase.DatabaseError{
			Op:  "count grade transitions",
			Err: err,
		}
	}

	// Execute query
	err = query.Order(orderByCreatedAtDesc).Scan(ctx, &transitions)
	if err != nil {
		return nil, 0, &modelBase.DatabaseError{
			Op:  "list grade transitions",
			Err: err,
		}
	}

	return transitions, count, nil
}

// FindByAcademicYear retrieves grade transitions for a specific academic year
func (r *GradeTransitionRepository) FindByAcademicYear(ctx context.Context, year string) ([]*education.GradeTransition, error) {
	var transitions []*education.GradeTransition
	err := r.db.NewSelect().
		TableExpr(tableGradeTransitions).
		ColumnExpr("*").
		Where("academic_year = ?", year).
		Order(orderByCreatedAtDesc).
		Scan(ctx, &transitions)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find grade transitions by academic year",
			Err: err,
		}
	}

	return transitions, nil
}

// FindByStatus retrieves grade transitions with a specific status
func (r *GradeTransitionRepository) FindByStatus(ctx context.Context, status string) ([]*education.GradeTransition, error) {
	var transitions []*education.GradeTransition
	err := r.db.NewSelect().
		TableExpr(tableGradeTransitions).
		ColumnExpr("*").
		Where("status = ?", status).
		Order(orderByCreatedAtDesc).
		Scan(ctx, &transitions)

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

	// Use transaction from context if available
	var db bun.IDB = r.db
	if tx, ok := modelBase.TxFromContext(ctx); ok && tx != nil {
		db = tx
	}

	_, err := db.NewInsert().
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
	}

	// Use transaction from context if available
	var db bun.IDB = r.db
	if tx, ok := modelBase.TxFromContext(ctx); ok && tx != nil {
		db = tx
	}

	_, err := db.NewInsert().
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
	// Use transaction from context if available
	var db bun.IDB = r.db
	if tx, ok := modelBase.TxFromContext(ctx); ok && tx != nil {
		db = tx
	}

	_, err := db.NewDelete().
		TableExpr(tableGradeTransitionMappings).
		Where(whereTransitionID, transitionID).
		Exec(ctx)
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
	err := r.db.NewSelect().
		TableExpr(tableGradeTransitionMappings).
		ColumnExpr("*").
		Where(whereTransitionID, transitionID).
		Order("from_class ASC").
		Scan(ctx, &mappings)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "get grade transition mappings",
			Err: err,
		}
	}

	return mappings, nil
}

// CreateHistory creates a new history record
func (r *GradeTransitionRepository) CreateHistory(ctx context.Context, h *education.GradeTransitionHistory) error {
	if h == nil {
		return fmt.Errorf("history cannot be nil")
	}

	if err := h.Validate(); err != nil {
		return err
	}

	// Use transaction from context if available
	var db bun.IDB = r.db
	if tx, ok := modelBase.TxFromContext(ctx); ok && tx != nil {
		db = tx
	}

	_, err := db.NewInsert().
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
	}

	// Use transaction from context if available
	var db bun.IDB = r.db
	if tx, ok := modelBase.TxFromContext(ctx); ok && tx != nil {
		db = tx
	}

	_, err := db.NewInsert().
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
	err := r.db.NewSelect().
		TableExpr(tableGradeTransitionHistory).
		ColumnExpr("*").
		Where(whereTransitionID, transitionID).
		Order("created_at ASC").
		Scan(ctx, &history)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "get grade transition history",
			Err: err,
		}
	}

	return history, nil
}

// GetDistinctClasses retrieves all distinct school_class values from students
func (r *GradeTransitionRepository) GetDistinctClasses(ctx context.Context) ([]string, error) {
	var classes []string
	err := r.db.NewSelect().
		TableExpr(`users.students`).
		ColumnExpr(`DISTINCT school_class`).
		Where(`school_class IS NOT NULL AND school_class != ''`).
		Order("school_class ASC").
		Scan(ctx, &classes)

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
	count, err := r.db.NewSelect().
		TableExpr(`users.students`).
		Where(`school_class = ?`, className).
		Count(ctx)

	if err != nil {
		return 0, &modelBase.DatabaseError{
			Op:  "get student count by class",
			Err: err,
		}
	}

	return count, nil
}

// GetStudentsByClasses retrieves students in the given classes with their names
func (r *GradeTransitionRepository) GetStudentsByClasses(ctx context.Context, classes []string) ([]*education.StudentClassInfo, error) {
	if len(classes) == 0 {
		return []*education.StudentClassInfo{}, nil
	}

	// Use transaction from context if available
	var db bun.IDB = r.db
	if tx, ok := modelBase.TxFromContext(ctx); ok && tx != nil {
		db = tx
	}

	var students []*education.StudentClassInfo
	err := db.NewSelect().
		ColumnExpr(`s.id AS student_id`).
		ColumnExpr(`s.person_id`).
		ColumnExpr(`CONCAT(p.first_name, ' ', p.last_name) AS person_name`).
		ColumnExpr(`s.school_class`).
		TableExpr(`users.students AS s`).
		Join(`INNER JOIN users.persons AS p ON p.id = s.person_id`).
		Where(`s.school_class IN (?)`, bun.In(classes)).
		Order(`s.school_class ASC, p.last_name ASC, p.first_name ASC`).
		Scan(ctx, &students)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "get students by classes",
			Err: err,
		}
	}

	return students, nil
}

// UpdateStudentClasses updates student classes based on transition mappings
// This is a join-based UPDATE for efficiency
func (r *GradeTransitionRepository) UpdateStudentClasses(ctx context.Context, transitionID int64) (int64, error) {
	// Use transaction from context if available
	var db bun.IDB = r.db
	if tx, ok := modelBase.TxFromContext(ctx); ok && tx != nil {
		db = tx
	}

	// Execute bulk UPDATE using JOIN on mappings
	result, err := db.ExecContext(ctx, `
		UPDATE users.students s
		SET school_class = m.to_class,
		    updated_at = NOW()
		FROM education.grade_transition_mappings m
		WHERE m.transition_id = ?
		  AND m.to_class IS NOT NULL
		  AND s.school_class = m.from_class
	`, transitionID)

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

// DeleteStudentsByClasses deletes students in the given classes (for graduates)
func (r *GradeTransitionRepository) DeleteStudentsByClasses(ctx context.Context, classes []string) (int64, error) {
	if len(classes) == 0 {
		return 0, nil
	}

	// Use transaction from context if available
	var db bun.IDB = r.db
	if tx, ok := modelBase.TxFromContext(ctx); ok && tx != nil {
		db = tx
	}

	result, err := db.NewDelete().
		Model((*struct{})(nil)).
		ModelTableExpr(`users.students`).
		Where(`school_class IN (?)`, bun.In(classes)).
		Exec(ctx)

	if err != nil {
		return 0, &modelBase.DatabaseError{
			Op:  "delete students by classes",
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
