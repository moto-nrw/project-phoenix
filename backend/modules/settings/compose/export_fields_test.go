package compose_test

import (
	"encoding/json"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/settings/compose"
	"github.com/stretchr/testify/require"
)

// The return export keeps the non-secret settings of a school and drops its
// secrets and keys the registry does not know (#3838).
func TestSettingExportFieldsDropSecretsAndUnknownKeys(t *testing.T) {
	t.Parallel()
	fields := compose.NewExportFields()
	row := func(key string) map[string]json.RawMessage {
		return map[string]json.RawMessage{"setting_key": json.RawMessage(`"` + key + `"`)}
	}
	value, err := fields.ProjectExportField("config.setting_values", "value", row("sftp.host"), json.RawMessage(`"sftp.example.invalid"`))
	require.NoError(t, err)
	require.JSONEq(t, `"sftp.example.invalid"`, string(value))
	value, err = fields.ProjectExportField("config.setting_audit", "new_value", row("sftp.host"), json.RawMessage(`"sftp.example.invalid"`))
	require.NoError(t, err)
	require.JSONEq(t, `"sftp.example.invalid"`, string(value))

	for _, secret := range []struct{ table, column, key string }{
		{"config.setting_values", "value", "security.ogs_device_pin"},
		{"config.setting_values", "value", "sftp.password"},
		{"config.setting_audit", "old_value", "sftp.password"},
		{"config.traeger_setting_values", "value", "no.longer.registered"},
	} {
		value, err := fields.ProjectExportField(secret.table, secret.column, row(secret.key), json.RawMessage(`"synthetic-secret-4711"`))
		require.NoError(t, err, secret.key)
		require.JSONEq(t, "null", string(value), secret.key)
	}

	for name, call := range map[string]func() (json.RawMessage, error){
		"foreign table": func() (json.RawMessage, error) {
			return fields.ProjectExportField("platform.schools", "settings", row("sftp.host"), json.RawMessage(`{}`))
		},
		"unknown column": func() (json.RawMessage, error) {
			return fields.ProjectExportField("config.setting_values", "setting_key", row("sftp.host"), json.RawMessage(`"x"`))
		},
		"row without key": func() (json.RawMessage, error) {
			return fields.ProjectExportField("config.setting_values", "value", nil, json.RawMessage(`"x"`))
		},
	} {
		_, err := call()
		require.Error(t, err, name)
	}
}
