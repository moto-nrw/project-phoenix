package database

import (
	"context"
	"database/sql"
	"errors"
	models2 "github.com/moto-nrw/project-phoenix/models"

	"github.com/uptrace/bun"
)

// AgStore implements database operations for activity group management
type AgStore struct {
	db *bun.DB
}

// NewAgStore returns an AgStore
func NewAgStore(db *bun.DB) *AgStore {
	return &AgStore{
		db: db,
	}
}

// ======== AG Category Methods ========

// CreateAgCategory creates a new activity group category
func (s *AgStore) CreateAgCategory(ctx context.Context, category *models2.AgCategory) error {
	_, err := s.db.NewInsert().
		Model(category).
		Exec(ctx)
	return err
}

// GetAgCategoryByID retrieves an activity group category by ID
func (s *AgStore) GetAgCategoryByID(ctx context.Context, id int64) (*models2.AgCategory, error) {
	category := new(models2.AgCategory)
	err := s.db.NewSelect().
		Model(category).
		Where("id = ?", id).
		Scan(ctx)

	if err != nil {
		return nil, err
	}

	return category, nil
}

// UpdateAgCategory updates an existing activity group category
func (s *AgStore) UpdateAgCategory(ctx context.Context, category *models2.AgCategory) error {
	_, err := s.db.NewUpdate().
		Model(category).
		Column("name").
		WherePK().
		Exec(ctx)

	return err
}

// DeleteAgCategory deletes an activity group category
func (s *AgStore) DeleteAgCategory(ctx context.Context, id int64) error {
	// Check if the category is in use by any activity groups
	used, err := s.db.NewSelect().
		Model((*models2.Ag)(nil)).
		Where("ag_category_id = ?", id).
		Exists(ctx)

	if err != nil {
		return err
	}

	if used {
		return errors.New("cannot delete category that is in use by activity groups")
	}

	_, err = s.db.NewDelete().
		Model((*models2.AgCategory)(nil)).
		Where("id = ?", id).
		Exec(ctx)

	return err
}

// ListAgCategories returns a list of all activity group categories
func (s *AgStore) ListAgCategories(ctx context.Context) ([]models2.AgCategory, error) {
	var categories []models2.AgCategory

	err := s.db.NewSelect().
		Model(&categories).
		OrderExpr("name ASC").
		Scan(ctx)

	if err != nil {
		return nil, err
	}

	return categories, nil
}

// ======== Activity Group Methods ========

// CreateAg creates a new activity group
func (s *AgStore) CreateAg(ctx context.Context, ag *models2.Ag, studentIDs []int64, timeslots []*models2.AgTime) error {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Create the activity group
	_, err = tx.NewInsert().
		Model(ag).
		Exec(ctx)

	if err != nil {
		return err
	}

	// Add timeslots if provided
	if len(timeslots) > 0 {
		for _, timeslot := range timeslots {
			timeslot.AgID = ag.ID

			_, err = tx.NewInsert().
				Model(timeslot).
				Exec(ctx)

			if err != nil {
				return err
			}
		}
	}

	// Add students if provided
	if len(studentIDs) > 0 {
		for _, studentID := range studentIDs {
			studentAg := &models2.StudentAg{
				StudentID: studentID,
				AgID:      ag.ID,
			}

			_, err = tx.NewInsert().
				Model(studentAg).
				Exec(ctx)

			if err != nil {
				return err
			}
		}
	}

	return tx.Commit()
}

// GetAgByID retrieves an activity group by ID with related data
func (s *AgStore) GetAgByID(ctx context.Context, id int64) (*models2.Ag, error) {
	ag := new(models2.Ag)
	err := s.db.NewSelect().
		Model(ag).
		Relation("Supervisor").
		Relation("Supervisor.CustomUser").
		Relation("AgCategory").
		Relation("Datespan").
		Relation("Times").
		Relation("Times.Timespan").
		Relation("Students").
		Relation("Students.CustomUser").
		Where("ag.id = ?", id).
		Scan(ctx)

	if err != nil {
		return nil, err
	}

	return ag, nil
}

// UpdateAg updates an existing activity group
func (s *AgStore) UpdateAg(ctx context.Context, ag *models2.Ag) error {
	_, err := s.db.NewUpdate().
		Model(ag).
		Column("name", "max_participant", "is_open_ags",
			"supervisor_id", "ag_category_id", "datespan_id", "modified_at").
		WherePK().
		Exec(ctx)

	return err
}

// DeleteAg deletes an activity group
func (s *AgStore) DeleteAg(ctx context.Context, id int64) error {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Delete AG timeslots
	_, err = tx.NewDelete().
		Model((*models2.AgTime)(nil)).
		Where("ag_id = ?", id).
		Exec(ctx)

	if err != nil {
		return err
	}

	// Delete student enrollments
	_, err = tx.NewDelete().
		Model((*models2.StudentAg)(nil)).
		Where("ag_id = ?", id).
		Exec(ctx)

	if err != nil {
		return err
	}

	// Delete the activity group
	_, err = tx.NewDelete().
		Model((*models2.Ag)(nil)).
		Where("id = ?", id).
		Exec(ctx)

	if err != nil {
		return err
	}

	return tx.Commit()
}

// ListAgs returns a list of all activity groups with optional filtering
func (s *AgStore) ListAgs(ctx context.Context, filters map[string]interface{}) ([]models2.Ag, error) {
	var ags []models2.Ag

	query := s.db.NewSelect().
		Model(&ags).
		Relation("Supervisor").
		Relation("Supervisor.CustomUser").
		Relation("AgCategory")

	// Apply filters
	if categoryID, ok := filters["category_id"].(int64); ok && categoryID > 0 {
		query = query.Where("ag_category_id = ?", categoryID)
	}

	if supervisorID, ok := filters["supervisor_id"].(int64); ok && supervisorID > 0 {
		query = query.Where("supervisor_id = ?", supervisorID)
	}

	if isOpen, ok := filters["is_open"].(bool); ok {
		query = query.Where("is_open_ags = ?", isOpen)
	}

	if searchTerm, ok := filters["search"].(string); ok && searchTerm != "" {
		query = query.Where("name ILIKE ?", "%"+searchTerm+"%")
	}

	err := query.OrderExpr("name ASC").
		Scan(ctx)

	if err != nil {
		return nil, err
	}

	return ags, nil
}

// ======== AG Time Methods ========

// CreateAgTime creates a new activity group timeslot
func (s *AgStore) CreateAgTime(ctx context.Context, agTime *models2.AgTime) error {
	_, err := s.db.NewInsert().
		Model(agTime).
		Exec(ctx)

	return err
}

// GetAgTimeByID retrieves an activity group timeslot by ID
func (s *AgStore) GetAgTimeByID(ctx context.Context, id int64) (*models2.AgTime, error) {
	agTime := new(models2.AgTime)
	err := s.db.NewSelect().
		Model(agTime).
		Relation("Timespan").
		Relation("Ag").
		Where("id = ?", id).
		Scan(ctx)

	if err != nil {
		return nil, err
	}

	return agTime, nil
}

// UpdateAgTime updates an existing activity group timeslot
func (s *AgStore) UpdateAgTime(ctx context.Context, agTime *models2.AgTime) error {
	_, err := s.db.NewUpdate().
		Model(agTime).
		Column("weekday", "timespan_id").
		WherePK().
		Exec(ctx)

	return err
}

// DeleteAgTime deletes an activity group timeslot
func (s *AgStore) DeleteAgTime(ctx context.Context, id int64) error {
	_, err := s.db.NewDelete().
		Model((*models2.AgTime)(nil)).
		Where("id = ?", id).
		Exec(ctx)

	return err
}

// ListAgTimes returns a list of all timeslots for a specific activity group
func (s *AgStore) ListAgTimes(ctx context.Context, agID int64) ([]models2.AgTime, error) {
	var agTimes []models2.AgTime

	err := s.db.NewSelect().
		Model(&agTimes).
		Relation("Timespan").
		Where("ag_id = ?", agID).
		OrderExpr("weekday ASC, (SELECT starttime FROM timespans WHERE id = timespan_id) ASC").
		Scan(ctx)

	if err != nil {
		return nil, err
	}

	return agTimes, nil
}

// ======== Student Enrollment Methods ========

// EnrollStudent enrolls a student in an activity group
func (s *AgStore) EnrollStudent(ctx context.Context, agID, studentID int64) error {
	// Check if the student is already enrolled
	exists, err := s.db.NewSelect().
		Model((*models2.StudentAg)(nil)).
		Where("ag_id = ? AND student_id = ?", agID, studentID).
		Exists(ctx)

	if err != nil {
		return err
	}

	if exists {
		return errors.New("student is already enrolled in this activity group")
	}

	// Check if the activity group has reached maximum participants
	ag := new(models2.Ag)
	err = s.db.NewSelect().
		Model(ag).
		Column("id", "max_participant").
		Where("id = ?", agID).
		Scan(ctx)

	if err != nil {
		return err
	}

	// Count current enrollments
	count, err := s.db.NewSelect().
		Model((*models2.StudentAg)(nil)).
		Where("ag_id = ?", agID).
		Count(ctx)

	if err != nil {
		return err
	}

	if count >= ag.MaxParticipant {
		return errors.New("activity group has reached maximum number of participants")
	}

	// Enroll the student
	studentAg := &models2.StudentAg{
		StudentID: studentID,
		AgID:      agID,
	}

	_, err = s.db.NewInsert().
		Model(studentAg).
		Exec(ctx)

	return err
}

// UnenrollStudent removes a student from an activity group
func (s *AgStore) UnenrollStudent(ctx context.Context, agID, studentID int64) error {
	_, err := s.db.NewDelete().
		Model((*models2.StudentAg)(nil)).
		Where("ag_id = ? AND student_id = ?", agID, studentID).
		Exec(ctx)

	return err
}

// ListEnrolledStudents returns a list of all students enrolled in a specific activity group
func (s *AgStore) ListEnrolledStudents(ctx context.Context, agID int64) ([]*models2.Student, error) {
	var students []*models2.Student

	err := s.db.NewSelect().
		Model(&students).
		Join("JOIN student_ags sa ON sa.student_id = student.id").
		Where("sa.ag_id = ?", agID).
		Relation("CustomUser").
		Relation("Group").
		OrderExpr("custom_user.second_name ASC, custom_user.first_name ASC").
		Scan(ctx)

	if err != nil {
		return nil, err
	}

	return students, nil
}

// ListStudentAgs returns a list of all activity groups a student is enrolled in
func (s *AgStore) ListStudentAgs(ctx context.Context, studentID int64) ([]models2.Ag, error) {
	var ags []models2.Ag

	err := s.db.NewSelect().
		Model(&ags).
		Join("JOIN student_ags sa ON sa.ag_id = ag.id").
		Where("sa.student_id = ?", studentID).
		Relation("Supervisor").
		Relation("Supervisor.CustomUser").
		Relation("AgCategory").
		Relation("Times").
		Relation("Times.Timespan").
		OrderExpr("ag.name ASC").
		Scan(ctx)

	if err != nil {
		return nil, err
	}

	return ags, nil
}
