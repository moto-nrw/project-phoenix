package api

import (
	"context"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestActiveStudentDataAccessUsesSessionPermissions(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name        string
		granted     []string
		hasStaff    bool
		lookupError error
		wantAdmin   bool
		wantStaff   bool
		wantLookup  bool
	}{
		{name: "admin skips lookup", granted: []string{"admin:*"}, wantAdmin: true},
		{name: "full wildcard skips lookup", granted: []string{"*:*"}, wantAdmin: true},
		{name: "verified staff", granted: []string{"users:read"}, hasStaff: true, wantStaff: true, wantLookup: true},
		{name: "guest stays redacted", wantLookup: true},
		{name: "lookup error stays redacted", hasStaff: true, lookupError: errors.New("staff unavailable"), wantLookup: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx := context.WithValue(context.Background(), sessiontoken.CtxPermissions, tc.granted)
			called := false
			admin, staff := activeAuthorization().StudentDataAccess(ctx, func(lookupContext context.Context) (bool, error) {
				require.Same(t, ctx, lookupContext)
				called = true
				return tc.hasStaff, tc.lookupError
			})
			assert.Equal(t, tc.wantAdmin, admin)
			assert.Equal(t, tc.wantStaff, staff)
			assert.Equal(t, tc.wantLookup, called)
		})
	}
}
