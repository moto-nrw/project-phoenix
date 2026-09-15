package parent_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	configModels "github.com/moto-nrw/project-phoenix/models/config"
	configService "github.com/moto-nrw/project-phoenix/services/config"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// cutoffPassedSettings keeps every parent feature on and sets the same-day
// pickup cutoff to midnight, so today is closed at any time the test runs.
type cutoffPassedSettings struct{ configService.SettingsService }

func (cutoffPassedSettings) ResolveBoolForTenant(ctx context.Context, tenantID int64, key string) (bool, error) {
	return alwaysOnSettings{}.ResolveBoolForTenant(ctx, tenantID, key)
}

func (cutoffPassedSettings) ResolveStringForTenant(_ context.Context, _ int64, key string) (string, error) {
	if key == configModels.KeyParentPickupChangeCutoffTime {
		return "00:00", nil
	}
	return "", nil
}

func (s cutoffPassedSettings) ResolveStringForTenantInTx(ctx context.Context, tenantID int64, key string) (string, error) {
	if key == configModels.KeyParentPickupChangeEnabled {
		return "true", nil
	}
	return s.ResolveStringForTenant(ctx, tenantID, key)
}

// #3163: a client that skips the portal's lock still gets refused, with a
// code of its own rather than a generic 400.
func TestDeleteCareExceptionEndpoint_RejectsTodayAfterCutoff(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	router := newWriteRouterWithSettings(t, db, cutoffPassedSettings{})
	token := parentToken(t, chain.AccountID)
	sid := strconv.FormatInt(chain.StudentID, 10)

	berlin, err := time.LoadLocation("Europe/Berlin")
	require.NoError(t, err)
	now := time.Now().In(berlin)
	today := now.Format("2006-01-02")
	tomorrow := time.Date(now.Year(), now.Month(), now.Day()+1, 12, 0, 0, 0, berlin).Format("2006-01-02")

	rr := doRequest(t, router, http.MethodDelete,
		"/me/children/"+sid+"/care-exception?date="+today, token, nil)
	require.Equal(t, http.StatusConflict, rr.Code, rr.Body.String())
	var body map[string]any
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
	assert.Equal(t, "pickup_change_cutoff_passed", body["code"])

	// Tomorrow has no cutoff.
	rr = doRequest(t, router, http.MethodDelete,
		"/me/children/"+sid+"/care-exception?date="+tomorrow, token, nil)
	assert.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
}
