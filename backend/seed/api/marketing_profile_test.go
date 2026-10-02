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
	"time"

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
	assertMarketingDailyLife(t, mock, profile)
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
	assert.JSONEq(t, `"detailed"`, string(profile.Settings[profileSettingPresenceMode].Value), "rooms record where the children are")
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
	distinct := map[string]bool{}
	for _, picture := range mock.pictures {
		distinct[picture] = true
	}
	assert.Len(t, distinct, len(mock.pictures), "no two people of the school share a picture")
}

// readMarketingPicture checks that an upload carries a PNG in field and
// returns its consent flag and the picture.
func readMarketingPicture(t *testing.T, r *seedHTTPRequest, field string) (string, string) {
	t.Helper()
	require.NoError(t, r.ParseMultipartForm(1<<20))
	file, _, err := r.FormFile(field)
	require.NoError(t, err)
	defer func() { _ = file.Close() }()
	content, err := io.ReadAll(file)
	require.NoError(t, err)
	assert.True(t, bytes.HasPrefix(content, []byte("\x89PNG\r\n\x1a\n")), "%s upload is a PNG", field)
	return r.FormValue("consent_acknowledged"), string(content)
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
	pictures        []string         // every uploaded picture, children and staff
	rooms           map[int64]string // room id → name
	activities      map[int64]map[string]any
	instances       map[int64]map[string]any // planned blocks, by id
	started         map[int64]bool
	inRoom          map[int64]int64 // student id → running block
	notices         []map[string]any
	news            map[int64]bool // announcement id → published
	teamMessages    []marketingMockMessage
	parentMessages  []marketingMockMessage
	staffReplies    []marketingMockMessage
	parentRequests  []marketingMockRequest
	mealPlan        map[string][]any // date → dishes
	appointments    []map[string]any
	setupCompleted  bool
	setupDismissed  bool
}

// marketingMockMessage is one message the fake received: who sent it
// (bearer token), where to, and the text.
type marketingMockMessage struct {
	sender, target, body string
}

// marketingMockRequest is one request a family filed in the parents portal.
type marketingMockRequest struct {
	sender, kind, student string
	body                  map[string]any
}

func newMarketingProfileAPIMock() *marketingProfileAPIMock {
	return &marketingProfileAPIMock{
		nextID: 7500, settings: make(map[string]json.RawMessage), students: make(map[int64]map[string]any),
		guardianOf: make(map[int64]int64), guardianEmail: make(map[int64]string), attendance: make(map[int64]string),
		parentPasswords: make(map[string]string), parentGuardian: make(map[string]int64), photos: make(map[int64]string),
		rooms: make(map[int64]string), activities: make(map[int64]map[string]any), instances: make(map[int64]map[string]any),
		started: make(map[int64]bool), inRoom: make(map[int64]int64), news: make(map[int64]bool),
		mealPlan: make(map[string][]any),
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
		_, picture := readMarketingPicture(t, r, "avatar")
		m.avatars = append(m.avatars, strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
		m.pictures = append(m.pictures, picture)
		return nil, 0
	}
	if data, ok := m.respondIdentity(r, body); ok {
		return data, 0
	}
	if data, ok := m.respondDailyLife(t, r, body); ok {
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
			if block, ok := m.inRoom[id]; ok {
				row["current_location"] = "Anwesend - " + m.rooms[int64(m.instances[block]["room_id"].(float64))]
			}
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
		consent, picture := readMarketingPicture(t, r, "photo")
		m.photos[id] = consent
		m.pictures = append(m.pictures, picture)
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
	case strings.HasPrefix(path, "/parent/me/messages/children/"):
		m.parentMessages = append(m.parentMessages, marketingMockMessage{
			sender: strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "), target: parts[4], body: body["body"].(string),
		})
		return map[string]any{"thread_id": strconv.Itoa(8000 + len(m.parentMessages)), "messages": []map[string]any{{"id": "9001"}}}, 0
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

// respondDailyLife answers the rooms, the planned blocks, the messages, the
// notice, the news, and the onboarding wizard of the marketing school.
func (m *marketingProfileAPIMock) respondDailyLife(t *testing.T, r *seedHTTPRequest, body map[string]any) (any, bool) {
	t.Helper()
	path := r.URL.Path
	parts := strings.Split(strings.Trim(path, "/"), "/")
	sender := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	switch {
	case path == "/api/rooms":
		id := m.id()
		m.rooms[id] = body["name"].(string)
		return map[string]any{"id": id}, true
	case path == "/api/activities/categories":
		return []map[string]any{{"id": 51, "name": "Sport"}, {"id": 54, "name": "Spiele"}}, true
	case path == "/api/activities":
		id := m.id()
		m.activities[id] = body
		return map[string]any{"id": id}, true
	case path == "/api/timetable/periods/bootstrap":
		return nil, true
	case path == "/api/timetable/instances":
		id := m.id()
		m.instances[id] = body
		return map[string]any{"id": strconv.FormatInt(id, 10)}, true
	case strings.HasPrefix(path, "/api/timetable/instances/") && strings.HasSuffix(path, "/start"):
		m.started[mustParseID(t, parts[3])] = true
		return nil, true
	case strings.HasPrefix(path, "/api/timetable/operations/instances/") && strings.HasSuffix(path, "/check-in"):
		block := mustParseID(t, parts[4])
		require.True(t, m.started[block], "children check into a running block")
		m.inRoom[mustParseID(t, parts[6])] = block
		return map[string]any{}, true
	case path == "/api/timetable/operations/planned-now":
		return map[string]any{"instances": m.plannedNow()}, true
	case path == "/api/staff-notices/":
		m.notices = append(m.notices, body)
		return map[string]any{"id": "8100"}, true
	case path == "/api/parent-announcements/":
		id := m.id()
		m.news[id] = false
		return map[string]any{"id": strconv.FormatInt(id, 10)}, true
	case strings.HasPrefix(path, "/api/parent-announcements/") && strings.HasSuffix(path, "/publish"):
		m.news[mustParseID(t, parts[2])] = true
		return nil, true
	case strings.HasPrefix(path, "/parent/me/children/") && (strings.HasSuffix(path, "/care-exception") || strings.HasSuffix(path, "/sick-note")):
		m.parentRequests = append(m.parentRequests, marketingMockRequest{sender: sender, kind: parts[4], student: parts[3], body: body})
		return map[string]any{"id": strconv.Itoa(8200 + len(m.parentRequests))}, true
	case path == "/api/calendar/appointments":
		m.appointments = append(m.appointments, body)
		return map[string]any{"id": strconv.Itoa(8300 + len(m.appointments))}, true
	case strings.HasPrefix(path, "/api/meal-plan/") && r.Method == seedHTTPMethodPut:
		m.mealPlan[parts[2]] = body["dishes"].([]any)
		return nil, true
	case path == "/api/staff-messages/threads/open":
		return map[string]any{"thread_id": "account-" + fmt.Sprint(body["account_id"])}, true
	case strings.HasPrefix(path, "/api/staff-messages/threads/"):
		m.teamMessages = append(m.teamMessages, marketingMockMessage{sender: sender, target: parts[3], body: body["body"].(string)})
		return nil, true
	case strings.HasPrefix(path, "/api/messages/threads/"):
		m.staffReplies = append(m.staffReplies, marketingMockMessage{sender: sender, target: parts[3], body: body["body"].(string)})
		return nil, true
	case path == "/api/school-setup":
		return map[string]any{"steps": []map[string]any{{"key": "invite_staff", "applies": true}}}, true
	case strings.HasPrefix(path, "/api/school-setup/steps/"):
		return nil, true
	case path == "/api/school-setup/complete":
		m.setupCompleted = true
		return nil, true
	case path == "/api/school-setup/dismissal":
		m.setupDismissed = body["dismissed"] == true
		return nil, true
	}
	return nil, false
}

func (m *marketingProfileAPIMock) plannedNow() []map[string]any {
	instances := []map[string]any{}
	for id, block := range m.instances {
		present := 0
		for _, running := range m.inRoom {
			if running == id {
				present++
			}
		}
		status := "planned"
		if m.started[id] {
			status = "active"
		}
		instances = append(instances, map[string]any{"title": block["title"], "status": status, "present_students_count": present})
	}
	return instances
}

func mustParseID(t *testing.T, raw string) int64 {
	t.Helper()
	id, err := strconv.ParseInt(raw, 10, 64)
	require.NoError(t, err)
	return id
}

// assertMarketingDailyLife checks the everyday life at the reference clock:
// every present child sits in a running block of its own group, the blocks
// span the reference clock, the admin has unread messages from the team and
// from a family, the families have news and a reply, and the onboarding
// wizard is gone.
func assertMarketingDailyLife(t *testing.T, mock *marketingProfileAPIMock, profile *SeedProfile) {
	t.Helper()
	require.Len(t, mock.rooms, len(marketingRooms()))
	require.Len(t, mock.instances, len(marketingSessions()))
	titles := []string{}
	for id, block := range mock.instances {
		assert.True(t, mock.started[id], "block %v runs", block["title"])
		assert.Less(t, block["start_time"], marketingReferenceClock)
		assert.Greater(t, block["end_time"], marketingReferenceClock)
		activity := mock.activities[int64(block["activity_group_id"].(float64))]
		require.NotNil(t, activity, "block %v has its own activity", block["title"])
		assert.Equal(t, block["title"], activity["name"])
		titles = append(titles, block["title"].(string))
	}
	assert.ElementsMatch(t, []string{"Bauecke", "Fußball"}, titles)

	inRoom := map[string]int{}
	for _, student := range profile.Entities.Students {
		block, ok := mock.inRoom[student.ID]
		if mock.attendance[student.ID] != "checked_in" {
			assert.False(t, ok, "%s is not at school and sits in no room", student.Key)
			continue
		}
		require.True(t, ok, "present child %s sits in a running block", student.Key)
		assert.Contains(t, mock.instances[block]["student_ids"], float64(student.ID), "%s is planned for its block", student.Key)
		inRoom[mock.rooms[int64(mock.instances[block]["room_id"].(float64))]]++
	}
	assert.Equal(t, map[string]int{"Bauraum": 3, "Turnhalle": 4}, inRoom)

	require.Len(t, mock.notices, 1)
	assert.Equal(t, marketingNoticeTitle, mock.notices[0]["title"])
	assert.Equal(t, map[int64]bool{}, filterNews(mock.news, false), "every announcement is published")
	assert.Len(t, mock.news, 1)

	admin := profile.Credentials.Accounts.Admin[0]
	assert.Equal(t, []marketingMockMessage{{
		sender: marketingMockStaffPrefix + "miriam.sommer@example.test",
		target: fmt.Sprintf("account-%d", admin.AccountID), body: marketingTeamMessage,
	}}, mock.teamMessages, "a caregiver writes to the admin, who leaves it unread")
	require.Len(t, mock.parentMessages, 2)
	assert.Equal(t, strconv.FormatInt(profile.Entities.Students["emir-yilmaz"].ID, 10), mock.parentMessages[0].target)
	assert.Equal(t, strconv.FormatInt(profile.Entities.Students["lina-becker"].ID, 10), mock.parentMessages[1].target)
	require.Len(t, mock.staffReplies, 1, "the team answers one family and leaves the other message open")
	assert.Equal(t, marketingMockAdminToken, mock.staffReplies[0].sender)

	assert.True(t, mock.setupCompleted && mock.setupDismissed, "the onboarding checklist does not cover the home page")
	assertMarketingRequests(t, mock, profile)
}

// assertMarketingRequests checks what fills the request inbox, the absence
// list and the meal plan: one open pickup change and one excused absence,
// both filed by parents for the next weekday, and a dish on every weekday of
// the current week.
func assertMarketingRequests(t *testing.T, mock *marketingProfileAPIMock, profile *SeedProfile) {
	t.Helper()
	parentToken := func(key string) string {
		return fmt.Sprintf("%s%d", marketingMockParentPrefix, profile.Entities.Guardians[key].ID)
	}
	studentID := func(key string) string { return strconv.FormatInt(profile.Entities.Students[key].ID, 10) }
	next := marketingNextWeekday(todaySeedDate()).String()
	require.Len(t, mock.parentRequests, 2)
	pickup, absence := mock.parentRequests[0], mock.parentRequests[1]
	assert.Equal(t, parentToken("sarah-yilmaz"), pickup.sender, "the parents-portal account shows its own open request")
	assert.Equal(t, "care-exception", pickup.kind)
	assert.Equal(t, studentID("elif-yilmaz"), pickup.student)
	assert.Equal(t, next, pickup.body["date"])
	assert.NotEmpty(t, pickup.body["reason"])
	assert.Equal(t, parentToken("julia-wagner"), absence.sender)
	assert.Equal(t, "sick-note", absence.kind)
	assert.Equal(t, studentID("mia-wagner"), absence.student)
	assert.Equal(t, []any{next}, absence.body["dates"])
	assert.Equal(t, "excused", absence.body["status"])

	require.Len(t, mock.appointments, 2)
	meeting, festival := mock.appointments[0], mock.appointments[1]
	assert.Equal(t, todaySeedDate().String(), meeting["start_date"], "the week view of the calendar shows a meeting today")
	assert.Equal(t, []any{map[string]any{"type": "all_staff"}}, meeting["targets"])
	assert.Equal(t, marketingNewsFriday(todaySeedDate()).String(), festival["start_date"], "the festival is the Friday the news announces")
	assert.Equal(t, time.Friday, marketingNewsFriday(todaySeedDate()).Weekday())
	assert.Contains(t, festival["targets"], map[string]any{"type": "all_school_parents"})
	assert.Equal(t, "rsvp_required", festival["delivery_mode"], "families can accept in the parents portal")

	monday := todaySeedDate().AddDays(-(int(todaySeedDate().Weekday()) + 6) % 7)
	require.Len(t, mock.mealPlan, 5)
	for offset := range 5 {
		assert.NotEmpty(t, mock.mealPlan[monday.AddDays(offset).String()], "dishes on weekday %d", offset+1)
	}
}

func filterNews(news map[int64]bool, published bool) map[int64]bool {
	out := map[int64]bool{}
	for id, state := range news {
		if state == published {
			out[id] = state
		}
	}
	return out
}
