package e2e

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/e2e/scenarios"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHostsForScenario_DefaultPrepareScenario(t *testing.T) {
	hosts, err := HostsForScenario(scenarios.DefaultPrepareScenario().Name)
	require.NoError(t, err)

	assert.Equal(t, []string{
		"localtest.me",
		"demo-school.localtest.me",
		"operator.localtest.me",
		"second-school.localtest.me",
	}, hosts)
}

func TestHostsForScenario_UnknownScenario(t *testing.T) {
	hosts, err := HostsForScenario("definitely-not-a-scenario")
	require.Error(t, err)
	assert.Nil(t, hosts)
	assert.Contains(t, err.Error(), "unknown e2e scenario")
}
