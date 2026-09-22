package behavior_test

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"

	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	authjwt "github.com/moto-nrw/project-phoenix/modules/identityaccess/legacy/jwt"
	"github.com/moto-nrw/project-phoenix/services"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// The operator second factor lives in Identity & Access since #3331. The
// behaviour tests in this directory still run against a real test database
// and the same rows the service root composes over, so the cooldown, the
// single-use code and the disable cascade they pin are what production runs.
// The three tables are the module's own, so a test that has to inspect or
// substitute a row reads it here instead of through a repository.

// newOperatorMFATestModule composes the module the way the service root does.
func newOperatorMFATestModule(t *testing.T, db *bun.DB, options ...services.AuthTestOption) services.AuthTestModule {
	t.Helper()
	module, err := services.NewAuthTestModule(db, testpkg.TenantRuntime(t, db), options...)
	require.NoError(t, err)
	require.NotNil(t, module.TokenAuth)
	return module
}

// newTestOperatorMFAService wires the operator second factor with a
// successful mailer, so StartOperatorMFAChallenge exercises the
// delivered-code path. The signer it returns is the one the module mints and
// parses the operator challenge JWTs with.
func newTestOperatorMFAService(t *testing.T, options ...services.AuthTestOption) (identityaccess.OperatorMFAFlows, *authjwt.TokenAuth, *bun.DB) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	module := newOperatorMFATestModule(t, db, withDeliveringMailer(options)...)
	return module.OperatorMFA, module.TokenAuth, db
}

// operatorChallengeRow is the test's read side of
// platform.operator_mfa_email_challenges.
type operatorChallengeRow struct {
	bun.BaseModel `bun:"table:platform.operator_mfa_email_challenges,alias:operator_mfa_email_challenge"`
	ID            int64      `bun:"id,pk,autoincrement"`
	OperatorID    int64      `bun:"operator_id,notnull"`
	CodeHash      string     `bun:"code_hash,notnull"`
	ExpiresAt     time.Time  `bun:"expires_at,notnull"`
	ConsumedAt    *time.Time `bun:"consumed_at"`
	IPAddress     net.IP     `bun:"ip_address,type:inet,nullzero"`
}

// activeOperatorChallenge returns the operator's redeemable code row, or nil
// when none is redeemable — the same condition the flows resolve on.
func activeOperatorChallenge(t *testing.T, db *bun.DB, operatorID int64) *operatorChallengeRow {
	t.Helper()
	row := new(operatorChallengeRow)
	err := db.NewSelect().Model(row).
		Where("operator_id = ?", operatorID).
		Where("consumed_at IS NULL").
		Where("expires_at > now()").
		Order("expires_at DESC").
		Limit(1).
		Scan(context.Background())
	if err != nil {
		return nil
	}
	return row
}

// substituteOperatorChallengeCode rewrites a challenge's hash to one the test
// knows the plaintext of. The flows never expose the mailed code, so this is
// how a verification happy path is driven end to end.
func substituteOperatorChallengeCode(t *testing.T, db *bun.DB, challengeID int64, code string) {
	t.Helper()
	hash := testpkg.HashTestPassword(t, code)
	_, err := db.NewUpdate().Model((*operatorChallengeRow)(nil)).
		Set("code_hash = ?", hash).
		Where("id = ?", challengeID).
		Exec(context.Background())
	require.NoError(t, err)
}

// deleteOperatorChallenges clears the operator's codes, so a test that wrote
// its own row leaves the partial unique index free for the next one.
func deleteOperatorChallenges(t *testing.T, db *bun.DB, operatorID int64) {
	t.Helper()
	t.Cleanup(func() {
		_, _ = db.NewDelete().Model((*operatorChallengeRow)(nil)).
			Where("operator_id = ?", operatorID).Exec(context.Background())
	})
}

// operatorRow is the test's read side of platform.operators, so a test can
// assert on the lockout counter the flows reset.
type operatorRow struct {
	bun.BaseModel  `bun:"table:platform.operators,alias:operator"`
	ID             int64      `bun:"id,pk,autoincrement"`
	MFAAttempts    int        `bun:"mfa_attempts,notnull"`
	MFALockedUntil *time.Time `bun:"mfa_locked_until"`
}

func findOperatorRow(t *testing.T, db *bun.DB, operatorID int64) operatorRow {
	t.Helper()
	row := operatorRow{}
	require.NoError(t, db.NewSelect().Model(&row).Where("id = ?", operatorID).Scan(context.Background()))
	return row
}
