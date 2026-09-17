package users

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/uptrace/bun"
)

// MessageableGuardianRepository answers which guardians of a child may receive
// a parent message. It stays with People Directory because the rows it reads —
// the relationship and the guardian profile — belong to that owner; the school
// membership is the Identity & Access owner query it filters through.
// Communication receives the answer, not the join.
type MessageableGuardianRepository struct {
	db *bun.DB
	// activeMemberships is the Identity & Access owner query for the school
	// mapping; the membership check filters through it (#2721).
	activeMemberships ActiveMembershipQuery
}

// NewMessageableGuardianRepository wires the recipient lookup with the
// owner's active-membership query.
func NewMessageableGuardianRepository(db *bun.DB, activeMemberships ActiveMembershipQuery) *MessageableGuardianRepository {
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
// without this join staff could open a thread the recipient cannot see and the
// broadcast would still wake that revoked account if it is logged in for
// another school. Matching the owner's (account_id, tenant_id) pair against
// sg.tenant_id keeps the membership check scoped to the same school as the
// relationship row.
func (r *MessageableGuardianRepository) ListGuardiansForStudent(ctx context.Context, studentID int64) ([]*users.MessageableGuardian, error) {
	if r.activeMemberships == nil {
		return nil, &modelBase.DatabaseError{Op: "list guardians for student", Err: errors.New("active membership query is required")}
	}
	var rows []*users.MessageableGuardian
	query := base.GetDB(ctx, r.db).NewSelect().
		TableExpr("users.students_guardians AS sg").
		ColumnExpr("gp.account_id AS account_id").
		ColumnExpr("btrim(COALESCE(gp.first_name,'') || ' ' || COALESCE(gp.last_name,'')) AS name").
		ColumnExpr("sg.relationship_type AS relationship_type").
		ColumnExpr("sg.is_primary AS is_primary").
		ColumnExpr("COALESCE(gp.portal_locale, 'de') AS portal_locale").
		Join("JOIN users.guardian_profiles AS gp ON gp.id = sg.guardian_profile_id").
		Where("sg.student_id = ?", studentID).
		Where("(gp.account_id, sg.tenant_id) IN (?)", r.activeMemberships(ctx)).
		Where("gp.account_id IS NOT NULL").
		Where("gp.has_account = true").
		Where(`sg.permissions @> ?::jsonb`, `{"parent_portal.access": true}`).
		OrderExpr("sg.is_primary DESC, name ASC")

	query = base.WithTenantFilter(ctx, query, "sg")

	if err := query.Scan(ctx, &rows); err != nil {
		return nil, &modelBase.DatabaseError{Op: "list guardians for student", Err: base.TranslateNotFound(err)}
	}
	return rows, nil
}
