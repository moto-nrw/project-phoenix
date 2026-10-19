package owners

import (
	"context"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/settings"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

type parentSettingsBatchDouble struct {
	parentSettingsBatchSource
	resolve func(context.Context, int64, []string) (*settings.Snapshot, error)
}

func (s parentSettingsBatchDouble) ResolveManyForTenant(ctx context.Context, tenantID int64, keys []string) (*settings.Snapshot, error) {
	return s.resolve(ctx, tenantID, keys)
}

// Losing the optional batch contract silently turns one read into per-key
// reads. A typed nil snapshot must also retain the existing single-read path.
func TestParentSettingsBindingPreservesBatchResults(t *testing.T) {
	t.Parallel()

	testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	tenantID := testpkg.Tenant(t)
	keys := []string{settings.KeyParentRequestReasonPolicy}
	wantErr := errors.New("settings batch unavailable")
	for _, tc := range []struct {
		name     string
		snapshot *settings.Snapshot
		err      error
	}{
		{name: "resolved batch", snapshot: &settings.Snapshot{}},
		{name: "missing snapshot"},
		{name: "failed batch", err: wantErr},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			calls := 0
			source := parentSettingsBatchDouble{resolve: func(gotCtx context.Context, gotTenant int64, gotKeys []string) (*settings.Snapshot, error) {
				calls++
				require.True(t, gotCtx == ctx)
				require.Equal(t, tenantID, gotTenant)
				require.Equal(t, keys, gotKeys)
				return tc.snapshot, tc.err
			}}
			batch, ok := BindParentSettings(source).(ParentSettingsBatch)
			require.True(t, ok, "the workflow must retain the owner's batch capability")
			snapshot, err := batch.ResolveManyForTenant(ctx, tenantID, keys)
			require.ErrorIs(t, err, tc.err)
			require.Equal(t, 1, calls)
			if tc.snapshot == nil {
				require.Nil(t, snapshot, "a typed nil must not become a non-nil read port")
				return
			}
			require.Same(t, tc.snapshot, snapshot)
		})
	}
}
