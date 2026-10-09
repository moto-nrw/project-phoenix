package api

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type externalStaffWire struct {
	ID                   int64  `json:"id"`
	IsExternal           bool   `json:"is_external"`
	ExternalOrganization string `json:"external_organization"`
	Person               *struct {
		FirstName string `json:"first_name"`
		LastName  string `json:"last_name"`
		AccountID *int64 `json:"account_id"`
	} `json:"person"`
}

// TestBetreuerRecordsAnExternalCaregiver pins #3823: a Betreuer records an
// external caregiver by name, without an account, and the staff directory
// marks the new entry as external.
func TestBetreuerRecordsAnExternalCaregiver(t *testing.T) {
	t.Parallel()

	ctx := setupStaffCompositionRoute(t)
	ctx.createStaff("Interne", "Kollegin")
	betreuer := testutil.WithJWTBearer(staffCompositionToken(t, betreuerPermissions...))

	rr := testutil.ExecuteRequest(ctx.router, testutil.NewAuthenticatedRequest(t, http.MethodPost, "/staff/externals",
		map[string]any{"first_name": "  Lea ", "last_name": "Gastmann", "organization": " Musikschule "}, betreuer))
	require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())
	var created struct {
		Data externalStaffWire `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &created))
	require.Positive(t, created.Data.ID)
	assert.True(t, created.Data.IsExternal)
	assert.Equal(t, "Musikschule", created.Data.ExternalOrganization)
	require.NotNil(t, created.Data.Person)
	assert.Equal(t, "Lea", created.Data.Person.FirstName)
	assert.Equal(t, "Gastmann", created.Data.Person.LastName)
	assert.Nil(t, created.Data.Person.AccountID, "an external caregiver has no account")

	rr = testutil.ExecuteRequest(ctx.router, testutil.NewAuthenticatedRequest(t, http.MethodGet, "/staff", nil, betreuer))
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	var listed struct {
		Data []externalStaffWire `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &listed))
	external := map[int64]bool{}
	for _, entry := range listed.Data {
		external[entry.ID] = entry.IsExternal
	}
	require.Len(t, external, 2)
	assert.True(t, external[created.Data.ID], "the directory marks the new entry as external")
	for id, isExternal := range external {
		if id != created.Data.ID {
			assert.False(t, isExternal, "regular staff stay unmarked")
		}
	}
}

func TestExternalCaregiverRequiresBothNamesAndCreatePermission(t *testing.T) {
	t.Parallel()

	ctx := setupStaffCompositionRoute(t)
	betreuer := testutil.WithJWTBearer(staffCompositionToken(t, betreuerPermissions...))
	for _, body := range []map[string]any{
		{"first_name": "Lea", "last_name": "  "},
		{"first_name": "", "last_name": "Gastmann"},
	} {
		rr := testutil.ExecuteRequest(ctx.router, testutil.NewAuthenticatedRequest(t, http.MethodPost, "/staff/externals", body, betreuer))
		assert.Equal(t, http.StatusBadRequest, rr.Code, rr.Body.String())
	}

	readOnly := testutil.WithJWTBearer(staffCompositionToken(t, "users:read"))
	rr := testutil.ExecuteRequest(ctx.router, testutil.NewAuthenticatedRequest(t, http.MethodPost, "/staff/externals",
		map[string]any{"first_name": "Lea", "last_name": "Gastmann"}, readOnly))
	assert.Equal(t, http.StatusForbidden, rr.Code, rr.Body.String())

	rr = testutil.ExecuteRequest(ctx.router, testutil.NewAuthenticatedRequest(t, http.MethodGet, "/staff", nil, betreuer))
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	assert.NotContains(t, rr.Body.String(), "Gastmann", "refused requests leave no staff entry behind")
}
