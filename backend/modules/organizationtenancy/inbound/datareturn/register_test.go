package datareturn_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy/inbound/datareturn"
	"github.com/stretchr/testify/require"
)

func TestRegisterIsServedWithoutSession(t *testing.T) {
	t.Parallel()
	router := datareturn.NewResource().Router()

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/register", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "application/json; charset=utf-8", rec.Header().Get("Content-Type"))
	var register organizationtenancy.ReturnRegister
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &register))
	require.Equal(t, organizationtenancy.ExportFormatVersion, register.FormatVersion)
	require.NotEmpty(t, register.Tables)
	require.NotEmpty(t, register.Standards)

}
