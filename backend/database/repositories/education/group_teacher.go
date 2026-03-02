// backend/database/repositories/education/group_teacher.go
package education

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/education"
	"github.com/uptrace/bun"
)

// GroupTeacherRepository implements education.GroupTeacherRepository interface
type GroupTeacherRepository struct {
	*base.Repository[*education.GroupTeacher]
	db *bun.DB
}

// NewGroupTeacherRepository creates a new GroupTeacherRepository
func NewGroupTeacherRepository(db *bun.DB) education.GroupTeacherRepository {
	repo := base.NewRepository[*education.GroupTeacher](db, "education.group_teacher", "GroupTeacher")
	repo.TenantScoped = true
	return &GroupTeacherRepository{
		Repository: repo,
		db:         db,
	}
}

// FindByGroup retrieves all group-teacher relationships for a group
func (r *GroupTeacherRepository) FindByGroup(ctx context.Context, groupID int64) ([]*education.GroupTeacher, error) {
	var groupTeachers []*education.GroupTeacher
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&groupTeachers).
		ModelTableExpr(`education.group_teacher AS "group_teacher"`).
		Where("group_id = ?", groupID)

	if where, val, ok := base.TenantWhere(ctx, "group_teacher"); ok {
		query = query.Where(where, val)
	}

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by group",
			Err: err,
		}
	}

	return groupTeachers, nil
}

// FindByTeacher retrieves all group-teacher relationships for a teacher
func (r *GroupTeacherRepository) FindByTeacher(ctx context.Context, teacherID int64) ([]*education.GroupTeacher, error) {
	var groupTeachers []*education.GroupTeacher
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&groupTeachers).
		ModelTableExpr(`education.group_teacher AS "group_teacher"`).
		Where("teacher_id = ?", teacherID)

	if where, val, ok := base.TenantWhere(ctx, "group_teacher"); ok {
		query = query.Where(where, val)
	}

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by teacher",
			Err: err,
		}
	}

	return groupTeachers, nil
}

// FindByGroupIDs retrieves all group-teacher relationships for multiple groups in a single query
func (r *GroupTeacherRepository) FindByGroupIDs(ctx context.Context, groupIDs []int64) ([]*education.GroupTeacher, error) {
	if len(groupIDs) == 0 {
		return []*education.GroupTeacher{}, nil
	}

	var groupTeachers []*education.GroupTeacher
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&groupTeachers).
		ModelTableExpr(`education.group_teacher AS "group_teacher"`).
		Where(`"group_teacher".group_id IN (?)`, bun.List(groupIDs))

	if where, val, ok := base.TenantWhere(ctx, "group_teacher"); ok {
		query = query.Where(where, val)
	}

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by group IDs",
			Err: err,
		}
	}

	return groupTeachers, nil
}

// Create overrides the base Create method to handle validation
func (r *GroupTeacherRepository) Create(ctx context.Context, groupTeacher *education.GroupTeacher) error {
	if groupTeacher == nil {
		return fmt.Errorf("group teacher cannot be nil")
	}

	// Validate group teacher
	if err := groupTeacher.Validate(); err != nil {
		return err
	}

	// Use the base Create method
	return r.Repository.Create(ctx, groupTeacher)
}

// Update overrides the base Update method to handle validation
func (r *GroupTeacherRepository) Update(ctx context.Context, groupTeacher *education.GroupTeacher) error {
	if groupTeacher == nil {
		return fmt.Errorf("group teacher cannot be nil")
	}

	// Validate group teacher
	if err := groupTeacher.Validate(); err != nil {
		return err
	}

	// Use the base Update method
	return r.Repository.Update(ctx, groupTeacher)
}

// List retrieves group-teacher relationships matching the provided filters
func (r *GroupTeacherRepository) List(ctx context.Context, filters map[string]interface{}) ([]*education.GroupTeacher, error) {
	// Convert old filter format to new QueryOptions
	options := modelBase.NewQueryOptions()
	filter := modelBase.NewFilter()

	for field, value := range filters {
		if value != nil {
			// Default to exact match for fields
			filter.Equal(field, value)
		}
	}

	options.Filter = filter

	return r.ListWithOptions(ctx, options)
}

// ListWithOptions provides a type-safe way to list group-teacher relationships with query options
func (r *GroupTeacherRepository) ListWithOptions(ctx context.Context, options *modelBase.QueryOptions) ([]*education.GroupTeacher, error) {
	var groupTeachers []*education.GroupTeacher
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&groupTeachers).
		ModelTableExpr(`education.group_teacher AS "group_teacher"`)

	if where, val, ok := base.TenantWhere(ctx, "group_teacher"); ok {
		query = query.Where(where, val)
	}

	// Apply query options
	if options != nil {
		query = options.ApplyToQuery(query)
	}

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "list with options",
			Err: err,
		}
	}

	return groupTeachers, nil
}
