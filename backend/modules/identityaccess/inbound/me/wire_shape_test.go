package me_test

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// The wire goldens below pin the JSON the /api/me read routes rendered before
// the caller-context cutover (#3501). Expected keys come from the retained
// implementation: users.PersonAccount for GET /api/me, the profile map built
// by the legacy user-context service for GET /api/me/profile, the embedded
// education.Group plus via_substitution for GET /api/me/groups, and
// usercontext.NavigationContext for GET /api/me/navigation.

func sortedKeys(object map[string]any) []string {
	keys := make([]string, 0, len(object))
	for key := range object {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func sorted(keys ...string) []string {
	sort.Strings(keys)
	return keys
}

// assertInstant compares an RFC 3339 timestamp by instant: the offset it is
// rendered with follows the process time zone, not the contract.
func assertInstant(t *testing.T, want time.Time, got any) {
	t.Helper()
	text, ok := got.(string)
	require.True(t, ok, "timestamp must be a string, got %T", got)
	parsed, err := time.Parse(time.RFC3339, text)
	require.NoError(t, err)
	assert.True(t, want.Equal(parsed), "want %s, got %s", want, text)
}

func getData(t *testing.T, tc *testContext, path string, accountID int64, message string) any {
	t.Helper()
	claims := testutil.TeacherTestClaims(int(accountID))
	req := testutil.NewAuthenticatedRequest(t, "GET", path, nil,
		testutil.WithJWTBearer(testutil.MintTestJWT(t, claims)),
	)
	rr := testutil.ExecuteRequest(tc.router, req)
	require.Equal(t, http.StatusOK, rr.Code, "body: %s", rr.Body.String())

	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	assert.Equal(t, sorted("status", "data", "message"), sortedKeys(response))
	assert.Equal(t, "success", response["status"])
	assert.Equal(t, message, response["message"])
	return response["data"]
}

func TestWireShape_CurrentUserAndProfile(t *testing.T) {
	t.Parallel()
	tc := setupUserContextRoute(t)

	person, account := testpkg.CreateTestPersonWithAccount(t, tc.db, "Wire", "Shape")
	card := testpkg.CreateTestRFIDCard(t, tc.db, "WIRESHAPE")
	testpkg.LinkRFIDToStudent(t, tc.db, person.ID, card.ID)

	username := fmt.Sprintf("wire_%d", account.ID)
	avatar := fmt.Sprintf("/uploads/avatars/global/%d_wire.png", account.ID)
	lastLogin := time.Date(2026, 9, 1, 8, 0, 0, 0, time.UTC)
	_, err := tc.db.ExecContext(context.Background(),
		`UPDATE auth.accounts SET username = ?, avatar = ?, last_login = ? WHERE id = ?`,
		username, avatar, lastLogin, account.ID,
	)
	require.NoError(t, err)
	_, err = tc.db.ExecContext(context.Background(),
		`INSERT INTO users.profiles (account_id, tenant_id, bio, settings) VALUES (?, ?, ?, ?::jsonb)`,
		account.ID, testpkg.Tenant(t), "Wire bio", `{"theme":"dark"}`,
	)
	require.NoError(t, err)

	t.Run("GET /api/me", func(t *testing.T) {
		data, ok := getData(t, tc, "/", account.ID, "Current user retrieved successfully").(map[string]any)
		require.True(t, ok, "data must be an object")
		assert.Equal(t, sorted(
			"id", "created_at", "updated_at", "email", "avatar", "active", "last_login",
			"username",
			"is_password_otp",
		), sortedKeys(data))
		assert.Equal(t, float64(account.ID), data["id"])
		assert.Equal(t, username, data["username"])
		assert.Equal(t, avatar, data["avatar"])
		assertInstant(t, lastLogin, data["last_login"])
	})

	t.Run("GET /api/me/profile with person", func(t *testing.T) {
		data, ok := getData(t, tc, "/profile", account.ID, "Current profile retrieved successfully").(map[string]any)
		require.True(t, ok, "data must be an object")
		assert.Equal(t, sorted(
			"email", "username", "last_login", "avatar", "id", "first_name", "last_name",
			"created_at", "updated_at", "rfid_card", "bio", "settings",
		), sortedKeys(data))
		assert.Equal(t, float64(person.ID), data["id"], "id is the person ID when a person exists")
		assert.Equal(t, "Wire", data["first_name"])
		assert.Equal(t, "Shape", data["last_name"])
		assert.Equal(t, card.ID, data["rfid_card"])
		assert.Equal(t, username, data["username"])
		assert.Equal(t, avatar, data["avatar"])
		assertInstant(t, lastLogin, data["last_login"])
		assert.Equal(t, "Wire bio", data["bio"])
		assert.Equal(t, `{"theme": "dark"}`, data["settings"], "settings is the jsonb text, not an object")
	})
}

func TestWireShape_ProfileAccountFallback(t *testing.T) {
	t.Parallel()
	tc := setupUserContextRoute(t)

	account := testpkg.CreateTestAccount(t, tc.db, "wire-fallback@example.com")
	_, err := tc.db.ExecContext(context.Background(),
		`UPDATE auth.accounts SET username = NULL, avatar = NULL, last_login = NULL WHERE id = ?`, account.ID)
	require.NoError(t, err)

	data, ok := getData(t, tc, "/profile", account.ID, "Current profile retrieved successfully").(map[string]any)
	require.True(t, ok, "data must be an object")
	// username and last_login are always present (null when unset); avatar,
	// rfid_card, bio and settings only when non-empty.
	assert.Equal(t, sorted(
		"email", "username", "last_login", "id", "first_name", "last_name", "created_at", "updated_at",
	), sortedKeys(data))
	assert.Equal(t, float64(account.ID), data["id"], "id falls back to the account ID")
	assert.Equal(t, "", data["first_name"])
	assert.Equal(t, "", data["last_name"])
	assert.Nil(t, data["username"])
	assert.Nil(t, data["last_login"])
}

func TestWireShape_GroupsAndNavigation(t *testing.T) {
	t.Parallel()
	tc := setupUserContextRoute(t)

	teacher, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "Wire", "Navigation")
	group := testpkg.CreateTestEducationGroup(t, tc.db, "WireGroup")
	testpkg.CreateTestGroupTeacher(t, tc.db, group.ID, teacher.ID)

	activityGroup := testpkg.CreateTestActivityGroup(t, tc.db, "WireActivity")
	room := testpkg.CreateTestRoom(t, tc.db, "WireRoom")
	session := testpkg.CreateTestActiveGroup(t, tc.db, activityGroup.ID, room.ID)
	testpkg.CreateTestGroupSupervisor(t, tc.db, teacher.Staff.ID, session.ID, "supervisor")

	groupKeys := sorted("id", "created_at", "updated_at", "tenant_id", "name", "via_substitution")

	t.Run("GET /api/me/groups", func(t *testing.T) {
		data, ok := getData(t, tc, "/groups", account.ID, "Educational groups retrieved successfully").([]any)
		require.True(t, ok, "data must be an array")
		require.Len(t, data, 1)
		item, ok := data[0].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, groupKeys, sortedKeys(item), "the education group JSON is flattened next to via_substitution")
		assert.Equal(t, float64(group.ID), item["id"])
		assert.Equal(t, group.Name, item["name"])
		assert.Equal(t, false, item["via_substitution"])
	})

	t.Run("GET /api/me/navigation", func(t *testing.T) {
		data, ok := getData(t, tc, "/navigation", account.ID, "Navigation context retrieved successfully").(map[string]any)
		require.True(t, ok, "data must be an object")
		assert.Equal(t, sorted(
			"educational_groups", "supervised_groups", "current_staff", "incomplete", "unavailable_sections",
		), sortedKeys(data))
		assert.Equal(t, false, data["incomplete"])
		assert.Equal(t, []any{}, data["unavailable_sections"], "an empty section list renders as [], not null")

		groups, ok := data["educational_groups"].([]any)
		require.True(t, ok, "educational_groups must be an array")
		require.Len(t, groups, 1)
		item, ok := groups[0].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, groupKeys, sortedKeys(item))
		assert.Equal(t, float64(group.ID), item["id"])

		supervised, ok := data["supervised_groups"].([]any)
		require.True(t, ok, "supervised_groups must be an array")
		require.Len(t, supervised, 1)
		sessionItem, ok := supervised[0].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, float64(session.ID), sessionItem["id"])

		staff, ok := data["current_staff"].(map[string]any)
		require.True(t, ok, "current_staff must be an object for a staff member")
		assert.Equal(t, float64(teacher.Staff.ID), staff["id"])
		assert.NotContains(t, staff, "personnel_number")
	})
}

func TestWireShape_NavigationWithoutStaff(t *testing.T) {
	t.Parallel()
	tc := setupUserContextRoute(t)

	_, account := testpkg.CreateTestPersonWithAccount(t, tc.db, "Wire", "NoStaff")

	data, ok := getData(t, tc, "/navigation", account.ID, "Navigation context retrieved successfully").(map[string]any)
	require.True(t, ok, "data must be an object")
	assert.Equal(t, sorted(
		"educational_groups", "supervised_groups", "current_staff", "incomplete", "unavailable_sections",
	), sortedKeys(data))
	assert.Equal(t, []any{}, data["educational_groups"])
	assert.Equal(t, []any{}, data["supervised_groups"])
	assert.Nil(t, data["current_staff"], "a caller who is no staff member renders current_staff as null")
	assert.Equal(t, false, data["incomplete"])
	assert.Equal(t, []any{}, data["unavailable_sections"])
}
