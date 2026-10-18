package delivery

import (
	"database/sql"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSubscriptionNotFoundPreservesSQLCauseAndRepositoryMarker(t *testing.T) {
	t.Parallel()
	cause := fmt.Errorf("subscription lookup: %w", sql.ErrNoRows)
	translated := translateNotFound(cause)
	require.ErrorIs(t, translated, sql.ErrNoRows)
	var marker interface{ RepositoryNotFound() }
	require.ErrorAs(t, translated, &marker)
	assert.Equal(t, "repository: not found\nsubscription lookup: sql: no rows in result set", translated.Error())
	other := errors.New("connection failed")
	assert.Same(t, other, translateNotFound(other))
	assert.NoError(t, translateNotFound(nil))
}
