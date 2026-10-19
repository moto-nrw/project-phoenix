package owners

import (
	"log/slog"
	"testing"

	"github.com/stretchr/testify/require"
)

// The operator flows refuse to compose without their ledger, identity chain
// rows and owners (ADR 0059): a missing one is a construction error, not a
// failure of the first operator request.
func TestNewOperatorDependenciesRequiresEveryDependency(t *testing.T) {
	t.Parallel()

	_, err := NewOperatorDependencies(operatorRepositories{}, nil, nil, nil, slog.Default())
	require.EqualError(t, err, "operator authentication composition: repositories and owner capabilities are required")
}
