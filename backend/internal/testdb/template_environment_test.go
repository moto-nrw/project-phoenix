package testdb

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTemplateBuildEnvironmentPinsRequiredDatabaseConfiguration(t *testing.T) {
	t.Parallel()
	templateDSN := "postgres://test.invalid/template"
	environment := templateBuildEnvironment([]string{
		"DB_MAX_OPEN_CONNS=0",
		"DB_MAX_IDLE_CONNS=invalid",
		"DB_CONN_MAX_LIFETIME=",
		"DB_CONN_MAX_IDLE_TIME=-1s",
	}, templateDSN)

	effective := make(map[string]string, len(environment))
	for _, entry := range environment {
		name, value, found := strings.Cut(entry, "=")
		require.True(t, found)
		effective[name] = value
	}

	assert.Equal(t, "test", effective["APP_ENV"])
	assert.Equal(t, templateDSN, effective["TEST_DB_DSN"])
	assert.Equal(t, AuthRolePassword, effective["PHOENIX_AUTH_PASSWORD"])
	assert.Equal(t, "4", effective["DB_MAX_OPEN_CONNS"])
	assert.Equal(t, "2", effective["DB_MAX_IDLE_CONNS"])
	assert.Equal(t, "30m", effective["DB_CONN_MAX_LIFETIME"])
	assert.Equal(t, "10m", effective["DB_CONN_MAX_IDLE_TIME"])
}
