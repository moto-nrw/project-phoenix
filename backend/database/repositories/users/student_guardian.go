// backend/database/repositories/users/student_guardian.go
package users

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// StudentGuardianRepository implements users.StudentGuardianRepository interface
type StudentGuardianRepository struct {
	*base.Repository[*users.StudentGuardian]
	db *bun.DB
}

// NewStudentGuardianRepository creates a new StudentGuardianRepository
func NewStudentGuardianRepository(db *bun.DB) users.StudentGuardianRepository {
	repo := base.NewRepository[*users.StudentGuardian](db, "users.students_guardians", "StudentGuardian")
	repo.TenantScoped = true
	return &StudentGuardianRepository{
		Repository: repo,
		db:         db,
	}
}

// LinkIfNotExists inserts the student↔guardian relationship, treating a
// duplicate link (same tenant_id + student_id + guardian_profile_id) as a no-op
// via ON CONFLICT DO NOTHING. It returns true when a new row was inserted and
// false when the link already existed.
//
// Why ON CONFLICT rather than a read-then-insert check: re-linking the same
// guardian must never raise a unique violation, and in PostgreSQL a violation
// raised inside the request's tenant transaction would ALSO abort that
// transaction — breaking the atomic student-create path. DO NOTHING avoids both:
// it is race-safe (two concurrent links can't both succeed-then-500) and it
// leaves the transaction usable. The conflict target matches the
// UNIQUE(tenant_id, student_id, guardian_profile_id) index
// (idx_students_guardians_tenant, migration 001014003) — NOT the legacy
// (student_id, guardian_profile_id) constraint, which that migration dropped.
//
// On conflict the existing row is left UNTOUCHED: relationship flags change only
// through the dedicated update path, never via a silent re-link upsert. Mirrors
// base.Create's validate + tenant-autoset + ModelTableExpr insert, adding only
// the conflict clause.
func (r *StudentGuardianRepository) LinkIfNotExists(ctx context.Context, rel *users.StudentGuardian) (bool, error) {
	if rel == nil {
		return false, fmt.Errorf("student guardian cannot be nil")
	}
	if strings.TrimSpace(rel.GuardianRole) == "" && len(rel.Permissions) == 0 {
		authorize.ApplyDefaultStudentGuardianRole(rel)
	}
	if err := rel.Validate(); err != nil {
		return false, err
	}
	if rel.GetTenantID() == 0 {
		if tid := tenant.FromContext(ctx); tid != 0 {
			rel.SetTenantID(tid)
		}
	}

	res, err := base.GetDB(ctx, r.db).NewInsert().
		Model(rel).
		ModelTableExpr(`users.students_guardians`).
		On(`CONFLICT (tenant_id, student_id, guardian_profile_id) DO NOTHING`).
		Exec(ctx)
	if err != nil {
		return false, &modelBase.DatabaseError{
			Op:  "link guardian if not exists",
			Err: base.TranslateNotFound(err),
		}
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return false, &modelBase.DatabaseError{
			Op:  "link guardian if not exists (rows affected)",
			Err: base.TranslateNotFound(err),
		}
	}

	return affected > 0, nil
}

// FindByStudentID retrieves relationships by student ID
func (r *StudentGuardianRepository) FindByStudentID(ctx context.Context, studentID int64) ([]*users.StudentGuardian, error) {
	var relationships []*users.StudentGuardian
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&relationships).
		ModelTableExpr(`users.students_guardians AS "student_guardian"`).
		Where(`"student_guardian".student_id = ?`, studentID)

	query = base.WithTenantFilter(ctx, query, "student_guardian")

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by student ID",
			Err: base.TranslateNotFound(err),
		}
	}

	return relationships, nil
}

// FindByStudentIDs retrieves relationships for many students in a single query.
func (r *StudentGuardianRepository) FindByStudentIDs(ctx context.Context, studentIDs []int64) ([]*users.StudentGuardian, error) {
	if len(studentIDs) == 0 {
		return []*users.StudentGuardian{}, nil
	}
	var relationships []*users.StudentGuardian
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&relationships).
		ModelTableExpr(`users.students_guardians AS "student_guardian"`).
		Where(`"student_guardian".student_id IN (?)`, bun.List(studentIDs))

	query = base.WithTenantFilter(ctx, query, "student_guardian")

	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by student IDs",
			Err: base.TranslateNotFound(err),
		}
	}

	return relationships, nil
}

// FindByGuardianProfileID retrieves relationships by guardian profile ID
func (r *StudentGuardianRepository) FindByGuardianProfileID(ctx context.Context, guardianProfileID int64) ([]*users.StudentGuardian, error) {
	var relationships []*users.StudentGuardian
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&relationships).
		ModelTableExpr(`users.students_guardians AS "student_guardian"`).
		Where(`"student_guardian".guardian_profile_id = ?`, guardianProfileID)

	query = base.WithTenantFilter(ctx, query, "student_guardian")

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by guardian profile ID",
			Err: base.TranslateNotFound(err),
		}
	}

	return relationships, nil
}

// FindByGuardianProfileIDs retrieves relationships for many guardian profiles
// in a single tenant-scoped query.
func (r *StudentGuardianRepository) FindByGuardianProfileIDs(ctx context.Context, guardianProfileIDs []int64) ([]*users.StudentGuardian, error) {
	if len(guardianProfileIDs) == 0 {
		return []*users.StudentGuardian{}, nil
	}
	var relationships []*users.StudentGuardian
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&relationships).
		ModelTableExpr(`users.students_guardians AS "student_guardian"`).
		Where(`"student_guardian".guardian_profile_id IN (?)`, bun.List(guardianProfileIDs))
	query = base.WithTenantFilter(ctx, query, "student_guardian")
	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by guardian profile IDs",
			Err: base.TranslateNotFound(err),
		}
	}
	return relationships, nil
}

// ListLinkedChildrenForGuardians returns every child linked to any of the given
// guardian profiles in ONE join query (students_guardians → students → persons),
// projecting only id + name. This is what lets the guardian picker enrich its
// matches with linked children without a per-guardian N+1 (#1513). Tenant
// isolation is enforced purely by RLS on the ambient tenant transaction — the
// query carries no explicit tenant predicate, and the incoming guardian profile
// ids are themselves already RLS-scoped (they come from SearchByText under the
// same tenant tx). Soft-deleted persons are excluded; students carry no
// soft-delete column.
func (r *StudentGuardianRepository) ListLinkedChildrenForGuardians(ctx context.Context, guardianProfileIDs []int64) ([]*users.GuardianLinkedChild, error) {
	if len(guardianProfileIDs) == 0 {
		return []*users.GuardianLinkedChild{}, nil
	}

	// Raw SQL: this three-table join (link → student → person) projecting a flat
	// custom row doesn't fit bun's model-relation inference cleanly, and the
	// repo conventions bless raw SQL for joins the generic shape can't express.
	// Tenant isolation is enforced by RLS on the ambient tenant transaction,
	// exactly like SearchByText on the guardian-profile repo.
	const query = `
		SELECT sg.guardian_profile_id AS guardian_profile_id,
		       s.id                   AS student_id,
		       p.first_name           AS first_name,
		       p.last_name            AS last_name
		FROM users.students_guardians AS sg
		JOIN users.students AS s ON s.id = sg.student_id
		JOIN users.persons  AS p ON p.id = s.person_id
		WHERE sg.guardian_profile_id IN (?)
		  AND p.deleted_at IS NULL
		ORDER BY p.last_name ASC, p.first_name ASC`

	// bun.List expands the slice to a comma-separated list ("1, 2, 3") with no
	// surrounding parens, so it slots straight into the "IN (?)" placeholder
	// above. (bun.Tuple would wrap in its own parens → "IN ((1, 2, 3))".)
	var rows []*users.GuardianLinkedChild
	if err := base.GetDB(ctx, r.db).NewRaw(query, bun.List(guardianProfileIDs)).Scan(ctx, &rows); err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "list linked children for guardians",
			Err: base.TranslateNotFound(err),
		}
	}

	return rows, nil
}

// AccountHasStudentPermission reports whether the guardian account holds the
// named parent_portal.* permission on its relationship to the given student at
// the tenant, backed by an ACTIVE auth.account_tenants mapping. It is the
// per-child authorization probe for parent-portal actions that resolve a
// concrete student only deep inside a service — e.g. existing_students
// re-enrollment (#1663), where the school-wide submit flag cannot prove
// authority over one specific child.
//
// tenant_id is filtered explicitly (not via RLS/TenantWhere): the parent submit
// path runs under an admin transaction where RLS is bypassed, so there is no
// ambient tenant predicate. The active-mapping conjunction mirrors the parent
// picker's GuardianSubmitStatus query so a deactivated guardian's lingering
// students_guardians rows never report authority.
func (r *StudentGuardianRepository) AccountHasStudentPermission(ctx context.Context, accountID, studentID, tenantID int64, permission string) (bool, error) {
	if accountID <= 0 || studentID <= 0 || tenantID <= 0 {
		return false, fmt.Errorf("student guardian: account_id, student_id and tenant_id must be positive")
	}
	if strings.TrimSpace(permission) == "" {
		return false, fmt.Errorf("student guardian: permission must not be empty")
	}
	const query = `
		SELECT EXISTS (
			SELECT 1
			FROM users.students_guardians AS sg
			JOIN users.guardian_profiles AS gp
				ON gp.id = sg.guardian_profile_id
				AND gp.tenant_id = sg.tenant_id
			JOIN auth.account_tenants AS act
				ON act.tenant_id  = gp.tenant_id
				AND act.account_id = gp.account_id
				AND act.status     = 'active'
			WHERE sg.tenant_id  = ?
				AND sg.student_id = ?
				AND gp.account_id = ?
				AND COALESCE((sg.permissions ->> ?)::boolean, false) = TRUE
		)`
	var granted bool
	if err := base.GetDB(ctx, r.db).NewRaw(query, tenantID, studentID, accountID, permission).Scan(ctx, &granted); err != nil {
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
func (r *StudentGuardianRepository) FilterAccountsWithStudentAccess(ctx context.Context, guardianAccountIDs, studentIDs []int64, tenantID int64, permission string) ([]int64, error) {
	if tenantID <= 0 {
		return nil, fmt.Errorf("student guardian: tenant_id must be positive")
	}
	if strings.TrimSpace(permission) == "" {
		return nil, fmt.Errorf("student guardian: permission must not be empty")
	}
	if len(guardianAccountIDs) == 0 || len(studentIDs) == 0 {
		return nil, nil
	}
	for _, accountID := range guardianAccountIDs {
		if accountID <= 0 {
			return nil, fmt.Errorf("student guardian: account ids must be positive")
		}
	}
	for _, studentID := range studentIDs {
		if studentID <= 0 {
			return nil, fmt.Errorf("student guardian: student ids must be positive")
		}
	}

	const query = `
		SELECT DISTINCT gp.account_id AS account_id
		FROM users.students_guardians AS sg
		JOIN users.guardian_profiles AS gp
			ON gp.id = sg.guardian_profile_id
			AND gp.tenant_id = sg.tenant_id
		JOIN auth.account_tenants AS act
			ON act.tenant_id  = gp.tenant_id
			AND act.account_id = gp.account_id
			AND act.status     = 'active'
		WHERE sg.tenant_id   = ?
			AND sg.student_id  IN (?)
			AND gp.account_id  IN (?)
			AND COALESCE((sg.permissions ->> ?)::boolean, false) = TRUE`
	var granted []int64
	if err := base.GetDB(ctx, r.db).NewRaw(query,
		tenantID,
		bun.List(studentIDs),
		bun.List(guardianAccountIDs),
		permission,
	).Scan(ctx, &granted); err != nil {
		return nil, fmt.Errorf("student guardian: filter accounts with student access: %w", err)
	}
	if len(granted) == 0 {
		return nil, nil
	}

	grantedSet := make(map[int64]struct{}, len(granted))
	for _, accountID := range granted {
		grantedSet[accountID] = struct{}{}
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
	return permitted, nil
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
// lingering relationship rows never report authority.
//
// tenant_id is filtered explicitly (not via RLS/TenantWhere): the enrollment
// submit path runs under an admin transaction where RLS is bypassed, so there is
// no ambient tenant predicate.
func (r *StudentGuardianRepository) GuardianEmailHasStudentPermission(ctx context.Context, email string, studentID, tenantID int64, permission string) (bool, error) {
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
	const query = `
		SELECT EXISTS (
			SELECT 1
			FROM users.students_guardians AS sg
			JOIN users.guardian_profiles AS gp
				ON gp.id = sg.guardian_profile_id
				AND gp.tenant_id = sg.tenant_id
			WHERE sg.tenant_id  = ?
				AND sg.student_id = ?
				AND LOWER(gp.email) = ?
				AND COALESCE((sg.permissions ->> ?)::boolean, false) = TRUE
				AND (
					gp.account_id IS NULL
					OR EXISTS (
						SELECT 1
						FROM auth.account_tenants AS act
						WHERE act.tenant_id  = gp.tenant_id
							AND act.account_id = gp.account_id
							AND act.status     = 'active'
					)
				)
		)`
	var granted bool
	if err := base.GetDB(ctx, r.db).NewRaw(query, tenantID, studentID, normalizedEmail, permission).Scan(ctx, &granted); err != nil {
		return false, fmt.Errorf("student guardian: email permission for student: %w", err)
	}
	return granted, nil
}

// FindByStudentAndGuardianForUpdate returns the relationship row joining the
// student and guardian profile, locked FOR UPDATE for the current transaction,
// or users.ErrStudentGuardianNotFound when none exists. The FOR UPDATE row lock
// serializes this read against a concurrent staff edit/delete of the same
// students_guardians row, so a parent write path can re-check role/account/
// existence on a row that cannot change until this transaction commits.
func (r *StudentGuardianRepository) FindByStudentAndGuardianForUpdate(ctx context.Context, studentID, guardianProfileID int64) (*users.StudentGuardian, error) {
	relationship := new(users.StudentGuardian)
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(relationship).
		ModelTableExpr(`users.students_guardians AS "student_guardian"`).
		Where(`"student_guardian".student_id = ?`, studentID).
		Where(`"student_guardian".guardian_profile_id = ?`, guardianProfileID).
		For("UPDATE")

	query = base.WithTenantFilter(ctx, query, "student_guardian")

	if err := query.Scan(ctx); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, users.ErrStudentGuardianNotFound
		}
		return nil, &modelBase.DatabaseError{
			Op:  "find by student and guardian for update",
			Err: base.TranslateNotFound(err),
		}
	}

	return relationship, nil
}

// SetPrimary sets a guardian as the primary guardian for a student
func (r *StudentGuardianRepository) SetPrimary(ctx context.Context, id int64, isPrimary bool) error {
	// Database has a trigger that automatically manages the primary status
	// Just update the current relationship
	query := base.GetDB(ctx, r.db).NewUpdate().
		Model((*users.StudentGuardian)(nil)).
		ModelTableExpr(`users.students_guardians AS "student_guardian"`).
		Set("is_primary = ?", isPrimary).
		Where(`"student_guardian".id = ?`, id)

	query = base.WithTenantFilter(ctx, query, "student_guardian")

	result, err := query.Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "set primary",
			Err: base.TranslateNotFound(err),
		}
	}

	return base.AssertRowsAffected(result, 1, "set primary student_guardian")
}

// Legacy method to maintain compatibility with old interface
func (r *StudentGuardianRepository) List(ctx context.Context, filters map[string]interface{}) ([]*users.StudentGuardian, error) {
	// Convert old filter format to new QueryOptions
	options := modelBase.NewQueryOptions()
	filter := modelBase.NewFilter()

	for field, value := range filters {
		if value != nil {
			switch field {
			case "is_primary":
				filter.Equal("is_primary", value)
			case "is_emergency_contact":
				filter.Equal("is_emergency_contact", value)
			case "can_pickup":
				filter.Equal("can_pickup", value)
			case "relationship_type":
				if strValue, ok := value.(string); ok {
					filter.Equal("relationship_type", strValue)
				}
			default:
				// Default to exact match for other fields
				filter.Equal(field, value)
			}
		}
	}

	options.Filter = filter

	return r.ListWithOptions(ctx, options)
}

// ListEmergencyContactRows returns one row per (guardian, phone number) for
// the given students, ordered so that emergency contacts, primary guardians,
// and primary/priority phone numbers come first. The caller aggregates the
// rows into display strings. Custom method (backend-conventions Rule 2):
// three-table join with NULLS LAST ordering for the emergency list.
func (r *StudentGuardianRepository) ListEmergencyContactRows(ctx context.Context, studentIDs []int64) ([]users.GuardianEmergencyContactRow, error) {
	if len(studentIDs) == 0 {
		return nil, nil
	}

	var rows []users.GuardianEmergencyContactRow
	query := base.GetDB(ctx, r.db).NewSelect().
		TableExpr(`users.students_guardians AS "student_guardian"`).
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
		Join(`JOIN users.guardian_profiles AS "guardian" ON "guardian".id = "student_guardian".guardian_profile_id`).
		Join(`LEFT JOIN users.guardian_phone_numbers AS "phone" ON "phone".guardian_profile_id = "guardian".id`).
		Where(`"student_guardian".student_id IN (?)`, bun.List(studentIDs)).
		OrderExpr(`"student_guardian".is_emergency_contact DESC`).
		OrderExpr(`"student_guardian".is_primary DESC`).
		OrderExpr(`"student_guardian".emergency_priority ASC`).
		OrderExpr(`"phone".is_primary DESC NULLS LAST`).
		OrderExpr(`"phone".priority ASC NULLS LAST`).
		OrderExpr(`"student_guardian".student_id ASC`).
		OrderExpr(`"guardian".id ASC`)

	if tenantID := tenant.FromContext(ctx); tenantID > 0 {
		query = query.Where(`"student_guardian".tenant_id = ?`, tenantID)
		query = query.Where(`"guardian".tenant_id = ?`, tenantID)
		query = query.Where(`("phone".tenant_id = ? OR "phone".tenant_id IS NULL)`, tenantID)
	}

	if err := query.Scan(ctx, &rows); err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "list emergency contact rows",
			Err: base.TranslateNotFound(err),
		}
	}
	return rows, nil
}

// SetPayer moves the payment mark for one child (#2608): it clears the mark
// from every guardian of the child and, when guardianProfileID is non-nil,
// sets it on that guardian's relationship. Both statements run in the caller's
// tenant transaction, which is what makes the move atomic — the partial unique
// index uq_students_guardians_payer allows only one marked row per child, so a
// set-before-clear would raise a unique violation and abort the transaction.
//
// A domain operation rather than a filter (backend rule 2): "exactly one of
// these rows carries the flag" is an invariant across rows, which no per-row
// update expresses. Returns ErrStudentGuardianNotFound when the named guardian
// is not linked to the child.
func (r *StudentGuardianRepository) SetPayer(ctx context.Context, studentID int64, guardianProfileID *int64) error {
	db := base.GetDB(ctx, r.db)

	clear := db.NewUpdate().
		Model((*users.StudentGuardian)(nil)).
		ModelTableExpr(`users.students_guardians AS "student_guardian"`).
		Set("is_payer = FALSE").
		Where(`"student_guardian".student_id = ?`, studentID).
		Where(`"student_guardian".is_payer`)
	clear = base.WithTenantFilter(ctx, clear, "student_guardian")

	if _, err := clear.Exec(ctx); err != nil {
		return &modelBase.DatabaseError{Op: "clear student payer", Err: base.TranslateNotFound(err)}
	}

	if guardianProfileID == nil {
		return nil
	}

	set := db.NewUpdate().
		Model((*users.StudentGuardian)(nil)).
		ModelTableExpr(`users.students_guardians AS "student_guardian"`).
		Set("is_payer = TRUE").
		Where(`"student_guardian".student_id = ?`, studentID).
		Where(`"student_guardian".guardian_profile_id = ?`, *guardianProfileID)
	set = base.WithTenantFilter(ctx, set, "student_guardian")

	result, err := set.Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{Op: "set student payer", Err: base.TranslateNotFound(err)}
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return &modelBase.DatabaseError{Op: "set student payer", Err: base.TranslateNotFound(err)}
	}
	if affected == 0 {
		return users.ErrStudentGuardianNotFound
	}
	return nil
}

// ListPaymentAssignments returns one row per non-alumnus child of the tenant,
// with the guardian marked as payer joined in — LEFT, so children without an
// assigned payer appear with a nil GuardianProfileID. Showing the gaps is the
// point: a Bankverbindungen list that silently omits the children nobody
// assigned looks complete while missing exactly the rows that need work.
func (r *StudentGuardianRepository) ListPaymentAssignments(ctx context.Context) ([]users.GuardianPaymentAssignment, error) {
	var rows []users.GuardianPaymentAssignment
	query := base.GetDB(ctx, r.db).NewSelect().
		TableExpr(`users.students AS "student"`).
		ColumnExpr(`"student".id AS student_id`).
		ColumnExpr(`"student_person".first_name AS student_first_name`).
		ColumnExpr(`"student_person".last_name AS student_last_name`).
		ColumnExpr(`"student".school_class`).
		ColumnExpr(`"guardian".id AS guardian_profile_id`).
		ColumnExpr(`COALESCE("guardian".first_name, '') AS guardian_first_name`).
		ColumnExpr(`COALESCE("guardian".last_name, '') AS guardian_last_name`).
		ColumnExpr(`COALESCE("student_guardian".relationship_type, '') AS relationship_type`).
		Join(`JOIN users.persons AS "student_person" ON "student_person".id = "student".person_id`).
		Join(`LEFT JOIN users.students_guardians AS "student_guardian"
			ON "student_guardian".student_id = "student".id AND "student_guardian".is_payer`).
		Join(`LEFT JOIN users.guardian_profiles AS "guardian" ON "guardian".id = "student_guardian".guardian_profile_id`).
		Where(`"student".status != ?`, string(users.StudentStatusAlumnus)).
		Where(`"student_person".deleted_at IS NULL`).
		OrderExpr(`"student_person".last_name ASC`).
		OrderExpr(`"student_person".first_name ASC`).
		OrderExpr(`"student".id ASC`)

	if tenantID := tenant.FromContext(ctx); tenantID > 0 {
		query = query.Where(`"student".tenant_id = ?`, tenantID)
		query = query.Where(`("student_guardian".tenant_id = ? OR "student_guardian".tenant_id IS NULL)`, tenantID)
		query = query.Where(`("guardian".tenant_id = ? OR "guardian".tenant_id IS NULL)`, tenantID)
	}

	if err := query.Scan(ctx, &rows); err != nil {
		return nil, &modelBase.DatabaseError{Op: "list guardian payment assignments", Err: base.TranslateNotFound(err)}
	}
	return rows, nil
}
