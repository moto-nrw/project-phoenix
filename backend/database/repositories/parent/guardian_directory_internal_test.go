package parent

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// TestParentReadsFailClosedWithoutMembershipQuery pins that a parent
// repository composed without the Identity & Access membership query (#2721)
// reports an error before any read instead of widening the school scope.
func TestParentReadsFailClosedWithoutMembershipQuery(t *testing.T) {
	t.Parallel()

	runtime := RuntimeFunc(func(context.Context) bun.IDB {
		t.Fatal("no statement may run without the membership query")
		return nil
	})
	ctx := context.Background()
	accountID := int64(len(t.Name()))

	_, err := NewChildRepository(runtime, nil).ListByAccount(ctx, accountID)
	require.ErrorIs(t, err, errMembershipQueryRequired)

	phases := &EnrollablePhaseRepository{runtime: runtime}
	_, err = phases.GuardianSubmitStatus(ctx, accountID, accountID)
	require.ErrorIs(t, err, errMembershipQueryRequired)
}
