package education

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/uptrace/bun"
)

// GroupSubstitution represents a teacher substitution for a group
type GroupSubstitution struct {
	base.Model
	GroupID             int64     `bun:"group_id,notnull" json:"group_id"`
	RegularTeacherID    int64     `bun:"regular_teacher_id,notnull" json:"regular_teacher_id"`
	SubstituteTeacherID int64     `bun:"substitute_teacher_id,notnull" json:"substitute_teacher_id"`
	StartDate           time.Time `bun:"start_date,notnull" json:"start_date"`
	EndDate             time.Time `bun:"end_date,notnull" json:"end_date"`
	Reason              string    `bun:"reason" json:"reason,omitempty"`

	// Relations
	Group             *Group         `bun:"rel:belongs-to,join:group_id=id" json:"group,omitempty"`
	RegularTeacher    *users.Teacher `bun:"rel:belongs-to,join:regular_teacher_id=id" json:"regular_teacher,omitempty"`
	SubstituteTeacher *users.Teacher `bun:"rel:belongs-to,join:substitute_teacher_id=id" json:"substitute_teacher,omitempty"`
}

// TableName returns the table name for the GroupSubstitution model
func (gs *GroupSubstitution) TableName() string {
	return "education.group_substitution"
}

// GetID returns the group substitution ID
func (gs *GroupSubstitution) GetID() interface{} {
	return gs.ID
}

// GetCreatedAt returns the creation timestamp
func (gs *GroupSubstitution) GetCreatedAt() time.Time {
	return gs.CreatedAt
}

// GetUpdatedAt returns the last update timestamp
func (gs *GroupSubstitution) GetUpdatedAt() time.Time {
	return gs.UpdatedAt
}

// Validate validates the group substitution fields
func (gs *GroupSubstitution) Validate() error {
	if gs.GroupID <= 0 {
		return errors.New("group ID is required")
	}

	if gs.RegularTeacherID <= 0 {
		return errors.New("regular teacher ID is required")
	}

	if gs.SubstituteTeacherID <= 0 {
		return errors.New("substitute teacher ID is required")
	}

	if gs.StartDate.IsZero() {
		return errors.New("start date is required")
	}

	if gs.EndDate.IsZero() {
		return errors.New("end date is required")
	}

	if gs.EndDate.Before(gs.StartDate) {
		return errors.New("end date must be after start date")
	}

	// Regular teacher and substitute teacher cannot be the same
	if gs.RegularTeacherID == gs.SubstituteTeacherID {
		return errors.New("regular teacher and substitute teacher cannot be the same")
	}

	return nil
}

// IsActive checks if the substitution is currently active
func (gs *GroupSubstitution) IsActive() bool {
	now := time.Now()
	return !now.Before(gs.StartDate) && !now.After(gs.EndDate)
}

// GroupSubstitutionRepository defines operations for working with group substitutions
type GroupSubstitutionRepository interface {
	base.Repository[*GroupSubstitution]
	FindByGroup(ctx context.Context, groupID int64) ([]*GroupSubstitution, error)
	FindByRegularTeacher(ctx context.Context, teacherID int64) ([]*GroupSubstitution, error)
	FindBySubstituteTeacher(ctx context.Context, teacherID int64) ([]*GroupSubstitution, error)
	FindActive(ctx context.Context) ([]*GroupSubstitution, error)
	FindActiveByGroup(ctx context.Context, groupID int64) ([]*GroupSubstitution, error)
	FindByDateRange(ctx context.Context, startDate, endDate time.Time) ([]*GroupSubstitution, error)
}

// DefaultGroupSubstitutionRepository is the default implementation of GroupSubstitutionRepository
type DefaultGroupSubstitutionRepository struct {
	db *bun.DB
}

// NewGroupSubstitutionRepository creates a new group substitution repository
func NewGroupSubstitutionRepository(db *bun.DB) GroupSubstitutionRepository {
	return &DefaultGroupSubstitutionRepository{db: db}
}

// Create inserts a new group substitution into the database
func (r *DefaultGroupSubstitutionRepository) Create(ctx context.Context, groupSubstitution *GroupSubstitution) error {
	if err := groupSubstitution.Validate(); err != nil {
		return err
	}

	_, err := r.db.NewInsert().Model(groupSubstitution).Exec(ctx)
	if err != nil {
		return &base.DatabaseError{Op: "create", Err: err}
	}

	return nil
}

// FindByID retrieves a group substitution by its ID
func (r *DefaultGroupSubstitutionRepository) FindByID(ctx context.Context, id interface{}) (*GroupSubstitution, error) {
	groupSubstitution := new(GroupSubstitution)
	err := r.db.NewSelect().Model(groupSubstitution).Where("id = ?", id).Scan(ctx)
	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_id", Err: err}
	}
	return groupSubstitution, nil
}

// FindByGroup retrieves all substitutions for a group
func (r *DefaultGroupSubstitutionRepository) FindByGroup(ctx context.Context, groupID int64) ([]*GroupSubstitution, error) {
	var groupSubstitutions []*GroupSubstitution
	err := r.db.NewSelect().
		Model(&groupSubstitutions).
		Where("group_id = ?", groupID).
		Order("start_date ASC").
		Scan(ctx)

	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_group", Err: err}
	}
	return groupSubstitutions, nil
}

// FindByRegularTeacher retrieves all substitutions for a regular teacher
func (r *DefaultGroupSubstitutionRepository) FindByRegularTeacher(ctx context.Context, teacherID int64) ([]*GroupSubstitution, error) {
	var groupSubstitutions []*GroupSubstitution
	err := r.db.NewSelect().
		Model(&groupSubstitutions).
		Where("regular_teacher_id = ?", teacherID).
		Order("start_date ASC").
		Scan(ctx)

	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_regular_teacher", Err: err}
	}
	return groupSubstitutions, nil
}

// FindBySubstituteTeacher retrieves all substitutions for a substitute teacher
func (r *DefaultGroupSubstitutionRepository) FindBySubstituteTeacher(ctx context.Context, teacherID int64) ([]*GroupSubstitution, error) {
	var groupSubstitutions []*GroupSubstitution
	err := r.db.NewSelect().
		Model(&groupSubstitutions).
		Where("substitute_teacher_id = ?", teacherID).
		Order("start_date ASC").
		Scan(ctx)

	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_substitute_teacher", Err: err}
	}
	return groupSubstitutions, nil
}

// FindActive retrieves all currently active substitutions
func (r *DefaultGroupSubstitutionRepository) FindActive(ctx context.Context) ([]*GroupSubstitution, error) {
	now := time.Now()
	var groupSubstitutions []*GroupSubstitution
	err := r.db.NewSelect().
		Model(&groupSubstitutions).
		Where("start_date <= ?", now).
		Where("end_date >= ?", now).
		Order("start_date ASC").
		Scan(ctx)

	if err != nil {
		return nil, &base.DatabaseError{Op: "find_active", Err: err}
	}
	return groupSubstitutions, nil
}

// FindActiveByGroup retrieves all currently active substitutions for a group
func (r *DefaultGroupSubstitutionRepository) FindActiveByGroup(ctx context.Context, groupID int64) ([]*GroupSubstitution, error) {
	now := time.Now()
	var groupSubstitutions []*GroupSubstitution
	err := r.db.NewSelect().
		Model(&groupSubstitutions).
		Where("group_id = ?", groupID).
		Where("start_date <= ?", now).
		Where("end_date >= ?", now).
		Order("start_date ASC").
		Scan(ctx)

	if err != nil {
		return nil, &base.DatabaseError{Op: "find_active_by_group", Err: err}
	}
	return groupSubstitutions, nil
}

// FindByDateRange retrieves all substitutions within a date range
func (r *DefaultGroupSubstitutionRepository) FindByDateRange(ctx context.Context, startDate, endDate time.Time) ([]*GroupSubstitution, error) {
	var groupSubstitutions []*GroupSubstitution
	err := r.db.NewSelect().
		Model(&groupSubstitutions).
		Where("(start_date BETWEEN ? AND ?) OR (end_date BETWEEN ? AND ?) OR (start_date <= ? AND end_date >= ?)",
			startDate, endDate, startDate, endDate, startDate, endDate).
		Order("start_date ASC").
		Scan(ctx)

	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_date_range", Err: err}
	}
	return groupSubstitutions, nil
}

// Update updates an existing group substitution
func (r *DefaultGroupSubstitutionRepository) Update(ctx context.Context, groupSubstitution *GroupSubstitution) error {
	if err := groupSubstitution.Validate(); err != nil {
		return err
	}

	_, err := r.db.NewUpdate().Model(groupSubstitution).WherePK().Exec(ctx)
	if err != nil {
		return &base.DatabaseError{Op: "update", Err: err}
	}
	return nil
}

// Delete removes a group substitution
func (r *DefaultGroupSubstitutionRepository) Delete(ctx context.Context, id interface{}) error {
	_, err := r.db.NewDelete().Model((*GroupSubstitution)(nil)).Where("id = ?", id).Exec(ctx)
	if err != nil {
		return &base.DatabaseError{Op: "delete", Err: err}
	}
	return nil
}

// List retrieves group substitutions matching the filters
func (r *DefaultGroupSubstitutionRepository) List(ctx context.Context, filters map[string]interface{}) ([]*GroupSubstitution, error) {
	var groupSubstitutions []*GroupSubstitution
	query := r.db.NewSelect().Model(&groupSubstitutions)

	// Apply filters
	for key, value := range filters {
		query = query.Where("? = ?", bun.Ident(key), value)
	}

	// Execute the query
	err := query.Scan(ctx)
	if err != nil {
		return nil, &base.DatabaseError{Op: "list", Err: err}
	}

	return groupSubstitutions, nil
}
