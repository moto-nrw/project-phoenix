package domain

import "encoding/json"

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
