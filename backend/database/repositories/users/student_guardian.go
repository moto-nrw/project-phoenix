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
			Err: err,
		}
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return false, &modelBase.DatabaseError{
			Op:  "link guardian if not exists (rows affected)",
			Err: err,
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
			Err: err,
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
			Err: err,
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
			Err: err,
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
			Err: err,
		}
	}

	return rows, nil
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
			Err: err,
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
			Err: err,
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
			Err: err,
		}
	}
	return rows, nil
}
