package users

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	directory "github.com/moto-nrw/project-phoenix/modules/peopledirectory"

	"github.com/uptrace/bun"

	users "github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/rows"
	ports "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
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

// GuardianRelationshipRepository implements StudentGuardianRepository
// over the three owners.
type GuardianRelationshipRepository struct {
	runtime  Runtime
	students StudentDirectoryRows
	links    GuardianLinkRows
	// activeMemberships supplies bounded account-school facts for permission checks.
	activeMemberships SchoolMembershipLookup
	pickup            GuardianPickupPermissionCommand
	access            GuardianStudentAccessCommand
	// defaultRole gives a new relationship that names neither a role nor
	// permissions the preset its type and flags imply. The authorization
	// policy owns the presets, so the composition binds it.
	defaultRole func(*directory.GuardianRelationshipSnapshot)
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

// WithGuardianDefaultRole installs the authorization policy's default role
// preset a link without a role or permissions is written with.
func WithGuardianDefaultRole(apply func(*directory.GuardianRelationshipSnapshot)) GuardianRelationshipOption {
	return func(r *GuardianRelationshipRepository) { r.defaultRole = apply }
}

// newRawGuardianRelationshipRepository creates the relationship store. Reads work
// without the owner commands; a write fails closed until they are bound.
func newRawGuardianRelationshipRepository(runtime Runtime, students StudentDirectoryRows, links GuardianLinkRows, options ...GuardianRelationshipOption) rawStudentGuardianRepository {
	repository := &GuardianRelationshipRepository{runtime: requireRuntime(runtime), students: requireStudentRows(students), links: requireGuardianLinkRows(links)}
	for _, option := range options {
		option(repository)
	}
	return repository
}

var errGuardianDefaultRoleUnbound = errors.New("student guardian: the default role policy is required to link without a role")

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

// write protects all relationship halves with a savepoint of the caller's
// unit of work. Missing transactions fail before any owner is changed.
func (r *GuardianRelationshipRepository) write(ctx context.Context, fn func(context.Context) error) error {
	if r.pickup == nil || r.access == nil {
		return errGuardianOwnersUnbound
	}
	return r.runtime.WithSavepoint(ctx, fn)
}

// writeTenant is the school a new relationship is written in: the row's own,
// or the one the caller is scoped to.
func (r *GuardianRelationshipRepository) writeTenant(ctx context.Context, rel *users.StudentGuardian) int64 {
	if rel.GetTenantID() == 0 {
		if id := r.runtime.TenantID(ctx); id != 0 {
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
	db := r.runtime.DB(ctx)
	result, err := db.NewRaw(`UPDATE users.guardian_profiles SET childless_since = NULL
		WHERE tenant_id = ? AND id = ?`, rel.TenantID, rel.GuardianProfileID).Exec(ctx)
	if err != nil {
		return 0, nil, fmt.Errorf("clear linked guardian retention mark: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return 0, nil, fmt.Errorf("count linked guardian retention marks: %w", err)
	}
	if affected == 0 {
		return 0, nil, directory.ErrGuardianNotFound
	}
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
	query := r.runtime.DB(ctx).NewSelect().
		TableExpr(`users.student_guardian_relationships AS "relationship"`).
		ColumnExpr(`"relationship".tenant_id, "relationship".student_id`).
		Where(`"relationship".id = ?`, id).
		For("UPDATE")
	query = withTenantFilter(ctx, r.runtime, query, "relationship")
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
	r.writeTenant(ctx, rel)
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
		return &ports.DatabaseError{Op: "create", Err: err}
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
		if r.defaultRole == nil {
			return false, errGuardianDefaultRoleUnbound
		}
		native := NativeGuardianRelationshipValue(rel)
		r.defaultRole(native)
		mapped, err := GuardianRelationshipRow(native)
		if err != nil {
			return false, err
		}
		*rel = *mapped
	}
	if err := rel.Validate(); err != nil {
		return false, err
	}
	r.writeTenant(ctx, rel)
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
		return false, &ports.DatabaseError{Op: "link guardian if not exists", Err: translateNotFound(err)}
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
		return &ports.DatabaseError{Op: "update", Err: err}
	}
	if !found {
		return assertRowsAffectedCount(0, 1, "update StudentGuardian")
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
		return 0, &ports.DatabaseError{Op: "update columns", Err: err}
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
	db := r.runtime.DB(ctx)
	if err := updateGuardianRelationshipColumns(ctx, db, rel, named, studentID); err != nil {
		return false, err
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
	query := r.runtime.DB(ctx).NewDelete().
		TableExpr(`users.student_guardian_relationships AS "relationship"`).
		Where(`"relationship".id = ?`, id)
	query = withTenantFilter(ctx, r.runtime, query, "relationship")
	if _, err := query.Exec(ctx); err != nil {
		return &ports.DatabaseError{Op: "delete", Err: err}
	}
	return nil
}

// SetPrimary sets a guardian as the primary guardian for a student, demoting
// the child's other primary first.
func (r *GuardianRelationshipRepository) SetPrimary(ctx context.Context, id int64, isPrimary bool) error {
	err := inExistingTransaction(ctx, r.runtime, func(ctx context.Context) error {
		tenantID, studentID, found, err := r.lockRelationship(ctx, id)
		if err != nil {
			return err
		}
		if !found {
			return assertRowsAffectedCount(0, 1, "set primary student_guardian")
		}
		db := r.runtime.DB(ctx)
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
		var databaseErr *ports.DatabaseError
		if errors.As(err, &databaseErr) {
			return err
		}
		return &ports.DatabaseError{Op: "set primary", Err: translateNotFound(err)}
	}
	return nil
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
	db := r.runtime.DB(ctx)

	clear := db.NewUpdate().
		TableExpr(`users.student_guardian_relationships AS "relationship"`).
		Set("is_payer = FALSE").
		Where(`"relationship".student_id = ?`, studentID).
		Where(`"relationship".is_payer`)
	clear = withTenantFilter(ctx, r.runtime, clear, "relationship")

	if _, err := clear.Exec(ctx); err != nil {
		return &ports.DatabaseError{Op: "clear student payer", Err: translateNotFound(err)}
	}

	if guardianProfileID == nil {
		return nil
	}

	set := db.NewUpdate().
		TableExpr(`users.student_guardian_relationships AS "relationship"`).
		Set("is_payer = TRUE").
		Where(`"relationship".student_id = ?`, studentID).
		Where(`"relationship".guardian_profile_id = ?`, *guardianProfileID)
	set = withTenantFilter(ctx, r.runtime, set, "relationship")

	result, err := set.Exec(ctx)
	if err != nil {
		return &ports.DatabaseError{Op: "set student payer", Err: translateNotFound(err)}
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return &ports.DatabaseError{Op: "set student payer", Err: translateNotFound(err)}
	}
	if affected == 0 {
		return users.ErrStudentGuardianNotFound
	}
	return nil
}

func updateGuardianRelationshipColumns(ctx context.Context, db bun.IDB, rel *users.StudentGuardian, named []string, studentID int64) error {
	if len(named) == 0 {
		return nil
	}
	tenantID := rel.TenantID
	if slices.Contains(named, "is_primary") && rel.IsPrimary {
		target := studentID
		if slices.Contains(named, "student_id") {
			target = rel.StudentID
		}
		if err := demoteOtherPrimaries(ctx, db, tenantID, target, rel.ID); err != nil {
			return err
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
		return err
	}
	return nil
}
