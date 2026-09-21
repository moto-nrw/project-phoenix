package test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// InvitationTokenFixture is test-owned setup data, not an auth model or
// persistence contract. Tests may deliberately insert expired or spent links.
type InvitationTokenFixture struct {
	ID               int64 `bun:"id,pk,autoincrement"`
	TenantID         int64 `bun:"tenant_id"`
	Email            string
	Token            string
	RoleID           int64
	CreatedBy        *int64
	ExpiresAt        time.Time
	UsedAt           *time.Time
	FirstName        *string
	LastName         *string
	Position         *string
	CaregiverEnabled bool
	PersonID         *int64
	EmailSentAt      *time.Time
	EmailError       *string
	EmailRetryCount  int
}

// InsertTestInvitationToken preserves the exact supplied address, token and
// school. CreateTestInvitationToken adds fixture uniqueness when it is wanted.
func InsertTestInvitationToken(tb testing.TB, db *bun.DB, invitation *InvitationTokenFixture) {
	tb.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := db.NewInsert().Model(invitation).ModelTableExpr("auth.invitation_tokens").Returning("id").Exec(ctx)
	require.NoError(tb, err, "Failed to create test invitation token")
}
