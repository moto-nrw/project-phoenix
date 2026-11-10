//go:build testhooks

package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"

	provisioningHTTP "github.com/moto-nrw/project-phoenix/modules/organizationtenancy/inbound/operator"
	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	"github.com/moto-nrw/project-phoenix/roots/owners"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	querybudgets "github.com/moto-nrw/project-phoenix/test/query-budgets"
)

// TestOperatorBelegRetentionOverviewQueryBudget measures the overview of the
// Beleg retention of a Träger (#4720) on the real operator chain: the
// statements stay the same with 3 and with 8 active suspensions.
func TestOperatorBelegRetentionOverviewQueryBudget(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	provisioning, identity := buildOperatorProvisioningWithIdentity(t, db)
	operatorID := testpkg.CreateTestOperator(t, db).ID
	organizationID, _ := provisionTestSchool(t, db, provisioning, operatorID, "beleg-budget")
	ctx := provisioningContext(t, db)
	auth, err := NewConfiguredSessionAuth(viper.GetString("auth_jwt_secret"), viper.GetDuration("auth_jwt_expiry"), viper.GetDuration("auth_jwt_refresh_expiry"))
	require.NoError(t, err)
	token, _, err := identity.IssueTokensForAuthenticatedOperator(ctx, operatorID, nil, "127.0.0.1", "beleg-budget")
	require.NoError(t, err)
	routes := newOperatorIdentity(auth, identity).routes()
	router := chi.NewRouter()
	router.Use(routes.sessions.Operator(ReadOnlyPreviewMiddleware, SecurityPrincipalMiddleware)...)
	router.Route("/operator", func(r chi.Router) {
		mountOperatorBelegRetentionRoutes(r, provisioningHTTP.NewProvisioningResource(provisioningHTTP.ProvisioningConfig{Service: provisioning, BelegRetentionAudit: belegRetentionAuditOf(t, provisioning)}))
	})
	retention := owners.NewBelegRetention(nil)
	counter := testpkg.CaptureQueriesForContext(t, db)
	suspensions := 0
	for _, n := range []int{3, 8} {
		for suspensions < n {
			require.NoError(t, tenant.WithinAdmin(ctx, func(adminCtx context.Context) error {
				_, err := retention.RecordSuspension(adminCtx, organizationID, operatorID, parentbilling.NewRetentionSuspension{
					BelegType: parentbilling.BelegCareNotice, BelegID: int64(suspensions + 1), Reason: "Prüfung",
					Scope: fmt.Sprintf("Bescheid %d", suspensions+1), Responsible: "Kämmerei",
					ReviewOn: calendar.NewDate(2099, time.January, 1),
				})
				return err
			}))
			suspensions++
		}
		t.Run(fmt.Sprintf("N%d", n), func(t *testing.T) {
			counter.Reset()
			request := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/operator/organizations/%d/beleg-retention", organizationID), nil).
				WithContext(counter.Context(ctx))
			request.Header.Set("Authorization", "Bearer "+token)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			statements := counter.Queries()
			require.Equal(t, http.StatusOK, response.Code, response.Body.String())
			var envelope struct {
				Data parentbilling.BelegRetentionOverview `json:"data"`
			}
			require.NoError(t, json.Unmarshal(response.Body.Bytes(), &envelope))
			require.Len(t, envelope.Data.Suspensions, n)
			t.Logf("MEASURE operator.beleg_retention.overview N%d statements=%d", n, len(statements))
			querybudgets.AssertQueryBudget(t, "operator.beleg_retention.overview", counter.Measurement(statements))
		})
	}
}
