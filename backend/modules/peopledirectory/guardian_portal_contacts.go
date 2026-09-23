package peopledirectory

import (
	"context"
	"encoding/json"
)

// GuardianPortalContact is an active portal profile and one of its child
// relationships. StudentID is nil for a profile without relationships.
// PortalPermission is the stored parent_portal.access JSON value, not a grant
// decision. Historical values are not uniformly boolean; the consumer applies
// its existing authorization helper. An absent permission is nil.
type GuardianPortalContact struct {
	GuardianProfileID, TenantID, AccountID int64
	FirstName, LastName                    string
	Email, PortalLocale                    *string
	StudentID                              *int64
	PortalPermission                       json.RawMessage
}

// GuardianPortalContacts queries requested profiles or profiles linked to the
// requested students, within the ambient school/transaction. Empty selections
// return no contacts. Only active accounts with same-school guardian access
// are returned; callers must still enforce each relationship's permissions.
type GuardianPortalContacts interface {
	ListGuardianPortalContacts(context.Context, []int64, []int64) ([]GuardianPortalContact, error)
}

func (m *Module) ListGuardianPortalContacts(ctx context.Context, guardianIDs, studentIDs []int64) ([]GuardianPortalContact, error) {
	if len(guardianIDs) == 0 && len(studentIDs) == 0 {
		return []GuardianPortalContact{}, nil
	}
	return m.engine.ListGuardianPortalContacts(ctx, guardianIDs, studentIDs)
}
