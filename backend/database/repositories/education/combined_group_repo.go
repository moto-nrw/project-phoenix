package education

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/education"
	"github.com/uptrace/bun"
)

// CombinedGroupRepository implements education.CombinedGroupRepository
type CombinedGroupRepository struct {
	db *bun.DB
}

// NewCombinedGroupRepository creates a new combined group repository
func NewCombinedGroupRepository(db *bun.DB) education.CombinedGroupRepository {
	return &CombinedGroupRepository{db: db}
}

// Create inserts a new combined group into the database
func (r *CombinedGroupRepository) Create(ctx context.Context, cg *education.CombinedGroup) error {
	if err := cg.Validate(); err != nil {
		return err
	}

	_, err := r.db.NewInsert().Model(cg).Exec(ctx)
	if err != nil {
		return &base.DatabaseError{Op: "create", Err: err}
	}

	return nil
}

// FindByID retrieves a combined group by its ID
func (r *CombinedGroupRepository) FindByID(ctx context.Context, id interface{}) (*education.CombinedGroup, error) {
	cg := new(education.CombinedGroup)
	err := r.db.NewSelect().Model(cg).Where("id = ?", id).Scan(ctx)
	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_id", Err: err}
	}
	return cg, nil
}

// FindByName retrieves a combined group by its name
func (r *CombinedGroupRepository) FindByName(ctx context.Context, name string) (*education.CombinedGroup, error) {
	cg := new(education.CombinedGroup)
	err := r.db.NewSelect().Model(cg).Where("name = ?", name).Scan(ctx)
	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_name", Err: err}
	}
	return cg, nil
}

// FindActive retrieves all active combined groups
func (r *CombinedGroupRepository) FindActive(ctx context.Context) ([]*education.CombinedGroup, error) {
	var combinedGroups []*education.CombinedGroup
	err := r.db.NewSelect().
		Model(&combinedGroups).
		Where("is_active = ?", true).
		Where("valid_until IS NULL OR valid_until > ?", time.Now()).
		Scan(ctx)

	if err != nil {
		return nil, &base.DatabaseError{Op: "find_active", Err: err}
	}
	return combinedGroups, nil
}

// FindByAccessPolicy retrieves all combined groups with a specific access policy
func (r *CombinedGroupRepository) FindByAccessPolicy(ctx context.Context, accessPolicy string) ([]*education.CombinedGroup, error) {
	var combinedGroups []*education.CombinedGroup
	err := r.db.NewSelect().
		Model(&combinedGroups).
		Where("access_policy = ?", accessPolicy).
		Scan(ctx)

	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_access_policy", Err: err}
	}
	return combinedGroups, nil
}

// FindByGroup retrieves all combined groups that include a specific group
func (r *CombinedGroupRepository) FindByGroup(ctx context.Context, groupID int64) ([]*education.CombinedGroup, error) {
	var combinedGroups []*education.CombinedGroup
	err := r.db.NewSelect().
		Model(&combinedGroups).
		Join("JOIN education.combined_group_members cgm ON combined_groups.id = cgm.combined_group_id").
		Where("cgm.group_id = ?", groupID).
		Scan(ctx)

	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_group", Err: err}
	}
	return combinedGroups, nil
}

// FindByTeacher retrieves all combined groups associated with a specific teacher
func (r *CombinedGroupRepository) FindByTeacher(ctx context.Context, teacherID int64) ([]*education.CombinedGroup, error) {
	var combinedGroups []*education.CombinedGroup
	err := r.db.NewSelect().
		Model(&combinedGroups).
		Join("JOIN education.combined_group_teacher cgt ON combined_groups.id = cgt.combined_group_id").
		Where("cgt.teacher_id = ?", teacherID).
		Scan(ctx)

	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_teacher", Err: err}
	}
	return combinedGroups, nil
}

// UpdateStatus updates the active status of a combined group
func (r *CombinedGroupRepository) UpdateStatus(ctx context.Context, id int64, isActive bool) error {
	_, err := r.db.NewUpdate().
		Model((*education.CombinedGroup)(nil)).
		Set("is_active = ?", isActive).
		Where("id = ?", id).
		Exec(ctx)

	if err != nil {
		return &base.DatabaseError{Op: "update_status", Err: err}
	}
	return nil
}

// Update updates an existing combined group
func (r *CombinedGroupRepository) Update(ctx context.Context, cg *education.CombinedGroup) error {
	if err := cg.Validate(); err != nil {
		return err
	}

	_, err := r.db.NewUpdate().Model(cg).WherePK().Exec(ctx)
	if err != nil {
		return &base.DatabaseError{Op: "update", Err: err}
	}
	return nil
}

// Delete removes a combined group
func (r *CombinedGroupRepository) Delete(ctx context.Context, id interface{}) error {
	_, err := r.db.NewDelete().Model((*education.CombinedGroup)(nil)).Where("id = ?", id).Exec(ctx)
	if err != nil {
		return &base.DatabaseError{Op: "delete", Err: err}
	}
	return nil
}

// List retrieves combined groups matching the filters
func (r *CombinedGroupRepository) List(ctx context.Context, filters map[string]interface{}) ([]*education.CombinedGroup, error) {
	var combinedGroups []*education.CombinedGroup
	query := r.db.NewSelect().Model(&combinedGroups)

	// Apply filters
	for key, value := range filters {
		query = query.Where("? = ?", bun.Ident(key), value)
	}

	// Execute the query
	err := query.Scan(ctx)
	if err != nil {
		return nil, &base.DatabaseError{Op: "list", Err: err}
	}

	return combinedGroups, nil
}
