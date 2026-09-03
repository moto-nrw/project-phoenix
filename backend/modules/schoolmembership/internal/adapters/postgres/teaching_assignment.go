package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/schoolmembership/internal/domain"
	"github.com/uptrace/bun"
)

type classAssignmentRow struct {
	bun.BaseModel `bun:"table:class_teachers,alias:class_assignment"`
	ID            int64     `bun:"id,pk,autoincrement"`
	TenantID      int64     `bun:"tenant_id,notnull"`
	CreatedAt     time.Time `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt     time.Time `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
	StaffID       int64     `bun:"staff_id,notnull"`
	SchoolClass   string    `bun:"school_class,notnull"`
}

type groupAssignmentRow struct {
	bun.BaseModel `bun:"table:group_teacher,alias:group_assignment"`
	ID            int64     `bun:"id,pk,autoincrement"`
	TenantID      int64     `bun:"tenant_id,notnull"`
	CreatedAt     time.Time `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt     time.Time `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
	GroupID       int64     `bun:"group_id,notnull"`
	TeacherID     int64     `bun:"teacher_id,notnull"`
}

func (s *Store) ListClassAssignments(ctx context.Context, filter domain.ClassAssignmentFilter) ([]domain.ClassAssignment, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	rows := []classAssignmentRow{}
	query := withTenant(classAssignmentSelect(db, &rows), "class_assignment", tenantID)
	if filter.IDs != nil {
		if len(filter.IDs) == 0 {
			return []domain.ClassAssignment{}, domain.OperationStats{}, nil
		}
		query = query.Where(`"class_assignment".id IN (?)`, bun.List(filter.IDs))
	}
	if filter.StaffIDs != nil {
		if len(filter.StaffIDs) == 0 {
			return []domain.ClassAssignment{}, domain.OperationStats{}, nil
		}
		query = query.Where(`"class_assignment".staff_id IN (?)`, bun.List(filter.StaffIDs))
	}
	if filter.ClassKeys != nil {
		if len(filter.ClassKeys) == 0 {
			return []domain.ClassAssignment{}, domain.OperationStats{}, nil
		}
		query = query.Where(`LOWER(BTRIM("class_assignment".school_class)) IN (?)`, bun.List(filter.ClassKeys))
	}
	query = query.OrderExpr(`LOWER(BTRIM("class_assignment".school_class)) ASC, "class_assignment".id ASC`)
	stats, err := scanAll(ctx, query, "list class assignments")
	if err != nil {
		return nil, stats, err
	}
	result := make([]domain.ClassAssignment, 0, len(rows))
	for _, row := range rows {
		result = append(result, classAssignmentToDomain(row))
	}
	stats.Rows = int64(len(result))
	return result, stats, nil
}

func (s *Store) CreateClassAssignment(ctx context.Context, staffID int64, schoolClass string) (domain.ClassAssignment, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.ClassAssignment{}, domain.OperationStats{}, err
	}
	if tenantID <= 0 {
		return domain.ClassAssignment{}, domain.OperationStats{}, errors.New("school membership postgres: tenant is required to create a class assignment")
	}
	row := classAssignmentRow{TenantID: tenantID, StaffID: staffID, SchoolClass: schoolClass}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = db.NewInsert().Model(&row).ModelTableExpr(`education.class_teachers`).Returning("*").Scan(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return domain.ClassAssignment{}, stats, wrapClassAssignmentWriteError("create", err)
	}
	stats.Rows = 1
	return classAssignmentToDomain(row), stats, nil
}

func (s *Store) UpdateClassAssignment(ctx context.Context, id, staffID int64, schoolClass string) (domain.ClassAssignment, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.ClassAssignment{}, domain.OperationStats{}, err
	}
	row := classAssignmentRow{ID: id, StaffID: staffID, SchoolClass: schoolClass}
	query := withTenant(db.NewUpdate().Model(&row).
		ModelTableExpr(`education.class_teachers AS "class_assignment"`).
		Column("staff_id", "school_class").
		Where(`"class_assignment".id = ?`, id), "class_assignment", tenantID)
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = query.Returning("*").Scan(ctx)
	stats.StatementDuration = time.Since(started)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.ClassAssignment{}, stats, domain.ErrClassAssignmentNotFound
	}
	if err != nil {
		return domain.ClassAssignment{}, stats, wrapClassAssignmentWriteError("update", err)
	}
	stats.Rows = 1
	return classAssignmentToDomain(row), stats, nil
}

func (s *Store) DeleteClassAssignment(ctx context.Context, id int64) (domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	query := withTenant(db.NewDelete().Model((*classAssignmentRow)(nil)).
		ModelTableExpr(`education.class_teachers AS "class_assignment"`).
		Where(`"class_assignment".id = ?`, id), "class_assignment", tenantID)
	return execDelete(ctx, query, "delete class assignment")
}

func (s *Store) DeleteClassAssignmentsByStaff(ctx context.Context, staffID int64) (domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	query := withTenant(db.NewDelete().Model((*classAssignmentRow)(nil)).
		ModelTableExpr(`education.class_teachers AS "class_assignment"`).
		Where(`"class_assignment".staff_id = ?`, staffID), "class_assignment", tenantID)
	return execDelete(ctx, query, "delete class assignments by staff")
}

func (s *Store) ListGroupAssignments(ctx context.Context, filter domain.GroupAssignmentFilter) ([]domain.GroupAssignment, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	rows := []groupAssignmentRow{}
	query := withTenant(groupAssignmentSelect(db, &rows), "group_assignment", tenantID)
	if filter.IDs != nil {
		if len(filter.IDs) == 0 {
			return []domain.GroupAssignment{}, domain.OperationStats{}, nil
		}
		query = query.Where(`"group_assignment".id IN (?)`, bun.List(filter.IDs))
	}
	if filter.GroupIDs != nil {
		if len(filter.GroupIDs) == 0 {
			return []domain.GroupAssignment{}, domain.OperationStats{}, nil
		}
		query = query.Where(`"group_assignment".group_id IN (?)`, bun.List(filter.GroupIDs))
	}
	if filter.TeacherIDs != nil {
		if len(filter.TeacherIDs) == 0 {
			return []domain.GroupAssignment{}, domain.OperationStats{}, nil
		}
		query = query.Where(`"group_assignment".teacher_id IN (?)`, bun.List(filter.TeacherIDs))
	}
	if filter.TeacherStaffIDs != nil {
		if len(filter.TeacherStaffIDs) == 0 {
			return []domain.GroupAssignment{}, domain.OperationStats{}, nil
		}
		query = query.
			Join(`INNER JOIN users.teachers AS "teacher" ON "teacher".id = "group_assignment".teacher_id`).
			Where(`"teacher".staff_id IN (?)`, bun.List(filter.TeacherStaffIDs))
	}
	query = query.OrderExpr(`"group_assignment".id ASC`)
	stats, err := scanAll(ctx, query, "list group assignments")
	if err != nil {
		return nil, stats, err
	}
	result := make([]domain.GroupAssignment, 0, len(rows))
	for _, row := range rows {
		result = append(result, groupAssignmentToDomain(row))
	}
	stats.Rows = int64(len(result))
	return result, stats, nil
}

func (s *Store) CreateGroupAssignment(ctx context.Context, groupID, teacherID int64) (domain.GroupAssignment, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.GroupAssignment{}, domain.OperationStats{}, err
	}
	if tenantID <= 0 {
		return domain.GroupAssignment{}, domain.OperationStats{}, errors.New("school membership postgres: tenant is required to create a group assignment")
	}
	row := groupAssignmentRow{TenantID: tenantID, GroupID: groupID, TeacherID: teacherID}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = db.NewInsert().Model(&row).ModelTableExpr(`education.group_teacher`).Returning("*").Scan(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return domain.GroupAssignment{}, stats, wrapGroupAssignmentWriteError("create", err)
	}
	stats.Rows = 1
	return groupAssignmentToDomain(row), stats, nil
}

func (s *Store) UpdateGroupAssignment(ctx context.Context, id, groupID, teacherID int64) (domain.GroupAssignment, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.GroupAssignment{}, domain.OperationStats{}, err
	}
	row := groupAssignmentRow{ID: id, GroupID: groupID, TeacherID: teacherID}
	query := withTenant(db.NewUpdate().Model(&row).
		ModelTableExpr(`education.group_teacher AS "group_assignment"`).
		Column("group_id", "teacher_id").
		Where(`"group_assignment".id = ?`, id), "group_assignment", tenantID)
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = query.Returning("*").Scan(ctx)
	stats.StatementDuration = time.Since(started)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.GroupAssignment{}, stats, domain.ErrGroupAssignmentNotFound
	}
	if err != nil {
		return domain.GroupAssignment{}, stats, wrapGroupAssignmentWriteError("update", err)
	}
	stats.Rows = 1
	return groupAssignmentToDomain(row), stats, nil
}

func (s *Store) DeleteGroupAssignment(ctx context.Context, id int64) (domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	query := withTenant(db.NewDelete().Model((*groupAssignmentRow)(nil)).
		ModelTableExpr(`education.group_teacher AS "group_assignment"`).
		Where(`"group_assignment".id = ?`, id), "group_assignment", tenantID)
	return execDelete(ctx, query, "delete group assignment")
}

func (s *Store) DeleteGroupAssignmentsByTeacher(ctx context.Context, teacherID int64) (domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	query := withTenant(db.NewDelete().Model((*groupAssignmentRow)(nil)).
		ModelTableExpr(`education.group_teacher AS "group_assignment"`).
		Where(`"group_assignment".teacher_id = ?`, teacherID), "group_assignment", tenantID)
	return execDelete(ctx, query, "delete group assignments by teacher")
}

func classAssignmentSelect(db bun.IDB, model any) *bun.SelectQuery {
	return db.NewSelect().Model(model).ModelTableExpr(`education.class_teachers AS "class_assignment"`)
}

func groupAssignmentSelect(db bun.IDB, model any) *bun.SelectQuery {
	return db.NewSelect().Model(model).ModelTableExpr(`education.group_teacher AS "group_assignment"`)
}

func classAssignmentToDomain(row classAssignmentRow) domain.ClassAssignment {
	return domain.ClassAssignment{
		ID: row.ID, TenantID: row.TenantID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		StaffID: row.StaffID, SchoolClass: row.SchoolClass,
	}
}

func groupAssignmentToDomain(row groupAssignmentRow) domain.GroupAssignment {
	return domain.GroupAssignment{
		ID: row.ID, TenantID: row.TenantID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		GroupID: row.GroupID, TeacherID: row.TeacherID,
	}
}

func execDelete(ctx context.Context, query *bun.DeleteQuery, operation string) (domain.OperationStats, error) {
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	result, err := query.Exec(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return stats, fmt.Errorf("school membership postgres: %s: %w", operation, err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return stats, fmt.Errorf("school membership postgres: %s: count rows: %w", operation, err)
	}
	stats.Rows = rows
	return stats, nil
}

func wrapClassAssignmentWriteError(operation string, err error) error {
	if isUniqueViolationOn(err, "uniq_class_teachers_staff_class") {
		return fmt.Errorf("%w: %w", domain.ErrClassAssignmentConflict, err)
	}
	return fmt.Errorf("school membership postgres: %s class assignment: %w", operation, err)
}

func wrapGroupAssignmentWriteError(operation string, err error) error {
	if isUniqueViolationOn(err, "idx_group_teacher_tenant", "uk_group_teacher") {
		return fmt.Errorf("%w: %w", domain.ErrGroupAssignmentConflict, err)
	}
	return fmt.Errorf("school membership postgres: %s group assignment: %w", operation, err)
}
