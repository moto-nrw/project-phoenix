package me_test

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	rootAPI "github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	nativeUploads "github.com/moto-nrw/project-phoenix/modules/delivery/objects"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/inbound/me"
	"github.com/moto-nrw/project-phoenix/modules/settings"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// The signed-in app shell reads these school switches from the navigation
// context instead of the settings schema (#4996).

func navigationSwitches(t *testing.T, router chi.Router, accountID int64) map[string]any {
	t.Helper()
	req := testutil.NewAuthenticatedRequest(t, "GET", "/navigation", nil,
		testutil.WithJWTBearer(testutil.MintTestJWT(t, testutil.TeacherTestClaims(int(accountID)))),
	)
	rr := testutil.ExecuteRequest(router, req)
	require.Equal(t, http.StatusOK, rr.Code, "body: %s", rr.Body.String())
	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	data, ok := response["data"].(map[string]any)
	require.True(t, ok, "data must be an object")
	switches, ok := data["school_switches"].(map[string]any)
	require.True(t, ok, "school_switches must be an object")
	return switches
}

func TestNavigation_SchoolSwitchesFollowTheSchoolsSettings(t *testing.T) {
	t.Parallel()
	db, module := setupUserContextModule(t)
	schoolSettings := rootAPI.NewAuthTestSettings(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
	router := me.NewResource(module.Caller(), module, rootAPI.TenantSessionChain, nativeUploads.UploadFiles{}, schoolSettings).Router()
	_, account := testpkg.CreateTestTeacherWithAccount(t, db, "Switch", "Reader")

	assert.Equal(t, map[string]any{
		"parent_news_enabled":      true,
		"parent_invite_mode":       settings.ParentInviteModeDisabled,
		"timetable_day_start_time": "09:00",
		"timetable_day_end_time":   "17:00",
	}, navigationSwitches(t, router, account.ID), "a school without overrides gets the registry defaults")

	ctx := testpkg.Ctx(t)
	require.NoError(t, schoolSettings.SetValue(ctx, settings.KeyParentNewsEnabled, false, nil, nil))
	require.NoError(t, schoolSettings.SetValue(ctx, settings.KeyGuardianParentInviteMode, settings.ParentInviteModeStaffApproval, nil, nil))
	require.NoError(t, schoolSettings.SetValue(ctx, settings.KeyTimetableDayStartTime, "07:30", nil, nil))
	require.NoError(t, schoolSettings.SetValue(ctx, settings.KeyTimetableDayEndTime, "16:15", nil, nil))

	assert.Equal(t, map[string]any{
		"parent_news_enabled":      false,
		"parent_invite_mode":       settings.ParentInviteModeStaffApproval,
		"timetable_day_start_time": "07:30",
		"timetable_day_end_time":   "16:15",
	}, navigationSwitches(t, router, account.ID), "the school's overrides round-trip")
}

type failingSchoolSettings struct{}

func (failingSchoolSettings) ResolveBool(context.Context, string) (bool, error) {
	return false, errors.New("private settings failure")
}

func (failingSchoolSettings) ResolveString(context.Context, string) (string, error) {
	return "", errors.New("private settings failure")
}

// An unreadable switch fails the navigation instead of rendering a guess.
func TestNavigation_UnreadableSchoolSwitchesFailTheRequest(t *testing.T) {
	t.Parallel()
	db, module := setupUserContextModule(t)
	router := me.NewResource(module.Caller(), module, rootAPI.TenantSessionChain, nativeUploads.UploadFiles{}, failingSchoolSettings{}).Router()
	_, account := testpkg.CreateTestTeacherWithAccount(t, db, "Switch", "Failure")

	req := testutil.NewAuthenticatedRequest(t, "GET", "/navigation", nil,
		testutil.WithJWTBearer(testutil.MintTestJWT(t, testutil.TeacherTestClaims(int(account.ID)))),
	)
	rr := testutil.ExecuteRequest(router, req)
	require.Equal(t, http.StatusInternalServerError, rr.Code, "body: %s", rr.Body.String())
	assert.NotContains(t, rr.Body.String(), "private settings failure")
	assert.NotContains(t, rr.Body.String(), "school_switches")
}
