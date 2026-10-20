package migrations

import (
	"context"
	"errors"
	"fmt"

	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// sqlStateError mimics a driver error with a SQLSTATE so tests can inject
// deadlocks and serialization failures at the commit seam.
type sqlStateError string

func (e sqlStateError) Error() string     { return "injected SQLSTATE " + string(e) }
func (e sqlStateError) Field(byte) string { return string(e) }

func TestRequestChildStorageBackfillRequiresDatabase(t *testing.T) {
	t.Parallel()
	_, err := RunRequestChildStorageBackfill(context.Background(), nil, RequestChildStorageBackfillOptions{})
	require.ErrorContains(t, err, "database is required")
	assert.Equal(t, time.Duration(0), percentile95(nil))
	assert.Equal(t, 3*time.Millisecond, percentile95([]time.Duration{3 * time.Millisecond, time.Millisecond, 2 * time.Millisecond}))
	code, retryable := classifyRetryableSQLError(fmt.Errorf("wrapped: %w", sqlStateError("55P03")))
	assert.True(t, retryable)
	assert.Equal(t, "55P03", code)
	_, retryable = classifyRetryableSQLError(errors.New("plain"))
	assert.False(t, retryable)
}
