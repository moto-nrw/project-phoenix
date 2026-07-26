// backend/database/repositories/users/teacher.go
package users

import (
	"context"
	"database/sql"
	"errors"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// TeacherRepository implements users.TeacherRepository interface
type TeacherRepository struct {
	*base.Repository[*users.Teacher]
	db *bun.DB
}

// NewTeacherRepository creates a new TeacherRepository
func NewTeacherRepository(db *bun.DB) users.TeacherRepository {
	repo := base.NewRepository[*users.Teacher](db, "users.teachers", "Teacher")
	repo.TenantScoped = true
	return &TeacherRepository{
		Repository: repo,
		db:         db,
	}
}

// FindByStaffID retrieves a teacher by their staff ID
// Returns (nil, nil) if no teacher record exists for the given staff ID
func (r *TeacherRepository) FindByStaffID(ctx context.Context, staffID int64) (*users.Teacher, error) {
	teacher := new(users.Teacher)
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(teacher).
		ModelTableExpr(`users.teachers AS "teacher"`).
		Where(`"teacher".staff_id = ?`, staffID)

	query = base.WithTenantFilter(ctx, query, "teacher")

	err := query.Scan(ctx)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil // Staff exists but is not a teacher
		}
		return nil, &modelBase.DatabaseError{
			Op:  "find by staff ID",
			Err: err,
		}
	}

	return teacher, nil
}

// FindByStaffIDs retrieves teachers by multiple staff IDs in a single query
// Returns a map of staff_id -> Teacher for efficient lookup
func (r *TeacherRepository) FindByStaffIDs(ctx context.Context, staffIDs []int64) (map[int64]*users.Teacher, error) {
	if len(staffIDs) == 0 {
		return make(map[int64]*users.Teacher), nil
	}

	var teachers []*users.Teacher
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&teachers).
		ModelTableExpr(`users.teachers AS "teacher"`).
		Where(`"teacher".staff_id IN (?)`, bun.List(staffIDs))

	query = base.WithTenantFilter(ctx, query, "teacher")

	err := query.Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by staff IDs",
			Err: err,
		}
	}

	// Build map keyed by staff_id for O(1) lookups
	result := make(map[int64]*users.Teacher, len(teachers))
	for _, t := range teachers {
		result[t.StaffID] = t
	}

	return result, nil
}

// FindBySpecialization retrieves teachers by their specialization
func (r *TeacherRepository) FindBySpecialization(ctx context.Context, specialization string) ([]*users.Teacher, error) {
	var teachers []*users.Teacher
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&teachers).
		ModelTableExpr(`users.teachers AS "teacher"`).
		Where("LOWER(specialization) = LOWER(?)", specialization)

	query = base.WithTenantFilter(ctx, query, "teacher")

	err := query.Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by specialization",
			Err: err,
		}
	}

	return teachers, nil
}

// FindByGroupID retrieves teachers assigned to a group
func (r *TeacherRepository) FindByGroupID(ctx context.Context, groupID int64) ([]*users.Teacher, error) {
	var teachers []*users.Teacher
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&teachers).
		ModelTableExpr(`users.teachers AS "teacher"`).
		Join(`JOIN education.group_teacher gt ON gt.teacher_id = "teacher".id`).
		Where("gt.group_id = ?", groupID)

	query = base.WithTenantFilter(ctx, query, "teacher")

	err := query.Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by group ID",
			Err: err,
		}
	}

	return teachers, nil
}

// Legacy method to maintain compatibility with old interface
func (r *TeacherRepository) List(ctx context.Context, filters map[string]interface{}) ([]*users.Teacher, error) {
	options := modelBase.NewQueryOptions()
	filter := modelBase.NewFilter()

	for field, value := range filters {
		if value != nil {
			applyTeacherFilter(filter, field, value)
		}
	}

	options.Filter = filter
	return r.ListWithOptions(ctx, options)
}

// applyTeacherFilter applies a single filter based on field name
func applyTeacherFilter(filter *modelBase.Filter, field string, value interface{}) {
	switch field {
	case "specialization_like":
		applyTeacherStringLikeFilter(filter, "specialization", value)
	case "role_like":
		applyTeacherStringLikeFilter(filter, "role", value)
	case "has_qualifications":
		applyNullableFieldFilter(filter, "qualifications", value)
	default:
		filter.Equal(field, value)
	}
}

// applyTeacherStringLikeFilter applies LIKE filter for string fields
func applyTeacherStringLikeFilter(filter *modelBase.Filter, column string, value interface{}) {
	if strValue, ok := value.(string); ok {
		filter.ILike(column, "%"+strValue+"%")
	}
}

// teacherResult is the scan target for teacherWithStaffPersonQuery.
type teacherResult struct {
	Teacher *users.Teacher `bun:"teacher"`
	Staff   *users.Staff   `bun:"staff"`
	Person  *users.Person  `bun:"person"`
}

// teacherWithStaffPersonQuery builds the shared teacher→staff→person JOIN
// with the explicit ColumnExpr aliasing stanza. Callers add their WHERE
// clauses and the tenant filter. includeTenantID controls the teacher
// tenant_id projection: FindWithStaffAndPersonByIDs historically omits it
// (Teacher.TenantID serializes as json "tenant_id", so projecting it there
// would be a wire change), the other callers include it.
func (r *TeacherRepository) teacherWithStaffPersonQuery(ctx context.Context, model any, includeTenantID bool) *bun.SelectQuery {
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(model).
		ModelTableExpr(`users.teachers AS "teacher"`).
		// Teacher columns with proper aliasing
		ColumnExpr(`"teacher".id AS "teacher__id", "teacher".created_at AS "teacher__created_at", "teacher".updated_at AS "teacher__updated_at"`)
	if includeTenantID {
		query = query.ColumnExpr(`"teacher".tenant_id AS "teacher__tenant_id"`)
	}
	return query.
		ColumnExpr(`"teacher".staff_id AS "teacher__staff_id", "teacher".specialization AS "teacher__specialization"`).
		ColumnExpr(`"teacher".role AS "teacher__role", "teacher".qualifications AS "teacher__qualifications"`).
		// Staff columns
		ColumnExpr(`"staff".id AS "staff__id", "staff".created_at AS "staff__created_at", "staff".updated_at AS "staff__updated_at"`).
		ColumnExpr(`"staff".person_id AS "staff__person_id", "staff".staff_notes AS "staff__staff_notes"`).
		// Person columns
		ColumnExpr(`"person".id AS "person__id", "person".created_at AS "person__created_at", "person".updated_at AS "person__updated_at"`).
		ColumnExpr(`"person".first_name AS "person__first_name", "person".last_name AS "person__last_name"`).
		ColumnExpr(`"person".tag_id AS "person__tag_id", "person".account_id AS "person__account_id"`).
		// JOINs
		Join(`INNER JOIN users.staff AS "staff" ON "staff".id = "teacher".staff_id`).
		Join(`INNER JOIN users.persons AS "person" ON "person".id = "staff".person_id`)
}

// attachTeacherStaffPerson wires the scanned staff/person rows onto the teacher.
func attachTeacherStaffPerson(result teacherResult) *users.Teacher {
	teacher := result.Teacher
	if result.Staff != nil && result.Staff.ID != 0 {
		teacher.Staff = result.Staff
		if result.Person != nil && result.Person.ID != 0 {
			teacher.Staff.Person = result.Person
		}
	}
	return teacher
}

// mapTeacherResults converts scanned rows to Teacher objects with Staff and Person attached.
func mapTeacherResults(results []teacherResult) []*users.Teacher {
	teachers := make([]*users.Teacher, len(results))
	for i, result := range results {
		teachers[i] = attachTeacherStaffPerson(result)
	}
	return teachers
}

// FindWithStaffAndPerson retrieves a teacher with their associated staff and person data
func (r *TeacherRepository) FindWithStaffAndPerson(ctx context.Context, id int64) (*users.Teacher, error) {
	result := &teacherResult{
		Teacher: new(users.Teacher),
		Staff:   new(users.Staff),
		Person:  new(users.Person),
	}

	query := r.teacherWithStaffPersonQuery(ctx, result, true).
		Where(`"teacher".id = ?`, id)

	query = base.WithTenantFilter(ctx, query, "teacher")

	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find with staff and person",
			Err: err,
		}
	}

	return attachTeacherStaffPerson(*result), nil
}

// ListAllWithStaffAndPerson retrieves all teachers with their staff and person data in a single query
func (r *TeacherRepository) ListAllWithStaffAndPerson(ctx context.Context) ([]*users.Teacher, error) {
	var results []teacherResult

	query := r.teacherWithStaffPersonQuery(ctx, &results, true).
		Where(`"teacher".deleted_at IS NULL`).
		Where(`"staff".deleted_at IS NULL`)

	query = base.WithTenantFilter(ctx, query, "teacher")

	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "list all with staff and person",
			Err: err,
		}
	}

	return mapTeacherResults(results), nil
}

// FindWithStaffAndPersonByIDs retrieves teachers with staff and person data for multiple IDs in a single query
func (r *TeacherRepository) FindWithStaffAndPersonByIDs(ctx context.Context, ids []int64) ([]*users.Teacher, error) {
	if len(ids) == 0 {
		return []*users.Teacher{}, nil
	}

	var results []teacherResult

	query := r.teacherWithStaffPersonQuery(ctx, &results, false).
		Where(`"teacher".id IN (?)`, bun.List(ids))

	query = base.WithTenantFilter(ctx, query, "teacher")

	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find with staff and person by IDs",
			Err: err,
		}
	}

	return mapTeacherResults(results), nil
}

// activeCaregiverQuery builds the canonical operational caregiver lookup:
// teachers joined through staff/person/account to active tenant mappings and
// the system "user"/"teacher" roles. Shared by ListActiveCaregivers and
// FindActiveCaregiverByAccountID. Custom query (backend-conventions Rule 2):
// seven-table join projecting into the ActiveCaregiver read model.
func (r *TeacherRepository) activeCaregiverQuery(ctx context.Context) *bun.SelectQuery {
	query := base.GetDB(ctx, r.db).NewSelect().
		Model((*users.ActiveCaregiver)(nil)).
		ModelTableExpr(`users.teachers AS "teacher"`).
		ColumnExpr(`"account".id AS account_id`).
		ColumnExpr(`"person".id AS person_id`).
		ColumnExpr(`"staff".id AS staff_id`).
		ColumnExpr(`"teacher".id AS teacher_id`).
		ColumnExpr(`"person".first_name`).
		ColumnExpr(`"person".last_name`).
		ColumnExpr(`"account".email`).
		ColumnExpr(`"staff".created_at`).
		ColumnExpr(`"staff".updated_at`).
		Join(`INNER JOIN users.staff AS "staff" ON "staff".id = "teacher".staff_id`).
		Join(`INNER JOIN users.persons AS "person" ON "person".id = "staff".person_id`).
		Join(`INNER JOIN auth.accounts AS "account" ON "account".id = "person".account_id`).
		Join(`INNER JOIN auth.account_tenants AS "at" ON "at".account_id = "account".id AND "at".tenant_id = "teacher".tenant_id`).
		Join(`INNER JOIN auth.account_roles AS "ar" ON "ar".account_id = "account".id AND "ar".tenant_id = "teacher".tenant_id`).
		Join(`INNER JOIN auth.roles AS "role" ON "role".id = "ar".role_id`).
		Where(`"account".active = TRUE`).
		Where(`"at".status = ?`, authModels.AccountTenantStatusActive).
		Where(`"role".is_system = TRUE`).
		Where(`"role".tenant_id IS NULL`).
		Where(`LOWER("role".name) IN (?, ?)`, "user", "teacher").
		Distinct()

	if tenantID := tenant.FromContext(ctx); tenantID > 0 {
		query = query.
			Where(`"at".tenant_id = ?`, tenantID).
			Where(`"teacher".tenant_id = ?`, tenantID).
			Where(`"staff".tenant_id = ?`, tenantID).
			Where(`"person".tenant_id = ?`, tenantID).
			Where(`"ar".tenant_id = ?`, tenantID)
	}

	return query
}

// ListActiveCaregivers returns every active caregiver for the tenant in
// context, ordered by name then staff id.
func (r *TeacherRepository) ListActiveCaregivers(ctx context.Context) ([]*users.ActiveCaregiver, error) {
	var caregivers []*users.ActiveCaregiver
	query := r.activeCaregiverQuery(ctx).
		OrderExpr(`"person".first_name ASC, "person".last_name ASC, "staff".id ASC`)

	if err := query.Scan(ctx, &caregivers); err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "list active caregivers",
			Err: err,
		}
	}
	return caregivers, nil
}

// FindActiveCaregiverByAccountID returns the active caregiver bound to the
// account, or nil when the account is not an active caregiver.
func (r *TeacherRepository) FindActiveCaregiverByAccountID(ctx context.Context, accountID int64) (*users.ActiveCaregiver, error) {
	var caregivers []*users.ActiveCaregiver
	query := r.activeCaregiverQuery(ctx).
		Where(`"account".id = ?`, accountID).
		Limit(1)

	if err := query.Scan(ctx, &caregivers); err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find active caregiver by account ID",
			Err: err,
		}
	}
	if len(caregivers) == 0 {
		return nil, nil
	}
	return caregivers[0], nil
}
