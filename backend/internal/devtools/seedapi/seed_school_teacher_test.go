package api

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSeedSchoolTeacherUsesSchoolSessionAndAssignedOGSClass(t *testing.T) {
	t.Parallel()

	var paths []string
	var bodies []map[string]any
	password := t.Name()
	srv := newSeedHTTPTestServer(func(w seedHTTPResponseWriter, r *seedHTTPRequest) {
		paths = append(paths, r.URL.Path)
		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		bodies = append(bodies, body)
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/auth/register":
			assert.Equal(t, "Bearer admin", r.Header.Get("Authorization"))
			assert.EqualValues(t, 71, body["role_id"])
			assert.Equal(t, password, body["password"])
			assert.Equal(t, password, body["confirm_password"])
			assert.Equal(t, "lehrkraft@demo-vollbetrieb.moto-ogs.de", body["email"])
			_, _ = fmt.Fprint(w, `{"data":{"id":81,"school_identity":{"staff_id":"91"}}}`)
		case "/api/staff/91/school-classes":
			assert.Equal(t, "Bearer admin", r.Header.Get("Authorization"))
			assert.Equal(t, []any{DemoStudents[0].Class}, body["school_classes"])
			_, _ = fmt.Fprint(w, `{"status":"success"}`)
		case "/school/auth/login":
			assert.Empty(t, r.Header.Get("Authorization"), "school login is public, not a tenant session")
			assert.Equal(t, "vollbetrieb", body["tenant_slug"])
			assert.Equal(t, password, body["password"])
			_, _ = fmt.Fprint(w, `{"status":"success","access_token":"school-session"}`)
		default:
			assert.True(t, strings.HasPrefix(r.URL.Path, "/school/class-day/"), r.URL.Path)
			assert.Equal(t, "PUT", r.Method)
			assert.Equal(t, "Bearer school-session", r.Header.Get("Authorization"))
			_, _ = fmt.Fprint(w, `{"status":"success"}`)
		}
	})
	defer srv.Close()

	client := newTestClient(srv.URL, false)
	fs := NewFixedSeeder(client, false, "")
	fs.roleIDs["lehrkraft"] = 71
	fs.studentIDByIndex[0] = 101
	fs.staffCredentials = []StaffCredentials{{Password: password}}
	fs.accountScope = "vollbetrieb"
	s := &Seeder{}
	rt := &Runtime{Client: client, FixedSeeder: fs, Bootstrap: &bootstrapSeedState{TenantSlug: "vollbetrieb"}, TenantAuth: AuthRef{Kind: AuthBearer, Token: "admin"}}
	require.NoError(t, (seedSchoolTeacherStep{seeder: s}).Run(t.Context(), rt))

	day := DemoStudents[0].Class + "/" + todaySeedDate().String()
	assert.Equal(t, []string{
		"/auth/register", "/api/staff/91/school-classes", "/school/auth/login",
		"/school/class-day/arrival-exceptions/" + day,
		"/school/class-day/day-hints/" + day,
		"/school/class-day/day-hints/" + day + "/students/101",
		"/school/class-day/morning-attendance/" + day,
	}, paths)
	assert.Equal(t, "12:00", bodies[3]["arrival_time"])
	assert.NotEmpty(t, bodies[4]["body"])
	assert.NotEmpty(t, bodies[5]["body"])
	assert.Equal(t, []any{map[string]any{"student_id": float64(101), "status": "present"}}, bodies[6]["marks"])
	require.NotNil(t, s.schoolTeacherAccount)
	assert.EqualValues(t, 91, s.schoolTeacherAccount.StaffID)
	state := &SeedState{}
	s.populateSeedAccounts(state, fs)
	require.Len(t, state.Accounts.Lehrkraft, 1)
	assert.Equal(t, *s.schoolTeacherAccount, state.Accounts.Lehrkraft[0])
}

func TestFullOperationProfileOpensAllSchoolWrites(t *testing.T) {
	t.Parallel()

	settings := fullOperationSettings()
	assert.Equal(t, SettingManagedByTenant, settings[profileSettingSchoolWriteScope].ManagedBy)
	assert.JSONEq(t, `["class_arrival_exceptions","class_handover","class_trips","day_hints"]`, string(settings[profileSettingSchoolWriteScope].Value))
	assert.JSONEq(t, `true`, string(settings[profileSettingSchoolMorning].Value))
}

func TestSeedSchoolTeacherScreenshotWeekendsPreserveAccountWithoutDayWrites(t *testing.T) {
	t.Parallel()

	for _, date := range []string{"2026-10-30", "2026-10-31", "2026-11-01"} {
		for _, screenshots := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/screenshots=%t", date, screenshots), func(t *testing.T) {
				t.Parallel()
				today, err := parseSeedDate(date)
				require.NoError(t, err)
				wantDayWrites := !screenshots || date == "2026-10-30"
				var paths []string
				srv := newSeedHTTPTestServer(func(w seedHTTPResponseWriter, r *seedHTTPRequest) {
					paths = append(paths, r.URL.Path)
					w.Header().Set("Content-Type", "application/json")
					switch r.URL.Path {
					case "/auth/register":
						_, _ = fmt.Fprint(w, `{"data":{"id":81,"school_identity":{"staff_id":"91"}}}`)
					case "/api/staff/91/school-classes":
						_, _ = fmt.Fprint(w, `{"status":"success"}`)
					case "/school/auth/login":
						_, _ = fmt.Fprint(w, `{"access_token":"school-session"}`)
					default:
						assert.True(t, wantDayWrites, "no class-day writes on screenshot weekends")
						_, _ = fmt.Fprint(w, `{"status":"success"}`)
					}
				})
				defer srv.Close()

				client := newTestClient(srv.URL, false)
				fs := NewFixedSeeder(client, false, "")
				fs.roleIDs["lehrkraft"] = 71
				fs.studentIDByIndex[0] = 101
				fs.staffCredentials = []StaffCredentials{{Password: t.Name()}}
				s := &Seeder{options: SeedOptions{ProductScreenshots: screenshots}}
				rt := &Runtime{Client: client, FixedSeeder: fs, Bootstrap: &bootstrapSeedState{TenantSlug: "vollbetrieb"}, TenantAuth: AuthRef{Kind: AuthBearer, Token: "admin"}}
				require.NoError(t, (seedSchoolTeacherStep{seeder: s}).runOn(rt, today))

				wantPaths := []string{"/auth/register", "/api/staff/91/school-classes", "/school/auth/login"}
				if wantDayWrites {
					day := DemoStudents[0].Class + "/" + date
					wantPaths = append(wantPaths,
						"/school/class-day/arrival-exceptions/"+day,
						"/school/class-day/day-hints/"+day,
						"/school/class-day/day-hints/"+day+"/students/101",
						"/school/class-day/morning-attendance/"+day)
				}
				assert.Equal(t, wantPaths, paths)
				state := &SeedState{}
				s.populateSeedAccounts(state, fs)
				require.Len(t, state.Accounts.Lehrkraft, 1)
				assert.EqualValues(t, 81, state.Accounts.Lehrkraft[0].AccountID)
				assert.EqualValues(t, 91, state.Accounts.Lehrkraft[0].StaffID)
			})
		}
	}
}
