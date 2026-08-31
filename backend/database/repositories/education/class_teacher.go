// backend/database/repositories/education/class_teacher.go
package education

import (
	"context"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/education"
	"github.com/uptrace/bun"
)

// ClassTeacherRepository implements education.ClassTeacherRepository
type ClassTeacherRepository struct {
	*base.Repository[*education.ClassTeacher]
	db *bun.DB
}

// NewClassTeacherRepository creates a new ClassTeacherRepository
func NewClassTeacherRepository(db *bun.DB) education.ClassTeacherRepository {
	repo := base.NewRepository[*education.ClassTeacher](db, "education.class_teachers", "ClassTeacher")
	repo.TenantScoped = true
	return &ClassTeacherRepository{
		Repository: repo,
		db:         db,
	}
}

// FindByStaff retrieves all class assignments for a staff member
func (r *ClassTeacherRepository) FindByStaff(ctx context.Context, staffID int64) ([]*education.ClassTeacher, error) {
	var classTeachers []*education.ClassTeacher
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&classTeachers).
		ModelTableExpr(`education.class_teachers AS "class_teacher"`).
		Where(`"class_teacher".staff_id = ?`, staffID).
		OrderExpr(`LOWER(BTRIM("class_teacher".school_class)) ASC`)

	query = base.WithTenantFilter(ctx, query, "class_teacher")

	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by staff",
			Err: base.TranslateNotFound(err),
		}
	}

	return classTeachers, nil
}

// DeleteByStaffID removes all class assignments for a staff member. Used by
// staff offboarding, where the staff row is only soft-deleted and the
// ON DELETE CASCADE therefore never cleans up assignments.
func (r *ClassTeacherRepository) DeleteByStaffID(ctx context.Context, staffID int64) (int64, error) {
	query := base.GetDB(ctx, r.db).NewDelete().
		Model((*education.ClassTeacher)(nil)).
		ModelTableExpr(`education.class_teachers AS "class_teacher"`).
		Where(`"class_teacher".staff_id = ?`, staffID)

	query = base.WithTenantFilter(ctx, query, "class_teacher")

	result, err := query.Exec(ctx)
	if err != nil {
		return 0, &modelBase.DatabaseError{
			Op:  "delete by staff id",
			Err: base.TranslateNotFound(err),
		}
	}
	rows, _ := result.RowsAffected()
	return rows, nil
}
