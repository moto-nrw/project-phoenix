package presence

import (
	"context"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/stretchr/testify/require"
)

func TestGetSchulhofRoomColor(t *testing.T) {
	t.Parallel()
	color := "#A3D977"
	boom := errors.New("lookup failed")
	for _, tc := range []struct {
		name  string
		color *string
		err   error
	}{
		{"configured", &color, nil}, {"missing", nil, nil}, {"failure", nil, boom},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := t.Context()
			calls := 0
			svc := &service{ServiceDependencies: ServiceDependencies{PrincipalReader: testAttendancePrincipal, YardRoomColor: func(got context.Context) (*string, error) {
				calls++
				require.Equal(t, ctx, got)
				return tc.color, tc.err
			}}}
			got, err := svc.GetSchulhofRoomColor(ctx)
			require.Equal(t, tc.color, got)
			require.ErrorIs(t, err, tc.err)
			require.Equal(t, 1, calls)
			resolved := studentpresence.ResolveYardRoomColor(ctx, svc)
			if tc.err != nil {
				require.Nil(t, resolved)
			} else {
				require.Equal(t, tc.color, resolved)
			}
		})
	}
	svc := &service{}
	got, err := svc.GetSchulhofRoomColor(t.Context())
	require.NoError(t, err)
	require.Nil(t, got)
}
