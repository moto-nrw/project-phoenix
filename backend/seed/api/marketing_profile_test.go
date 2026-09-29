package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMarketingPlanTimesFollowReferenceClock(t *testing.T) {
	t.Parallel()
	require.Equal(t, marketingReferenceClock, marketingClock(0))
	for index := range 2 {
		arrival, pickup := marketingPlanTimes(marketingPresent, index)
		assert.Less(t, arrival, marketingReferenceClock, "present children arrived before the reference clock")
		assert.GreaterOrEqual(t, pickup, "14:00", "present children are picked up in the afternoon")
	}
	arrival, pickup := marketingPlanTimes(marketingPickedUp, 0)
	assert.Less(t, arrival, marketingReferenceClock)
	assert.Less(t, pickup, marketingReferenceClock, "picked-up children left before the reference clock")
	assert.Less(t, arrival, pickup)
	arrival, pickup = marketingPlanTimes(marketingNotArrived, 0)
	assert.Greater(t, arrival, marketingReferenceClock, "children not yet arrived are expected after the reference clock")
	assert.GreaterOrEqual(t, pickup, "14:00")

	arrivals, pickups := marketingWeeklySchedules(marketingPresent, 0)
	require.Len(t, arrivals, 5)
	require.Len(t, pickups, 5)
	for index, row := range arrivals {
		assert.Equal(t, index+1, row["weekday"], "every weekday carries the same plan")
		assert.Equal(t, "07:45", row["expected_arrival"])
		assert.Equal(t, "15:00", pickups[index]["pickup_time"])
	}
}

func TestMarketingParentVerificationRejectsForeignChild(t *testing.T) {
	t.Parallel()
	srv := newSeedHTTPTestServer(func(w seedHTTPResponseWriter, r *seedHTTPRequest) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/parent/auth/login" {
			_, _ = fmt.Fprint(w, `{"data":{"access_token":"parent"}}`)
			return
		}
		_, _ = fmt.Fprint(w, `{"data":[{"student_id":"11"},{"student_id":"12"}]}`)
	})
	defer srv.Close()
	rt := &Runtime{Adapter: newSeedTestAdapter(srv.URL), Client: newTestClient(srv.URL, false)}

	err := verifyMarketingParentChildren(t.Context(), rt, "parent@example.test", "secret", []int64{11})

	require.ErrorContains(t, err, "marketing parent sees children [11 12], expected [11]")
}

// The profile is built only through the API. The fake keeps what the seed
// writes and answers reads from it, so the assertions cover the recorded
// state as well as the requests that produced it.
func TestSeedMarketingProfileStep(t *testing.T) {
	t.Parallel()
	mock := newMarketingProfileAPIMock()
	var mu sync.Mutex
	srv := newSeedHTTPTestServer(func(w seedHTTPResponseWriter, r *seedHTTPRequest) {
		mu.Lock()
		defer mu.Unlock()
		if mock.serve(t, w, r) {
			return
		}
		if r.URL.Path == "/auth/login" { // the developer admin in the primary school
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprint(w, `{"access_token":"developer-primary","refresh_token":"refresh"}`)
			return
		}
		t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		w.WriteHeader(seedHTTPStatusNotFound)
	})
	defer srv.Close()

	seeder := NewSeeder(newSeedTestAdapter(srv.URL), newSeedTestRandom(), false, SeedOptions{})
	primary := newRuntime(seeder, "operator@example.test", "operator-password", "1234")
	primary.SetOperatorAuth(AuthRef{Kind: AuthBearer, Token: "operator-token"})
	primary.Bootstrap = &bootstrapSeedState{TenantSlug: DefaultProfileKey}
	primary.FixedSeeder = NewFixedSeeder(seeder.client, false, "")
	primary.FixedSeeder.staffCredentials = []StaffCredentials{{Email: "demo1@mail.de", Password: "sdlXK26%", Name: "Anna Müller"}}

	require.NoError(t, seedMarketingProfileStep{seeder: seeder}.Run(t.Context(), primary))

	profile := primary.AdditionalProfiles[marketingProfileKey]
	require.NotNil(t, profile)
	assertMarketingProfile(t, profile)
	assertMarketingAPIWrites(t, mock, profile)
}

func assertMarketingProfile(t *testing.T, profile *SeedProfile) {
	t.Helper()
	definition := marketingProfileDefinition()
	assert.Equal(t, marketingProfileKey, profile.Key)
	assert.Equal(t, "OGS Sonnenhang", profile.Name)
	assert.Equal(t, SeedOrganizationRef{ID: marketingMockOrganizationID, Name: "Demo-Träger Marketing", Slug: "demo-traeger-marketing"}, profile.Organization)
	assert.Equal(t, "marketing", profile.School.TenantSlug)
	assert.Equal(t, definition.Settings, profile.Settings)
	assert.JSONEq(t, `false`, string(profile.Settings[profileSettingAttendanceNFC].Value))
	assert.JSONEq(t, `true`, string(profile.Settings[profileSettingAttendanceWeb].Value))
	assert.JSONEq(t, `"binary"`, string(profile.Settings[profileSettingPresenceMode].Value))
	assert.Equal(t, definition.Expected, profile.Expected)
	assert.Equal(t, SeedStateScenarios{DefaultPlayer: "web", DefaultMode: "binary"}, profile.Scenarios)

	admins := profile.Credentials.Accounts.Admin
	require.Len(t, admins, 2)
	assert.Equal(t, AccountCredentials{Key: "schul-admin", AccountID: 7101, Email: "marketing-admin@example.test", Password: "Marketing1234%", Name: "Katrin Brandt", StaffID: 7100}, admins[0], "the pipeline signs in with accounts.admin[0]")
	assert.Equal(t, "demo1@mail.de", admins[1].Email)
	assert.Equal(t, "Katrin Brandt", profile.Credentials.SchoolAdmin.Name)
	require.Len(t, profile.Credentials.Accounts.Betreuer, 2)
	assert.Equal(t, []string{"fuchsbau", "eulennest"}, []string{profile.Credentials.Accounts.Betreuer[0].Group, profile.Credentials.Accounts.Betreuer[1].Group})

	entities := profile.Entities
	assert.Len(t, entities.Students, 12)
	assert.Len(t, entities.Guardians, 11)
	assert.Equal(t, map[string]SeedEntityRef{"fuchsbau": {ID: 7201, Name: "Fuchsbau"}, "eulennest": {ID: 7202, Name: "Eulennest"}}, entities.Groups)
	assert.Equal(t, map[string]int{"fuchsbau": 6, "eulennest": 6}, countBy(slices.Collect(maps.Values(entities.Students)), func(student SeedStudent) string { return student.GroupKey }))
	assert.True(t, profile.Devices[webManualDeviceID].Protected)

	parents := profile.Credentials.Parents
	require.GreaterOrEqual(t, len(parents), 3)
	assert.Equal(t, "sarah-yilmaz", parents[0].Key)
	assert.Equal(t, []int64{entities.Students["emir-yilmaz"].ID, entities.Students["elif-yilmaz"].ID}, parents[0].StudentIDs, "one parent account has two children")
	for _, parent := range parents {
		assert.NotEmpty(t, parent.Email)
		assert.Equal(t, defaultSeedParentPassword, parent.Password)
		assert.NotEmpty(t, parent.Name)
		assert.NotZero(t, parent.AccountID)
		assert.Equal(t, entities.Guardians[parent.Key].ID, parent.GuardianID)
	}
}

func assertMarketingAPIWrites(t *testing.T, mock *marketingProfileAPIMock, profile *SeedProfile) {
	t.Helper()
	assert.Equal(t, "Katrin Brandt", mock.adminName)
	presence := make(map[string]int)
	for _, student := range profile.Entities.Students {
		created := mock.students[student.ID]
		require.NotNil(t, created, "student %s was created through the API", student.Key)
		state := mock.attendance[student.ID]
		if state == "" {
			state = "not_arrived"
		}
		presence[state]++
		arrival, pickup := marketingScheduleTimes(t, created)
		switch state {
		case "checked_in":
			assert.Less(t, arrival, marketingReferenceClock, student.Key)
			assert.Greater(t, pickup, marketingReferenceClock, student.Key)
		case "checked_out":
			assert.Less(t, pickup, marketingReferenceClock, student.Key)
		default:
			assert.Greater(t, arrival, marketingReferenceClock, student.Key)
		}
	}
	assert.Equal(t, map[string]int{"checked_in": 7, "checked_out": 2, "not_arrived": 3}, presence)
	sibling := mock.students[profile.Entities.Students["elif-yilmaz"].ID]["guardians"].([]any)[0].(map[string]any)
	assert.InDelta(t, profile.Entities.Guardians["sarah-yilmaz"].ID, sibling["guardian_profile_id"], 0, "the sibling links the existing contact")
	assert.Len(t, mock.parentPasswords, len(profile.Credentials.Parents))
	for _, group := range mock.groups {
		assert.Len(t, group["teacher_ids"], 1, "each group has its caregiver")
	}
	assert.Len(t, mock.photos, 8, "two of three children get a picture, the rest keep initials")
	for id, consent := range mock.photos {
		assert.Equal(t, "true", consent, "student %d photo carries the parents' consent", id)
	}
	assert.ElementsMatch(t, []string{
		marketingMockAdminToken,
		marketingMockStaffPrefix + "miriam.sommer@example.test",
		marketingMockStaffPrefix + "jonas.albrecht@example.test",
	}, mock.avatars, "every staff member uploads their own picture while signed in")
}

// readMarketingPicture checks that an upload carries a PNG in field and
// returns its consent flag.
func readMarketingPicture(t *testing.T, r *seedHTTPRequest, field string) string {
	t.Helper()
	require.NoError(t, r.ParseMultipartForm(1<<20))
	file, _, err := r.FormFile(field)
	require.NoError(t, err)
	defer func() { _ = file.Close() }()
	head := make([]byte, 8)
	_, err = io.ReadFull(file, head)
	require.NoError(t, err)
	assert.Equal(t, "\x89PNG\r\n\x1a\n", string(head), "%s upload is a PNG", field)
	return r.FormValue("consent_acknowledged")
}

// marketingScheduleTimes returns the one arrival and pickup time a child has
// on all five weekdays.
func marketingScheduleTimes(t *testing.T, student map[string]any) (string, string) {
	t.Helper()
	times := func(field, value string) string {
		rows := student[field].([]any)
		require.Len(t, rows, 5)
		clock := ""
		for index, raw := range rows {
			row := raw.(map[string]any)
			assert.InDelta(t, index+1, row["weekday"], 0)
			if clock == "" {
				clock = row[value].(string)
			}
			assert.Equal(t, clock, row[value])
		}
		return clock
	}
	return times("arrival_schedules", "expected_arrival"), times("pickup_schedules", "pickup_time")
}

const (
	marketingMockOrganizationID = 7001
	marketingMockSchoolID       = 7002
	marketingMockAdminToken     = "marketing-admin-token"
	marketingMockDeveloperToken = "marketing-developer-token"
	marketingMockParentPrefix   = "marketing-parent-"
	marketingMockStaffPrefix    = "marketing-staff-"
)

// marketingProfileAPIMock fakes the endpoints the marketing profile drives.
// Chained in front of the full-workflow fixture, it claims only requests of
// the marketing school: its tokens, its school path, and bodies naming it.
type marketingProfileAPIMock struct {
	active          bool
	nextID          int64
	adminName       string
	settings        map[string]json.RawMessage
	groups          []map[string]any
	staff           int
	students        map[int64]map[string]any
	studentOrder    []int64
	guardianOf      map[int64]int64
	guardianEmail   map[int64]string
	attendance      map[int64]string
	parentPasswords map[string]string
	parentGuardian  map[string]int64
	photos          map[int64]string // student id → consent_acknowledged
	avatars         []string         // signed-in accounts that uploaded one
}

func newMarketingProfileAPIMock() *marketingProfileAPIMock {
	return &marketingProfileAPIMock{
		nextID: 7500, settings: make(map[string]json.RawMessage), students: make(map[int64]map[string]any),
		guardianOf: make(map[int64]int64), guardianEmail: make(map[int64]string), attendance: make(map[int64]string),
		parentPasswords: make(map[string]string), parentGuardian: make(map[string]int64), photos: make(map[int64]string),
	}
}

func peekSeedBody(t *testing.T, r *seedHTTPRequest) map[string]any {
	t.Helper()
	if r.Body == nil || r.Method == seedHTTPMethodGet {
		return nil
	}
	raw, err := io.ReadAll(r.Body)
	require.NoError(t, err)
	r.Body = io.NopCloser(bytes.NewReader(raw))
	var body map[string]any
	_ = json.Unmarshal(raw, &body) // non-object bodies are never marketing requests
	return body
}

func (m *marketingProfileAPIMock) claims(r *seedHTTPRequest, body map[string]any) bool {
	auth := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	switch {
	case r.URL.Path == "/operator/organizations" && body["slug"] == "demo-traeger-marketing":
		m.active = true
		return true
	case !m.active:
		return false
	case auth == marketingMockAdminToken || auth == marketingMockDeveloperToken ||
		strings.HasPrefix(auth, marketingMockParentPrefix) || strings.HasPrefix(auth, marketingMockStaffPrefix):
		return true
	case strings.HasPrefix(r.URL.Path, fmt.Sprintf("/operator/schools/%d/", marketingMockSchoolID)):
		return true
	case r.URL.Path == "/operator/schools" && body["slug"] == marketingProfileKey:
		return true
	case strings.HasPrefix(r.URL.Path, "/auth/invitations/marketing-") || strings.HasPrefix(r.URL.Path, "/auth/guardian-invitations/marketing-"):
		return true
	case r.URL.Path == "/auth/login" && strings.HasSuffix(fmt.Sprint(body["email"]), "@example.test"):
		return true
	case r.URL.Path == "/parent/auth/login":
		_, ok := m.parentGuardian[fmt.Sprint(body["email"])]
		return ok
	case r.URL.Path == "/auth/switch-tenant":
		return body["tenant_slug"] == marketingProfileKey
	}
	return false
}

func (m *marketingProfileAPIMock) serve(t *testing.T, w seedHTTPResponseWriter, r *seedHTTPRequest) bool {
	t.Helper()
	body := peekSeedBody(t, r)
	if !m.claims(r, body) {
		return false
	}
	w.Header().Set("Content-Type", "application/json")
	auth := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if r.URL.Path == "/auth/switch-tenant" && auth == marketingMockAdminToken {
		w.WriteHeader(seedHTTPStatusUnauthorized)
		_, _ = fmt.Fprint(w, `{"status":"error","error":"tenant access denied"}`)
		return true
	}
	data, status := m.respond(t, r, body)
	if status != 0 {
		w.WriteHeader(status)
		_, _ = fmt.Fprint(w, `{"status":"error","error":"rejected by fake"}`)
		return true
	}
	if token, ok := data.(marketingMockToken); ok {
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": string(token), "refresh_token": "refresh"})
		return true
	}
	envelope := map[string]any{"status": "success", "data": data}
	if r.Method == seedHTTPMethodGet && r.URL.Path == "/api/students" {
		envelope["pagination"] = map[string]any{"total_records": len(m.students)}
	}
	_ = json.NewEncoder(w).Encode(envelope)
	return true
}

type marketingMockToken string

func (m *marketingProfileAPIMock) id() int64 {
	m.nextID++
	return m.nextID
}

func (m *marketingProfileAPIMock) respond(t *testing.T, r *seedHTTPRequest, body map[string]any) (any, int) {
	t.Helper()
	path := r.URL.Path
	if path == "/api/me/profile/avatar" {
		readMarketingPicture(t, r, "avatar")
		m.avatars = append(m.avatars, strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
		return nil, 0
	}
	if data, ok := m.respondIdentity(r, body); ok {
		return data, 0
	}
	if strings.HasPrefix(path, "/api/students/") || strings.HasPrefix(path, "/api/guardians/") || strings.HasPrefix(path, "/auth/guardian-invitations/") || strings.HasPrefix(path, "/parent/") {
		return m.respondFamily(t, r, body)
	}
	switch {
	case strings.HasSuffix(path, "/settings/schema"):
		items := make([]map[string]any, 0, len(m.settings))
		for key, value := range m.settings {
			items = append(items, map[string]any{"key": key, "value": value})
		}
		return map[string]any{"tabs": []map[string]any{{"categories": []map[string]any{{"items": items}}}}}, 0
	case strings.Contains(path, "/settings/values/"):
		raw, err := json.Marshal(body["value"])
		require.NoError(t, err)
		m.settings[path[strings.LastIndex(path, "/")+1:]] = raw
		return nil, 0
	case path == "/api/groups":
		m.groups = append(m.groups, body)
		return map[string]any{"id": 7200 + len(m.groups)}, 0
	case path == "/api/students" && r.Method == seedHTTPMethodPost:
		return m.createStudent(t, body), 0
	case path == "/api/students":
		return m.studentRows(), 0
	case path == "/api/staff/":
		return make([]map[string]any, m.staff), 0
	case path == "/api/active/visits":
		return []any{}, 0
	case path == "/api/iot/" && r.URL.Query().Get("device_type") == "virtual":
		return []map[string]any{{"id": 7300, "device_id": webManualDeviceID, "device_type": "virtual", "name": "Web-Portal (Manuell)"}}, 0
	case path == "/api/iot/":
		return []any{}, 0
	}
	t.Errorf("marketing fake has no answer for %s %s", r.Method, path)
	return nil, seedHTTPStatusNotFound
}

// respondIdentity answers the organization, school, account, and role calls.
func (m *marketingProfileAPIMock) respondIdentity(r *seedHTTPRequest, body map[string]any) (any, bool) {
	switch r.URL.Path {
	case "/operator/organizations":
		return map[string]any{"id": marketingMockOrganizationID}, true
	case "/operator/schools":
		return map[string]any{"id": marketingMockSchoolID, "subdomain": body["slug"]}, true
	case fmt.Sprintf("/operator/schools/%d/invite-admin", marketingMockSchoolID):
		m.staff++
		return map[string]any{"token": "marketing-admin-invite"}, true
	case "/auth/invitations/marketing-admin-invite/accept":
		return nil, true
	case "/auth/login":
		if body["email"] == "marketing-admin@example.test" {
			return marketingMockToken(marketingMockAdminToken), true
		}
		return marketingMockToken(marketingMockStaffPrefix + fmt.Sprint(body["email"])), true
	case "/api/me/profile":
		m.adminName = fmt.Sprintf("%s %s", body["first_name"], body["last_name"])
		return nil, true
	case "/api/staff/by-role":
		return []map[string]any{{"id": 7100, "account_id": 7101}}, true
	case "/auth/roles":
		return []map[string]any{{"id": "9100", "name": "admin"}, {"id": "9101", "name": "user"}}, true
	case "/auth/register":
		m.staff++
		id := m.id()
		return map[string]any{"id": id, "school_identity": map[string]any{
			"person_id": strconv.FormatInt(id+1, 10), "staff_id": strconv.FormatInt(id+2, 10), "teacher_id": strconv.FormatInt(id+3, 10),
		}}, true
	case "/auth/link-to-tenant":
		m.staff++
		return map[string]any{"school_identity": map[string]any{"staff_id": "7400"}}, true
	case "/auth/switch-tenant":
		return marketingMockToken(marketingMockDeveloperToken), true
	case "/auth/account/tenants":
		return []map[string]any{{"tenant_id": marketingMockSchoolID}}, true
	}
	return nil, false
}

func (m *marketingProfileAPIMock) createStudent(t *testing.T, body map[string]any) map[string]any {
	t.Helper()
	id := m.id()
	contacts := body["guardians"].([]any)
	require.Len(t, contacts, 1)
	contact := contacts[0].(map[string]any)
	if linked, ok := contact["guardian_profile_id"].(float64); ok {
		m.guardianOf[id] = int64(linked)
	} else {
		guardianID := m.id()
		m.guardianOf[id] = guardianID
		m.guardianEmail[guardianID] = contact["email"].(string)
	}
	m.students[id] = body
	m.studentOrder = append(m.studentOrder, id)
	return map[string]any{"id": id}
}

func (m *marketingProfileAPIMock) studentRows() []map[string]any {
	rows := make([]map[string]any, 0, len(m.studentOrder))
	for _, id := range m.studentOrder {
		student := m.students[id]
		row := map[string]any{"id": id, "school_class": student["school_class"], "group_id": student["group_id"], "current_location": "Schule"}
		switch m.attendance[id] {
		case "checked_in":
			row["current_location"] = "Anwesend"
		case "checked_out":
			row["current_location"], row["actual_pickup_time"] = "Abwesend", "2026-09-29T10:00:00+02:00"
		}
		rows = append(rows, row)
	}
	return rows
}

// respondFamily answers the child, contact, attendance, and parent calls.
func (m *marketingProfileAPIMock) respondFamily(t *testing.T, r *seedHTTPRequest, body map[string]any) (any, int) {
	t.Helper()
	path := r.URL.Path
	parts := strings.Split(strings.Trim(path, "/"), "/")
	switch {
	case strings.HasSuffix(path, "/photo"):
		id, err := strconv.ParseInt(parts[2], 10, 64)
		require.NoError(t, err)
		m.photos[id] = readMarketingPicture(t, r, "photo")
		return nil, 0
	case path == "/api/students/arrival-settings":
		return map[string]any{"care_days_source": "weekly_plan"}, 0
	case strings.HasSuffix(path, "/school-checkin"):
		id, err := strconv.ParseInt(parts[2], 10, 64)
		require.NoError(t, err)
		status := "checked_in"
		if body["action"] == "out" {
			status = "checked_out"
		}
		m.attendance[id] = status
		return map[string]any{"status": status, "changed": true}, 0
	case strings.HasSuffix(path, "/arrival-schedules") || strings.HasSuffix(path, "/pickup-schedules"):
		id, err := strconv.ParseInt(parts[2], 10, 64)
		require.NoError(t, err)
		field := strings.ReplaceAll(parts[3], "-", "_")
		return map[string]any{"schedules": m.students[id][field]}, 0
	case strings.HasPrefix(path, "/api/guardians/students/"):
		id, err := strconv.ParseInt(parts[3], 10, 64)
		require.NoError(t, err)
		guardianID := m.guardianOf[id]
		return []map[string]any{{"guardian": map[string]any{"id": guardianID, "email": m.guardianEmail[guardianID]}}}, 0
	case strings.HasSuffix(path, "/invite"):
		return map[string]any{"token": "marketing-guardian-" + parts[2]}, 0
	case strings.HasPrefix(path, "/auth/guardian-invitations/marketing-guardian-"):
		guardianID, err := strconv.ParseInt(strings.TrimPrefix(parts[2], "marketing-guardian-"), 10, 64)
		require.NoError(t, err)
		email := m.guardianEmail[guardianID]
		m.parentGuardian[email] = guardianID
		m.parentPasswords[email] = body["password"].(string)
		return map[string]any{"account_id": guardianID + 1000}, 0
	case path == "/parent/auth/login":
		email := body["email"].(string)
		if m.parentPasswords[email] != body["password"] {
			return nil, seedHTTPStatusUnauthorized
		}
		return map[string]any{"access_token": fmt.Sprintf("%s%d", marketingMockParentPrefix, m.parentGuardian[email])}, 0
	case path == "/parent/me/children":
		guardianID, err := strconv.ParseInt(strings.TrimPrefix(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "), marketingMockParentPrefix), 10, 64)
		require.NoError(t, err)
		children := []map[string]any{}
		for _, id := range m.studentOrder {
			if m.guardianOf[id] == guardianID {
				children = append(children, map[string]any{"student_id": strconv.FormatInt(id, 10)})
			}
		}
		return children, 0
	}
	t.Errorf("marketing fake has no answer for %s %s", r.Method, path)
	return nil, seedHTTPStatusNotFound
}
