package users

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"strings"

	owner0 "github.com/moto-nrw/project-phoenix/modules/peopledirectory"

	"github.com/uptrace/bun"

	users "github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/rows"
	ports "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
)

// --- queries ------------------------------------------------------------------

func (r *GuardianRelationshipRepository) linkQuery(ctx context.Context, model any) (*bun.SelectQuery, error) {
	db, err := r.runtime.DB(ctx)
	if err != nil {
		return nil, err
	}
	return guardianLinkModelRows(ctx, r.runtime, r.links, db, model), nil
}

// FindByID retrieves one relationship with its three halves.
func (r *GuardianRelationshipRepository) FindByID(ctx context.Context, id any) (*users.StudentGuardian, error) {
	relationship := new(users.StudentGuardian)
	ambientDB1, databaseErr1 := r.linkQuery(ctx, relationship)
	if databaseErr1 != nil {
		return *new(*users.StudentGuardian), databaseErr1
	}
	query := withTenantFilter(ctx, r.runtime, ambientDB1.Where(`"student_guardian".id = ?`, id), "student_guardian")
	if err := query.Scan(ctx); err != nil {
		return nil, &ports.DatabaseError{Op: "find by id", Err: translateNotFound(err)}
	}
	return relationship, nil
}

// listFilterColumns are the columns of the old row shape List accepts.
var listFilterColumns = []string{
	"id", "student_id", "guardian_profile_id", "relationship_type", "guardian_role", "is_primary",
	"is_emergency_contact", "can_pickup", "emergency_priority", "is_payer",
}

// List retrieves the relationships matching every equality filter. A nil
// filter value is ignored, as it was on the old table.
func (r *GuardianRelationshipRepository) List(ctx context.Context, filters map[string]any) ([]*users.StudentGuardian, error) {
	var relationships []*users.StudentGuardian
	ambientDB2, databaseErr2 := r.linkQuery(ctx, &relationships)
	if databaseErr2 != nil {
		return *new([]*users.StudentGuardian), databaseErr2
	}
	query := ambientDB2
	for field, value := range filters {
		if value == nil {
			continue
		}
		if !slices.Contains(listFilterColumns, field) {
			return nil, fmt.Errorf("student guardian: unsupported list filter %q", field)
		}
		query = query.Where(`? = ?`, bun.Ident("student_guardian."+field), value)
	}
	query = withTenantFilter(ctx, r.runtime, query, "student_guardian")
	if err := query.Scan(ctx); err != nil {
		return nil, &ports.DatabaseError{Op: "list", Err: translateNotFound(err)}
	}
	return relationships, nil
}

// FindByStudentID retrieves relationships by student ID
func (r *GuardianRelationshipRepository) FindByStudentID(ctx context.Context, studentID int64) ([]*users.StudentGuardian, error) {
	var relationships []*users.StudentGuardian
	ambientDB3, databaseErr3 := r.linkQuery(ctx, &relationships)
	if databaseErr3 != nil {
		return *new([]*users.StudentGuardian), databaseErr3
	}
	query := withTenantFilter(ctx, r.runtime, ambientDB3.
		Where(`"student_guardian".student_id = ?`, studentID), "student_guardian")
	if err := query.Scan(ctx); err != nil {
		return nil, &ports.DatabaseError{Op: "find by student ID", Err: translateNotFound(err)}
	}
	return relationships, nil
}

// FindByStudentIDs retrieves relationships for many students in a single query.
func (r *GuardianRelationshipRepository) FindByStudentIDs(ctx context.Context, studentIDs []int64) ([]*users.StudentGuardian, error) {
	if len(studentIDs) == 0 {
		return []*users.StudentGuardian{}, nil
	}
	var relationships []*users.StudentGuardian
	ambientDB4, databaseErr4 := r.linkQuery(ctx, &relationships)
	if databaseErr4 != nil {
		return *new([]*users.StudentGuardian), databaseErr4
	}
	query := withTenantFilter(ctx, r.runtime, ambientDB4.
		Where(`"student_guardian".student_id IN (?)`, bun.List(studentIDs)), "student_guardian")
	if err := query.Scan(ctx); err != nil {
		return nil, &ports.DatabaseError{Op: "find by student IDs", Err: translateNotFound(err)}
	}
	return relationships, nil
}

// FindByGuardianProfileID retrieves relationships by guardian profile ID
func (r *GuardianRelationshipRepository) FindByGuardianProfileID(ctx context.Context, guardianProfileID int64) ([]*users.StudentGuardian, error) {
	var relationships []*users.StudentGuardian
	ambientDB5, databaseErr5 := r.linkQuery(ctx, &relationships)
	if databaseErr5 != nil {
		return *new([]*users.StudentGuardian), databaseErr5
	}
	query := withTenantFilter(ctx, r.runtime, ambientDB5.
		Where(`"student_guardian".guardian_profile_id = ?`, guardianProfileID), "student_guardian")
	if err := query.Scan(ctx); err != nil {
		return nil, &ports.DatabaseError{Op: "find by guardian profile ID", Err: translateNotFound(err)}
	}
	return relationships, nil
}

// FindByStudentAndGuardianForUpdate returns the relationship joining the
// student and guardian profile after locking its relationship row FOR UPDATE
// for the current transaction, or users.ErrStudentGuardianNotFound when none
// exists. Every write of a link takes the same row lock first, so a parent
// write path can re-check role/account/existence on a link that cannot change
// until this transaction commits.
func (r *GuardianRelationshipRepository) FindByStudentAndGuardianForUpdate(ctx context.Context, studentID, guardianProfileID int64) (*users.StudentGuardian, error) {
	var locked []int64
	ambientDB6, databaseErr6 := r.runtime.DB(ctx)
	if databaseErr6 != nil {
		return *new(*users.StudentGuardian), databaseErr6
	}
	lock := ambientDB6.NewSelect().
		TableExpr(`users.student_guardian_relationships AS "relationship"`).
		ColumnExpr(`"relationship".id`).
		Where(`"relationship".student_id = ?`, studentID).
		Where(`"relationship".guardian_profile_id = ?`, guardianProfileID).
		For("UPDATE")
	lock = withTenantFilter(ctx, r.runtime, lock, "relationship")
	if err := lock.Scan(ctx, &locked); err != nil {
		return nil, &ports.DatabaseError{Op: "find by student and guardian for update", Err: translateNotFound(err)}
	}
	if len(locked) == 0 {
		return nil, users.ErrStudentGuardianNotFound
	}
	relationship, err := r.FindByID(ctx, locked[0])
	if err != nil {
		if errors.Is(err, ports.ErrRepositoryNotFound) || errors.Is(err, sql.ErrNoRows) {
			return nil, users.ErrStudentGuardianNotFound
		}
		return nil, err
	}
	return relationship, nil
}

// ListLinkedChildrenForGuardians returns every child linked to any of the given
// guardian profiles in ONE join query (relationship → student → person),
// projecting only id + name. This is what lets the guardian picker enrich its
// matches with linked children without a per-guardian N+1 (#1513). Tenant
// isolation is enforced by RLS on the ambient tenant transaction plus the
// student directory's own tenant scope. Soft-deleted persons are excluded.
func (r *GuardianRelationshipRepository) ListLinkedChildrenForGuardians(ctx context.Context, guardianProfileIDs []int64) ([]*owner0.GuardianLinkedChildSnapshot, error) {
	if len(guardianProfileIDs) == 0 {
		return []*owner0.GuardianLinkedChildSnapshot{}, nil
	}
	const query = `
		SELECT sg.guardian_profile_id AS guardian_profile_id,
		       s.id                   AS student_id,
		       p.first_name           AS first_name,
		       p.last_name            AS last_name
		FROM users.student_guardian_relationships AS sg
		JOIN (?) AS s ON s.id = sg.student_id
		JOIN users.persons  AS p ON p.id = s.person_id
		WHERE sg.guardian_profile_id IN (?)
		  AND p.deleted_at IS NULL
		ORDER BY p.last_name ASC, p.first_name ASC`
	var rows []*owner0.GuardianLinkedChildSnapshot
	ambientDB7, databaseErr7 := r.runtime.DB(ctx)
	if databaseErr7 != nil {
		return *new([]*owner0.GuardianLinkedChildSnapshot), databaseErr7
	}
	db := ambientDB7
	if err := db.NewRaw(query, studentRows(ctx, r.runtime, r.students, db), bun.List(guardianProfileIDs)).Scan(ctx, &rows); err != nil {
		return nil, &ports.DatabaseError{Op: "list linked children for guardians", Err: translateNotFound(err)}
	}
	return rows, nil
}

// AccountHasStudentPermission reports whether the guardian account holds the
// named parent_portal.* permission on its relationship to the given student
// at the tenant, backed by an ACTIVE auth.account_tenants mapping. It is the
// per-child authorization probe for parent-portal actions that resolve a
// concrete student only deep inside a service — e.g. existing_students
// re-enrollment (#1663), where the school-wide submit flag cannot prove
// authority over one specific child.
//
// tenant_id is filtered explicitly (not via RLS/TenantWhere): the parent submit
// path runs under an admin transaction where RLS is bypassed, so there is no
// ambient tenant predicate. The active-mapping conjunction mirrors the parent
// picker's GuardianSubmitStatus query so a deactivated guardian's lingering
// relationships never report authority.
func (r *GuardianRelationshipRepository) AccountHasStudentPermission(ctx context.Context, accountID, studentID, tenantID int64, permission string) (bool, error) {
	if accountID <= 0 || studentID <= 0 || tenantID <= 0 {
		return false, fmt.Errorf("student guardian: account_id, student_id and tenant_id must be positive")
	}
	if strings.TrimSpace(permission) == "" {
		return false, fmt.Errorf("student guardian: permission must not be empty")
	}
	memberships, err := r.memberships(ctx, []int64{accountID}, tenantID)
	if err != nil {
		return false, err
	}
	if !slices.Contains(memberships[accountID], tenantID) {
		return false, nil
	}
	const query = `
		SELECT EXISTS (
			SELECT 1
			FROM (?) AS sg
			JOIN users.guardian_profiles AS gp
				ON gp.id = sg.guardian_profile_id
				AND gp.tenant_id = sg.tenant_id
			WHERE sg.student_id = ?
				AND gp.account_id = ?
				AND COALESCE((sg.permissions ->> ?)::boolean, false) = TRUE
		)`
	ambientDB8, databaseErr8 := r.runtime.DB(ctx)
	if databaseErr8 != nil {
		return *new(bool), databaseErr8
	}
	db := ambientDB8
	var granted bool
	if err := db.NewRaw(query, r.links.GuardianLinkRows(db, tenantID), studentID, accountID, permission).Scan(ctx, &granted); err != nil {
		return false, fmt.Errorf("student guardian: account permission for student: %w", err)
	}
	return granted, nil
}

// FilterAccountsWithStudentAccess returns the subset of guardianAccountIDs whose
// relationship to at least ONE of studentIDs still carries the named
// parent_portal.* permission at the tenant, backed by an ACTIVE
// auth.account_tenants mapping. It is the batched sibling of
// AccountHasStudentPermission, for delivery paths that hold a whole recipient
// list resolved in an earlier transaction and must ask the question again where
// the sending happens (#1671): between the producer's check and the send, a
// school can revoke a guardian's access to a child.
//
// The result preserves the caller's order and can only ever narrow the input —
// an account not passed in is never returned.
//
// tenant_id is filtered explicitly (not via RLS/TenantWhere) for the same reason
// AccountHasStudentPermission does it: the caller may run outside a tenant-scoped
// transaction.
func (r *GuardianRelationshipRepository) FilterAccountsWithStudentAccess(ctx context.Context, guardianAccountIDs, studentIDs []int64, tenantID int64, permission string) ([]int64, error) {
	if tenantID <= 0 {
		return nil, fmt.Errorf("student guardian: tenant_id must be positive")
	}
	if strings.TrimSpace(permission) == "" {
		return nil, fmt.Errorf("student guardian: permission must not be empty")
	}
	if len(guardianAccountIDs) == 0 || len(studentIDs) == 0 {
		return nil, nil
	}
	if err := validateGuardianAccessIDs(guardianAccountIDs, studentIDs); err != nil {
		return nil, err
	}

	memberships, err := r.memberships(ctx, guardianAccountIDs, tenantID)
	if err != nil {
		return nil, err
	}
	const query = `
		SELECT DISTINCT gp.account_id AS account_id
		FROM (?) AS sg
		JOIN users.guardian_profiles AS gp
			ON gp.id = sg.guardian_profile_id
			AND gp.tenant_id = sg.tenant_id
		WHERE sg.student_id  IN (?)
			AND gp.account_id  IN (?)
			AND COALESCE((sg.permissions ->> ?)::boolean, false) = TRUE`
	ambientDB9, databaseErr9 := r.runtime.DB(ctx)
	if databaseErr9 != nil {
		return *new([]int64), databaseErr9
	}
	db := ambientDB9
	var granted []int64
	if err := db.NewRaw(query,
		r.links.GuardianLinkRows(db, tenantID),
		bun.List(studentIDs),
		bun.List(guardianAccountIDs),
		permission,
	).Scan(ctx, &granted); err != nil {
		return nil, fmt.Errorf("student guardian: filter accounts with student access: %w", err)
	}
	if len(granted) == 0 {
		return nil, nil
	}

	return permittedGuardianAccounts(guardianAccountIDs, granted, memberships, tenantID), nil
}

// GuardianEmailHasStudentPermission is the accountless sibling of
// AccountHasStudentPermission: it reports whether the guardian identified by
// EMAIL holds the named parent_portal.* permission on its relationship to the
// given student at the tenant. It exists for the flows where the submitter has
// no portal account at all — a late enrollment invite is minted per guardian
// email, and the guardian it names may well have never logged in (#1663).
//
// Email is a legitimate identity key here because guardian_profiles is unique
// on (tenant_id, LOWER(email)) (migration 1.15.145), so at most one profile per
// school can answer. For late invites the request service passes the address the
// school issued the invite to, never an unverified replacement entered in the
// form. That proves the token recipient already holds re-enrollment authority
// over the student before approval applies any corrected contact data.
//
// Unlike the account variant an active auth.account_tenants mapping is not
// required — an accountless guardian has none — but a profile that DOES carry an
// account must still have an active mapping, so a deactivated guardian's
// lingering relationships never report authority.
//
// tenant_id is filtered explicitly (not via RLS/TenantWhere): the enrollment
// submit path runs under an admin transaction where RLS is bypassed, so there is
// no ambient tenant predicate.
func (r *GuardianRelationshipRepository) GuardianEmailHasStudentPermission(ctx context.Context, email string, studentID, tenantID int64, permission string) (bool, error) {
	if studentID <= 0 || tenantID <= 0 {
		return false, fmt.Errorf("student guardian: student_id and tenant_id must be positive")
	}
	normalizedEmail := strings.ToLower(strings.TrimSpace(email))
	if normalizedEmail == "" {
		return false, fmt.Errorf("student guardian: email must not be empty")
	}
	if strings.TrimSpace(permission) == "" {
		return false, fmt.Errorf("student guardian: permission must not be empty")
	}
	if r.activeMemberships == nil {
		return false, errors.New("student guardian: active membership query is required")
	}
	const query = `
			SELECT gp.account_id
			FROM (?) AS sg
			JOIN users.guardian_profiles AS gp
				ON gp.id = sg.guardian_profile_id
				AND gp.tenant_id = sg.tenant_id
			WHERE sg.student_id = ?
				AND LOWER(gp.email) = ?
				AND COALESCE((sg.permissions ->> ?)::boolean, false) = TRUE`
	ambientDB10, databaseErr10 := r.runtime.DB(ctx)
	if databaseErr10 != nil {
		return *new(bool), databaseErr10
	}
	db := ambientDB10
	var rows []struct{ AccountID *int64 }
	if err := db.NewRaw(query, r.links.GuardianLinkRows(db, tenantID), studentID, normalizedEmail, permission).Scan(ctx, &rows); err != nil {
		return false, fmt.Errorf("student guardian: email permission for student: %w", err)
	}
	accountIDs := make([]int64, 0, len(rows))
	for _, row := range rows {
		if row.AccountID == nil {
			return true, nil
		}
		accountIDs = append(accountIDs, *row.AccountID)
	}
	if len(accountIDs) == 0 {
		return false, nil
	}
	memberships, err := r.memberships(ctx, accountIDs, tenantID)
	if err != nil {
		return false, err
	}
	for _, accountID := range accountIDs {
		if slices.Contains(memberships[accountID], tenantID) {
			return true, nil
		}
	}
	return false, nil
}

// ListEmergencyContactRows returns one row per (guardian, phone number) for
// the given students, ordered so that emergency contacts, primary guardians,
// and primary/priority phone numbers come first. The caller aggregates the
// rows into display strings. Custom method (backend-conventions Rule 2):
// three-table join with NULLS LAST ordering for the emergency list.
func (r *GuardianRelationshipRepository) ListEmergencyContactRows(ctx context.Context, studentIDs []int64) ([]users.GuardianEmergencyContactRow, error) {
	if len(studentIDs) == 0 {
		return nil, nil
	}

	var rows []users.GuardianEmergencyContactRow
	ambientDB11, databaseErr11 := r.runtime.DB(ctx)
	if databaseErr11 != nil {
		return *new([]users.GuardianEmergencyContactRow), databaseErr11
	}
	query := guardianLinkRows(ctx, r.runtime, r.links, ambientDB11).
		ColumnExpr(`"student_guardian".student_id`).
		ColumnExpr(`"guardian".id AS guardian_profile_id`).
		ColumnExpr(`"guardian".first_name`).
		ColumnExpr(`"guardian".last_name`).
		ColumnExpr(`"guardian".email`).
		ColumnExpr(`"phone".phone_number`).
		ColumnExpr(`"student_guardian".relationship_type`).
		ColumnExpr(`"student_guardian".pickup_notes`).
		ColumnExpr(`"student_guardian".can_pickup`).
		ColumnExpr(`"student_guardian".is_emergency_contact`).
		Join(`JOIN users.guardian_profiles AS "guardian" ON "guardian".id = "student_guardian".guardian_profile_id AND "guardian".tenant_id = "student_guardian".tenant_id`).
		Join(`LEFT JOIN users.guardian_phone_numbers AS "phone" ON "phone".guardian_profile_id = "guardian".id AND "phone".tenant_id = "guardian".tenant_id`).
		Where(`"student_guardian".student_id IN (?)`, bun.List(studentIDs)).
		OrderExpr(`"student_guardian".is_emergency_contact DESC`).
		OrderExpr(`"student_guardian".is_primary DESC`).
		OrderExpr(`"student_guardian".emergency_priority ASC`).
		OrderExpr(`"phone".is_primary DESC NULLS LAST`).
		OrderExpr(`"phone".priority ASC NULLS LAST`).
		OrderExpr(`"student_guardian".student_id ASC`).
		OrderExpr(`"guardian".id ASC`)

	if err := query.Scan(ctx, &rows); err != nil {
		return nil, &ports.DatabaseError{
			Op:  "list emergency contact rows",
			Err: translateNotFound(err),
		}
	}
	return rows, nil
}

// ListPaymentAssignments returns one row per non-alumnus child of the tenant,
// with the guardian marked as payer joined in — LEFT, so children without an
// assigned payer appear with a nil GuardianProfileID. Showing the gaps is the
// point: a Bankverbindungen list that silently omits the children nobody
// assigned looks complete while missing exactly the rows that need work.
func (r *GuardianRelationshipRepository) ListPaymentAssignments(ctx context.Context) ([]owner0.GuardianPaymentAssignment, error) {
	var rows []owner0.GuardianPaymentAssignment
	ambientDB12, databaseErr12 := r.runtime.DB(ctx)
	if databaseErr12 != nil {
		return *new([]owner0.GuardianPaymentAssignment), databaseErr12
	}
	query := studentRows(ctx, r.runtime, r.students, ambientDB12).
		ColumnExpr(`"student".id AS student_id`).
		ColumnExpr(`"student_person".first_name AS student_first_name`).
		ColumnExpr(`"student_person".last_name AS student_last_name`).
		ColumnExpr(`"student".school_class`).
		ColumnExpr(`"guardian".id AS guardian_profile_id`).
		ColumnExpr(`COALESCE("guardian".first_name, '') AS guardian_first_name`).
		ColumnExpr(`COALESCE("guardian".last_name, '') AS guardian_last_name`).
		ColumnExpr(`COALESCE("student_guardian".relationship_type, '') AS relationship_type_raw`).
		Join(`JOIN users.persons AS "student_person" ON "student_person".id = "student".person_id`).
		Join(`LEFT JOIN users.student_guardian_relationships AS "student_guardian"
			ON "student_guardian".student_id = "student".id AND "student_guardian".tenant_id = "student".tenant_id AND "student_guardian".is_payer`).
		Join(`LEFT JOIN users.guardian_profiles AS "guardian" ON "guardian".id = "student_guardian".guardian_profile_id AND "guardian".tenant_id = "student_guardian".tenant_id`).
		Where(`"student".status != ?`, string(users.StudentStatusAlumnus)).
		Where(`"student_person".deleted_at IS NULL`).
		OrderExpr(`"student_person".last_name ASC`).
		OrderExpr(`"student_person".first_name ASC`).
		OrderExpr(`"student".id ASC`)

	if err := query.Scan(ctx, &rows); err != nil {
		return nil, &ports.DatabaseError{Op: "list guardian payment assignments", Err: translateNotFound(err)}
	}
	return rows, nil
}

func validateGuardianAccessIDs(guardianAccountIDs, studentIDs []int64) error {
	for _, accountID := range guardianAccountIDs {
		if accountID <= 0 {
			return fmt.Errorf("student guardian: account ids must be positive")
		}
	}
	for _, studentID := range studentIDs {
		if studentID <= 0 {
			return fmt.Errorf("student guardian: student ids must be positive")
		}
	}

	return nil
}

func permittedGuardianAccounts(guardianAccountIDs, granted []int64, memberships map[int64][]int64, tenantID int64) []int64 {
	grantedSet := make(map[int64]struct{}, len(granted))
	for _, accountID := range granted {
		if slices.Contains(memberships[accountID], tenantID) {
			grantedSet[accountID] = struct{}{}
		}
	}
	permitted := make([]int64, 0, len(granted))
	seen := make(map[int64]struct{}, len(granted))
	for _, accountID := range guardianAccountIDs {
		if _, ok := grantedSet[accountID]; !ok {
			continue
		}
		if _, duplicate := seen[accountID]; duplicate {
			continue
		}
		seen[accountID] = struct{}{}
		permitted = append(permitted, accountID)
	}
	return permitted
}
