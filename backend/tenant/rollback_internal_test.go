package tenant

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestRollbackMarker_Lifecycle covers the marker plumbing that lets a
// handler request a tx rollback even after rendering a non-5xx response.
func TestRollbackMarker_Lifecycle(t *testing.T) {
	t.Parallel()

	t.Run("marker is added once and reused on re-wrap", func(t *testing.T) {
		base := context.Background()
		ctx1 := WithRollbackMarker(base)
		assert.False(t, RollbackRequested(ctx1), "a fresh marker must start un-requested")

		// Re-wrapping must NOT install a second marker — MarkRollback on the
		// re-wrapped ctx has to flip the SAME flag the outer ctx observes.
		ctx2 := WithRollbackMarker(ctx1)
		MarkRollback(ctx2)
		assert.True(t, RollbackRequested(ctx1),
			"re-wrap must reuse the existing marker so the outer ctx sees the request")
	})

	t.Run("MarkRollback flips the flag", func(t *testing.T) {
		ctx := WithRollbackMarker(context.Background())
		assert.False(t, RollbackRequested(ctx))
		MarkRollback(ctx)
		assert.True(t, RollbackRequested(ctx))
	})

	t.Run("MarkRollback without a marker is a safe no-op", func(t *testing.T) {
		// A handler running outside the tenant tx middleware (no marker in ctx)
		// must not panic when it calls MarkRollback.
		assert.NotPanics(t, func() { MarkRollback(context.Background()) })
		assert.False(t, RollbackRequested(context.Background()))
	})
}
