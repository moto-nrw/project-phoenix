package api

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/moto-ogs/project-phoenix/modules/identityaccess"
	"github.com/moto-ogs/project-phoenix/modules/identityaccess/sessiontoken"
	"github.com/moto-ogs/project-phoenix/modules/organizationtenancy"
	provisioningAPI "github.com/moto-ogs/project-phoenix/modules/organizationtenancy/inbound/operator"
	"github.com/moto-ogs/project-phoenix/tenant"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestLastSchoolOffboardingWaitsForTraegerAssignment(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	for _, operator := range []bool{false, true} {
		t.Run(fmt.Sprintf("operator=%t", operator), func(t *testing.T) {
			testpkg.OwnTenant(t)
			service, identity := buildOperatorProvisioningWithIdentity(t, db)
			ctx, cancel := context.WithTimeout(provisioningContext(t, db), 20*time.Second)
			defer cancel()
			account := testpkg.CreateTestAccount(t, db, "offboarding-assignment-race")
			var organizationID int64
			require.NoError(t, db.NewRaw(`SELECT organization_id FROM platform.schools WHERE id = ?`, testpkg.Tenant(t)).Scan(ctx, &organizationID))
			testpkg.CreateTestTraegerMember(t, db, organizationID, account.ID)
			_, err := db.ExecContext(ctx, `UPDATE platform.organization_accounts SET status = 'inactive' WHERE organization_id = ? AND account_id = ?`, organizationID, account.ID)
			require.NoError(t, err)
			actor := testpkg.CreateTestOperator(t, db)
			schoolCtx := tenant.WithTenantID(ctx, testpkg.Tenant(t))
			preview, err := identity.PreviewStaffOffboarding(schoolCtx, account.ID)
			require.NoError(t, err)
			require.True(t, preview.DeactivateAccount)
			locked := make(chan int, 1)
			release := make(chan struct{})
			grantDone := make(chan error, 1)
			var workers sync.WaitGroup
			defer func() { cancel(); workers.Wait() }()
			workers.Go(func() {
				grantDone <- testpkg.WithinAdminTransaction(ctx, func(txCtx context.Context) error {
					_, grantErr := service.AssignTraegerMember(txCtx, organizationtenancy.AssignTraegerMember{
						OrganizationID: organizationID, Email: account.Email, Roles: []string{"read_only"}, OperatorID: actor.ID, ClientIP: provisioningTestClientIP,
					})
					if grantErr != nil {
						return grantErr
					}
					txValue, _ := tenant.TransactionFromContext(txCtx)
					var pid int
					if err := txValue.(bun.Tx).NewRaw(`SELECT pg_backend_pid()`).Scan(txCtx, &pid); err != nil {
						return err
					}
					locked <- pid
					select {
					case <-release:
						return nil
					case <-ctx.Done():
						return ctx.Err()
					}
				})
			})
			var pid int
			select {
			case pid = <-locked:
			case err := <-grantDone:
				t.Fatalf("grant before lock gate: %v", err)
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}
			offboardDone := make(chan error, 1)
			workers.Go(func() {
				if operator {
					_, err := identity.RevokeAccountTenantAccess(ctx, account.ID, testpkg.Tenant(t), actor.ID, provisioningTestClientIP.String())
					offboardDone <- err
				} else {
					_, err := identity.ExecuteStaffOffboarding(schoolCtx, account.ID, preview.Revision)
					offboardDone <- err
				}
			})
			waitForTraegerAccountLock(t, ctx, db, pid)
			close(release)
			require.NoError(t, <-grantDone)
			if operator {
				require.NoError(t, <-offboardDone)
			} else {
				require.ErrorIs(t, <-offboardDone, identityaccess.ErrStaffOffboardingConflict)
				fresh, err := identity.PreviewStaffOffboarding(schoolCtx, account.ID)
				require.NoError(t, err)
				require.False(t, fresh.DeactivateAccount)
				require.NotEqual(t, preview.Revision, fresh.Revision)
				_, err = identity.ExecuteStaffOffboarding(schoolCtx, account.ID, fresh.Revision)
				require.NoError(t, err)
			}
			var active bool
			require.NoError(t, db.NewRaw(`SELECT active FROM auth.accounts WHERE id = ?`, account.ID).Scan(ctx, &active))
			require.True(t, active)
			var status string
			require.NoError(t, db.NewRaw(`SELECT status FROM auth.account_tenants WHERE account_id = ? AND tenant_id = ?`, account.ID, testpkg.Tenant(t)).Scan(ctx, &status))
			require.Equal(t, "inactive", status)
		})
	}
}

func waitForTraegerAccountLock(t *testing.T, ctx context.Context, db *bun.DB, holderPID int) {
	t.Helper()
	require.Eventually(t, func() bool {
		var waiting bool
		err := db.NewRaw(`SELECT EXISTS (SELECT 1 FROM pg_stat_activity
			WHERE datname = current_database() AND ? = ANY(pg_blocking_pids(pid)))`, holderPID).Scan(ctx, &waiting)
		return err == nil && waiting
	}, 5*time.Second, 10*time.Millisecond, "the second command must wait on the first command's account lock")
}

// The reverse order of TestLastSchoolOffboardingWaitsForTraegerAssignment
// (#6535): the offboarding of the last school holds the account lock first
// and commits the deactivation; the operator's Träger grant waits on that
// lock and then finds the account deactivated, so it grants nothing.
func TestTraegerAssignmentWaitsForLastSchoolOffboarding(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	for _, operator := range []bool{false, true} {
		t.Run(fmt.Sprintf("operator=%t", operator), func(t *testing.T) {
			testpkg.OwnTenant(t)
			service, identity := buildOperatorProvisioningWithIdentity(t, db)
			ctx, cancel := context.WithTimeout(provisioningContext(t, db), 20*time.Second)
			defer cancel()
			account := testpkg.CreateTestAccount(t, db, "assignment-offboarding-race")
			var organizationID int64
			require.NoError(t, db.NewRaw(`SELECT organization_id FROM platform.schools WHERE id = ?`, testpkg.Tenant(t)).Scan(ctx, &organizationID))
			actor := testpkg.CreateTestOperator(t, db)
			schoolCtx := tenant.WithTenantID(ctx, testpkg.Tenant(t))
			preview, err := identity.PreviewStaffOffboarding(schoolCtx, account.ID)
			require.NoError(t, err)
			require.True(t, preview.DeactivateAccount)

			locked := make(chan int, 1)
			release := make(chan struct{})
			offboardDone := make(chan error, 1)
			var workers sync.WaitGroup
			defer func() { cancel(); workers.Wait() }()
			holdUntilRelease := func(txCtx context.Context, tx bun.IDB) error {
				var pid int
				if err := tx.NewRaw(`SELECT pg_backend_pid()`).Scan(txCtx, &pid); err != nil {
					return err
				}
				locked <- pid
				select {
				case <-release:
					return nil
				case <-ctx.Done():
					return ctx.Err()
				}
			}
			workers.Go(func() {
				if operator {
					offboardDone <- testpkg.WithinAdminTransaction(ctx, func(txCtx context.Context) error {
						if _, err := identity.RevokeAccountTenantAccess(txCtx, account.ID, testpkg.Tenant(t), actor.ID, provisioningTestClientIP.String()); err != nil {
							return err
						}
						txValue, _ := tenant.TransactionFromContext(txCtx)
						return holdUntilRelease(txCtx, txValue.(bun.Tx))
					})
					return
				}
				offboardDone <- testpkg.WithinTenantTransaction(schoolCtx, db, testpkg.Tenant(t), func(txCtx context.Context, tx bun.Tx) error {
					if _, err := identity.ExecuteStaffOffboarding(txCtx, account.ID, preview.Revision); err != nil {
						return err
					}
					return holdUntilRelease(txCtx, tx)
				})
			})
			var pid int
			select {
			case pid = <-locked:
			case err := <-offboardDone:
				t.Fatalf("offboarding before lock gate: %v", err)
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}

			resource := provisioningAPI.NewProvisioningResource(provisioningAPI.ProvisioningConfig{Service: service})
			router := chi.NewRouter()
			mountOperatorProvisioningRoutes(router, resource, nil)
			granted := make(chan *httptest.ResponseRecorder, 1)
			workers.Go(func() {
				body := bytes.NewBufferString(fmt.Sprintf(`{"email":%q,"roles":["read_only"]}`, account.Email))
				req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/organizations/%d/traeger-members", organizationID), body)
				req.Header.Set("Content-Type", "application/json")
				req = req.WithContext(context.WithValue(ctx, sessiontoken.CtxClaims, sessiontoken.AppClaims{ID: int(actor.ID), Scope: "platform"}))
				rec := httptest.NewRecorder()
				router.ServeHTTP(rec, req)
				granted <- rec
			})
			waitForTraegerAccountLock(t, ctx, db, pid)
			close(release)
			require.NoError(t, <-offboardDone)

			rec := <-granted
			assert.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
			assert.Contains(t, rec.Body.String(), `"code":"traeger.account_inactive"`)
			requireNoTraegerGrant(t, db, actor.ID, account.ID)
		})
	}
}
