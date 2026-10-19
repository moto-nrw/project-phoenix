package owners

import (
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	identityaccessCompose "github.com/moto-nrw/project-phoenix/modules/identityaccess/compose"
)

// The account authentication refuses to compose without its repositories,
// token codec and audit command (ADR 0059).
func TestNewIdentityAccessWithSessionsRequiresEveryDependency(t *testing.T) {
	t.Parallel()

	_, err := NewIdentityAccessWithSessions(identityaccessCompose.Dependencies{}, sessionRepositories{}, nil, nil, nil, slog.Default())
	require.EqualError(t, err, "identity access composition: repositories, token codec and audit command are required")
}

// A root without an Identity & Access observer gets no observation sink, so
// the module refuses to compose instead of dropping its metrics.
func TestRequiredIdentityAccessObservationWithoutObserver(t *testing.T) {
	t.Parallel()

	assert.Nil(t, requiredIdentityAccessObservation(nil))
	assert.NotNil(t, requiredIdentityAccessObservation(func(string, time.Duration, int64, int64, time.Duration, string, error) {}))
}
