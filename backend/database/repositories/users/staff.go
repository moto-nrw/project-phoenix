// backend/database/repositories/users/staff.go
package users

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	authModels "github.com/moto-nrw/project-phoenix/models/auth"
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

// FindByIDForUpdate retrieves a staff row with a SELECT … FOR UPDATE lock.
// Payroll-number changes use the lock to derive their audit old_value from
// the last committed value when concurrent updates target the same staff row.
func (r *StaffRepository) FindByIDForUpdate(ctx context.Context, id int64) (*users.Staff, error) {
	staff := new(users.Staff)
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(staff).
		ModelTableExpr(`users.staff AS "staff"`).
		Where(`"staff".id = ?`, id).
		For("UPDATE")

	query = base.WithTenantFilter(ctx, query, "staff")

	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find staff by id for update",
			Err: err,
		}
	}
	return staff, nil
}

// FindByPersonID retrieves a staff member by their person ID
func (r *StaffRepository) FindByPersonID(ctx context.Context, personID int64) (*users.Staff, error) {
	staff := new(users.Staff)
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(staff).
		ModelTableExpr(`users.staff AS "staff"`).
		Where(`"staff".person_id = ?`, personID)

	query = base.WithTenantFilter(ctx, query, "staff")

	err := query.Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by person ID",
			Err: err,
		}
	}

	return staff, nil
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

	query = base.WithTenantFilter(ctx, query, "staff")

	result, err := query.Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "clear work time model",
			Err: err,
		}
	}

	return base.AssertRowsAffected(result, 1, "clear work time model")
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

// staffResult is the scan target for staffWithPersonQuery.
type staffResult struct {
	Staff  *users.Staff  `bun:"staff"`
	Person *users.Person `bun:"person"`
}

// staffWithPersonQuery builds the shared staff→person LEFT JOIN with the
// explicit ColumnExpr aliasing stanza. Callers add their WHERE clauses and
// the tenant filter. joinSuffix appends extra join conditions (e.g.
// `AND "person".deleted_at IS NULL` for FindWithPersonByIDs).
func (r *StaffRepository) staffWithPersonQuery(ctx context.Context, results *[]staffResult, joinSuffix string) *bun.SelectQuery {
	return base.GetDB(ctx, r.db).NewSelect().
		Model(results).
		ModelTableExpr(`users.staff AS "staff"`).
		ColumnExpr(`"staff".id AS "staff__id"`).
		ColumnExpr(`"staff".created_at AS "staff__created_at"`).
		ColumnExpr(`"staff".updated_at AS "staff__updated_at"`).
		ColumnExpr(`"staff".tenant_id AS "staff__tenant_id"`).
		ColumnExpr(`"staff".person_id AS "staff__person_id"`).
		ColumnExpr(`"staff".staff_notes AS "staff__staff_notes"`).
		// employment_type is part of the model and the cross-staff
		// time-tracking overview filters on it (#1417); without it the field
		// scanned back as NULL for every staff member.
		ColumnExpr(`"staff".employment_type AS "staff__employment_type"`).
		ColumnExpr(`"staff".work_time_model_id AS "staff__work_time_model_id"`).
		// personnel_number feeds the DATEV writers (#1417): without it every
		// staff member would be "skipped, no Personalnummer" in the export.
		ColumnExpr(`"staff".personnel_number AS "staff__personnel_number"`).
		ColumnExpr(`"staff".rotation_anchor_date AS "staff__rotation_anchor_date"`).
		ColumnExpr(`"person".id AS "person__id"`).
		ColumnExpr(`"person".created_at AS "person__created_at"`).
		ColumnExpr(`"person".updated_at AS "person__updated_at"`).
		ColumnExpr(`"person".first_name AS "person__first_name"`).
		ColumnExpr(`"person".last_name AS "person__last_name"`).
		ColumnExpr(`"person".tag_id AS "person__tag_id"`).
		ColumnExpr(`"person".account_id AS "person__account_id"`).
		Join(`LEFT JOIN users.persons AS "person" ON "person".id = "staff".person_id` + joinSuffix)
}

// ListAllWithPerson retrieves all staff members with their associated person data in a single query
func (r *StaffRepository) ListAllWithPerson(ctx context.Context) ([]*users.Staff, error) {
	var results []staffResult

	query := r.staffWithPersonQuery(ctx, &results, "").
		Where(`"staff".deleted_at IS NULL`)

	query = base.WithTenantFilter(ctx, query, "staff")

	if err := query.Scan(ctx); err != nil {
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

// FindReachableCalendarStaffIDs returns the subset of the given staff IDs (or
// all staff for the current tenant when ids is empty) that can actually use the
// calendar. Reachability mirrors the tenant login + calendar route gates: the
// staff's linked account must be active, have an ACTIVE auth.account_tenants
// mapping for THIS tenant (an account active only in another tenant is not
// reachable here), and hold an effective calendar:own permission for
// GET /api/calendar/my.
//
// Effective permissions are resolved exactly like auth does
// (PermissionRepository.FindByAccountIDForTenant): role-granted UNION
// directly-granted (auth.account_permissions), tenant-scoped. The final
// calendar:own decision is made in Go with authorize.HasPermission so it honors
// the same wildcard grants the route allows (calendar:*, admin:*, *:*, and
// direct grants) instead of only exact role-based rows.
// GetStaffContactInfo returns name + account email for a single staff member.
// Multi-schema join (staff → person → account) the generic repository cannot
// express; used to address absence-decision emails to the requester (#1419).
func (r *StaffRepository) GetStaffContactInfo(ctx context.Context, staffID int64) (*users.StaffWithRoleInfo, error) {
	var result users.StaffWithRoleInfo

	query := base.GetDB(ctx, r.db).NewSelect().
		ModelTableExpr(`users.staff AS "staff"`).
		ColumnExpr(`"staff".id AS staff_id`).
		ColumnExpr(`"staff".person_id`).
		ColumnExpr(`"person".first_name`).
		ColumnExpr(`"person".last_name`).
		ColumnExpr(`"account".id AS account_id`).
		ColumnExpr(`"account".email`).
		Join(`INNER JOIN users.persons AS "person" ON "person".id = "staff".person_id`).
		Join(`INNER JOIN auth.accounts AS "account" ON "account".id = "person".account_id`).
		Where(`"staff".id = ?`, staffID).
		Where(`"staff".deleted_at IS NULL`)

	query = base.WithTenantFilter(ctx, query, "staff")

	if err := query.Scan(ctx, &result); err != nil {
		return nil, &modelBase.DatabaseError{Op: "get staff contact info", Err: err}
	}
	return &result, nil
}

// ListStaffWithPermission returns all active staff whose effective permissions
// (role-granted UNION directly-granted, tenant-scoped) match permissionName —
// wildcard-aware via the same matcher route authorization uses. Used for the
// absence-request email fan-out to approvers (#1419).
func (r *StaffRepository) ListStaffWithPermission(ctx context.Context, permissionName string) ([]*users.StaffWithRoleInfo, error) {
	tenantWhere, tenantID, hasTenant := base.TenantWhere(ctx, "staff")

	db := base.GetDB(ctx, r.db)

	effective := db.NewSelect().
		ColumnExpr(`"account_role".account_id AS account_id`).
		ColumnExpr(`"role_permission".permission_id AS permission_id`).
		TableExpr(`auth.account_roles AS "account_role"`).
		Join(`JOIN auth.role_permissions AS "role_permission" ON "role_permission".role_id = "account_role".role_id`).
		Where(`"account_role".tenant_id = ?`, tenantID).
		UnionAll(
			db.NewSelect().
				ColumnExpr(`"account_permission".account_id AS account_id`).
				ColumnExpr(`"account_permission".permission_id AS permission_id`).
				TableExpr(`auth.account_permissions AS "account_permission"`).
				Where(`"account_permission".granted = ?`, true).
				Where(`"account_permission".tenant_id = ?`, tenantID),
		)

	type staffPermissionInfoRow struct {
		users.StaffWithRoleInfo
		PermissionName string `bun:"permission_name"`
	}
	var rows []staffPermissionInfoRow

	query := db.NewSelect().
		With("effective_permissions", effective).
		ColumnExpr(`DISTINCT "staff".id AS staff_id`).
		ColumnExpr(`"person".id AS person_id`).
		ColumnExpr(`"person".first_name AS first_name`).
		ColumnExpr(`"person".last_name AS last_name`).
		ColumnExpr(`"account".id AS account_id`).
		ColumnExpr(`"account".email AS email`).
		// resource:action, matching Permission.GetFullName so authorize.HasPermission parses it.
		ColumnExpr(`("permission".resource || ':' || "permission".action) AS permission_name`).
		TableExpr(`users.staff AS "staff"`).
		Join(`JOIN users.persons AS "person" ON "person".id = "staff".person_id AND "person".deleted_at IS NULL`).
		Join(`JOIN auth.accounts AS "account" ON "account".id = "person".account_id`).
		Join(`JOIN auth.account_tenants AS "account_tenant" ON "account_tenant".account_id = "account".id AND "account_tenant".tenant_id = "staff".tenant_id`).
		Join(`JOIN effective_permissions AS "effective_permission" ON "effective_permission".account_id = "account".id`).
		Join(`JOIN auth.permissions AS "permission" ON "permission".id = "effective_permission".permission_id`).
		Where(`"staff".deleted_at IS NULL`).
		Where(`"account".active = ?`, true).
		Where(`"account_tenant".status = ?`, authModels.AccountTenantStatusActive)

	if hasTenant {
		query = query.Where(tenantWhere, tenantID)
	}

	if err := query.Scan(ctx, &rows); err != nil {
		return nil, &modelBase.DatabaseError{Op: "list staff with permission", Err: err}
	}

	permissionsByStaff := make(map[int64][]string)
	infoByStaff := make(map[int64]*users.StaffWithRoleInfo)
	order := make([]int64, 0, len(rows))
	for i := range rows {
		row := &rows[i]
		if _, seen := infoByStaff[row.StaffID]; !seen {
			info := row.StaffWithRoleInfo
			infoByStaff[row.StaffID] = &info
			order = append(order, row.StaffID)
		}
		permissionsByStaff[row.StaffID] = append(permissionsByStaff[row.StaffID], row.PermissionName)
	}

	result := make([]*users.StaffWithRoleInfo, 0, len(order))
	for _, staffID := range order {
		if authorize.HasPermission(permissionName, permissionsByStaff[staffID]) {
			result = append(result, infoByStaff[staffID])
		}
	}
	return result, nil
}

func (r *StaffRepository) FindReachableCalendarStaffIDs(ctx context.Context, ids []int64) (map[int64]bool, error) {
	tenantWhere, tenantID, hasTenant := base.TenantWhere(ctx, "staff")

	db := base.GetDB(ctx, r.db)

	// Effective permission ids per account = role-granted UNION directly-granted,
	// scoped to the current tenant — the same union auth uses. The explicit
	// tenant filter also scopes it under the superuser (test) connection where
	// RLS is bypassed.
	effective := db.NewSelect().
		ColumnExpr(`"account_role".account_id AS account_id`).
		ColumnExpr(`"role_permission".permission_id AS permission_id`).
		TableExpr(`auth.account_roles AS "account_role"`).
		Join(`JOIN auth.role_permissions AS "role_permission" ON "role_permission".role_id = "account_role".role_id`).
		Where(`"account_role".tenant_id = ?`, tenantID).
		UnionAll(
			db.NewSelect().
				ColumnExpr(`"account_permission".account_id AS account_id`).
				ColumnExpr(`"account_permission".permission_id AS permission_id`).
				TableExpr(`auth.account_permissions AS "account_permission"`).
				Where(`"account_permission".granted = ?`, true).
				Where(`"account_permission".tenant_id = ?`, tenantID),
		)

	type staffPermissionRow struct {
		StaffID        int64  `bun:"staff_id"`
		PermissionName string `bun:"permission_name"`
	}
	var rows []staffPermissionRow

	query := db.NewSelect().
		With("effective_permissions", effective).
		ColumnExpr(`DISTINCT "staff".id AS staff_id`).
		// resource:action, matching Permission.GetFullName so authorize.HasPermission parses it.
		ColumnExpr(`("permission".resource || ':' || "permission".action) AS permission_name`).
		TableExpr(`users.staff AS "staff"`).
		Join(`JOIN users.persons AS "person" ON "person".id = "staff".person_id AND "person".deleted_at IS NULL`).
		Join(`JOIN auth.accounts AS "account" ON "account".id = "person".account_id`).
		Join(`JOIN auth.account_tenants AS "account_tenant" ON "account_tenant".account_id = "account".id AND "account_tenant".tenant_id = "staff".tenant_id`).
		Join(`JOIN effective_permissions AS "effective_permission" ON "effective_permission".account_id = "account".id`).
		Join(`JOIN auth.permissions AS "permission" ON "permission".id = "effective_permission".permission_id`).
		Where(`"staff".deleted_at IS NULL`).
		Where(`"account".active = ?`, true).
		Where(`"account_tenant".status = ?`, authModels.AccountTenantStatusActive)

	if len(ids) > 0 {
		query = query.Where(`"staff".id IN (?)`, bun.List(ids))
	}
	if hasTenant {
		query = query.Where(tenantWhere, tenantID)
	}

	if err := query.Scan(ctx, &rows); err != nil {
		return nil, &modelBase.DatabaseError{Op: "find reachable calendar staff", Err: err}
	}

	// Group each staff's effective permission names and apply the same
	// wildcard-aware matcher the route authorization uses.
	permissionsByStaff := make(map[int64][]string)
	for _, row := range rows {
		permissionsByStaff[row.StaffID] = append(permissionsByStaff[row.StaffID], row.PermissionName)
	}
	result := make(map[int64]bool, len(permissionsByStaff))
	for staffID, names := range permissionsByStaff {
		if authorize.HasPermission(permissions.CalendarOwn, names) {
			result[staffID] = true
		}
	}
	return result, nil
}

// FindWithPerson retrieves a staff member with their associated person data
func (r *StaffRepository) FindWithPerson(ctx context.Context, id int64) (*users.Staff, error) {
	// First get the staff member
	staff := new(users.Staff)
	staffQuery := base.GetDB(ctx, r.db).NewSelect().
		Model(staff).
		ModelTableExpr(`users.staff AS "staff"`).
		Where(`"staff".id = ?`, id)

	staffQuery = base.WithTenantFilter(ctx, staffQuery, "staff")

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

		personQuery = base.WithTenantFilter(ctx, personQuery, "person")

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

	query = base.WithTenantFilter(ctx, query, "staff")

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

	var results []staffResult

	query := r.staffWithPersonQuery(ctx, &results, ` AND "person".deleted_at IS NULL`).
		Where(`"staff".id IN (?)`, bun.List(ids))

	query = base.WithTenantFilter(ctx, query, "staff")

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

	query = base.WithTenantFilter(ctx, query, "staff")

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

	updateQuery = base.WithTenantFilter(ctx, updateQuery, "staff")

	result, err := updateQuery.Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "add notes",
			Err: err,
		}
	}

	return base.AssertRowsAffected(result, 1, "add notes")
}
