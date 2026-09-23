package users

import (
	"context"
	"errors"
	"slices"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/guardianlinkview"
	"github.com/uptrace/bun"
)

// MessageableGuardianRepository answers which guardians of a child may receive
// a parent message. It stays with People Directory because the rows it reads —
// the relationship and the guardian profile — belong to that owner; the school
// membership is a bounded Identity & Access fact it filters through.
// Communication receives the answer, not the join.
type MessageableGuardianRepository struct {
	db                *bun.DB
	activeMemberships SchoolMembershipLookup
}

// SchoolMembershipLookup returns active school IDs per account, restricted to
// both supplied sets. Membership alone does not grant relationship permissions.
type SchoolMembershipLookup func(context.Context, []int64, []int64) (map[int64][]int64, error)

// NewMessageableGuardianRepository wires the recipient lookup with the
// owner's bounded membership lookup.
func NewMessageableGuardianRepository(db *bun.DB, activeMemberships SchoolMembershipLookup) *MessageableGuardianRepository {
	return &MessageableGuardianRepository{db: db, activeMemberships: activeMemberships}
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
		return nil, &modelBase.DatabaseError{Op: "list guardians for student", Err: errors.New("active membership query is required")}
	}
	var rows []struct {
		users.MessageableGuardian
		SchoolID int64 `bun:"school_id"`
	}
	query := guardianlinkview.Query(base.GetDB(ctx, r.db), 0).
		ColumnExpr("student_guardian.tenant_id AS school_id").
		ColumnExpr("gp.account_id AS account_id").
		ColumnExpr("btrim(COALESCE(gp.first_name,'') || ' ' || COALESCE(gp.last_name,'')) AS name").
		ColumnExpr("student_guardian.relationship_type AS relationship_type").
		ColumnExpr("student_guardian.is_primary AS is_primary").
		ColumnExpr("COALESCE(gp.portal_locale, 'de') AS portal_locale").
		Join("JOIN users.guardian_profiles AS gp ON gp.id = student_guardian.guardian_profile_id").
		Where("student_guardian.student_id = ?", studentID).
		Where("gp.account_id IS NOT NULL").
		Where("gp.has_account = true").
		Where(`"student_guardian".permissions @> ?::jsonb`, `{"parent_portal.access": true}`).
		OrderExpr("student_guardian.is_primary DESC, name ASC")

	query = base.WithTenantFilter(ctx, query, "student_guardian")

	if err := query.Scan(ctx, &rows); err != nil {
		return nil, &modelBase.DatabaseError{Op: "list guardians for student", Err: base.TranslateNotFound(err)}
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
		return nil, &modelBase.DatabaseError{Op: "list guardians for student", Err: err}
	}
	result := make([]*users.MessageableGuardian, 0, len(rows))
	for _, row := range rows {
		if slices.Contains(memberships[row.AccountID], row.SchoolID) {
			result = append(result, &row.MessageableGuardian)
		}
	}
	return result, nil
}
