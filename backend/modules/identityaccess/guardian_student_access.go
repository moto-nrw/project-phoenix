package identityaccess

import (
	"context"
	"encoding/json"
	"errors"
)

// Parents-portal access of a guardian to one child (#2756). The account the
// relationship is bound to and the parent_portal.* permissions it grants
// belong to Identity & Access (auth.guardian_student_access). The
// relationship they hang off belongs to People Directory; its unit of work
// creates both together and holds the relationship row lock while it writes
// this one. The binding follows the guardian profile's account afterwards.

// ErrGuardianStudentAccessTenantMismatch refuses a command whose school
// differs from the school of the caller's tenant transaction.
var ErrGuardianStudentAccessTenantMismatch = errors.New("identity access: guardian access belongs to another school")

// GuardianStudentAccessGrant is the access half of a new relationship.
// Permissions is the stored JSON object; empty means {}.
type GuardianStudentAccessGrant struct {
	TenantID       int64
	RelationshipID int64
	AccountID      *int64
	Permissions    json.RawMessage
}

// GuardianStudentAccess is the command side of the relationship access. Every
// command runs in the caller's transaction and names its school explicitly,
// so it also serves the administrative transactions that write across
// schools.
type GuardianStudentAccess interface {
	// GrantGuardianStudentAccess records the access of a new relationship.
	GrantGuardianStudentAccess(context.Context, GuardianStudentAccessGrant) error
	// SetGuardianStudentPermissions replaces the permissions of one
	// relationship and reports whether it has an access row in that school.
	SetGuardianStudentPermissions(ctx context.Context, tenantID, relationshipID int64, permissions json.RawMessage) (bool, error)
}
