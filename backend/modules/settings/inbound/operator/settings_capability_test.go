package operator_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/settings"
	settingsoperator "github.com/moto-nrw/project-phoenix/modules/settings/inbound/operator"
)

// The handler tests below drive the routes over a fake capability: they pin
// how the HTTP adapter maps the capability's results, independent of the
// settings rows the integration tests write.

const fakeSchoolID int64 = 4711

type setValueCall struct {
	schoolID  int64
	key       string
	value     any
	changedBy int64
	force     bool
}

type fakeOperatorSchoolSettings struct {
	schema    json.RawMessage
	setErr    error
	impactErr error
	setCalls  []setValueCall
}

func (f *fakeOperatorSchoolSettings) CheckOperatorWritable(string) error { return nil }

func (f *fakeOperatorSchoolSettings) Schema(context.Context, int64) (json.RawMessage, error) {
	return f.schema, nil
}

func (f *fakeOperatorSchoolSettings) Reveal(context.Context, int64, string) (any, error) {
	return nil, nil
}

func (f *fakeOperatorSchoolSettings) SetValue(_ context.Context, schoolID int64, key string, value any, changedBy int64, force bool) error {
	f.setCalls = append(f.setCalls, setValueCall{schoolID: schoolID, key: key, value: value, changedBy: changedBy, force: force})
	return f.setErr
}

func (f *fakeOperatorSchoolSettings) ResetValue(context.Context, int64, string, int64) error {
	return nil
}

func (f *fakeOperatorSchoolSettings) BookingAuthorityImpact(context.Context, int64) (*careplan.BookingAuthorityImpact, error) {
	return nil, f.impactErr
}

func fakeSettingsRouter(fake *fakeOperatorSchoolSettings) chi.Router {
	resource := settingsoperator.NewSettingsResource(settingsoperator.SettingsConfig{Settings: fake})
	router := chi.NewRouter()
	router.Get("/schools/{id}/settings/schema", resource.GetSchoolSettingsSchema)
	router.Get("/schools/{id}/settings/booking-authority-impact", resource.GetBookingAuthorityImpact)
	router.Put("/schools/{id}/settings/values/{key}", resource.SetSchoolSettingValue)
	return router
}

func fakeSchoolPath(suffix string) string {
	return fmt.Sprintf("/schools/%d%s", fakeSchoolID, suffix)
}

func TestOperatorSettingsRoutes_SchemaRendersCapabilityJSON(t *testing.T) {
	t.Parallel()
	fake := &fakeOperatorSchoolSettings{schema: json.RawMessage(`{"tabs":[{"key":"operations"}]}`)}

	req := newOperatorRequest(t, http.MethodGet, fakeSchoolPath("/settings/schema"), nil)
	rr := testutil.ExecuteRequest(fakeSettingsRouter(fake), req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	assert.Equal(t, map[string]any{"tabs": []any{map[string]any{"key": "operations"}}}, response["data"])
}

func TestOperatorSettingsRoutes_BookingAuthorityImpactUnavailable(t *testing.T) {
	t.Parallel()
	fake := &fakeOperatorSchoolSettings{impactErr: settings.ErrBookingAuthorityImpactUnavailable}

	req := newOperatorRequest(t, http.MethodGet, fakeSchoolPath("/settings/booking-authority-impact"), nil)
	rr := testutil.ExecuteRequest(fakeSettingsRouter(fake), req)

	testutil.AssertErrorResponse(t, rr, http.StatusInternalServerError)
	assert.Contains(t, rr.Body.String(), "Booking authority impact service is not configured")
}

func TestOperatorSettingsRoutes_SetValuePassesOperatorAndForce(t *testing.T) {
	t.Parallel()
	fake := &fakeOperatorSchoolSettings{}
	claims := operatorTestClaims()

	req := testutil.NewAuthenticatedRequest(t, http.MethodPut,
		fakeSchoolPath("/settings/values/")+settings.KeyPresenceMode+"?force=true",
		map[string]any{"value": presenceModeBinary}, testutil.WithClaims(t, claims))
	rr := testutil.ExecuteRequest(fakeSettingsRouter(fake), req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
	require.Len(t, fake.setCalls, 1)
	assert.Equal(t, setValueCall{
		schoolID:  fakeSchoolID,
		key:       settings.KeyPresenceMode,
		value:     presenceModeBinary,
		changedBy: int64(claims.ID),
		force:     true,
	}, fake.setCalls[0])
}

func TestOperatorSettingsRoutes_SetValueConflicts(t *testing.T) {
	t.Parallel()
	for name, sentinel := range map[string]error{
		"presence mode switch blocked": settings.ErrPresenceModeSwitchBlocked,
		"booking authority blocked":    careplan.ErrBookingAuthorityBlocked,
	} {
		t.Run(name, func(t *testing.T) {
			fake := &fakeOperatorSchoolSettings{setErr: fmt.Errorf("wrapped: %w", sentinel)}

			req := newOperatorRequest(t, http.MethodPut, fakeSchoolPath("/settings/values/checkout.wc_enabled"),
				map[string]any{"value": true})
			rr := testutil.ExecuteRequest(fakeSettingsRouter(fake), req)

			testutil.AssertErrorResponse(t, rr, http.StatusConflict)
			assert.Contains(t, rr.Body.String(), sentinel.Error())
		})
	}
}
