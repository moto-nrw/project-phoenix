// backend/database/repositories/users/staff.go
package users

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/uptrace/bun"
)

// StaffRepository implements users.StaffRepository interface
type StaffRepository struct {
	*base.Repository[*users.Staff]
	db *bun.DB
}

// NewStaffRepository creates a new StaffRepository
func NewStaffRepository(db *bun.DB) users.StaffRepository {
	repo := base.NewRepository[*users.Staff](db, "users.staff", "Staff")
	repo.TenantScoped = true
	return &StaffRepository{
		Repository: repo,
		db:         db,
	}
}

// FindByPersonID retrieves a staff member by their person ID
func (r *StaffRepository) FindByPersonID(ctx context.Context, personID int64) (*users.Staff, error) {
	staff := new(users.Staff)
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(staff).
		ModelTableExpr(`users.staff AS "staff"`).
		Where(`"staff".person_id = ?`, personID)

	if where, val, ok := base.TenantWhere(ctx, "staff"); ok {
		query = query.Where(where, val)
	}

	err := query.Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by person ID",
			Err: err,
		}
	}

	return staff, nil
}

// UpdateNotes updates staff notes
func (r *StaffRepository) UpdateNotes(ctx context.Context, id int64, notes string) error {
	query := base.GetDB(ctx, r.db).NewUpdate().
		Model((*users.Staff)(nil)).
		ModelTableExpr(`users.staff AS "staff"`).
		Set(`staff_notes = ?`, notes).
		Where(`"staff".id = ?`, id)

	if where, val, ok := base.TenantWhere(ctx, "staff"); ok {
		query = query.Where(where, val)
	}

	result, err := query.Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "update notes",
			Err: err,
		}
	}

	return base.AssertRowsAffected(result, 1, "update notes")
}

// ClearWorkTimeModel sets work_time_model_id to NULL. Used by staff
// offboarding before the soft delete so the retained row does not block
// work-time-model deletion via the RESTRICT FK.
func (r *StaffRepository) ClearWorkTimeModel(ctx context.Context, id int64) error {
	query := base.GetDB(ctx, r.db).NewUpdate().
		Model((*users.Staff)(nil)).
		ModelTableExpr(`users.staff AS "staff"`).
		Set(`work_time_model_id = NULL`).
		Where(`"staff".id = ?`, id)

	if where, val, ok := base.TenantWhere(ctx, "staff"); ok {
		query = query.Where(where, val)
	}

	result, err := query.Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "clear work time model",
			Err: err,
		}
	}

	return base.AssertRowsAffected(result, 1, "clear work time model")
}

// Create overrides the base Create method to handle validation
func (r *StaffRepository) Create(ctx context.Context, staff *users.Staff) error {
	if staff == nil {
		return fmt.Errorf("staff cannot be nil")
	}

	// Validate staff
	if err := staff.Validate(); err != nil {
		return err
	}

	// Use the base Create method
	return r.Repository.Create(ctx, staff)
}

// Update overrides the base Update method to handle validation
func (r *StaffRepository) Update(ctx context.Context, staff *users.Staff) error {
	if staff == nil {
		return fmt.Errorf("staff cannot be nil")
	}

	// Validate staff
	if err := staff.Validate(); err != nil {
		return err
	}

	// Use the base Update method
	return r.Repository.Update(ctx, staff)
}

// Legacy method to maintain compatibility with old interface
func (r *StaffRepository) List(ctx context.Context, filters map[string]interface{}) ([]*users.Staff, error) {
	// Convert old filter format to new QueryOptions
	options := modelBase.NewQueryOptions()
	filter := modelBase.NewFilter()

	for field, value := range filters {
		if value != nil {
			filter.Equal(field, value)
		}
	}

	options.Filter = filter

	return r.ListWithOptions(ctx, options)
}

// ListWithOptions provides a type-safe way to list staff with query options
func (r *StaffRepository) ListWithOptions(ctx context.Context, options *modelBase.QueryOptions) ([]*users.Staff, error) {
	var staffMembers []*users.Staff
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&staffMembers).
		ModelTableExpr(`users.staff AS "staff"`)

	if where, val, ok := base.TenantWhere(ctx, "staff"); ok {
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

	return staffMembers, nil
}

// ListAllWithPerson retrieves all staff members with their associated person data in a single query
func (r *StaffRepository) ListAllWithPerson(ctx context.Context) ([]*users.Staff, error) {
	type staffResult struct {
		Staff  *users.Staff  `bun:"staff"`
		Person *users.Person `bun:"person"`
	}

	var results []staffResult

	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&results).
		ModelTableExpr(`users.staff AS "staff"`).
		ColumnExpr(`"staff".id AS "staff__id"`).
		ColumnExpr(`"staff".created_at AS "staff__created_at"`).
		ColumnExpr(`"staff".updated_at AS "staff__updated_at"`).
		ColumnExpr(`"staff".tenant_id AS "staff__tenant_id"`).
		ColumnExpr(`"staff".person_id AS "staff__person_id"`).
		ColumnExpr(`"staff".staff_notes AS "staff__staff_notes"`).
		ColumnExpr(`"person".id AS "person__id"`).
		ColumnExpr(`"person".created_at AS "person__created_at"`).
		ColumnExpr(`"person".updated_at AS "person__updated_at"`).
		ColumnExpr(`"person".first_name AS "person__first_name"`).
		ColumnExpr(`"person".last_name AS "person__last_name"`).
		ColumnExpr(`"person".tag_id AS "person__tag_id"`).
		ColumnExpr(`"person".account_id AS "person__account_id"`).
		Join(`LEFT JOIN users.persons AS "person" ON "person".id = "staff".person_id`).
		Where(`"staff".deleted_at IS NULL`)

	if where, val, ok := base.TenantWhere(ctx, "staff"); ok {
		query = query.Where(where, val)
	}

	err := query.Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "list all with person",
			Err: err,
		}
	}

	// Convert results to Staff objects with Person attached
	staffMembers := make([]*users.Staff, len(results))
	for i, result := range results {
		staffMembers[i] = result.Staff
		if result.Staff != nil {
			result.Staff.Person = result.Person
		}
	}

	return staffMembers, nil
}

// FindWithPerson retrieves a staff member with their associated person data
func (r *StaffRepository) FindWithPerson(ctx context.Context, id int64) (*users.Staff, error) {
	// First get the staff member
	staff := new(users.Staff)
	staffQuery := base.GetDB(ctx, r.db).NewSelect().
		Model(staff).
		ModelTableExpr(`users.staff AS "staff"`).
		Where(`"staff".id = ?`, id)

	if where, val, ok := base.TenantWhere(ctx, "staff"); ok {
		staffQuery = staffQuery.Where(where, val)
	}

	err := staffQuery.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find with person - staff",
			Err: err,
		}
	}

	// Then get the person if exists
	if staff.PersonID > 0 {
		person := new(users.Person)
		personQuery := base.GetDB(ctx, r.db).NewSelect().
			Model(person).
			ModelTableExpr(`users.persons AS "person"`).
			Where(`"person".id = ?`, staff.PersonID)

		if where, val, ok := base.TenantWhere(ctx, "person"); ok {
			personQuery = personQuery.Where(where, val)
		}

		personErr := personQuery.Scan(ctx)

		if personErr == nil {
			staff.Person = person
		} else if !errors.Is(personErr, sql.ErrNoRows) {
			// Only ignore "not found" errors - propagate all other DB errors
			return nil, &modelBase.DatabaseError{
				Op:  "find with person - load person",
				Err: personErr,
			}
		}
		// Person not found is acceptable - staff.Person remains nil
	}

	return staff, nil
}

// FindByIDs retrieves multiple staff members by their IDs in a single query.
// Returns a map of ID -> Staff for efficient lookup.
func (r *StaffRepository) FindByIDs(ctx context.Context, ids []int64) (map[int64]*users.Staff, error) {
	if len(ids) == 0 {
		return make(map[int64]*users.Staff), nil
	}

	var staffMembers []*users.Staff
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&staffMembers).
		ModelTableExpr(`users.staff AS "staff"`).
		Where(`"staff".id IN (?)`, bun.List(ids))

	if where, val, ok := base.TenantWhere(ctx, "staff"); ok {
		query = query.Where(where, val)
	}

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by IDs",
			Err: err,
		}
	}

	result := make(map[int64]*users.Staff, len(staffMembers))
	for _, s := range staffMembers {
		result[s.ID] = s
	}

	return result, nil
}

// FindWithPersonByIDs retrieves multiple staff members with their associated person data in a single query.
// Returns a map of staff ID -> Staff (with Person populated) for efficient lookup.
func (r *StaffRepository) FindWithPersonByIDs(ctx context.Context, ids []int64) (map[int64]*users.Staff, error) {
	if len(ids) == 0 {
		return make(map[int64]*users.Staff), nil
	}

	type staffResult struct {
		Staff  *users.Staff  `bun:"staff"`
		Person *users.Person `bun:"person"`
	}

	var results []staffResult

	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&results).
		ModelTableExpr(`users.staff AS "staff"`).
		ColumnExpr(`"staff".id AS "staff__id"`).
		ColumnExpr(`"staff".created_at AS "staff__created_at"`).
		ColumnExpr(`"staff".updated_at AS "staff__updated_at"`).
		ColumnExpr(`"staff".tenant_id AS "staff__tenant_id"`).
		ColumnExpr(`"staff".person_id AS "staff__person_id"`).
		ColumnExpr(`"staff".staff_notes AS "staff__staff_notes"`).
		ColumnExpr(`"person".id AS "person__id"`).
		ColumnExpr(`"person".created_at AS "person__created_at"`).
		ColumnExpr(`"person".updated_at AS "person__updated_at"`).
		ColumnExpr(`"person".first_name AS "person__first_name"`).
		ColumnExpr(`"person".last_name AS "person__last_name"`).
		ColumnExpr(`"person".tag_id AS "person__tag_id"`).
		ColumnExpr(`"person".account_id AS "person__account_id"`).
		Join(`LEFT JOIN users.persons AS "person" ON "person".id = "staff".person_id AND "person".deleted_at IS NULL`).
		Where(`"staff".id IN (?)`, bun.List(ids))

	if where, val, ok := base.TenantWhere(ctx, "staff"); ok {
		query = query.Where(where, val)
	}

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find with person by IDs",
			Err: err,
		}
	}

	result := make(map[int64]*users.Staff, len(results))
	for _, r := range results {
		if r.Staff != nil {
			r.Staff.Person = r.Person
			result[r.Staff.ID] = r.Staff
		}
	}

	return result, nil
}

// ListStaffByRoles retrieves staff members who have any of the specified roles,
// including their person data, account ID, and email, using a single JOIN query.
func (r *StaffRepository) ListStaffByRoles(ctx context.Context, roles []string) ([]*users.StaffWithRoleInfo, error) {
	// Lowercase all role names for case-insensitive matching
	lowerRoles := make([]string, len(roles))
	for i, role := range roles {
		lowerRoles[i] = strings.ToLower(role)
	}

	var results []*users.StaffWithRoleInfo

	query := base.GetDB(ctx, r.db).NewSelect().
		ModelTableExpr(`users.staff AS "staff"`).
		ColumnExpr(`"staff".id AS staff_id`).
		ColumnExpr(`"staff".created_at`).
		ColumnExpr(`"staff".updated_at`).
		ColumnExpr(`"staff".person_id`).
		ColumnExpr(`"person".first_name`).
		ColumnExpr(`"person".last_name`).
		ColumnExpr(`"account".id AS account_id`).
		ColumnExpr(`"account".email`).
		Join(`INNER JOIN users.persons AS "person" ON "person".id = "staff".person_id`).
		Join(`INNER JOIN auth.accounts AS "account" ON "account".id = "person".account_id`).
		Join(`INNER JOIN auth.account_roles AS "ar" ON "ar".account_id = "account".id`).
		Join(`INNER JOIN auth.roles AS "role" ON "ar".role_id = "role".id`).
		Where(`LOWER("role".name) IN (?)`, bun.List(lowerRoles)).
		Where(`"staff".deleted_at IS NULL`)

	if where, val, ok := base.TenantWhere(ctx, "staff"); ok {
		query = query.Where(where, val)
	}

	err := query.Scan(ctx, &results)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "list staff by roles",
			Err: err,
		}
	}

	return results, nil
}

// AddNotes adds notes to a staff member's existing notes
func (r *StaffRepository) AddNotes(ctx context.Context, id int64, notes string) error {
	// First, retrieve the current staff record to get existing notes
	staff, err := r.FindByID(ctx, id)
	if err != nil {
		return err
	}

	// Add the new notes to existing notes
	staff.AddNotes(notes)

	// Update the staff record
	updateQuery := base.GetDB(ctx, r.db).NewUpdate().
		Model(staff).
		ModelTableExpr(`users.staff AS "staff"`).
		Column(`"staff".staff_notes`).
		WherePK()

	if where, val, ok := base.TenantWhere(ctx, "staff"); ok {
		updateQuery = updateQuery.Where(where, val)
	}

	result, err := updateQuery.Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "add notes",
			Err: err,
		}
	}

	return base.AssertRowsAffected(result, 1, "add notes")
}
