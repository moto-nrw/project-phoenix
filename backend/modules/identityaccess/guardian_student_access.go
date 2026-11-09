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

// GuardianStudentAccess is the command side of the relationship access.
// School commands join the caller's school or administrative transaction;
// release account locks join only the named Träger's transaction.
type GuardianStudentAccess interface {
	// LockLiveTraegerGuardianAccounts locks live candidates still assigned to
	// an active school of the Träger until its ambient transaction commits.
	// Results are unique and ordered by account ID.
	LockLiveTraegerGuardianAccounts(context.Context, int64, []int64) ([]int64, error)
	// LockLiveGuardianAccount takes a shared account-row lock until the caller's
	// transaction ends and reports whether the account has not been erased.
	LockLiveGuardianAccount(context.Context, int64) (bool, error)
	// GrantGuardianStudentAccess records the access of a new relationship.
	GrantGuardianStudentAccess(context.Context, GuardianStudentAccessGrant) error
	// SetGuardianStudentPermissions replaces the permissions of one
	// relationship and reports whether it has an access row in that school.
	SetGuardianStudentPermissions(ctx context.Context, tenantID, relationshipID int64, permissions json.RawMessage) (bool, error)
}
