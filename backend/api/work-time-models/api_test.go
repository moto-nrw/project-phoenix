package worktimemodels

import (
	"net/http"
	"testing"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/models/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRouter_RejectsUsersRead(t *testing.T) {
	testutil.SeedTestJWTConfig()
	resource := &Resource{}
	router := resource.Router()
	claims := testutil.DefaultTestClaims()
	claims.Roles = []string{"user"}
	claims.Permissions = []string{"users:read"}
	claims.IsAdmin = false
	token := testutil.MintTestJWT(t, claims)

	for _, tc := range []struct {
		method string
		path   string
	}{
		{method: http.MethodGet, path: "/"},
		{method: http.MethodGet, path: "/123"},
		{method: http.MethodPost, path: "/"},
		{method: http.MethodPut, path: "/123"},
		{method: http.MethodDelete, path: "/123"},
	} {
		req := testutil.NewAuthenticatedRequest(t, tc.method, tc.path, nil, testutil.WithJWTBearer(token))
		rr := testutil.ExecuteRequest(router, req)

		require.Equal(t, http.StatusForbidden, rr.Code)
	}
}

func TestBuildModelAndEntries_RejectsInvalidEntries(t *testing.T) {
	valid := ModelRequest{
		Name:               "Invalid entry test",
		RotationLength:     1,
		RotationAnchorDate: "2026-06-01",
	}

	tests := []struct {
		name  string
		entry EntryRequest
	}{
		{
			name: "day outside week",
			entry: EntryRequest{
				WeekIndex:     0,
				DayOfWeek:     7,
				TargetMinutes: 300,
			},
		},
		{
			name: "week outside rotation",
			entry: EntryRequest{
				WeekIndex:     1,
				DayOfWeek:     config.DayMonday,
				TargetMinutes: 300,
			},
		},
		{
			name: "minutes too large",
			entry: EntryRequest{
				WeekIndex:     0,
				DayOfWeek:     config.DayMonday,
				TargetMinutes: 721,
			},
		},
		{
			name: "negative minutes",
			entry: EntryRequest{
				WeekIndex:     0,
				DayOfWeek:     config.DayMonday,
				TargetMinutes: -1,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := valid
			req.Entries = []EntryRequest{tc.entry}
			_, _, err := buildModelAndEntries(req)
			require.Error(t, err)
		})
	}
}

func TestBuildModelAndEntries_RejectsDuplicateEntries(t *testing.T) {
	_, _, err := buildModelAndEntries(ModelRequest{
		Name:               "Duplicate entry test",
		RotationLength:     1,
		RotationAnchorDate: "2026-06-01",
		Entries: []EntryRequest{
			{
				WeekIndex:     0,
				DayOfWeek:     config.DayMonday,
				TargetMinutes: 300,
			},
			{
				WeekIndex:     0,
				DayOfWeek:     config.DayMonday,
				TargetMinutes: 240,
			},
		},
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate")
}
