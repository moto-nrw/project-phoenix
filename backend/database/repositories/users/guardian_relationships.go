package users

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/guardianlinkview"
	"github.com/moto-nrw/project-phoenix/modules/studentdirectoryview"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// The student-guardian relationship after Cutover #2756. People Directory
// owns users.student_guardian_relationships (type, role, primary, emergency
// contact and priority, payer) and writes it here. The pickup permission is
// Care Plan's and the parents-portal access Identity & Access's; this store
// reaches both through the consumer-owned ports below, inside one unit of
// work that holds the relationship row lock, so a link is always written as
// one row of three halves or not at all. Reads that need the whole row go
// through the tenant-safe guardian-link projection.

// GuardianPickupPermissionCommand is the Care Plan half of a relationship.
// Every command names its school and joins the caller's transaction.
type GuardianPickupPermissionCommand interface {
	CreateGuardianPickupPermission(ctx context.Context, tenantID, relationshipID int64, canPickup bool, notes *string) error
	// ChangeGuardianPickupPermission writes canPickup when it is non-nil and
	// notes when setNotes is true, and reports whether the permission exists.
	ChangeGuardianPickupPermission(ctx context.Context, tenantID, relationshipID int64, canPickup *bool, setNotes bool, notes *string) (bool, error)
}

// GuardianStudentAccessCommand is the Identity & Access half of a
// relationship: the guardian account binding and the parents-portal
// permissions.
type GuardianStudentAccessCommand interface {
	GrantGuardianStudentAccess(ctx context.Context, tenantID, relationshipID int64, accountID *int64, permissions json.RawMessage) error
	SetGuardianStudentPermissions(ctx context.Context, tenantID, relationshipID int64, permissions json.RawMessage) (bool, error)
}

// GuardianRelationshipRepository implements users.StudentGuardianRepository
// over the three owners.
type GuardianRelationshipRepository struct {
	db *bun.DB
	// activeMemberships supplies bounded account-school facts for permission checks.
	activeMemberships SchoolMembershipLookup
	pickup            GuardianPickupPermissionCommand
	access            GuardianStudentAccessCommand
}

// GuardianRelationshipOption configures a GuardianRelationshipRepository at
// construction.
type GuardianRelationshipOption func(*GuardianRelationshipRepository)

// WithGuardianRelationshipMemberships installs the Identity & Access bounded
// membership lookup the permission checks filter through (#2721).
func WithGuardianRelationshipMemberships(query SchoolMembershipLookup) GuardianRelationshipOption {
	return func(r *GuardianRelationshipRepository) { r.activeMemberships = query }
}

// WithGuardianRelationshipOwners installs the Care Plan and Identity & Access
// commands the relationship's unit of work writes the other two halves with.
func WithGuardianRelationshipOwners(pickup GuardianPickupPermissionCommand, access GuardianStudentAccessCommand) GuardianRelationshipOption {
	return func(r *GuardianRelationshipRepository) {
		r.pickup = pickup
		r.access = access
	}
}

// NewGuardianRelationshipRepository creates the relationship store. Reads work
// without the owner commands; a write fails closed until they are bound.
func NewGuardianRelationshipRepository(db *bun.DB, options ...GuardianRelationshipOption) users.StudentGuardianRepository {
	repository := &GuardianRelationshipRepository{db: db}
	for _, option := range options {
		option(repository)
	}
	return repository
}

var errGuardianOwnersUnbound = errors.New("student guardian: the Care Plan and Identity & Access commands are required to write a relationship")

// memberships returns the owner's bounded active-membership facts and fails closed
// when the composition did not bind it.
func (r *GuardianRelationshipRepository) memberships(ctx context.Context, accountIDs []int64, schoolID int64) (map[int64][]int64, error) {
	if r.activeMemberships == nil {
		return nil, errors.New("student guardian: active membership query is required")
	}
	return r.activeMemberships(ctx, accountIDs, []int64{schoolID})
}

// --- unit of work -------------------------------------------------------------

// write runs one multi-owner write as a single unit. Inside a caller's
// transaction it is a savepoint, so a failing owner command undoes the
// relationship half as well even when the caller handles the error and
// commits; without one it opens the tenant transaction of the context.
func (r *GuardianRelationshipRepository) write(ctx context.Context, fn func(context.Context) error) error {
	if r.pickup == nil || r.access == nil {
		return errGuardianOwnersUnbound
	}
	if _, ok := tenant.TransactionFromContext(ctx); ok {
		return tenant.WithSavepoint(ctx, fn)
	}
	if _, ok := modelBase.RepositoryTransaction(ctx); ok {
		return fn(ctx)
	}
	// A transaction of its own rolls back as a whole; no savepoint needed.
	return tenant.NewTransactionRunner().RunInTx(ctx, fn)
}

// writeTenant is the school a new relationship is written in: the row's own,
// or the one the caller is scoped to.
func writeTenant(ctx context.Context, rel *users.StudentGuardian) int64 {
	if rel.GetTenantID() == 0 {
		if id := tenant.FromContext(ctx); id != 0 {
			rel.SetTenantID(id)
		} else if id := modelBase.RepositoryTenantID(ctx); id != 0 {
			rel.SetTenantID(id)
		}
	}
	return rel.GetTenantID()
}

func permissionsJSON(permissions map[string]interface{}) (json.RawMessage, error) {
	if permissions == nil {
		return json.RawMessage(`{}`), nil
	}
	encoded, err := json.Marshal(permissions)
	if err != nil {
		return nil, fmt.Errorf("student guardian: encode permissions: %w", err)
	}
	return encoded, nil
}

// demoteOtherPrimaries clears the primary flag of the child's other
// relationships. The relationship table keeps one primary per child with a
// partial unique index instead of the old demotion trigger, so a promotion
// demotes first, exactly as the trigger did.
func demoteOtherPrimaries(ctx context.Context, db bun.IDB, tenantID, studentID, keepID int64) error {
	if _, err := db.NewRaw(`UPDATE users.student_guardian_relationships SET is_primary = FALSE
		WHERE tenant_id = ? AND student_id = ? AND is_primary AND id <> ?`, tenantID, studentID, keepID).Exec(ctx); err != nil {
		return fmt.Errorf("demote other primary guardians: %w", err)
	}
	return nil
}

// demoteOtherPrimariesForNewPair clears the primary flag of the child's other
// relationships, but only while the (tenant, student, guardian) pair is not
// linked yet. The demotion has to precede the insert because the relationship
// table keeps one primary per child with a partial unique index, and guarding
// it on the absent pair keeps a re-link of an already-linked guardian the pure
// no-op its callers document: the insert then writes nothing, so nothing may
// strip the child's primary either.
func demoteOtherPrimariesForNewPair(ctx context.Context, db bun.IDB, tenantID, studentID, guardianProfileID int64) error {
	if _, err := db.NewRaw(`UPDATE users.student_guardian_relationships AS relationship SET is_primary = FALSE
		WHERE relationship.tenant_id = ? AND relationship.student_id = ? AND relationship.is_primary
			AND NOT EXISTS (
				SELECT 1 FROM users.student_guardian_relationships AS linked
				WHERE linked.tenant_id = relationship.tenant_id
					AND linked.student_id = relationship.student_id
					AND linked.guardian_profile_id = ?)`,
		tenantID, studentID, guardianProfileID).Exec(ctx); err != nil {
		return fmt.Errorf("demote other primary guardians: %w", err)
	}
	return nil
}

const insertRelationshipSQL = `INSERT INTO users.student_guardian_relationships AS relationship
	(tenant_id, student_id, guardian_profile_id, relationship_type, guardian_role,
	 is_primary, is_emergency_contact, emergency_priority, is_payer)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	RETURNING id, created_at, updated_at,
		(SELECT g.account_id FROM users.guardian_profiles AS g WHERE g.tenant_id = relationship.tenant_id AND g.id = relationship.guardian_profile_id) AS account_id`

// insertRelationshipIfAbsentSQL leaves an existing pair untouched. The
// conflict target is the (tenant, student, guardian) unique key.
const insertRelationshipIfAbsentSQL = `INSERT INTO users.student_guardian_relationships AS relationship
	(tenant_id, student_id, guardian_profile_id, relationship_type, guardian_role,
	 is_primary, is_emergency_contact, emergency_priority, is_payer)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT (tenant_id, student_id, guardian_profile_id) DO NOTHING
	RETURNING id, created_at, updated_at,
		(SELECT g.account_id FROM users.guardian_profiles AS g WHERE g.tenant_id = relationship.tenant_id AND g.id = relationship.guardian_profile_id) AS account_id`

// insertRelationship writes the People Directory half and returns its id, or
// zero when ifAbsent is set and the pair is linked already, together with the
// portal account the guardian profile names, which the access row binds.
func (r *GuardianRelationshipRepository) insertRelationship(ctx context.Context, rel *users.StudentGuardian, ifAbsent bool) (int64, *int64, error) {
	db := base.GetDB(ctx, r.db)
	if rel.IsPrimary {
		demote := func() error { return demoteOtherPrimaries(ctx, db, rel.TenantID, rel.StudentID, 0) }
		if ifAbsent {
			demote = func() error {
				return demoteOtherPrimariesForNewPair(ctx, db, rel.TenantID, rel.StudentID, rel.GuardianProfileID)
			}
		}
		if err := demote(); err != nil {
			return 0, nil, err
		}
	}
	args := []any{rel.TenantID, rel.StudentID, rel.GuardianProfileID, rel.RelationshipType, rel.GuardianRole,
		rel.IsPrimary, rel.IsEmergencyContact, rel.EmergencyPriority, rel.IsPayer}
	insert := db.NewRaw(insertRelationshipSQL, args...)
	if ifAbsent {
		insert = db.NewRaw(insertRelationshipIfAbsentSQL, args...)
	}
	var rows []struct {
		ID        int64     `bun:"id"`
		CreatedAt time.Time `bun:"created_at"`
		UpdatedAt time.Time `bun:"updated_at"`
		AccountID *int64    `bun:"account_id"`
	}
	if err := insert.Scan(ctx, &rows); err != nil {
		return 0, nil, err
	}
	if len(rows) == 0 {
		return 0, nil, nil
	}
	rel.ID, rel.CreatedAt, rel.UpdatedAt = rows[0].ID, rows[0].CreatedAt, rows[0].UpdatedAt
	return rel.ID, rows[0].AccountID, nil
}

// createOwnerHalves writes the Care Plan and Identity halves of a new
// relationship; the access row binds the given account.
func (r *GuardianRelationshipRepository) createOwnerHalves(ctx context.Context, rel *users.StudentGuardian, accountID *int64, permissions json.RawMessage) error {
	if err := r.pickup.CreateGuardianPickupPermission(ctx, rel.TenantID, rel.ID, rel.CanPickup, rel.PickupNotes); err != nil {
		return err
	}
	return r.access.GrantGuardianStudentAccess(ctx, rel.TenantID, rel.ID, accountID, permissions)
}

// lockRelationship takes the relationship row lock every write of a link
// holds, and returns the relationship's school and child. found is false when
// the caller's school has no such relationship.
func (r *GuardianRelationshipRepository) lockRelationship(ctx context.Context, id int64) (tenantID, studentID int64, found bool, err error) {
	var rows []struct {
		TenantID  int64 `bun:"tenant_id"`
		StudentID int64 `bun:"student_id"`
	}
	query := base.GetDB(ctx, r.db).NewSelect().
		TableExpr(`users.student_guardian_relationships AS "relationship"`).
		ColumnExpr(`"relationship".tenant_id, "relationship".student_id`).
		Where(`"relationship".id = ?`, id).
		For("UPDATE")
	query = base.WithTenantFilter(ctx, query, "relationship")
	if err := query.Scan(ctx, &rows); err != nil {
		return 0, 0, false, fmt.Errorf("lock student guardian relationship: %w", err)
	}
	if len(rows) == 0 {
		return 0, 0, false, nil
	}
	return rows[0].TenantID, rows[0].StudentID, true, nil
}

// --- commands -----------------------------------------------------------------

// Create links a guardian to a child: the relationship, its pickup permission
// and its portal access, in one unit of work.
func (r *GuardianRelationshipRepository) Create(ctx context.Context, rel *users.StudentGuardian) error {
	if rel == nil {
		return fmt.Errorf("StudentGuardian cannot be nil or zero value")
	}
	if err := rel.Validate(); err != nil {
		return err
	}
	writeTenant(ctx, rel)
	permissions, err := permissionsJSON(rel.Permissions)
	if err != nil {
		return err
	}
	err = r.write(ctx, func(ctx context.Context) error {
		_, accountID, err := r.insertRelationship(ctx, rel, false)
		if err != nil {
			return err
		}
		return r.createOwnerHalves(ctx, rel, accountID, permissions)
	})
	if err != nil {
		return &modelBase.DatabaseError{Op: "create", Err: err}
	}
	return nil
}

// LinkIfNotExists inserts the student↔guardian relationship, treating a
// duplicate link (same tenant_id + student_id + guardian_profile_id) as a no-op
// via ON CONFLICT DO NOTHING. It returns true when a new link was written and
// false when the link already existed; the existing link is left untouched.
//
// Why ON CONFLICT rather than a read-then-insert check: re-linking the same
// guardian must never raise a unique violation, and in PostgreSQL a violation
// raised inside the request's tenant transaction would ALSO abort that
// transaction — breaking the atomic student-create path. DO NOTHING is
// race-safe and leaves the transaction usable. Only a new relationship gets
// its Care Plan and Identity halves.
//
// A primary link demotes the child's other primaries only when it writes a
// new row. The old table's BEFORE INSERT trigger demoted before the conflict
// check, so a re-link naming a primary could leave the child without one;
// that contradicted the no-op the link callers document and is not carried
// over.
func (r *GuardianRelationshipRepository) LinkIfNotExists(ctx context.Context, rel *users.StudentGuardian) (bool, error) {
	if rel == nil {
		return false, fmt.Errorf("student guardian cannot be nil")
	}
	if strings.TrimSpace(rel.GuardianRole) == "" && len(rel.Permissions) == 0 {
		authorize.ApplyDefaultStudentGuardianRole(rel)
	}
	if err := rel.Validate(); err != nil {
		return false, err
	}
	writeTenant(ctx, rel)
	permissions, err := permissionsJSON(rel.Permissions)
	if err != nil {
		return false, err
	}
	inserted := false
	err = r.write(ctx, func(ctx context.Context) error {
		id, accountID, err := r.insertRelationship(ctx, rel, true)
		if err != nil || id == 0 {
			return err
		}
		inserted = true
		return r.createOwnerHalves(ctx, rel, accountID, permissions)
	})
	if err != nil {
		return false, &modelBase.DatabaseError{Op: "link guardian if not exists", Err: base.TranslateNotFound(err)}
	}
	return inserted, nil
}

// Update rewrites every field of the relationship across its three owners.
func (r *GuardianRelationshipRepository) Update(ctx context.Context, rel *users.StudentGuardian) error {
	if rel == nil {
		return fmt.Errorf("StudentGuardian cannot be nil or zero value")
	}
	if err := rel.Validate(); err != nil {
		return err
	}
	found := false
	err := r.write(ctx, func(ctx context.Context) error {
		var err error
		found, err = r.updateColumns(ctx, rel, relationshipColumns, true, true)
		return err
	})
	if err != nil {
		return &modelBase.DatabaseError{Op: "update", Err: err}
	}
	if !found {
		return base.AssertRowsAffectedCount(0, 1, "update StudentGuardian")
	}
	return nil
}

// relationshipColumns are the People Directory columns a write may name.
var relationshipColumns = []string{
	"student_id", "guardian_profile_id", "relationship_type", "guardian_role",
	"is_primary", "is_emergency_contact", "emergency_priority", "is_payer",
}

// UpdateColumns writes only the named columns of the relationship, each to
// its owner. Use it to edit a bounded subset of fields without clobbering
// columns the caller does not own. It returns 1 when the relationship exists
// in the caller's school and 0 otherwise.
func (r *GuardianRelationshipRepository) UpdateColumns(ctx context.Context, rel *users.StudentGuardian, columns ...string) (int64, error) {
	if rel == nil {
		return 0, fmt.Errorf("StudentGuardian cannot be nil or zero value")
	}
	if len(columns) == 0 {
		return 0, fmt.Errorf("update columns StudentGuardian: at least one column required")
	}
	var named []string
	pickup, access := false, false
	for _, column := range columns {
		switch {
		case column == "can_pickup" || column == "pickup_notes":
			pickup = true
		case column == "permissions":
			access = true
		case slices.Contains(relationshipColumns, column):
			named = append(named, column)
		case column == "updated_at":
			// Each owner maintains its own timestamp.
		default:
			return 0, fmt.Errorf("update columns StudentGuardian: unknown column %q", column)
		}
	}
	found := false
	err := r.write(ctx, func(ctx context.Context) error {
		var err error
		found, err = r.updateColumnSubset(ctx, rel, named, columns, pickup, access)
		return err
	})
	if err != nil {
		return 0, &modelBase.DatabaseError{Op: "update columns", Err: err}
	}
	if !found {
		return 0, nil
	}
	return 1, nil
}

func (r *GuardianRelationshipRepository) updateColumnSubset(ctx context.Context, rel *users.StudentGuardian, named, columns []string, pickup, access bool) (bool, error) {
	if !pickup {
		return r.updateColumns(ctx, rel, named, false, access)
	}
	found, err := r.updateColumns(ctx, rel, named, false, access)
	if err != nil || !found {
		return found, err
	}
	var canPickup *bool
	if slices.Contains(columns, "can_pickup") {
		canPickup = &rel.CanPickup
	}
	_, err = r.pickup.ChangeGuardianPickupPermission(ctx, rel.TenantID, rel.ID, canPickup,
		slices.Contains(columns, "pickup_notes"), rel.PickupNotes)
	return true, err
}

// updateColumns locks the relationship, writes the named People Directory
// columns (demoting the child's other primaries first when the relationship
// becomes primary), then the pickup permission when withPickup is set and the
// permissions when withAccess is set. rel.TenantID is set to the
// relationship's school.
func (r *GuardianRelationshipRepository) updateColumns(ctx context.Context, rel *users.StudentGuardian, named []string, withPickup, withAccess bool) (bool, error) {
	tenantID, studentID, found, err := r.lockRelationship(ctx, rel.ID)
	if err != nil || !found {
		return false, err
	}
	rel.TenantID = tenantID
	db := base.GetDB(ctx, r.db)
	if len(named) > 0 {
		if slices.Contains(named, "is_primary") && rel.IsPrimary {
			target := studentID
			if slices.Contains(named, "student_id") {
				target = rel.StudentID
			}
			if err := demoteOtherPrimaries(ctx, db, tenantID, target, rel.ID); err != nil {
				return false, err
			}
		}
		query := db.NewUpdate().TableExpr(`users.student_guardian_relationships AS "relationship"`).
			Where(`"relationship".tenant_id = ?`, tenantID).
			Where(`"relationship".id = ?`, rel.ID)
		values := map[string]any{
			"student_id": rel.StudentID, "guardian_profile_id": rel.GuardianProfileID,
			"relationship_type": rel.RelationshipType, "guardian_role": rel.GuardianRole,
			"is_primary": rel.IsPrimary, "is_emergency_contact": rel.IsEmergencyContact,
			"emergency_priority": rel.EmergencyPriority, "is_payer": rel.IsPayer,
		}
		for _, column := range named {
			query = query.Set("? = ?", bun.Ident(column), values[column])
		}
		if _, err := query.Exec(ctx); err != nil {
			return false, err
		}
	}
	if withPickup {
		if _, err := r.pickup.ChangeGuardianPickupPermission(ctx, tenantID, rel.ID, &rel.CanPickup, true, rel.PickupNotes); err != nil {
			return false, err
		}
	}
	if withAccess {
		permissions, err := permissionsJSON(rel.Permissions)
		if err != nil {
			return false, err
		}
		if _, err := r.access.SetGuardianStudentPermissions(ctx, tenantID, rel.ID, permissions); err != nil {
			return false, err
		}
	}
	return true, nil
}

// Delete unlinks a guardian. The pickup permission and the portal access
// follow the relationship through their foreign-key cascade.
func (r *GuardianRelationshipRepository) Delete(ctx context.Context, id any) error {
	query := base.GetDB(ctx, r.db).NewDelete().
		TableExpr(`users.student_guardian_relationships AS "relationship"`).
		Where(`"relationship".id = ?`, id)
	query = base.WithTenantFilter(ctx, query, "relationship")
	if _, err := query.Exec(ctx); err != nil {
		return &modelBase.DatabaseError{Op: "delete", Err: err}
	}
	return nil
}

// SetPrimary sets a guardian as the primary guardian for a student, demoting
// the child's other primary first.
func (r *GuardianRelationshipRepository) SetPrimary(ctx context.Context, id int64, isPrimary bool) error {
	err := tenantAware(ctx, r.db, func(ctx context.Context) error {
		tenantID, studentID, found, err := r.lockRelationship(ctx, id)
		if err != nil {
			return err
		}
		if !found {
			return base.AssertRowsAffectedCount(0, 1, "set primary student_guardian")
		}
		db := base.GetDB(ctx, r.db)
		if isPrimary {
			if err := demoteOtherPrimaries(ctx, db, tenantID, studentID, id); err != nil {
				return err
			}
		}
		_, err = db.NewRaw(`UPDATE users.student_guardian_relationships SET is_primary = ?
			WHERE tenant_id = ? AND id = ?`, isPrimary, tenantID, id).Exec(ctx)
		return err
	})
	if err != nil {
		var databaseErr *modelBase.DatabaseError
		if errors.As(err, &databaseErr) {
			return err
		}
		return &modelBase.DatabaseError{Op: "set primary", Err: base.TranslateNotFound(err)}
	}
	return nil
}

// tenantAware runs a single-owner write that takes a row lock inside the
// caller's transaction, or opens the tenant transaction of the context.
func tenantAware(ctx context.Context, db *bun.DB, fn func(context.Context) error) error {
	if _, ok := modelBase.RepositoryTransaction(ctx); ok {
		return fn(ctx)
	}
	return tenant.NewTransactionRunner().RunInTx(ctx, fn)
}

// SetPayer moves the payment mark for one child (#2608): it clears the mark
// from every guardian of the child and, when guardianProfileID is non-nil,
// sets it on that guardian's relationship. Both statements run in the caller's
// tenant transaction, which is what makes the move atomic — the partial unique
// index uq_student_guardian_relationships_payer allows only one marked row per
// child, so a set-before-clear would raise a unique violation and abort the
// transaction.
//
// A domain operation rather than a filter (backend rule 2): "exactly one of
// these rows carries the flag" is an invariant across rows, which no per-row
// update expresses. Returns ErrStudentGuardianNotFound when the named guardian
// is not linked to the child.
func (r *GuardianRelationshipRepository) SetPayer(ctx context.Context, studentID int64, guardianProfileID *int64) error {
	db := base.GetDB(ctx, r.db)

	clear := db.NewUpdate().
		TableExpr(`users.student_guardian_relationships AS "relationship"`).
		Set("is_payer = FALSE").
		Where(`"relationship".student_id = ?`, studentID).
		Where(`"relationship".is_payer`)
	clear = base.WithTenantFilter(ctx, clear, "relationship")

	if _, err := clear.Exec(ctx); err != nil {
		return &modelBase.DatabaseError{Op: "clear student payer", Err: base.TranslateNotFound(err)}
	}

	if guardianProfileID == nil {
		return nil
	}

	set := db.NewUpdate().
		TableExpr(`users.student_guardian_relationships AS "relationship"`).
		Set("is_payer = TRUE").
		Where(`"relationship".student_id = ?`, studentID).
		Where(`"relationship".guardian_profile_id = ?`, *guardianProfileID)
	set = base.WithTenantFilter(ctx, set, "relationship")

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

// --- queries ------------------------------------------------------------------

func (r *GuardianRelationshipRepository) linkQuery(ctx context.Context, model any) *bun.SelectQuery {
	return guardianlinkview.ModelQuery(base.GetDB(ctx, r.db), 0, model)
}

// FindByID retrieves one relationship with its three halves.
func (r *GuardianRelationshipRepository) FindByID(ctx context.Context, id any) (*users.StudentGuardian, error) {
	relationship := new(users.StudentGuardian)
	query := base.WithTenantFilter(ctx, r.linkQuery(ctx, relationship).Where(`"student_guardian".id = ?`, id), "student_guardian")
	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{Op: "find by id", Err: base.TranslateNotFound(err)}
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
	query := r.linkQuery(ctx, &relationships)
	for field, value := range filters {
		if value == nil {
			continue
		}
		if !slices.Contains(listFilterColumns, field) {
			return nil, fmt.Errorf("student guardian: unsupported list filter %q", field)
		}
		query = query.Where(`? = ?`, bun.Ident("student_guardian."+field), value)
	}
	query = base.WithTenantFilter(ctx, query, "student_guardian")
	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{Op: "list", Err: base.TranslateNotFound(err)}
	}
	return relationships, nil
}

// FindByStudentID retrieves relationships by student ID
func (r *GuardianRelationshipRepository) FindByStudentID(ctx context.Context, studentID int64) ([]*users.StudentGuardian, error) {
	var relationships []*users.StudentGuardian
	query := base.WithTenantFilter(ctx, r.linkQuery(ctx, &relationships).
		Where(`"student_guardian".student_id = ?`, studentID), "student_guardian")
	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{Op: "find by student ID", Err: base.TranslateNotFound(err)}
	}
	return relationships, nil
}

// FindByStudentIDs retrieves relationships for many students in a single query.
func (r *GuardianRelationshipRepository) FindByStudentIDs(ctx context.Context, studentIDs []int64) ([]*users.StudentGuardian, error) {
	if len(studentIDs) == 0 {
		return []*users.StudentGuardian{}, nil
	}
	var relationships []*users.StudentGuardian
	query := base.WithTenantFilter(ctx, r.linkQuery(ctx, &relationships).
		Where(`"student_guardian".student_id IN (?)`, bun.List(studentIDs)), "student_guardian")
	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{Op: "find by student IDs", Err: base.TranslateNotFound(err)}
	}
	return relationships, nil
}

// FindByGuardianProfileID retrieves relationships by guardian profile ID
func (r *GuardianRelationshipRepository) FindByGuardianProfileID(ctx context.Context, guardianProfileID int64) ([]*users.StudentGuardian, error) {
	var relationships []*users.StudentGuardian
	query := base.WithTenantFilter(ctx, r.linkQuery(ctx, &relationships).
		Where(`"student_guardian".guardian_profile_id = ?`, guardianProfileID), "student_guardian")
	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{Op: "find by guardian profile ID", Err: base.TranslateNotFound(err)}
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
	lock := base.GetDB(ctx, r.db).NewSelect().
		TableExpr(`users.student_guardian_relationships AS "relationship"`).
		ColumnExpr(`"relationship".id`).
		Where(`"relationship".student_id = ?`, studentID).
		Where(`"relationship".guardian_profile_id = ?`, guardianProfileID).
		For("UPDATE")
	lock = base.WithTenantFilter(ctx, lock, "relationship")
	if err := lock.Scan(ctx, &locked); err != nil {
		return nil, &modelBase.DatabaseError{Op: "find by student and guardian for update", Err: base.TranslateNotFound(err)}
	}
	if len(locked) == 0 {
		return nil, users.ErrStudentGuardianNotFound
	}
	relationship, err := r.FindByID(ctx, locked[0])
	if err != nil {
		if errors.Is(err, modelBase.ErrNotFound) || errors.Is(err, sql.ErrNoRows) {
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
func (r *GuardianRelationshipRepository) ListLinkedChildrenForGuardians(ctx context.Context, guardianProfileIDs []int64) ([]*users.GuardianLinkedChild, error) {
	if len(guardianProfileIDs) == 0 {
		return []*users.GuardianLinkedChild{}, nil
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
	var rows []*users.GuardianLinkedChild
	db := base.GetDB(ctx, r.db)
	if err := db.NewRaw(query, studentdirectoryview.Query(db, tenant.FromContext(ctx)), bun.List(guardianProfileIDs)).Scan(ctx, &rows); err != nil {
		return nil, &modelBase.DatabaseError{Op: "list linked children for guardians", Err: base.TranslateNotFound(err)}
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
	db := base.GetDB(ctx, r.db)
	var granted bool
	if err := db.NewRaw(query, guardianlinkview.Query(db, tenantID), studentID, accountID, permission).Scan(ctx, &granted); err != nil {
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
	db := base.GetDB(ctx, r.db)
	var granted []int64
	if err := db.NewRaw(query,
		guardianlinkview.Query(db, tenantID),
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
	db := base.GetDB(ctx, r.db)
	var rows []struct{ AccountID *int64 }
	if err := db.NewRaw(query, guardianlinkview.Query(db, tenantID), studentID, normalizedEmail, permission).Scan(ctx, &rows); err != nil {
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
	query := guardianlinkview.Query(base.GetDB(ctx, r.db), 0).
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

// ListPaymentAssignments returns one row per non-alumnus child of the tenant,
// with the guardian marked as payer joined in — LEFT, so children without an
// assigned payer appear with a nil GuardianProfileID. Showing the gaps is the
// point: a Bankverbindungen list that silently omits the children nobody
// assigned looks complete while missing exactly the rows that need work.
func (r *GuardianRelationshipRepository) ListPaymentAssignments(ctx context.Context) ([]users.GuardianPaymentAssignment, error) {
	var rows []users.GuardianPaymentAssignment
	query := studentdirectoryview.Query(base.GetDB(ctx, r.db), tenant.FromContext(ctx)).
		ColumnExpr(`"student".id AS student_id`).
		ColumnExpr(`"student_person".first_name AS student_first_name`).
		ColumnExpr(`"student_person".last_name AS student_last_name`).
		ColumnExpr(`"student".school_class`).
		ColumnExpr(`"guardian".id AS guardian_profile_id`).
		ColumnExpr(`COALESCE("guardian".first_name, '') AS guardian_first_name`).
		ColumnExpr(`COALESCE("guardian".last_name, '') AS guardian_last_name`).
		ColumnExpr(`COALESCE("student_guardian".relationship_type, '') AS relationship_type`).
		Join(`JOIN users.persons AS "student_person" ON "student_person".id = "student".person_id`).
		Join(`LEFT JOIN users.student_guardian_relationships AS "student_guardian"
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
