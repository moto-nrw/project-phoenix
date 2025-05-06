package education

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/facilities"
	"github.com/uptrace/bun"
)

// Group represents an education group in the system
type Group struct {
	base.Model
	Name   string `bun:"name,notnull,unique" json:"name"`
	RoomID *int64 `bun:"room_id" json:"room_id,omitempty"`

	// Relations
	Room                      *facilities.Room       `bun:"rel:belongs-to,join:room_id=id" json:"room,omitempty"`
	Teachers                  []*GroupTeacher        `bun:"rel:has-many,join:id=group_id" json:"teachers,omitempty"`
	CombinedGroupsMemberships []*CombinedGroupMember `bun:"rel:has-many,join:id=group_id" json:"combined_groups_memberships,omitempty"`
	Substitutions             []*GroupSubstitution   `bun:"rel:has-many,join:id=group_id" json:"substitutions,omitempty"`
}

// TableName returns the table name for the Group model
func (g *Group) TableName() string {
	return "education.groups"
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

// GroupRepository defines operations for working with education groups
type GroupRepository interface {
	base.Repository[*Group]
	FindByName(ctx context.Context, name string) (*Group, error)
	FindByRoom(ctx context.Context, roomID int64) ([]*Group, error)
	FindByTeacher(ctx context.Context, teacherID int64) ([]*Group, error)
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

// FindByName retrieves a group by its name
func (r *DefaultGroupRepository) FindByName(ctx context.Context, name string) (*Group, error) {
	group := new(Group)
	err := r.db.NewSelect().Model(group).Where("name = ?", name).Scan(ctx)
	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_name", Err: err}
	}
	return group, nil
}

// FindByRoom retrieves all groups assigned to a specific room
func (r *DefaultGroupRepository) FindByRoom(ctx context.Context, roomID int64) ([]*Group, error) {
	var groups []*Group
	err := r.db.NewSelect().
		Model(&groups).
		Where("room_id = ?", roomID).
		Scan(ctx)

	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_room", Err: err}
	}
	return groups, nil
}

// FindByTeacher retrieves all groups associated with a specific teacher
func (r *DefaultGroupRepository) FindByTeacher(ctx context.Context, teacherID int64) ([]*Group, error) {
	var groups []*Group
	err := r.db.NewSelect().
		Model(&groups).
		Join("JOIN education.group_teacher gt ON groups.id = gt.group_id").
		Where("gt.teacher_id = ?", teacherID).
		Scan(ctx)

	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_teacher", Err: err}
	}
	return groups, nil
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
