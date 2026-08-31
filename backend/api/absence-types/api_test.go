package absencetypes

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"testing"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRouterManagesCustomTypeAllowance(t *testing.T) {
	t.Parallel()

	db, resource := setupAbsenceTypesRoute(t)
	router := resource.Router()
	admin, account := testpkg.CreateTestStaffWithAccount(t, db, "Lea", "Leitung")
	staff := testpkg.CreateTestStaff(t, db, "Rena", "Generation")

	claims := testutil.DefaultTestClaims()
	claims.ID = int(account.ID)
	claims.TenantID = testpkg.Tenant(t)
	claims.Permissions = []string{permissions.TimeTrackingManage}
	claims.IsAdmin = false
	token := testutil.MintTestJWT(t, claims)
	create := testutil.NewAuthenticatedRequest(t, "POST", "/", map[string]any{
		"name": "Regenerationstag", "allowance_enabled": true, "overrun_policy": "block",
	}, testutil.WithJWTBearer(token))
	created := testutil.ExecuteRequest(router, create)
	require.Equal(t, 201, created.Code, created.Body.String())
	var typeEnvelope struct {
		Data struct {
			ID               string `json:"id"`
			AllowanceEnabled bool   `json:"allowance_enabled"`
			OverrunPolicy    string `json:"overrun_policy"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(created.Body.Bytes(), &typeEnvelope))
	assert.True(t, typeEnvelope.Data.AllowanceEnabled)
	assert.Equal(t, "block", typeEnvelope.Data.OverrunPolicy)

	path := fmt.Sprintf("/%s/allowances/%d", typeEnvelope.Data.ID, staff.ID)
	ownClaims := claims
	ownClaims.Permissions = []string{permissions.TimeTrackingOwn}
	ownToken := testutil.MintTestJWT(t, ownClaims)
	denied := testutil.NewAuthenticatedRequest(t, "GET", path+"?year=2026", nil, testutil.WithJWTBearer(ownToken))
	assert.Equal(t, 403, testutil.ExecuteRequest(router, denied).Code)

	set := testutil.NewAuthenticatedRequest(t, "PUT", path, map[string]any{
		"year": 2026, "entitled_days": 2.5, "reason": "Tariflicher Anspruch",
	}, testutil.WithJWTBearer(token))
	saved := testutil.ExecuteRequest(router, set)
	require.Equal(t, 200, saved.Code, saved.Body.String())
	var allowanceEnvelope struct {
		Data struct {
			StaffID       string  `json:"staff_id"`
			EntitledDays  float64 `json:"entitled_days"`
			RemainingDays float64 `json:"remaining_days"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(saved.Body.Bytes(), &allowanceEnvelope))
	assert.Equal(t, fmt.Sprint(staff.ID), allowanceEnvelope.Data.StaffID)
	assert.Equal(t, 2.5, allowanceEnvelope.Data.EntitledDays)
	assert.Equal(t, 2.5, allowanceEnvelope.Data.RemainingDays)
	assert.NotEqual(t, staff.ID, admin.ID)
}

func setupAbsenceTypesRoute(t *testing.T) (*testpkg.DB, *Resource) {
	t.Helper()

	db, serviceFactory := testutil.SetupAPITest(t)
	resource := NewResource(serviceFactory.StaffAbsenceType, db, slog.Default())
	resource.SetActorResolver(func(ctx context.Context) (int64, error) {
		current, err := serviceFactory.UserContext.GetCurrentStaff(ctx)
		if err != nil {
			return 0, err
		}
		return current.ID, nil
	})
	return db, resource
}
