package test

import "time"

// TokenFixture is test-owned setup data for persisted refresh sessions. It
// deliberately supports expired tokens and unknown portal scopes.
type TokenFixture struct {
	ID          int64 `bun:"id,pk,autoincrement"`
	TenantID    int64
	AccountID   int64
	Token       string
	Expiry      time.Time
	Mobile      bool
	FamilyID    string
	Generation  int
	PortalScope string `bun:"portal_scope,notnull,default:'unknown'"`
}
