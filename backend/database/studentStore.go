package database

import (
	"context"
	"database/sql"
	"errors"
	models2 "github.com/moto-nrw/project-phoenix/models"
	"time"

	"github.com/uptrace/bun"
)

// StudentStore implements database operations for student management
type StudentStore struct {
	db *bun.DB
}

// NewStudentStore returns a StudentStore
func NewStudentStore(db *bun.DB) *StudentStore {
	return &StudentStore{
		db: db,
	}
}

// GetStudentByID retrieves a Student by ID with related CustomUser
func (s *StudentStore) GetStudentByID(ctx context.Context, id int64) (*models2.Student, error) {
	student := new(models2.Student)
	err := s.db.NewSelect().
		Model(student).
		Relation("CustomUser").
		Relation("Group").
		Where("student.id = ?", id).
		Scan(ctx)

	if err != nil {
		return nil, err
	}

	return student, nil
}

// GetStudentByCustomUserID retrieves a Student by CustomUserID
func (s *StudentStore) GetStudentByCustomUserID(ctx context.Context, customUserID int64) (*models2.Student, error) {
	student := new(models2.Student)
	err := s.db.NewSelect().
		Model(student).
		Relation("CustomUser").
		Relation("Group").
		Where("student.custom_users_id = ?", customUserID).
		Scan(ctx)

	if err != nil {
		return nil, err
	}

	return student, nil
}

// CreateStudent creates a new Student
func (s *StudentStore) CreateStudent(ctx context.Context, student *models2.Student) error {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Check if a student already exists for this custom user
	exists, err := tx.NewSelect().
		Model((*models2.Student)(nil)).
		Where("custom_users_id = ?", student.CustomUserID).
		Exists(ctx)

	if err != nil {
		return err
	}

	if exists {
		return errors.New("a student already exists for this custom user")
	}

	// Verify the group exists before creating the student
	if student.GroupID > 0 {
		groupExists, err := tx.NewSelect().
			Model((*models2.Group)(nil)).
			Where("id = ?", student.GroupID).
			Exists(ctx)

		if err != nil {
			return err
		}

		if !groupExists {
			return errors.New("specified group does not exist")
		}
	}

	// Create the student with verified group_id
	_, err = tx.NewInsert().
		Model(student).
		Exec(ctx)

	if err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	return nil
}

// UpdateStudent updates an existing Student
func (s *StudentStore) UpdateStudent(ctx context.Context, student *models2.Student) error {
	_, err := s.db.NewUpdate().
		Model(student).
		WherePK().
		Exec(ctx)

	return err
}

// DeleteStudent deletes a Student
func (s *StudentStore) DeleteStudent(ctx context.Context, id int64) error {
	_, err := s.db.NewDelete().
		Model((*models2.Student)(nil)).
		Where("id = ?", id).
		Exec(ctx)

	return err
}

// ListStudents returns a list of all Students with related CustomUser and Group
func (s *StudentStore) ListStudents(ctx context.Context, filters map[string]interface{}) ([]models2.Student, error) {
	var students []models2.Student

	query := s.db.NewSelect().
		Model(&students).
		Relation("CustomUser").
		Relation("Group")

	// Apply filters
	if groupID, ok := filters["group_id"].(int64); ok && groupID > 0 {
		query = query.Where("student.group_id = ?", groupID)
	}

	if searchTerm, ok := filters["search"].(string); ok && searchTerm != "" {
		query = query.Where("custom_user.first_name ILIKE ? OR custom_user.second_name ILIKE ?",
			"%"+searchTerm+"%", "%"+searchTerm+"%")
	}

	if inHouse, ok := filters["in_house"].(bool); ok {
		query = query.Where("student.in_house = ?", inHouse)
	}

	err := query.OrderExpr("custom_user.first_name ASC").
		Scan(ctx)

	if err != nil {
		return nil, err
	}

	return students, nil
}

// UpdateStudentLocation updates a student's location flags (in_house, wc, school_yard)
func (s *StudentStore) UpdateStudentLocation(ctx context.Context, id int64, locations map[string]bool) error {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback()

	student, err := s.GetStudentByID(ctx, id)
	if err != nil {
		return err
	}

	// Update student location flags
	if inHouse, ok := locations["in_house"]; ok {
		student.InHouse = inHouse
	}
	if wc, ok := locations["wc"]; ok {
		student.WC = wc
	}
	if schoolYard, ok := locations["school_yard"]; ok {
		student.SchoolYard = schoolYard
	}

	_, err = tx.NewUpdate().
		Model(student).
		Column("in_house", "wc", "school_yard", "modified_at").
		WherePK().
		Exec(ctx)

	if err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	return nil
}

// CreateStudentVisit creates a new Visit record for a student
func (s *StudentStore) CreateStudentVisit(ctx context.Context, studentID, roomID, timespanID int64) (*models2.Visit, error) {
	// Start a transaction
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// Create the basic visit record
	visit := &models2.Visit{
		Day:        time.Now(),
		StudentID:  studentID,
		RoomID:     roomID,
		TimespanID: timespanID,
		CreatedAt:  time.Now(),
	}

	// Check if the room is part of an active combined group
	var combinedGroupID int64
	err = tx.NewSelect().
		TableExpr("combined_group_groups cgg").
		Column("cgg.combinedgroup_id").
		Join("JOIN groups g ON g.id = cgg.group_id").
		Join("JOIN combined_groups cg ON cg.id = cgg.combinedgroup_id").
		Where("g.room_id = ? AND cg.is_active = ?", roomID, true).
		OrderExpr("cgg.id DESC").
		Limit(1).
		Scan(ctx, &combinedGroupID)

	// If a combined group was found, add it to the visit
	if err == nil && combinedGroupID > 0 {
		// Check if the combined group is not expired
		var combinedGroup models2.CombinedGroup
		err = tx.NewSelect().
			Model(&combinedGroup).
			Where("id = ?", combinedGroupID).
			Scan(ctx)

		if err == nil {
			// If the combined group has an expiration and is not expired, or has no expiration
			now := time.Now()
			if combinedGroup.ValidUntil == nil || combinedGroup.ValidUntil.After(now) {
				visit.CombinedGroupID = &combinedGroupID
			}
		}
	}

	// Insert the visit
	_, err = tx.NewInsert().
		Model(visit).
		Exec(ctx)

	if err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return visit, nil
}

// GetStudentVisits retrieves all visits for a student
func (s *StudentStore) GetStudentVisits(ctx context.Context, studentID int64, date *time.Time) ([]models2.Visit, error) {
	var visits []models2.Visit

	query := s.db.NewSelect().
		Model(&visits).
		Relation("Room").
		Relation("Timespan").
		Relation("CombinedGroup").
		Where("student_id = ?", studentID)

	if date != nil {
		query = query.Where("DATE(day) = DATE(?)", *date)
	}

	err := query.Order("day DESC").
		Scan(ctx)

	if err != nil {
		return nil, err
	}

	return visits, nil
}

// GetRoomVisits retrieves all visits for a room
func (s *StudentStore) GetRoomVisits(ctx context.Context, roomID int64, date *time.Time, active bool) ([]models2.Visit, error) {
	var visits []models2.Visit

	query := s.db.NewSelect().
		Model(&visits).
		Relation("Student").
		Relation("Student.CustomUser").
		Relation("Timespan").
		Relation("CombinedGroup").
		Where("room_id = ?", roomID)

	if date != nil {
		query = query.Where("DATE(day) = DATE(?)", *date)
	}

	if active {
		query = query.Where("timespan.endtime IS NULL")
	}

	err := query.Order("day DESC").
		Scan(ctx)

	if err != nil {
		return nil, err
	}

	return visits, nil
}

// GetStudentAsList returns a student as a StudentList object
func (s *StudentStore) GetStudentAsList(ctx context.Context, id int64) (*models2.StudentList, error) {
	student, err := s.GetStudentByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if student.CustomUser == nil || student.Group == nil {
		return nil, errors.New("student data incomplete")
	}

	studentList := &models2.StudentList{
		ID:          student.ID,
		Name:        student.CustomUser.FirstName + " " + student.CustomUser.SecondName,
		SchoolClass: student.SchoolClass,
		GroupName:   student.Group.Name,
		InHouse:     student.InHouse,
	}

	return studentList, nil
}

// GetCombinedGroupVisits retrieves all visits for a combined group
func (s *StudentStore) GetCombinedGroupVisits(ctx context.Context, combinedGroupID int64, date *time.Time, active bool) ([]models2.Visit, error) {
	var visits []models2.Visit

	query := s.db.NewSelect().
		Model(&visits).
		Relation("Student").
		Relation("Student.CustomUser").
		Relation("Room").
		Relation("Timespan").
		Where("combined_group_id = ?", combinedGroupID)

	if date != nil {
		query = query.Where("DATE(day) = DATE(?)", *date)
	}

	if active {
		query = query.Where("timespan.endtime IS NULL")
	}

	err := query.Order("day DESC").
		Scan(ctx)

	if err != nil {
		return nil, err
	}

	return visits, nil
}

// CreateFeedback creates a new Feedback record for a student
func (s *StudentStore) CreateFeedback(ctx context.Context, studentID int64, feedbackValue string, mensaFeedback bool) (*models2.Feedback, error) {
	now := time.Now()
	feedback := &models2.Feedback{
		FeedbackValue: feedbackValue,
		Day:           now,
		Time:          now,
		StudentID:     studentID,
		MensaFeedback: mensaFeedback,
		CreatedAt:     now,
	}

	_, err := s.db.NewInsert().
		Model(feedback).
		Exec(ctx)

	if err != nil {
		return nil, err
	}

	return feedback, nil
}

// GetRoomOccupancyByDeviceID retrieves room occupancy details by device ID
// This implements the RoomOccupancyStore interface required by the student API
func (s *StudentStore) GetRoomOccupancyByDeviceID(ctx context.Context, deviceID string) (*models2.RoomOccupancyDetail, error) {
	// Find the room occupancy
	var roomOccupancy struct {
		RoomID     int64
		TimespanID int64
	}

	err := s.db.NewSelect().
		TableExpr("room_occupancies").
		Column("room_id", "timespan_id").
		Where("device_id = ?", deviceID).
		Scan(ctx, &roomOccupancy)

	if err != nil {
		return nil, err
	}

	// Return the found details
	return &models2.RoomOccupancyDetail{
		RoomID:     roomOccupancy.RoomID,
		TimespanID: roomOccupancy.TimespanID,
	}, nil
}
