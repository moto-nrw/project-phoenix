package education

import (
	"context"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/education"
	"github.com/uptrace/bun"
)

// GroupRepository implements education.GroupRepository
type GroupRepository struct {
	db *bun.DB
}

// NewGroupRepository creates a new group repository
func NewGroupRepository(db *bun.DB) education.GroupRepository {
	return &GroupRepository{db: db}
}

// Create inserts a new group into the database
func (r *GroupRepository) Create(ctx context.Context, group *education.Group) error {
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
func (r *GroupRepository) FindByID(ctx context.Context, id interface{}) (*education.Group, error) {
	group := new(education.Group)
	err := r.db.NewSelect().Model(group).Where("id = ?", id).Scan(ctx)
	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_id", Err: err}
	}
	return group, nil
}

// FindByName retrieves a group by its name
func (r *GroupRepository) FindByName(ctx context.Context, name string) (*education.Group, error) {
	group := new(education.Group)
	err := r.db.NewSelect().Model(group).Where("name = ?", name).Scan(ctx)
	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_name", Err: err}
	}
	return group, nil
}

// FindByRoom retrieves all groups assigned to a specific room
func (r *GroupRepository) FindByRoom(ctx context.Context, roomID int64) ([]*education.Group, error) {
	var groups []*education.Group
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
func (r *GroupRepository) FindByTeacher(ctx context.Context, teacherID int64) ([]*education.Group, error) {
	var groups []*education.Group
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
func (r *GroupRepository) Update(ctx context.Context, group *education.Group) error {
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
func (r *GroupRepository) Delete(ctx context.Context, id interface{}) error {
	_, err := r.db.NewDelete().Model((*education.Group)(nil)).Where("id = ?", id).Exec(ctx)
	if err != nil {
		return &base.DatabaseError{Op: "delete", Err: err}
	}
	return nil
}

// List retrieves groups matching the filters
func (r *GroupRepository) List(ctx context.Context, filters map[string]interface{}) ([]*education.Group, error) {
	var groups []*education.Group
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
