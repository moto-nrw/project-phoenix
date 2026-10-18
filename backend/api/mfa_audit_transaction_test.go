package api

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"

	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestOperatorMFAAuditSharesTriggeringTransaction(t *testing.T) {
	t.Parallel()
	for _, action := range []string{"mfa_enrolled", "mfa_disabled", "mfa_admin_override", "mfa_verified", "mfa_failed", "mfa_locked"} {
		t.Run(action, func(t *testing.T) {
			db, ctx, _ := mailTransactionFixture(t)
			module, err := NewAuthTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
			require.NoError(t, err)
			operator := testpkg.CreateTestOperator(t, db)
			if action == "mfa_disabled" {
				require.NoError(t, module.OperatorMFA.EnrollOperatorMFA(ctx, operator.ID))
			}
			var targetID int64
			if action == "mfa_admin_override" {
				targetID = testpkg.CreateTestAccount(t, db, "audit-global-override").ID
			}
			if action == "mfa_verified" || action == "mfa_failed" || action == "mfa_locked" {
				hash, err := securityruntime.HashPassword("123456")
				require.NoError(t, err)
				_, err = module.Auth.CreateOperatorMFAChallenge(ctx, identityaccess.OperatorMFAChallenge{OperatorID: operator.ID, CodeHash: hash, ExpiresAt: time.Now().Add(time.Hour)})
				require.NoError(t, err)
				if action == "mfa_locked" {
					_, err = db.NewRaw(`UPDATE platform.operators SET mfa_attempts=? WHERE id=?`, identityaccess.MFALockoutThreshold-1, operator.ID).Exec(ctx)
					require.NoError(t, err)
				}
			}
			count := func(source bun.IDB, queryCtx context.Context, want int) {
				t.Helper()
				n, err := source.NewSelect().TableExpr("platform.operator_audit_log").Where("operator_id = ? AND action = ?", operator.ID, action).Count(queryCtx)
				require.NoError(t, err)
				assert.EqualValues(t, want, n)
			}
			rollback := errors.New("rollback triggering MFA change")
			trigger := func(commit bool) error {
				return tenant.WithAdminTx(ctx, db, func(txCtx context.Context, tx bun.Tx) error {
					var err error
					switch action {
					case "mfa_enrolled":
						err = module.OperatorMFA.EnrollOperatorMFA(txCtx, operator.ID)
					case "mfa_disabled":
						err = module.OperatorMFA.DisableOperatorMFA(txCtx, operator.ID)
					case "mfa_admin_override":
						err = module.MFA.OperatorSetGlobalMFAOverride(txCtx, operator.ID, targetID, identityaccess.MFAAdminOverrideForceOn, "atomic audit test")
					case "mfa_verified":
						err = module.OperatorMFA.VerifyOperatorMFACode(txCtx, operator.ID, "123456")
					default:
						require.ErrorIs(t, module.OperatorMFA.VerifyOperatorMFACode(txCtx, operator.ID, "000000"), identityaccess.ErrMFACodeInvalid)
					}
					if err != nil {
						return err
					}
					count(tx, txCtx, 1)
					if !commit {
						return rollback
					}
					return nil
				})
			}
			require.ErrorIs(t, trigger(false), rollback)
			count(db, ctx, 0)
			require.NoError(t, trigger(true))
			count(db, ctx, 1)
		})
	}
}

func TestOperatorMFAAuditFailureRollsBackEnrollment(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	db, ctx, _ := mailTransactionFixture(t)
	module, err := NewAuthTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	operator := testpkg.CreateTestOperator(t, db)
	_, err = db.NewRaw(`ALTER TABLE platform.operator_audit_log ADD CONSTRAINT reject_mfa_audit_test CHECK (action <> 'mfa_enrolled')`).Exec(ctx)
	require.NoError(t, err)
	require.Error(t, module.OperatorMFA.EnrollOperatorMFA(ctx, operator.ID))
	var n int64
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM platform.operator_mfa_credentials WHERE operator_id = ?`, operator.ID).Scan(ctx, &n))
	assert.Zero(t, n, "the credential cannot commit without its audit evidence")
}
