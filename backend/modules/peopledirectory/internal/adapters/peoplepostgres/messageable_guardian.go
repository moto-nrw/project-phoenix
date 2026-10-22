package users

import (
	"context"
	"errors"
	"slices"

	users "github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/rows"
	ports "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
	"github.com/uptrace/bun"
)

// MessageableGuardianRepository answers which guardians of a child may receive
// a parent message. It stays with People Directory because the rows it reads —
// the relationship and the guardian profile — belong to that owner; the school
// membership is a bounded Identity & Access fact it filters through.
// Communication receives the answer, not the join.
type MessageableGuardianRepository struct {
	runtime           Runtime
	links             GuardianLinkRows
	activeMemberships SchoolMembershipLookup
}

// SchoolMembershipLookup returns active school IDs per account, restricted to
// both supplied sets. Membership alone does not grant relationship permissions.
type SchoolMembershipLookup func(context.Context, []int64, []int64) (map[int64][]int64, error)

// NewMessageableGuardianRepository wires the recipient lookup with the
// owner's bounded membership lookup.
func NewMessageableGuardianRepository(runtime Runtime, links GuardianLinkRows, activeMemberships SchoolMembershipLookup) *MessageableGuardianRepository {
	return &MessageableGuardianRepository{runtime: requireRuntime(runtime), links: requireGuardianLinkRows(links), activeMemberships: activeMemberships}
}

// ListGuardiansForStudent returns the child's account-holding guardians who may
// actually use the parent portal for THIS child, primary first, for the staff
// "new conversation" recipient picker.
//
// A guardian is included only when the relationship grants
// parent_portal.access — pickup-only, emergency-contact and custom limited
// roles hold an account but no portal authority, and the parent-side reads
// reject those same threads, so offering them as recipients would let staff
// send a message the chosen guardian can never see. Permissions are written as
// `{"<perm>": true}`, so JSONB containment matches the same grant the Go check
// enforces; a NULL or empty permissions map fails containment and is correctly
// excluded.
//
// The account must ALSO hold an ACTIVE membership for this school. A guardian
// whose mapping is pending or inactive can no longer read its children, so
// without this check staff could open a thread the recipient cannot see and the
// broadcast would still wake that revoked account if it is logged in for
// another school. Matching each candidate's account and school against the
// owner's facts keeps this check scoped to the relationship's school.
func (r *MessageableGuardianRepository) ListGuardiansForStudent(ctx context.Context, studentID int64) ([]*users.MessageableGuardian, error) {
	if r.activeMemberships == nil {
		return nil, &ports.DatabaseError{Op: "list guardians for student", Err: errors.New("active membership query is required")}
	}
	var rows []struct {
		users.MessageableGuardian
		SchoolID int64 `bun:"school_id"`
	}
	ambientDB1, databaseErr1 := r.runtime.DB(ctx)
	if databaseErr1 != nil {
		return *new([]*users.MessageableGuardian), databaseErr1
	}
	query := r.portalGuardianLinks(ctx, ambientDB1).
		ColumnExpr("student_guardian.tenant_id AS school_id").
		ColumnExpr("gp.account_id AS account_id").
		ColumnExpr("btrim(COALESCE(gp.first_name,'') || ' ' || COALESCE(gp.last_name,'')) AS name").
		ColumnExpr("student_guardian.relationship_type AS relationship_type").
		ColumnExpr("student_guardian.is_primary AS is_primary").
		ColumnExpr("COALESCE(gp.portal_locale, 'de') AS portal_locale").
		Where("student_guardian.student_id = ?", studentID).
		OrderExpr("student_guardian.is_primary DESC, name ASC")

	if err := query.Scan(ctx, &rows); err != nil {
		return nil, &ports.DatabaseError{Op: "list guardians for student", Err: translateNotFound(err)}
	}
	if len(rows) == 0 {
		return []*users.MessageableGuardian{}, nil
	}
	accountIDs := make([]int64, 0, len(rows))
	schoolIDs := make([]int64, 0, len(rows))
	for _, row := range rows {
		accountIDs = append(accountIDs, row.AccountID)
		schoolIDs = append(schoolIDs, row.SchoolID)
	}
	memberships, err := r.activeMemberships(ctx, accountIDs, schoolIDs)
	if err != nil {
		return nil, &ports.DatabaseError{Op: "list guardians for student", Err: err}
	}
	result := make([]*users.MessageableGuardian, 0, len(rows))
	for _, row := range rows {
		if slices.Contains(memberships[row.AccountID], row.SchoolID) {
			result = append(result, &row.MessageableGuardian)
		}
	}
	return result, nil
}

// ListStudentIDsWithMessageableGuardian returns, ascending, the IDs of the
// school's children that have at least one guardian ListGuardiansForStudent
// would offer: the same portal account, parent_portal.access and active
// membership rules, evaluated for every child in two statements. The staff
// "new message" child list marks the others (#4362).
func (r *MessageableGuardianRepository) ListStudentIDsWithMessageableGuardian(ctx context.Context) ([]int64, error) {
	const op = "list students with messageable guardian"
	if r.activeMemberships == nil {
		return nil, &ports.DatabaseError{Op: op, Err: errors.New("active membership query is required")}
	}
	db, err := r.runtime.DB(ctx)
	if err != nil {
		return nil, err
	}
	var rows []struct {
		StudentID int64 `bun:"student_id"`
		AccountID int64 `bun:"account_id"`
		SchoolID  int64 `bun:"school_id"`
	}
	query := r.portalGuardianLinks(ctx, db).
		Distinct().
		ColumnExpr("student_guardian.student_id AS student_id").
		ColumnExpr("gp.account_id AS account_id").
		ColumnExpr("student_guardian.tenant_id AS school_id")
	if err := query.Scan(ctx, &rows); err != nil {
		return nil, &ports.DatabaseError{Op: op, Err: translateNotFound(err)}
	}
	if len(rows) == 0 {
		return []int64{}, nil
	}
	accountIDs := make([]int64, 0, len(rows))
	schoolIDs := make([]int64, 0, len(rows))
	for _, row := range rows {
		accountIDs = append(accountIDs, row.AccountID)
		schoolIDs = append(schoolIDs, row.SchoolID)
	}
	memberships, err := r.activeMemberships(ctx, accountIDs, schoolIDs)
	if err != nil {
		return nil, &ports.DatabaseError{Op: op, Err: err}
	}
	studentIDs := make([]int64, 0, len(rows))
	for _, row := range rows {
		if slices.Contains(memberships[row.AccountID], row.SchoolID) {
			studentIDs = append(studentIDs, row.StudentID)
		}
	}
	slices.Sort(studentIDs)
	return slices.Compact(studentIDs), nil
}

// portalGuardianLinks selects the caller school's student-guardian links whose
// guardian holds a portal account and whose relationship grants
// parent_portal.access, joined to the guardian profile as gp. Both recipient
// reads build on it so they apply one rule.
func (r *MessageableGuardianRepository) portalGuardianLinks(ctx context.Context, db bun.IDB) *bun.SelectQuery {
	query := guardianLinkRows(ctx, r.runtime, r.links, db).
		Join("JOIN users.guardian_profiles AS gp ON gp.id = student_guardian.guardian_profile_id").
		Where("gp.account_id IS NOT NULL").
		Where("gp.has_account = true").
		Where(`"student_guardian".permissions @> ?::jsonb`, `{"parent_portal.access": true}`)
	return withTenantFilter(ctx, r.runtime, query, "student_guardian")
}
