package organizationtenancy_test

import (
	"encoding/json"
	"testing"

	"github.com/moto-ogs/project-phoenix/modules/organizationtenancy"
	"github.com/stretchr/testify/require"
)

// Only the branding images leave the legacy settings container (#3838).
func TestSchoolSettingsExportFieldsKeepOnlyTheBrandingImages(t *testing.T) {
	t.Parallel()
	fields := organizationtenancy.SchoolSettingsExportFields{}
	value, err := fields.ProjectExportField("platform.schools", "settings", nil,
		json.RawMessage(`{"logoUrl":"/uploads/login-images/7_logo.png","smtp_password":"synthetic-secret","loginImageUrl":" "}`))
	require.NoError(t, err)
	require.JSONEq(t, `{"logoUrl":"/uploads/login-images/7_logo.png"}`, string(value))

	value, err = fields.ProjectExportField("platform.schools", "settings", nil, json.RawMessage(`{"api_key":"synthetic-secret"}`))
	require.NoError(t, err)
	require.JSONEq(t, "null", string(value))

	_, err = fields.ProjectExportField("platform.organizations", "settings", nil, json.RawMessage(`{}`))
	require.Error(t, err)
	_, err = fields.ProjectExportField("platform.schools", "settings", nil, json.RawMessage(`"not an object"`))
	require.Error(t, err)
}
