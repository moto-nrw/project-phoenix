package api

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/legacy/jwt"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

// Workload versions of the default checkpoint. checkpoint-1-v1 is the #3019
// scenario list, kept selectable for bridge runs; checkpoint-1-v2 (#3411) adds
// the flows the #2580 migration rebuilt and a concurrent contention run.
const (
	checkpointWorkloadV1 = "checkpoint-1-v1"
	checkpointWorkloadV2 = "checkpoint-1-v2"
	// serialCheckpointConcurrency is how many requests the serial runs keep in
	// flight: one, so their latencies stay comparable with #3019 and #3020.
	serialCheckpointConcurrency = 1
)

var runtimeCheckpointWorkload = flag.String("runtime-checkpoint-workload", checkpointWorkloadV2, "default checkpoint workload version: checkpoint-1-v2, or checkpoint-1-v1 for a bridge run")

var runtimeCheckpointConcurrency = flag.Int("runtime-checkpoint-concurrency", 16, "requests in flight per round of the checkpoint-1-v2 contention run")

// targetRiskWorkload is the checkpoint-1-v2 extension (#3411): login,
// refresh, MFA and passkey start, the kiosk scans with their device-key and
// PIN failures, the /api/students review and withdrawal surfaces, and the
// student deletion workflow as the cross-module write. Preparation runs
// outside the timed window; each scenario keeps one definition across runs
// and only the resolved body, path or token changes per request.
type targetRiskWorkload struct {
	handler http.Handler
	db      *testpkg.DB
	// fixtures creates every fixture in the workload's own tenant, and token
	// is an administrator of that tenant.
	fixtures  testing.TB
	tenantID  int64
	token     string
	subdomain string
	accountID int64
	// companionID is a fixed child every deletion subject is linked to, so
	// the locked path walks a companion edge without growing the tenant.
	companionID int64

	loginEmail, loginPassword string
	refreshToken              string
	mfaAccountID              int64
	tokenAuth                 *jwt.TokenAuth

	deviceKey  string
	studentTag string
	roomID     int64
	staffTag   string
	staffIn    bool

	parentToken       string
	parentStudentID   int64
	missingRequestID  int64
	missingCompletion int64
	completionID      int64
	stalePreviewID    int64

	// careEndIDs is the fixed selection of the care-end preview; ended lists
	// children whose care a measured request ended, removed before the next
	// preparation so the tenant volume stays flat.
	careEndIDs []string
	ended      []int64
	// bookedActivities every care-end child is enrolled in, so ending care
	// removes real bookings and the removal sets carry rows.
	bookedActivities []int64

	owned        map[string]bool
	slotCounters []*testpkg.QueryCounter

	decided, deleted, staffClocks, careEnded int
}

// checkpointTenantTB is the checkpoint test under a name of its own. The
// per-test tenant registry keys tenants by test name, so every fixture made
// through it lands in a second tenant: the checkpoint-1-v2 additions never
// change the rooms, activities or messages the checkpoint-1-v1 scenarios
// read, and those twenty scenarios stay comparable with #3019 and #3020.
type checkpointTenantTB struct {
	testing.TB
	name string
}

func (tb checkpointTenantTB) Name() string { return tb.name }

func newTargetRiskWorkload(t *testing.T, production *Runtime) *targetRiskWorkload {
	t.Helper()
	fixtures := checkpointTenantTB{TB: t, name: t.Name() + "/" + checkpointWorkloadV2}
	testpkg.OwnTenant(fixtures)
	db := testpkg.SetupTestDB(fixtures)
	ctx := testpkg.Ctx(fixtures)
	_, account := testpkg.CreateTestTeacherWithAccount(fixtures, db, "Checkpoint", "Reviewer")
	w := &targetRiskWorkload{
		handler: production.Handler(), db: db, fixtures: fixtures, tenantID: testpkg.Tenant(fixtures), accountID: account.ID,
		token: testutil.MintTestJWT(fixtures, testutil.AdminTestClaimsForTenant(int(account.ID), testpkg.Tenant(fixtures))),
	}
	require.NotEqual(t, testpkg.Tenant(t), w.tenantID, "checkpoint-1-v2 fixtures need their own tenant")
	require.NoError(t, db.NewRaw("SELECT subdomain FROM platform.schools WHERE id = ?", w.tenantID).Scan(ctx, &w.subdomain))
	var err error
	w.tokenAuth, err = testpkg.ConfiguredTokenAuth()
	require.NoError(t, err)

	suffix := testpkg.UniqueSuffix()
	w.loginEmail = fmt.Sprintf("checkpoint-login-%d@example.test", suffix)
	w.loginPassword = fmt.Sprintf("ckpt-%d-login", suffix)
	testpkg.CreateTestAccountWithPassword(fixtures, db, w.loginEmail, w.loginPassword)
	refreshEmail := fmt.Sprintf("checkpoint-refresh-%d@example.test", suffix)
	testpkg.CreateTestAccountWithPassword(fixtures, db, refreshEmail, w.loginPassword)
	w.refreshToken = w.login(t, refreshEmail).RefreshToken
	w.mfaAccountID = testpkg.CreateTestAccountWithPassword(fixtures, db, fmt.Sprintf("checkpoint-mfa-%d@example.test", suffix), w.loginPassword).ID

	w.deviceKey = *testpkg.CreateTestDevice(fixtures, db, fmt.Sprintf("checkpoint-kiosk-%d", suffix)).APIKey
	room := testpkg.CreateTestRoom(fixtures, db, "Checkpoint Kiosk Room")
	w.roomID = room.ID
	activity := testpkg.CreateTestActivityGroup(fixtures, db, "Checkpoint Kiosk Activity")
	testpkg.CreateTestActiveGroup(fixtures, db, activity.ID, room.ID)
	scanned := testpkg.CreateTestStudent(fixtures, db, "Checkpoint", "Scanned", "2a")
	card := testpkg.CreateTestRFIDCard(fixtures, db, "C4EC")
	testpkg.LinkRFIDToStudent(fixtures, db, scanned.PersonID, card.ID)
	w.studentTag = card.ID
	staff := testpkg.CreateTestStaff(fixtures, db, "Checkpoint", "Clock")
	staffCard := testpkg.CreateTestRFIDCard(fixtures, db, "A1654BEEF")
	testpkg.LinkRFIDToStudent(fixtures, db, staff.PersonID, staffCard.ID)
	w.staffTag = staffCard.ID

	chain := testpkg.CreateTestParentGuardianChain(fixtures, db)
	w.parentToken = careScheduleParentToken(t, chain)
	w.parentStudentID = chain.StudentID
	require.NoError(t, db.NewRaw("SELECT nextval(pg_get_serial_sequence('schedule.care_schedule_change_requests', 'id'))").Scan(ctx, &w.missingRequestID))
	require.NoError(t, db.NewRaw("SELECT nextval(pg_get_serial_sequence('users.care_withdrawal_completions', 'id'))").Scan(ctx, &w.missingCompletion))

	withdrawn := testpkg.CreateTestStudent(fixtures, db, "Checkpoint", "Withdrawn", "2a")
	w.completionID = w.pendingWithdrawal(t, production, withdrawn.ID)
	w.companionID = testpkg.CreateTestStudent(fixtures, db, "Checkpoint", "Companion", "2a").ID
	w.accompanied(t, w.companionID)
	w.stalePreviewID = w.linkedStudent(t, "Stale")
	for i := range 3 {
		w.bookedActivities = append(w.bookedActivities, testpkg.CreateTestActivityGroup(fixtures, db, fmt.Sprintf("Checkpoint Booked Activity %d", i)).ID)
	}
	for i := range 10 {
		w.careEndIDs = append(w.careEndIDs, strconv.FormatInt(w.bookedStudent(t, fmt.Sprintf("CareEnd%d", i)), 10))
	}
	return w
}

// bookedStudent creates a child enrolled in every booked activity through
// the Timetable HTTP route, outside timing.
func (w *targetRiskWorkload) bookedStudent(t *testing.T, name string) int64 {
	t.Helper()
	id := testpkg.CreateTestStudent(w.fixtures, w.db, "Checkpoint", name, "2a").ID
	for _, activityID := range w.bookedActivities {
		enrolled := checkpointRequest(w.handler, checkpointScenario{Method: http.MethodPost, Path: fmt.Sprintf("/api/activities/%d/students/%d", activityID, id), Authenticated: true}, w.token)
		require.Equal(t, http.StatusOK, enrolled.Code, "prepare activity booking: %s", enrolled.Body.String())
	}
	return id
}

// checkpointLastCareDay is a fixed future day: the care-end preview refuses a
// day in the past, and the phase fixture runs until 2027-07-31.
// The re-plan scenario first ends care on the earlier day, so its measured
// request has to restore the bookings that exit removed.
const (
	checkpointLastCareDay    = "2027-07-30"
	checkpointEarlierCareDay = "2027-07-29"
)

func careEndBody(studentIDs []string, token string) string {
	return careEndBodyOn(studentIDs, checkpointLastCareDay, token)
}

func careEndBodyOn(studentIDs []string, day, token string) string {
	encoded, _ := json.Marshal(map[string]any{
		"student_ids": studentIDs, "last_care_day": day, "reason": "other", "reason_note": "Checkpoint", "token": token,
	})
	return string(encoded)
}

// careEndToken reads the preview token that confirms exactly this selection.
func (w *targetRiskWorkload) careEndToken(t *testing.T, studentIDs []string, day string) string {
	t.Helper()
	response := checkpointRequest(w.handler, checkpointScenario{Method: http.MethodPost, Path: "/api/students/care-end/preview", Authenticated: true, Body: careEndBodyOn(studentIDs, day, "")}, w.token)
	require.Equal(t, http.StatusOK, response.Code, "prepare care-end preview: %s", response.Body.String())
	var envelope struct {
		Data struct {
			Token   string `json:"token"`
			Blocked bool   `json:"blocked"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &envelope))
	require.False(t, envelope.Data.Blocked)
	return envelope.Data.Token
}

type checkpointTokens struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

func (w *targetRiskWorkload) login(t *testing.T, email string) checkpointTokens {
	t.Helper()
	response := checkpointRequest(w.handler, checkpointScenario{Method: http.MethodPost, Path: "/auth/login",
		Body: fmt.Sprintf(`{"email":%q,"password":%q,"tenant_slug":%q}`, email, w.loginPassword, w.subdomain)}, "")
	require.Equal(t, http.StatusOK, response.Code, "prepare login: %s", response.Body.String())
	var tokens checkpointTokens
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &tokens))
	require.NotEmpty(t, tokens.RefreshToken)
	return tokens
}

// linkedStudent creates a child with one Monday companion edge to the fixed
// companion, the smallest graph the locked deletion path must walk.
func (w *targetRiskWorkload) linkedStudent(t *testing.T, name string) int64 {
	t.Helper()
	id := testpkg.CreateTestStudent(w.fixtures, w.db, "Checkpoint", name, "2a").ID
	w.accompanied(t, id)
	w.link(t, id, w.companionID)
	return id
}

// accompaniedPlan is the departure plan the Stammdaten form submits for a
// child who may leave with another child on Monday. The note answers "mit
// wem" on its own, so deleting a linked child never strands this one.
const accompaniedPlan = `"allowed_departure_modes":{"mon":["accompanied"]},"departure_companion_note":"Nachbarskind","departure_days":{"mon":"accompanied"}`

// accompanied stores that plan through PUT /api/students/{id}, outside timing.
func (w *targetRiskWorkload) accompanied(t *testing.T, studentID int64) {
	t.Helper()
	w.putStudent(t, studentID, "{"+accompaniedPlan+"}")
}

// link replaces a fresh child's empty companion list with Monday links to
// the companions, through the same route and the owner's companion rules.
// Every child involved must already have an accompanied Monday.
func (w *targetRiskWorkload) link(t *testing.T, subjectID int64, companionIDs ...int64) {
	t.Helper()
	links := make([]string, 0, len(companionIDs))
	for _, companionID := range companionIDs {
		links = append(links, fmt.Sprintf(`{"companion_student_id":%d,"weekdays":["mon"]}`, companionID))
	}
	w.putStudent(t, subjectID, "{"+accompaniedPlan+`,"companions":[`+strings.Join(links, ",")+`],"companions_fingerprint":""}`)
}

func (w *targetRiskWorkload) putStudent(t *testing.T, studentID int64, body string) {
	t.Helper()
	response := checkpointRequest(w.handler, checkpointScenario{Method: http.MethodPut, Path: fmt.Sprintf("/api/students/%d", studentID), Authenticated: true, Body: body}, w.token)
	require.Equal(t, http.StatusOK, response.Code, "prepare student plan: %s", response.Body.String())
}

// pendingWithdrawal stores one school-confirmed withdrawal task through the
// Care Plan repository of the production graph. A withdrawal cannot be
// raised over HTTP without booking-led enrollment, so the fixture asks the
// owner directly; the least-privilege pool needs the tenant transaction.
func (w *targetRiskWorkload) pendingWithdrawal(t *testing.T, production *Runtime, studentID int64) int64 {
	t.Helper()
	tenantID, err := tenant.NewTenantID(w.tenantID)
	require.NoError(t, err)
	ctx := tenant.WithUnitOfWork(testpkg.Ctx(w.fixtures), production.api.tenantRuntime)
	writer := tenantScopedUpsert(production.api.repos.CareWithdrawal.UpsertPending, func(fn func(context.Context) error) error {
		return tenant.WithinTenant(ctx, tenantID, fn)
	})
	return testpkg.CreateTestCareWithdrawalCompletion(w.fixtures, writer, studentID, w.accountID, "2026-09-14").ID
}

// tenantUpsert runs an owner's pending-task upsert inside the tenant
// transaction, whatever context the fixture passes.
type tenantUpsert[T any] struct {
	upsert func(context.Context, T) error
	within func(func(context.Context) error) error
}

func tenantScopedUpsert[T any](upsert func(context.Context, T) error, within func(func(context.Context) error) error) tenantUpsert[T] {
	return tenantUpsert[T]{upsert: upsert, within: within}
}

func (u tenantUpsert[T]) UpsertPending(_ context.Context, value T) error {
	return u.within(func(ctx context.Context) error { return u.upsert(ctx, value) })
}

// deletionBody reads the preview the confirmation dialog shows and returns
// the confirmation that deletes exactly that snapshot.
func (w *targetRiskWorkload) deletionBody(t *testing.T, studentID int64) string {
	t.Helper()
	response := checkpointRequest(w.handler, checkpointScenario{Method: http.MethodGet, Path: fmt.Sprintf("/api/students/%d/delete-impact", studentID), Authenticated: true}, w.token)
	require.Equal(t, http.StatusOK, response.Code, "prepare deletion preview: %s", response.Body.String())
	var envelope struct {
		Data struct {
			Fingerprint      string `json:"fingerprint"`
			ConfirmationName string `json:"confirmation_name"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &envelope))
	return fmt.Sprintf(`{"expected_fingerprint":%q,"confirmation_name":%q,"reason":"test_data","acknowledged":true}`,
		envelope.Data.Fingerprint, envelope.Data.ConfirmationName)
}

// Placeholders resolved per request by prepare.
const (
	checkpointStudentPlaceholder    = "{{student}}"
	checkpointRequestPlaceholder    = "{{request}}"
	checkpointBodyPlaceholder       = "{{prepared}}"
	checkpointStaleFingerprint      = "0000000000000000000000000000000000000000000000000000000000000000"
	checkpointDeviceKeyTokenMarker  = "device"
	checkpointRefreshTokenMarker    = "refresh"
	checkpointInvalidRefreshTokenID = "checkpoint-invalid-refresh-token"
)

// owns reports whether the scenario is one of this workload's additions;
// the checkpoint-1-v1 scenarios keep their own token and tenant.
// The set is built once: owns runs inside the timed window of every
// scenario, the checkpoint-1-v1 ones included.
func (w *targetRiskWorkload) owns(scenario checkpointScenario) bool {
	if w.owned == nil {
		w.owned = map[string]bool{}
		for _, own := range w.scenarios() {
			w.owned[own.Name] = true
		}
	}
	return w.owned[scenario.Name]
}

func (w *targetRiskWorkload) scenarios() []checkpointScenario {
	login := fmt.Sprintf(`{"email":%q,"password":%q,"tenant_slug":%q}`, w.loginEmail, w.loginPassword, w.subdomain)
	wrongPassword := fmt.Sprintf(`{"email":%q,"password":"ckpt-wrong-password","tenant_slug":%q}`, w.loginEmail, w.subdomain)
	passkey := fmt.Sprintf(`{"tenant_slug":%q}`, w.subdomain)
	checkin := fmt.Sprintf(`{"student_rfid":%q,"action":"checkin","room_id":%d}`, w.studentTag, w.roomID)
	pickup := fmt.Sprintf(`{"student_rfid":%q}`, w.studentTag)
	return []checkpointScenario{
		{"identity-access.login", "POST", "/auth/login", 200, false, login},
		{"identity-access.login-wrong-password", "POST", "/auth/login", 401, false, wrongPassword},
		{"identity-access.refresh", "POST", "/auth/refresh", 200, true, ""},
		{"identity-access.refresh-invalid-token", "POST", "/auth/refresh", 401, true, ""},
		{"identity-access.mfa-verify", "POST", "/auth/mfa/verify", 200, false, checkpointBodyPlaceholder},
		{"identity-access.mfa-verify-wrong-code", "POST", "/auth/mfa/verify", 401, false, checkpointBodyPlaceholder},
		{"identity-access.passkey-login-options", "POST", "/auth/passkeys/login/options", 200, false, passkey},
		{"identity-access.passkey-login-options-wrong-origin", "POST", "/auth/passkeys/login/options", 401, false, passkey},
		{"device-scan.checkin", "POST", "/api/iot/checkin", 200, true, checkin},
		{"device-scan.checkin-invalid-key", "POST", "/api/iot/checkin", 401, true, checkin},
		{"device-scan.pickup-query", "POST", "/api/iot/pickup-query", 200, true, pickup},
		{"device-scan.pickup-query-missing-pin", "POST", "/api/iot/pickup-query", 401, true, pickup},
		{"device-scan.status", "GET", "/api/iot/status", 200, true, ""},
		{"device-scan.status-wrong-pin", "GET", "/api/iot/status", 401, true, ""},
		{"device-scan.rfid-lookup", "GET", "/api/iot/rfid/" + w.studentTag, 200, true, ""},
		{"device-scan.rfid-lookup-malformed-key", "GET", "/api/iot/rfid/" + w.studentTag, 401, false, ""},
		{"device-scan.staff-clock", "POST", "/api/iot/staff-clock", 200, true, checkpointBodyPlaceholder},
		{"device-scan.staff-clock-wrong-pin", "POST", "/api/iot/staff-clock", 401, true, fmt.Sprintf(`{"rfid_tag":%q,"action":"checkin","status":"present"}`, w.staffTag)},
		{"people-directory.students", "GET", "/api/students/", 200, true, ""},
		{"people-directory.students-invalid-view", "GET", "/api/students/?view=bogus", 400, true, ""},
		{"request-review.queue", "GET", "/api/students/change-requests", 200, true, ""},
		{"request-review.queue-invalid-view", "GET", "/api/students/change-requests?view=bogus", 400, true, ""},
		{"request-review.care-schedule-decide", "POST", "/api/students/care-schedule-change-requests/" + checkpointRequestPlaceholder + "/decide", 200, true, `{"approve":true,"reason":"Checkpoint"}`},
		{"request-review.care-schedule-decide-not-found", "POST", fmt.Sprintf("/api/students/care-schedule-change-requests/%d/decide", w.missingRequestID), 404, true, `{"approve":true,"reason":"Checkpoint"}`},
		{"care-plan.withdrawals", "GET", "/api/students/care-withdrawals", 200, true, ""},
		{"care-plan.withdrawals-invalid-state", "GET", "/api/students/care-withdrawals?state=bogus", 400, true, ""},
		{"student-deletion.withdrawal-impact", "GET", fmt.Sprintf("/api/students/care-withdrawals/%d/deletion-impact", w.completionID), 200, true, ""},
		{"student-deletion.withdrawal-impact-not-found", "GET", fmt.Sprintf("/api/students/care-withdrawals/%d/deletion-impact", w.missingCompletion), 404, true, ""},
		{"care-plan.care-end-preview", "POST", "/api/students/care-end/preview", 200, true, careEndBody(w.careEndIDs, "")},
		{"care-plan.care-end-preview-past-day", "POST", "/api/students/care-end/preview", 400, true, strings.Replace(careEndBody(w.careEndIDs, ""), checkpointLastCareDay, "2020-01-31", 1)},
		{"care-plan.care-end", "POST", "/api/students/care-end", 200, true, checkpointBodyPlaceholder},
		{"care-plan.care-end-replan", "POST", "/api/students/care-end", 200, true, checkpointBodyPlaceholder},
		{"care-plan.care-end-stale-token", "POST", "/api/students/care-end", 409, true, careEndBody(w.careEndIDs, checkpointStaleFingerprint)},
		{"student-deletion.execute", "DELETE", "/api/students/" + checkpointStudentPlaceholder, 200, true, checkpointBodyPlaceholder},
		{"student-deletion.execute-stale-preview", "DELETE", fmt.Sprintf("/api/students/%d", w.stalePreviewID), 409, true,
			fmt.Sprintf(`{"expected_fingerprint":%q,"confirmation_name":"Checkpoint Stale","reason":"test_data","acknowledged":true}`, checkpointStaleFingerprint)},
	}
}

// headers are the non-bearer headers of a scenario. The kiosk contract is
// the PyrePortal one: Authorization carries the device key, X-Staff-PIN the
// school's device PIN (the fresh-tenant default of security.ogs_device_pin).
func (w *targetRiskWorkload) headers(scenario checkpointScenario) map[string]string {
	switch scenario.Name {
	case "identity-access.passkey-login-options":
		return map[string]string{"Origin": "http://" + w.subdomain + ".tenant.invalid"}
	case "identity-access.passkey-login-options-wrong-origin":
		return map[string]string{"Origin": "http://checkpoint-elsewhere.invalid"}
	case "device-scan.pickup-query-missing-pin":
		return nil
	case "device-scan.status-wrong-pin", "device-scan.staff-clock-wrong-pin":
		return map[string]string{"X-Staff-PIN": "0000"}
	case "device-scan.rfid-lookup-malformed-key":
		return map[string]string{"Authorization": "Token " + w.deviceKey, "X-Staff-PIN": "1234"}
	}
	if strings.HasPrefix(scenario.Name, "device-scan.") {
		return map[string]string{"X-Staff-PIN": "1234"}
	}
	return nil
}

// headerContract describes the headers for the report without their values.
func (w *targetRiskWorkload) headerContract(scenarios []checkpointScenario) map[string][]string {
	out := map[string][]string{}
	for _, scenario := range scenarios {
		var names []string
		for name := range w.headers(scenario) {
			names = append(names, name)
		}
		if scenario.Authenticated {
			names = append(names, "Authorization: Bearer "+w.bearerKind(scenario))
		}
		sort.Strings(names)
		if len(names) > 0 {
			out[scenario.Name] = names
		}
	}
	return out
}

func (w *targetRiskWorkload) bearerKind(scenario checkpointScenario) string {
	switch {
	case strings.HasPrefix(scenario.Name, "device-scan.checkin-invalid-key"):
		return "unknown device key"
	case strings.HasPrefix(scenario.Name, "device-scan."):
		return checkpointDeviceKeyTokenMarker + " key"
	case scenario.Name == "identity-access.refresh":
		return "latest " + checkpointRefreshTokenMarker + " token"
	case scenario.Name == "identity-access.refresh-invalid-token":
		return "malformed token"
	}
	return "staff access token"
}

// prepare resolves one request outside the timed window and returns it with
// the bearer it is sent with.
func (w *targetRiskWorkload) prepare(t *testing.T, scenario checkpointScenario) (checkpointScenario, string) {
	t.Helper()
	w.removeLeftovers(t, w.ended...)
	w.ended = nil
	token := w.token
	switch scenario.Name {
	case "identity-access.refresh":
		token = w.refreshToken
	case "identity-access.refresh-invalid-token":
		token = checkpointInvalidRefreshTokenID
	case "identity-access.mfa-verify":
		scenario.Body = w.mfaChallenge(t, "246810", "246810")
	case "identity-access.mfa-verify-wrong-code":
		// A wrong code counts toward the lockout; reset the counter so every
		// sample measures the same rejection instead of the 429 lock.
		_, err := w.db.NewRaw("UPDATE auth.accounts SET mfa_attempts = 0, mfa_locked_until = NULL WHERE id = ?", w.mfaAccountID).Exec(context.Background())
		require.NoError(t, err)
		scenario.Body = w.mfaChallenge(t, "246810", "135790")
	case "device-scan.checkin-invalid-key":
		token = "checkpoint-unknown-device-key"
	case "device-scan.staff-clock":
		if w.staffIn {
			scenario.Body = fmt.Sprintf(`{"rfid_tag":%q,"action":"checkout"}`, w.staffTag)
		} else {
			scenario.Body = fmt.Sprintf(`{"rfid_tag":%q,"action":"checkin","status":"present"}`, w.staffTag)
		}
	case "request-review.care-schedule-decide":
		scenario.Path = strings.Replace(scenario.Path, checkpointRequestPlaceholder, strconv.FormatInt(w.pendingCareRequest(t), 10), 1)
	case "care-plan.care-end":
		scenario.Body = w.prepareCareEnd(t, false)
	case "care-plan.care-end-replan":
		scenario.Body = w.prepareCareEnd(t, true)
	case "student-deletion.execute":
		subject := w.linkedStudent(t, fmt.Sprintf("Deleted%d", w.deleted))
		scenario.Path = strings.Replace(scenario.Path, checkpointStudentPlaceholder, strconv.FormatInt(subject, 10), 1)
		scenario.Body = w.deletionBody(t, subject)
	}
	if strings.HasPrefix(scenario.Name, "device-scan.") && scenario.Name != "device-scan.checkin-invalid-key" {
		token = w.deviceKey
	}
	return scenario, token
}

// prepareCareEnd books a fresh child and returns the confirmation that ends
// its care on the last day. For a re-plan, care first ends on the earlier
// day, so the measured confirmation restores what that exit removed.
func (w *targetRiskWorkload) prepareCareEnd(t *testing.T, replan bool) string {
	t.Helper()
	id := w.bookedStudent(t, fmt.Sprintf("Ended%d", w.careEnded))
	w.ended = append(w.ended, id)
	ids := []string{strconv.FormatInt(id, 10)}
	if replan {
		first := checkpointRequest(w.handler, checkpointScenario{Method: http.MethodPost, Path: "/api/students/care-end", Authenticated: true,
			Body: careEndBodyOn(ids, checkpointEarlierCareDay, w.careEndToken(t, ids, checkpointEarlierCareDay))}, w.token)
		require.Equal(t, http.StatusOK, first.Code, "prepare earlier care end: %s", first.Body.String())
	}
	return careEndBody(ids, w.careEndToken(t, ids, checkpointLastCareDay))
}

// mfaChallenge stores one open e-mail challenge for the MFA account and
// returns the verify body. The production mock mailer cannot deliver a code,
// so the challenge row is written the way the owner's issue path writes it.
func (w *targetRiskWorkload) mfaChallenge(t *testing.T, code, submitted string) string {
	t.Helper()
	var challengeID int64
	require.NoError(t, w.db.NewRaw(`INSERT INTO auth.mfa_email_challenges
		(account_id, scope, tenant_id, code_hash, expires_at, created_at, updated_at)
		VALUES (?, 'tenant', ?, ?, NOW() + INTERVAL '10 minutes', NOW(), NOW()) RETURNING id`,
		w.mfaAccountID, w.tenantID, testpkg.HashTestPassword(t, code)).Scan(context.Background(), &challengeID))
	challenge, err := w.tokenAuth.CreateMFAChallengeJWT(jwt.MFAChallengeClaims{
		AccountID: w.mfaAccountID, Scope: jwt.MFAChallengeScopeTenant, TenantID: w.tenantID, ChallengeID: challengeID,
	}, 10*time.Minute)
	require.NoError(t, err)
	return fmt.Sprintf(`{"challenge_token":%q,"code":%q}`, challenge, submitted)
}

// pendingCareRequest files one parent care-schedule request for the chain's
// child. The pickup time alternates so an approved plan never equals the
// next proposal.
func (w *targetRiskWorkload) pendingCareRequest(t *testing.T) int64 {
	t.Helper()
	pickup := "15:30"
	if w.decided%2 == 1 {
		pickup = "16:00"
	}
	body := map[string]any{"payload": map[string]any{"weekdays": []any{map[string]any{
		"weekday": 2, "scheduled": true, "pickup": pickup, "mode": "pickup",
	}}}}
	path := "/parent/me/children/" + strconv.FormatInt(w.parentStudentID, 10) + "/care-schedule/requests"
	created := doCareScheduleJSON(t, w.handler, http.MethodPost, path, w.parentToken, body)
	require.Equal(t, http.StatusCreated, created.Code, "prepare care-schedule request: %s", created.Body.String())
	return careSchedulePendingRequestID(t, created)
}

// observe carries state from a response into the next preparation.
func (w *targetRiskWorkload) observe(t *testing.T, scenario checkpointScenario, response *httptest.ResponseRecorder) {
	t.Helper()
	if response.Code != scenario.ExpectedStatus {
		return
	}
	switch scenario.Name {
	case "identity-access.refresh":
		var tokens checkpointTokens
		require.NoError(t, json.Unmarshal(response.Body.Bytes(), &tokens))
		require.NotEmpty(t, tokens.RefreshToken)
		w.refreshToken = tokens.RefreshToken
	case "device-scan.staff-clock":
		w.staffIn = !w.staffIn
		w.staffClocks++
	case "request-review.care-schedule-decide":
		w.decided++
	case "student-deletion.execute":
		w.deleted++
	case "care-plan.care-end", "care-plan.care-end-replan":
		w.careEnded++
	}
}

// finalState proves the stateful scenarios did what their status claims.
func (w *targetRiskWorkload) finalState(t *testing.T) map[string]int {
	t.Helper()
	w.removeLeftovers(t, w.ended...)
	w.ended = nil
	ctx := context.Background()
	var approved, stale int
	require.NoError(t, w.db.NewRaw("SELECT count(*) FROM schedule.care_schedule_change_requests WHERE tenant_id = ? AND student_id = ? AND status = 'approved'", w.tenantID, w.parentStudentID).Scan(ctx, &approved))
	require.NoError(t, w.db.NewRaw("SELECT count(*) FROM users.student_profiles WHERE tenant_id = ? AND id = ?", w.tenantID, w.stalePreviewID).Scan(ctx, &stale))
	require.Equal(t, w.decided, approved, "every measured decision must have approved its request")
	require.Equal(t, 1, stale, "a stale confirmation must leave the child in place")
	return map[string]int{"care_schedule_requests_approved": approved, "students_deleted": w.deleted, "staff_clock_transitions": w.staffClocks, "care_ended": w.careEnded}
}

// checkpointConcurrentOperation is one kind of request in a contention round.
type checkpointConcurrentOperation struct {
	Name            string             `json:"name"`
	Method          string             `json:"method"`
	Path            string             `json:"path"`
	AllowedStatuses []int              `json:"allowed_statuses"`
	Slots           int                `json:"slots_per_round"`
	Samples         []checkpointSample `json:"samples"`
	StatusCounts    map[string]int     `json:"status_counts"`
	P50MS           float64            `json:"p50_ms"`
	P95MS           float64            `json:"p95_ms"`
	UnexpectedCount int                `json:"unexpected_statuses"`
}

// checkpointConcurrentRun is one contention run: rounds of Concurrency
// simultaneous requests against one student graph.
type checkpointConcurrentRun struct {
	Name               string                               `json:"name"`
	Concurrency        int                                  `json:"concurrency"`
	PoolMaxOpen        int                                  `json:"pool_max_open_connections"`
	WarmupRounds       int                                  `json:"warmup_rounds"`
	MeasuredRounds     int                                  `json:"measured_rounds"`
	RoundWallMS        []float64                            `json:"round_wall_ms"`
	PoolWaitCount      int64                                `json:"pool_wait_count"`
	PoolWaitMS         float64                              `json:"pool_wait_ms"`
	RoundsWithPoolWait int                                  `json:"rounds_with_pool_wait"`
	LockSamples        checkpointLockSamples                `json:"lock_samples"`
	Deadlocks          int64                                `json:"deadlocks"`
	MetricsBefore      string                               `json:"metrics_before"`
	MetricsAfter       string                               `json:"metrics_after"`
	Operations         []*checkpointConcurrentOperation     `json:"operations"`
	JSONBRecordsets    []testpkg.RuntimeCheckpointRecordset `json:"jsonb_recordsets,omitempty"`
}

// contentionOperations lists the request kinds of one round. Slot 0 deletes
// the subject through the locked path and slot 1 deletes a companion, whose
// graph contains the subject; the remaining slots cycle through writers and
// readers of the same three rows, so the deletion's student-row locks are
// held against real waiters. A writer that loses the race gets the owner's
// documented answer: 404 once the child is gone, 409 when the deletion
// changed its companion list (companions_changed) or its preview.
func contentionOperations() []*checkpointConcurrentOperation {
	return []*checkpointConcurrentOperation{
		{Name: "student-deletion.execute-subject", Method: "DELETE", Path: "/api/students/{subject}", AllowedStatuses: []int{200, 409}},
		{Name: "student-deletion.execute-companion", Method: "DELETE", Path: "/api/students/{companion}", AllowedStatuses: []int{200, 404, 409}},
		{Name: "people-directory.update-subject", Method: "PUT", Path: "/api/students/{subject}", AllowedStatuses: []int{200, 404, 409}},
		{Name: "people-directory.update-second-companion", Method: "PUT", Path: "/api/students/{second}", AllowedStatuses: []int{200, 409}},
		{Name: "student-deletion.preview-companion", Method: "GET", Path: "/api/students/{companion}/delete-impact", AllowedStatuses: []int{200, 404}},
		{Name: "people-directory.students", Method: "GET", Path: "/api/students/", AllowedStatuses: []int{200}},
	}
}

// measureContention runs the concurrent part of checkpoint-1-v2. Each round
// builds a fresh subject with two linked companions and fetches both deletion
// previews outside timing, releases all requests at once, and afterwards
// deletes whatever the round left behind, so every round and every run starts
// from the same volume. Pool waits, lock observations, deadlocks and unit-of-
// work retries are measured over the measured rounds only.
func (w *targetRiskWorkload) measureContention(t *testing.T, production *Runtime, concurrency int) checkpointConcurrentRun {
	t.Helper()
	operations := contentionOperations()
	require.GreaterOrEqual(t, concurrency, len(operations), "every contention operation needs at least one slot per round")
	api := production.api
	slots := make([]*checkpointConcurrentOperation, concurrency)
	for i := range slots {
		index := i
		if i >= 2 {
			index = 2 + (i-2)%(len(operations)-2)
		}
		slots[i] = operations[index]
		slots[i].Slots++
	}
	// bun never removes a query hook, so the scoped counters are created once
	// and reused by every contention run instead of piling up per run.
	for len(w.slotCounters) < concurrency {
		w.slotCounters = append(w.slotCounters, testpkg.CaptureQueriesForContext(t, api.db))
	}
	counters := w.slotCounters[:concurrency]
	run := checkpointConcurrentRun{Name: "contention.student-graph", Concurrency: concurrency, PoolMaxOpen: api.db.Stats().MaxOpenConnections, WarmupRounds: 5, MeasuredRounds: 30, Operations: operations}
	deadlocks := func() int64 {
		var count int64
		require.NoError(t, w.db.NewRaw("SELECT deadlocks FROM pg_stat_database WHERE datname = current_database()").Scan(context.Background(), &count))
		return count
	}
	var stopSampling func() checkpointLockSamples
	var deadlocksBefore int64
	var recordsets testpkg.RuntimeCheckpointRecordsets
	for round := range run.WarmupRounds + run.MeasuredRounds {
		measured := round >= run.WarmupRounds
		subject := testpkg.CreateTestStudent(w.fixtures, w.db, "Contention", fmt.Sprintf("Subject%d", round), "3b").ID
		companion := testpkg.CreateTestStudent(w.fixtures, w.db, "Contention", fmt.Sprintf("Companion%d", round), "3b").ID
		second := testpkg.CreateTestStudent(w.fixtures, w.db, "Contention", fmt.Sprintf("Second%d", round), "3b").ID
		for _, id := range []int64{subject, companion, second} {
			w.accompanied(t, id)
		}
		w.link(t, subject, companion, second)
		ids := map[string]string{"{subject}": strconv.FormatInt(subject, 10), "{companion}": strconv.FormatInt(companion, 10), "{second}": strconv.FormatInt(second, 10)}
		bodies := map[string]string{
			"student-deletion.execute-subject":         w.deletionBody(t, subject),
			"student-deletion.execute-companion":       w.deletionBody(t, companion),
			"people-directory.update-subject":          fmt.Sprintf(`{"extra_info":"Contention round %d"}`, round),
			"people-directory.update-second-companion": fmt.Sprintf(`{"extra_info":"Contention round %d"}`, round),
		}
		requests := make([]*http.Request, concurrency)
		for i, operation := range slots {
			path := operation.Path
			for placeholder, id := range ids {
				path = strings.ReplaceAll(path, placeholder, id)
			}
			request := httptest.NewRequest(operation.Method, "http://localhost"+path, strings.NewReader(bodies[operation.Name]))
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("Authorization", "Bearer "+w.token)
			counters[i].Reset()
			requests[i] = request.WithContext(counters[i].Context(request.Context()))
		}
		if round == run.WarmupRounds {
			run.MetricsBefore = checkpointMetrics(t)
			deadlocksBefore = deadlocks()
			stopSampling = testpkg.SampleCheckpointLocks(func(ctx context.Context) (int, error) {
				var waiting int
				err := w.db.NewRaw("SELECT count(*) FROM pg_stat_activity WHERE datname = current_database() AND usename = 'phoenix_auth' AND wait_event_type = 'Lock'").Scan(ctx, &waiting)
				return waiting, err
			})
			t.Cleanup(func() { _ = stopSampling() })
		}
		responses := make([]*httptest.ResponseRecorder, concurrency)
		durations := make([]time.Duration, concurrency)
		start := make(chan struct{})
		var wg sync.WaitGroup
		for i := range slots {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				started := time.Now()
				recorder := httptest.NewRecorder()
				w.handler.ServeHTTP(recorder, requests[i])
				durations[i] = time.Since(started)
				responses[i] = recorder
			}()
		}
		before := api.db.Stats()
		roundStarted := time.Now()
		close(start)
		wg.Wait()
		wall := time.Since(roundStarted)
		after := api.db.Stats()
		if measured {
			run.RoundWallMS = append(run.RoundWallMS, float64(wall)/float64(time.Millisecond))
			waits := after.WaitCount - before.WaitCount
			run.PoolWaitCount += waits
			run.PoolWaitMS += float64(after.WaitDuration-before.WaitDuration) / float64(time.Millisecond)
			if waits > 0 {
				run.RoundsWithPoolWait++
			}
			for i, operation := range slots {
				recordContentionSample(operation, responses[i], durations[i], counters[i])
				recordsets.Observe(counters[i].Queries())
			}
		} else {
			for i, operation := range slots {
				require.Contains(t, operation.AllowedStatuses, responses[i].Code, "warmup %s: %s", operation.Name, responses[i].Body.String())
			}
		}
		w.removeLeftovers(t, subject, companion, second)
	}
	run.MetricsAfter = checkpointMetrics(t)
	run.LockSamples = stopSampling()
	require.Empty(t, run.LockSamples.Error)
	run.Deadlocks = deadlocks() - deadlocksBefore
	run.JSONBRecordsets = recordsets.Result()
	for _, operation := range operations {
		latencies := make([]float64, len(operation.Samples))
		for i, sample := range operation.Samples {
			latencies[i] = sample.DurationMS
		}
		sort.Float64s(latencies)
		operation.P50MS = latencies[int(math.Ceil(float64(len(latencies))*0.5))-1]
		operation.P95MS = latencies[int(math.Ceil(float64(len(latencies))*0.95))-1]
	}
	return run
}

func recordContentionSample(operation *checkpointConcurrentOperation, response *httptest.ResponseRecorder, duration time.Duration, counter *testpkg.QueryCounter) {
	sample := checkpointSample{DurationMS: float64(duration) / float64(time.Millisecond), Queries: counter.Total(), Status: response.Code}
	sample.RowsAffected, sample.StatementsWithRows = counter.Rows()
	writes := counter.WriteRows()
	sample.WriteRowsAffected = &writes
	if response.Code >= 400 {
		sample.ErrorBody = response.Body.String()
	}
	operation.Samples = append(operation.Samples, sample)
	if operation.StatusCounts == nil {
		operation.StatusCounts = map[string]int{}
	}
	operation.StatusCounts[strconv.Itoa(response.Code)]++
	allowed := false
	for _, status := range operation.AllowedStatuses {
		allowed = allowed || status == response.Code
	}
	if !allowed {
		operation.UnexpectedCount++
	}
}

// removeLeftovers deletes, outside measurement, every child of the round the
// concurrent requests did not delete, through the same HTTP workflow.
func (w *targetRiskWorkload) removeLeftovers(t *testing.T, ids ...int64) {
	t.Helper()
	for _, id := range ids {
		preview := checkpointRequest(w.handler, checkpointScenario{Method: http.MethodGet, Path: fmt.Sprintf("/api/students/%d/delete-impact", id), Authenticated: true}, w.token)
		if preview.Code == http.StatusNotFound {
			continue
		}
		deleted := checkpointRequest(w.handler, checkpointScenario{Method: http.MethodDelete, Path: fmt.Sprintf("/api/students/%d", id), Authenticated: true, Body: w.deletionBody(t, id)}, w.token)
		require.Equal(t, http.StatusOK, deleted.Code, "remove contention leftover %d: %s", id, deleted.Body.String())
	}
}

// fixtureRows reports the volume of the workload's own tenant after setup.
func (w *targetRiskWorkload) fixtureRows(t *testing.T) map[string]int64 {
	t.Helper()
	rows := map[string]int64{}
	for _, table := range []string{"users.student_profiles", "users.student_companions", "facilities.rooms", "activities.groups", "activities.student_enrollments", "auth.account_tenants", "schedule.care_schedule_change_requests", "users.care_withdrawal_completions"} {
		var count int64
		require.NoError(t, w.db.NewRaw("SELECT count(*) FROM "+table+" WHERE tenant_id = ?", w.tenantID).Scan(context.Background(), &count))
		rows[table] = count
	}
	return rows
}
