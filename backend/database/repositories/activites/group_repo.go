package activities

import (
	"context"

	"github.com/moto-nrw/project-phoenix/models/activities"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

// GroupRepository implements activities.GroupRepository
type GroupRepository struct {
	db *bun.DB
}

// NewGroupRepository creates a new group repository
func NewGroupRepository(db *bun.DB) activities.GroupRepository {
	return &GroupRepository{db: db}
}

// Create inserts a new group into the database
func (r *GroupRepository) Create(ctx context.Context, group *activities.Group) error {
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
func (r *GroupRepository) FindByID(ctx context.Context, id interface{}) (*activities.Group, error) {
	group := new(activities.Group)
	err := r.db.NewSelect().Model(group).Where("id = ?", id).Scan(ctx)
	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_id", Err: err}
	}
	return group, nil
}

// FindByName retrieves groups by name (partial match)
func (r *GroupRepository) FindByName(ctx context.Context, name string) ([]*activities.Group, error) {
	var groups []*activities.Group
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
func (r *GroupRepository) FindBySupervisor(ctx context.Context, supervisorID int64) ([]*activities.Group, error) {
	var groups []*activities.Group
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
func (r *GroupRepository) FindByCategory(ctx context.Context, categoryID int64) ([]*activities.Group, error) {
	var groups []*activities.Group
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
func (r *GroupRepository) FindByDateframe(ctx context.Context, dateframeID int64) ([]*activities.Group, error) {
	var groups []*activities.Group
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
func (r *GroupRepository) FindOpen(ctx context.Context) ([]*activities.Group, error) {
	var groups []*activities.Group
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
func (r *GroupRepository) GetEnrollmentCount(ctx context.Context, groupID int64) (int, error) {
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
func (r *GroupRepository) Update(ctx context.Context, group *activities.Group) error {
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
	_, err := r.db.NewDelete().Model((*activities.Group)(nil)).Where("id = ?", id).Exec(ctx)
	if err != nil {
		return &base.DatabaseError{Op: "delete", Err: err}
	}
	return nil
}

// List retrieves groups matching the filters
func (r *GroupRepository) List(ctx context.Context, filters map[string]interface{}) ([]*activities.Group, error) {
	var groups []*activities.Group
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
