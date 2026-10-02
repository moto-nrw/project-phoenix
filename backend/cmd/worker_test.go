package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkerCmd_IsRegisteredBesideServe(t *testing.T) {
	t.Parallel()
	command, _, err := RootCmd.Find([]string{"worker"})
	require.NoError(t, err)
	assert.Same(t, workerCmd, command)
	assert.NotNil(t, workerCmd.RunE)
	assert.Contains(t, workerCmd.Long, "/ready")
}

func TestServeCmd_EmbeddedWorkerStaysOnUntilTurnedOff(t *testing.T) {
	t.Parallel()
	flag := serveCmd.Flags().Lookup("embedded-worker")
	require.NotNil(t, flag)
	assert.Equal(t, "true", flag.DefValue, "serve keeps running the jobs until the cutover turns them off")
}
