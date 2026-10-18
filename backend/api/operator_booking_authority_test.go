package api

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	settingsOperatorAPI "github.com/moto-nrw/project-phoenix/modules/settings/inbound/operator"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// The operator switches a school to booking-led care in two steps: Care
// Plan's review of the children it would leave without a care day, then the
// settings write whose side effect Care Plan guards (#4072). The root binds
// both to the same Care Plan, so the review and the guard agree.

const operatorBookingsAuthoritativeKey = "enrollment.bookings_authoritative"

// operatorRequest serves one request of the operator whose session the
// routes read.
type operatorRequest func(method, suffix string, body any) *httptest.ResponseRecorder

// newOperatorBookingAuthorityRoutes mounts the review and the settings write
// the way the operator router does, over one module. The operator is a real
// account: the write records it (config.setting_values.updated_by).
func newOperatorBookingAuthorityRoutes(t *testing.T) (*bun.DB, OperatorSettingsTestModule, careplan.CareLifecycle, operatorRequest) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	module, careLifecycle, err := NewOperatorSettingsTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	settingsRoutes := settingsOperatorAPI.NewSettingsResource(settingsOperatorAPI.SettingsConfig{
		Settings:   module.OperatorSchoolSettings(module.SettingsSideEffects.Dispatch),
		OperatorID: operatorIDFromSession,
	})
	router := chi.NewRouter()
	router.Method(http.MethodGet, "/schools/{id}/settings/booking-authority-impact", newBookingAuthorityImpact(careLifecycle))
	router.Put("/schools/{id}/settings/values/{key}", settingsRoutes.SetSchoolSettingValue)
	router.Delete("/schools/{id}/settings/values/{key}", settingsRoutes.ResetSchoolSettingValue)
	account := testpkg.CreateTestAccount(t, db, "operator")
	operator := sessiontoken.AppClaims{ID: int(account.ID), Scope: "platform"}
	serve := func(method, suffix string, body any) *httptest.ResponseRecorder {
		path := fmt.Sprintf("/schools/%d%s", testpkg.Tenant(t), suffix)
		req := testutil.NewAuthenticatedRequest(t, method, path, body, testutil.WithClaims(t, operator))
		return testutil.ExecuteRequest(router, req)
	}
	return db, module, careLifecycle, serve
}

func TestOperatorBookingAuthorityReviewMatchesTheGuardedSwitch(t *testing.T) {
	t.Parallel()
	db, _, _, serve := newOperatorBookingAuthorityRoutes(t)

	review := func() map[string]any {
		rr := serve(http.MethodGet, "/settings/booking-authority-impact", nil)
		testutil.AssertSuccessResponse(t, rr, http.StatusOK)
		data, ok := testutil.ParseJSONResponse(t, rr.Body.Bytes())["data"].(map[string]any)
		require.True(t, ok, "the review answers in the operator envelope")
		return data
	}
	assert.Equal(t, []any{}, review()["blocking_children"], "a school without children has nothing blocking")

	// The review is advisory. A child can enter the unsafe set before the
	// operator confirms; the locked write evaluates the current rows again.
	student := testpkg.CreateTestStudent(t, db, "Ohne", "Buchung", "2a")
	blocking, ok := review()["blocking_children"].([]any)
	require.True(t, ok)
	require.Len(t, blocking, 1, "the child without a care day blocks the switch")
	assert.Equal(t, fmt.Sprint(student.ID), blocking[0].(map[string]any)["student_id"])

	rr := serve(http.MethodPut, "/settings/values/"+operatorBookingsAuthoritativeKey, map[string]any{"value": true})
	testutil.AssertErrorResponse(t, rr, http.StatusConflict)
	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	assert.Equal(t, "settings.booking_authority_blocked", response["code"])
	assert.Contains(t, rr.Body.String(), careplan.ErrBookingAuthorityBlocked.Error(),
		"the operator reads Care Plan's reason")

	overrides, err := db.NewSelect().TableExpr("config.setting_values").
		Where("tenant_id = ?", testpkg.Tenant(t)).
		Where("setting_key = ?", operatorBookingsAuthoritativeKey).
		Count(context.Background())
	require.NoError(t, err)
	assert.Zero(t, overrides, "the blocked write must roll the setting override back")
}

func TestOperatorBookingAuthorityResetClosesTheBookingExpiredTasks(t *testing.T) {
	t.Parallel()
	db, module, careLifecycle, serve := newOperatorBookingAuthorityRoutes(t)
	ctx := testpkg.Ctx(t)
	require.NoError(t, module.Settings.SetValue(ctx, operatorBookingsAuthoritativeKey, true, nil, nil))
	student := testpkg.CreateTestStudent(t, db, "Reset", "Buchungsmodus", "2c")
	testpkg.CreateTestBookingExpiredCareWithdrawal(t, db, student.ID)
	pendingFilter := careplan.CareWithdrawalFilter{StudentID: student.ID, Page: 1, PageSize: 1}
	_, pending, err := careLifecycle.ListPendingWithdrawals(ctx, pendingFilter)
	require.NoError(t, err)
	require.Equal(t, 1, pending, "the booking-expired task must be pending before the reset")

	rr := serve(http.MethodDelete, "/settings/values/"+operatorBookingsAuthoritativeKey, nil)
	testutil.AssertSuccessResponse(t, rr, http.StatusNoContent)

	_, pending, err = careLifecycle.ListPendingWithdrawals(ctx, pendingFilter)
	require.NoError(t, err)
	assert.Zero(t, pending)
}

func TestOperatorIDFromSessionNamesTheAuthenticatedOperator(t *testing.T) {
	t.Parallel()
	operator := sessiontoken.AppClaims{ID: 815, Scope: "platform"}
	req := testutil.NewAuthenticatedRequest(t, http.MethodGet, "/", nil, testutil.WithClaims(t, operator))

	assert.Equal(t, int64(operator.ID), operatorIDFromSession(req.Context()))
	assert.Zero(t, operatorIDFromSession(context.Background()), "without a session there is no operator")
}
