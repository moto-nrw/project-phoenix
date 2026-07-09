// backend/database/repositories/education/group_teacher.go
package education

import (
	"context"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/education"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
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

// DeleteByTeacherID removes all group assignments for a teacher. Used by
// staff offboarding, where the teacher row is only soft-deleted and the old
// ON DELETE CASCADE therefore no longer cleans up assignments.
func (r *GroupTeacherRepository) DeleteByTeacherID(ctx context.Context, teacherID int64) (int64, error) {
	query := base.GetDB(ctx, r.db).NewDelete().
		Model((*education.GroupTeacher)(nil)).
		ModelTableExpr(`education.group_teacher AS "group_teacher"`).
		Where(`"group_teacher".teacher_id = ?`, teacherID)

	query = base.WithTenantFilter(ctx, query, "group_teacher")

	result, err := query.Exec(ctx)
	if err != nil {
		return 0, &modelBase.DatabaseError{
			Op:  "delete by teacher id",
			Err: err,
		}
	}
	rows, _ := result.RowsAffected()
	return rows, nil
}

// FindByGroup retrieves all group-teacher relationships for a group
func (r *GroupTeacherRepository) FindByGroup(ctx context.Context, groupID int64) ([]*education.GroupTeacher, error) {
	var groupTeachers []*education.GroupTeacher
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&groupTeachers).
		ModelTableExpr(`education.group_teacher AS "group_teacher"`).
		Where("group_id = ?", groupID)

	query = base.WithTenantFilter(ctx, query, "group_teacher")

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

	query = base.WithTenantFilter(ctx, query, "group_teacher")

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

	query = base.WithTenantFilter(ctx, query, "group_teacher")

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by group IDs",
			Err: err,
		}
	}

	return groupTeachers, nil
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

// ListGroupTeacherBlockers returns the teacher's group assignments as
// caregiver-capability blocker rows, including the full teacher set per
// group (for sole-teacher detection). Custom raw-SQL method
// (backend-conventions Rule 2): correlated ARRAY_AGG subquery into the
// users blocker read model.
func (r *GroupTeacherRepository) ListGroupTeacherBlockers(ctx context.Context, teacherID, tenantID int64) ([]userModels.BlockerGroup, error) {
	var results []userModels.BlockerGroup
	err := base.GetDB(ctx, r.db).NewRaw(`
		SELECT gt.id,
		       gt.group_id,
		       COALESCE(g.name, 'Unbekannte Gruppe') AS group_name,
		       gt.teacher_id,
		       COALESCE((
		           SELECT ARRAY_AGG(gt_all.teacher_id ORDER BY gt_all.id)
		           FROM education.group_teacher AS gt_all
		           WHERE gt_all.tenant_id = gt.tenant_id
		             AND gt_all.group_id = gt.group_id
		       ), '{}'::bigint[]) AS teacher_ids
		FROM education.group_teacher AS gt
		LEFT JOIN education.groups AS g ON g.id = gt.group_id AND g.tenant_id = gt.tenant_id
		WHERE gt.tenant_id = ?
		  AND gt.teacher_id = ?
		ORDER BY g.name ASC
	`, tenantID, teacherID).Scan(ctx, &results)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "list group teacher blockers",
			Err: err,
		}
	}
	return results, nil
}
