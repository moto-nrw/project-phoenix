package test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/uptrace/bun"
)

// CreateTestSessionFamily stores a live refresh session for the account and
// returns its family. The production router accepts an access token only
// while its family lives (#5194). tenantID is the session's school; scope is
// the access token's scope ("" for the tenant portal).
func CreateTestSessionFamily(tb testing.TB, db *bun.DB, accountID, tenantID int64, scope string) string {
	tb.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	portalScope := scope
	if portalScope == "" {
		portalScope = "tenant"
	}
	token := &TokenFixture{
		TenantID:    tenantID,
		AccountID:   accountID,
		Token:       fmt.Sprintf("test-session-%d", uniqueFixtureSuffix()),
		Expiry:      time.Now().Add(time.Hour),
		FamilyID:    fmt.Sprintf("family-session-%d", uniqueFixtureSuffix()),
		PortalScope: portalScope,
	}
	insertTokenFixture(tb, ctx, db, token)
	return token.FamilyID
}
