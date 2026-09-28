package compose

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewErrorReportRelayWithoutDSNIsUnconfigured(t *testing.T) {
	t.Parallel()
	relay, err := NewErrorReportRelay("  ")
	require.NoError(t, err)
	assert.Nil(t, relay)
}

func TestNewErrorReportRelayRejectsMalformedDSN(t *testing.T) {
	t.Parallel()
	_, err := NewErrorReportRelay("https://sentry.example/not-a-project")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "SENTRY_PYREPORTAL_DSN")
}

func TestNewErrorReportRelayAcceptsProjectDSN(t *testing.T) {
	t.Parallel()
	relay, err := NewErrorReportRelay("https://public@o1.ingest.de.sentry.io/4242")
	require.NoError(t, err)
	assert.NotNil(t, relay)
}
