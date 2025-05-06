package activities

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/uptrace/bun"
)

// Group represents an activity group
type Group struct {
	base.Model
	Name            string `bun:"name,notnull" json:"name"`
	MaxParticipants int    `bun:"max_participants,notnull" json:"max_participants"`
	IsOpen          bool   `bun:"is_open,notnull,default:false" json:"is_open"`
	SupervisorID    int64  `bun:"supervisor_id,notnull" json:"supervisor_id"`
	CategoryID      int64  `bun:"category_id,notnull" json:"category_id"`
	DateframeID     *int64 `bun:"dateframe_id" json:"dateframe_id,omitempty"`

	// Relations
	Supervisor         *users.Teacher       `bun:"rel:belongs-to,join:supervisor_id=id" json:"supervisor,omitempty"`
	Category           *Category            `bun:"rel:belongs-to,join:category_id=id" json:"category,omitempty"`
	Dateframe          *schedule.Dateframe  `bun:"rel:belongs-to,join:dateframe_id=id" json:"dateframe,omitempty"`
	Schedules          []*Schedule          `bun:"rel:has-many,join:id=activity_group_id" json:"schedules,omitempty"`
	StudentEnrollments []*StudentEnrollment `bun:"rel:has-many,join:id=activity_group_id" json:"student_enrollments,omitempty"`
}

// TableName returns the table name for the Group model
func (g *Group) TableName() string {
	return "activities.groups"
}

// GetID returns the group ID
func (g *Group) GetID() interface{} {
	return g.ID
}

// GetCreatedAt returns the creation timestamp
func (g *Group) GetCreatedAt() time.Time {
	return g.CreatedAt
}

// GetUpdatedAt returns the last update timestamp
func (g *Group) GetUpdatedAt() time.Time {
	return g.UpdatedAt
}

// Validate validates the group fields
func (g *Group) Validate() error {
	if strings.TrimSpace(g.Name) == "" {
		return errors.New("group name is required")
	}

	if g.MaxParticipants <= 0 {
		return errors.New("max participants must be greater than zero")
	}

	if g.SupervisorID <= 0 {
		return errors.New("supervisor ID is required")
	}

	if g.CategoryID <= 0 {
		return errors.New("category ID is required")
	}

	return nil
}

// BeforeAppend sets default values before saving to the database
func (g *Group) BeforeAppend() error {
	// Call parent's BeforeAppend to set timestamps
	if err := g.Model.BeforeAppend(); err != nil {
		return err
	}

	// Trim whitespace
	g.Name = strings.TrimSpace(g.Name)

	return nil
}

// HasAvailableSpots checks if the group has available spots for enrollment
func (g *Group) HasAvailableSpots(enrolledCount int) bool {
	return g.MaxParticipants > enrolledCount
}

// GroupRepository defines operations for working with activity groups
type GroupRepository interface {
	base.Repository[*Group]
	FindByName(ctx context.Context, name string) ([]*Group, error)
	FindBySupervisor(ctx context.Context, supervisorID int64) ([]*Group, error)
	FindByCategory(ctx context.Context, categoryID int64) ([]*Group, error)
	FindByDateframe(ctx context.Context, dateframeID int64) ([]*Group, error)
	FindOpen(ctx context.Context) ([]*Group, error)
	GetEnrollmentCount(ctx context.Context, groupID int64) (int, error)
}

// DefaultGroupRepository is the default implementation of GroupRepository
type DefaultGroupRepository struct {
	db *bun.DB
}

// NewGroupRepository creates a new group repository
func NewGroupRepository(db *bun.DB) GroupRepository {
	return &DefaultGroupRepository{db: db}
}

// Create inserts a new group into the database
func (r *DefaultGroupRepository) Create(ctx context.Context, group *Group) error {
	if err := group.Validate(); err != nil {
		return err
	}

	_, err := r.db.NewInsert().Model(group).Exec(ctx)
	if err != nil {
		return &base.DatabaseError{Op: "create", Err: err}
	}

	return nil
}

// FindByID retrieves a group by its ID
func (r *DefaultGroupRepository) FindByID(ctx context.Context, id interface{}) (*Group, error) {
	group := new(Group)
	err := r.db.NewSelect().Model(group).Where("id = ?", id).Scan(ctx)
	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_id", Err: err}
	}
	return group, nil
}

// FindByName retrieves groups by name (partial match)
func (r *DefaultGroupRepository) FindByName(ctx context.Context, name string) ([]*Group, error) {
	var groups []*Group
	err := r.db.NewSelect().
		Model(&groups).
		Where("name ILIKE ?", "%"+name+"%").
		Order("name ASC").
		Scan(ctx)

	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_name", Err: err}
	}
	return groups, nil
}

// FindBySupervisor retrieves groups by supervisor
func (r *DefaultGroupRepository) FindBySupervisor(ctx context.Context, supervisorID int64) ([]*Group, error) {
	var groups []*Group
	err := r.db.NewSelect().
		Model(&groups).
		Where("supervisor_id = ?", supervisorID).
		Order("name ASC").
		Scan(ctx)

	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_supervisor", Err: err}
	}
	return groups, nil
}

// FindByCategory retrieves groups by category
func (r *DefaultGroupRepository) FindByCategory(ctx context.Context, categoryID int64) ([]*Group, error) {
	var groups []*Group
	err := r.db.NewSelect().
		Model(&groups).
		Where("category_id = ?", categoryID).
		Order("name ASC").
		Scan(ctx)

	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_category", Err: err}
	}
	return groups, nil
}

// FindByDateframe retrieves groups by dateframe
func (r *DefaultGroupRepository) FindByDateframe(ctx context.Context, dateframeID int64) ([]*Group, error) {
	var groups []*Group
	err := r.db.NewSelect().
		Model(&groups).
		Where("dateframe_id = ?", dateframeID).
		Order("name ASC").
		Scan(ctx)

	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_dateframe", Err: err}
	}
	return groups, nil
}

// FindOpen retrieves all open groups
func (r *DefaultGroupRepository) FindOpen(ctx context.Context) ([]*Group, error) {
	var groups []*Group
	err := r.db.NewSelect().
		Model(&groups).
		Where("is_open = ?", true).
		Order("name ASC").
		Scan(ctx)

	if err != nil {
		return nil, &base.DatabaseError{Op: "find_open", Err: err}
	}
	return groups, nil
}

// GetEnrollmentCount retrieves the current number of enrollments for a group
func (r *DefaultGroupRepository) GetEnrollmentCount(ctx context.Context, groupID int64) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM activities.student_enrollments
		WHERE activity_group_id = ?
	`, groupID).Scan(&count)

	if err != nil {
		return 0, &base.DatabaseError{Op: "get_enrollment_count", Err: err}
	}
	return count, nil
}

// Update updates an existing group
func (r *DefaultGroupRepository) Update(ctx context.Context, group *Group) error {
	if err := group.Validate(); err != nil {
		return err
	}

	_, err := r.db.NewUpdate().Model(group).WherePK().Exec(ctx)
	if err != nil {
		return &base.DatabaseError{Op: "update", Err: err}
	}
	return nil
}

// Delete removes a group
func (r *DefaultGroupRepository) Delete(ctx context.Context, id interface{}) error {
	_, err := r.db.NewDelete().Model((*Group)(nil)).Where("id = ?", id).Exec(ctx)
	if err != nil {
		return &base.DatabaseError{Op: "delete", Err: err}
	}
	return nil
}

// List retrieves groups matching the filters
func (r *DefaultGroupRepository) List(ctx context.Context, filters map[string]interface{}) ([]*Group, error) {
	var groups []*Group
	query := r.db.NewSelect().Model(&groups)

	// Apply filters
	for key, value := range filters {
		query = query.Where("? = ?", bun.Ident(key), value)
	}

	// Execute the query
	err := query.Scan(ctx)
	if err != nil {
		return nil, &base.DatabaseError{Op: "list", Err: err}
	}

	return groups, nil
}
