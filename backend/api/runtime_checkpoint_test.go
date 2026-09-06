package api

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

var runtimeCheckpointOutput = flag.String("runtime-checkpoint-output", "", "write the opt-in #3019 runtime measurement JSON to this file")
var runtimeCheckpointEnrollment = flag.Bool("runtime-checkpoint-enrollment", false, "measure the #2694 Enrollment read workload instead of the checkpoint baseline")
var runtimeCheckpointEnrollmentWrites = flag.Bool("runtime-checkpoint-enrollment-writes", false, "measure the #2694 Enrollment phase-update workload instead of the checkpoint baseline")

var runtimeCheckpointEnrollmentParents = flag.Bool("runtime-checkpoint-enrollment-parents", false, "add authenticated parent intake to the Enrollment write workload")

type checkpointSample = testpkg.RuntimeCheckpointSample

type checkpointScenario struct {
	Name           string `json:"name"`
	Method         string `json:"method"`
	Path           string `json:"path"`
	ExpectedStatus int    `json:"expected_status"`
	Authenticated  bool   `json:"authenticated"`
	Body           string `json:"body,omitempty"`
}

type checkpointResult struct {
	Scenario           checkpointScenario    `json:"scenario"`
	Samples            []checkpointSample    `json:"samples"`
	P50MS              float64               `json:"p50_ms"`
	P95MS              float64               `json:"p95_ms"`
	UnexpectedStatuses int                   `json:"unexpected_statuses"`
	MetricsBefore      string                `json:"metrics_before"`
	MetricsAfter       string                `json:"metrics_after"`
	LockSamples        checkpointLockSamples `json:"lock_samples"`
	Deadlocks          int64                 `json:"deadlocks"`
}

type checkpointLockSamples = testpkg.RuntimeCheckpointLockSamples

// measureRuntimeCheckpoint uses the existing production graph, without another
// composition root or replacement HTTP middleware. Run only this top-level
// test in a dedicated process so measurements cannot include parallel tests.
func measureRuntimeCheckpoint(t *testing.T, production *Runtime) {
	t.Helper()
	require.Equal(t, "^TestFullProductionRouterGolden$", flag.Lookup("test.run").Value.String(), "checkpoint requires a dedicated test process")
	require.False(t, *runtimeCheckpointEnrollment && *runtimeCheckpointEnrollmentWrites, "select exactly one Enrollment workload")
	require.True(t, !*runtimeCheckpointEnrollmentParents || *runtimeCheckpointEnrollmentWrites, "parent workload requires the Enrollment writes option")
	output, err := os.OpenFile(*runtimeCheckpointOutput, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	require.NoError(t, err, "use a new output path for each checkpoint execution")
	defer func() { _ = output.Close() }()
	api := production.api
	db := testpkg.SetupTestDB(t)
	_, account := testpkg.CreateTestTeacherWithAccount(t, db, "Checkpoint", "Staff")
	phase := testpkg.CreateTestEnrollmentPhase(t, db)
	_, err = db.NewRaw("UPDATE enrollment.phases SET service_start_date = '2026-08-01', service_end_date = '2027-07-31' WHERE id = ? AND tenant_id = ?", phase.ID, testpkg.Tenant(t)).Exec(context.Background())
	require.NoError(t, err)
	for i := range 10 {
		testpkg.CreateTestCareOffering(t, db, phase.ID, fmt.Sprintf("Checkpoint Offering %d", i))
	}
	for i := range 50 {
		testpkg.CreateTestStudent(t, db, "Checkpoint", fmt.Sprintf("Student%d", i), "1a")
		testpkg.CreateTestGuardianProfile(t, db, fmt.Sprintf("checkpoint%d", i))
	}
	for i := range 10 {
		testpkg.CreateTestRoom(t, db, fmt.Sprintf("Checkpoint Room %d", i))
		testpkg.CreateTestEducationGroup(t, db, fmt.Sprintf("Checkpoint Group %d", i))
		testpkg.CreateTestActivityGroup(t, db, fmt.Sprintf("Checkpoint Activity %d", i))
	}
	writeRoom := testpkg.CreateTestRoom(t, db, "Checkpoint Writable Room")
	templateID := testpkg.CreateRuntimeCheckpointTimetable(t, db, writeRoom.ID)
	var missingActivityID int64
	require.NoError(t, db.NewRaw("SELECT nextval(pg_get_serial_sequence('activities.groups', 'id'))").Scan(context.Background(), &missingActivityID))
	var slug string
	require.NoError(t, db.NewRaw("SELECT subdomain FROM platform.schools WHERE id = ?", testpkg.Tenant(t)).Scan(context.Background(), &slug))
	token := testutil.MintTestJWT(t, testutil.AdminTestClaimsForTenant(int(account.ID), testpkg.Tenant(t)))
	scenarios := []checkpointScenario{
		{"timetable.not-found", "GET", fmt.Sprintf("/api/activities/%d", missingActivityID), 404, true, ""},
		{"organization-tenancy.resolve", "GET", "/auth/tenant/resolve?slug=" + slug, 200, false, ""},
		{"facilities.update", "PUT", fmt.Sprintf("/api/rooms/%d", writeRoom.ID), 200, true, `{"name":"Checkpoint Updated Room","capacity":40}`},
		{"facilities.list", "GET", "/api/rooms/", 200, true, ""},
		{"school-structure.list", "GET", "/api/groups/", 200, true, ""},
		{"people-directory.guardians", "GET", "/api/guardians/", 200, true, ""},
		{"timetable.activities", "GET", "/api/activities/", 200, true, ""},
		{"timetable.categories", "GET", "/api/activities/categories", 200, true, ""},
		{"school-calendar.periods", "GET", "/api/timetable/periods/", 200, true, ""},
		{"care-plan.offerings", "GET", "/api/enrollment/care-offerings/", 200, true, ""},
		{"communication.messages", "GET", "/api/messages/", 200, true, ""},
		{"appointments.calendar", "GET", "/api/calendar/my?from=2026-09-01&to=2026-09-30", 200, true, ""},
		{"settings.schema", "GET", "/api/settings/schema", 200, true, ""},
		{"meal-plan.list", "GET", "/api/meal-plan/?week_start=2026-08-31", 200, true, ""},
		{"feedback.list", "GET", "/api/feedback/", 200, true, ""},
		{"school-membership.staff", "GET", "/api/staff/", 200, true, ""},
		{"facilities.invalid-id", "GET", "/api/rooms/invalid", 400, true, ""},
		{"timetable.invalid-id", "GET", "/api/activities/invalid", 400, true, ""},
		{"people-directory.invalid-id", "GET", "/api/guardians/invalid", 400, true, ""},
		{"security.unauthenticated", "GET", "/api/rooms/", 401, false, ""},
	}
	workloadVersion := "checkpoint-1-v1"
	var checkpointSchemaID, parentToken string
	var parentAccountID int64
	if *runtimeCheckpointEnrollment || *runtimeCheckpointEnrollmentWrites {
		enabled := checkpointRequest(production.Handler(), checkpointScenario{
			Method: "PUT", Path: "/api/settings/values/enrollment.enabled", Authenticated: true, Body: `{"value":true}`,
		}, token)
		require.Equal(t, http.StatusOK, enabled.Code, enabled.Body.String())
		require.NoError(t, db.NewRaw("SELECT slug FROM platform.schools WHERE id = ?", testpkg.Tenant(t)).Scan(context.Background(), &slug))
		workloadVersion = "enrollment-2694-reads-v1"
		scenarios = []checkpointScenario{
			{"enrollment.phases", "GET", "/api/enrollment/phases/", 200, true, ""},
			{"enrollment.phase", "GET", fmt.Sprintf("/api/enrollment/phases/%d", phase.ID), 200, true, ""},
			{"enrollment.schema-versions", "GET", "/api/enrollment/schema/versions", 200, true, ""},
			{"enrollment.public-phases", "GET", "/api/enrollment/phases/public/" + slug, 200, false, ""},
			{"enrollment.public-bootstrap", "GET", fmt.Sprintf("/api/enrollment/form-bootstrap/public/%s/%d", slug, phase.ID), 200, false, ""},
			{"enrollment.phase-invalid-id", "GET", "/api/enrollment/phases/invalid", 400, true, ""},
		}
		if *runtimeCheckpointEnrollmentWrites {
			duplicatePolicy := checkpointRequest(production.Handler(), checkpointScenario{
				Method: "PUT", Path: "/api/settings/values/enrollment.duplicate_handling", Authenticated: true, Body: `{"value":"block"}`,
			}, token)
			require.Equal(t, http.StatusOK, duplicatePolicy.Code, duplicatePolicy.Body.String())
			created := checkpointRequest(production.Handler(), checkpointScenario{
				Method: "POST", Path: "/api/enrollment/schema/", Authenticated: true, Body: `{"name":"Checkpoint Schema","fields":[]}`,
			}, token)
			require.Equal(t, http.StatusCreated, created.Code, created.Body.String())
			var schemaResponse struct {
				Data struct {
					ID string `json:"id"`
				} `json:"data"`
			}
			require.NoError(t, json.Unmarshal(created.Body.Bytes(), &schemaResponse))
			checkpointSchemaID = schemaResponse.Data.ID
			require.NotEmpty(t, checkpointSchemaID)
			conflict := checkpointRequest(production.Handler(), checkpointScenario{
				Method: "POST", Path: "/api/enrollment/schema/", Authenticated: true, Body: `{"name":"Checkpoint Reserved Schema","fields":[]}`,
			}, token)
			require.Equal(t, http.StatusCreated, conflict.Code, conflict.Body.String())
			captcha := checkpointRequest(production.Handler(), checkpointScenario{Method: "GET", Path: "/api/enrollment/captcha-config/" + slug}, token)
			require.Equal(t, http.StatusOK, captcha.Code, captcha.Body.String())
			var captchaResponse struct {
				Data struct {
					Enabled bool `json:"enabled"`
				} `json:"data"`
			}
			require.NoError(t, json.Unmarshal(captcha.Body.Bytes(), &captchaResponse))
			require.False(t, captchaResponse.Data.Enabled, "intake workload requires the fresh-tenant captcha-disabled configuration")
			throttled := checkpointScenario{"enrollment.public-submit-rate-limited", "POST", "/api/enrollment/" + slug + "/submit", 429, false, `{}`}
			for range 10 {
				attempt := checkpointRequest(production.Handler(), throttled, token)
				require.Equal(t, http.StatusBadRequest, attempt.Code, "prime IP bucket with invalid submissions: %s", attempt.Body.String())
			}
			emailThrottled := checkpointScenario{"enrollment.public-submit-email-rate-limited", "POST", "/api/enrollment/" + slug + "/submit", 429, false,
				`{"guardian_email":"checkpoint-throttled@example.test","custom_data":{"attempt":"{{attempt}}"}}`}
			for i := range 5 {
				attempt := checkpointSequencedRequest(production.Handler(), emailThrottled, token, 10000+i)
				require.Equal(t, http.StatusBadRequest, attempt.Code, "prime email bucket with distinct IPs: %s", attempt.Body.String())
			}
			var parentSubdomain string
			workloadVersion = "enrollment-2694-writes-v7"
			if *runtimeCheckpointEnrollmentParents {
				parentChain := testpkg.CreateTestParentGuardianChain(t, db)
				parentAccountID = parentChain.AccountID
				parentClaims := jwt.AppClaims{ID: int(parentAccountID), Sub: parentChain.Email, Scope: "parent", Roles: []string{"guardian"}}
				parentToken = testutil.MintTestJWT(t, parentClaims)
				require.NoError(t, db.NewRaw("SELECT subdomain FROM platform.schools WHERE id = ?", testpkg.Tenant(t)).Scan(context.Background(), &parentSubdomain))
				workloadVersion = "enrollment-2694-writes-v8"
			}
			scenarios = []checkpointScenario{
				{"enrollment.phase-update", "PUT", fmt.Sprintf("/api/enrollment/phases/%d", phase.ID), 200, true,
					`{"name":"Checkpoint Updated Phase","kind":"school_year","service_start_date":"2026-08-01","service_end_date":"2027-07-31","care_overflow_mode":"waitlist","care_offering_selection_mode":"optional","is_active":true}`},
				{"enrollment.phase-update-invalid", "PUT", fmt.Sprintf("/api/enrollment/phases/%d", phase.ID), 400, true, `{}`},
				{"enrollment.schema-rename", "PATCH", "/api/enrollment/schema/" + checkpointSchemaID, 200, true, `{"name":"Checkpoint Renamed Schema"}`},
				{"enrollment.schema-rename-conflict", "PATCH", "/api/enrollment/schema/" + checkpointSchemaID, 409, true, `{"name":"Checkpoint Reserved Schema"}`},
				{"enrollment.schema-publish-version", "PUT", "/api/enrollment/schema/" + checkpointSchemaID, 201, true, `{"fields":[]}`},
				{"enrollment.public-submit", "POST", "/api/enrollment/" + slug + "/submit", 201, false,
					fmt.Sprintf(`{"phase_id":%d,"guardian_first_name":"Checkpoint","guardian_last_name":"Applicant","guardian_email":"checkpoint-{{attempt}}@example.test","consent_flags":{"agb":true,"data_processing":true,"email_contact":true,"photo":true},"children":[{"first_name":"Checkpoint","last_name":"Child{{attempt}}","date_of_birth":"2018-04-15","target_grade_level":2}]}`, phase.ID)},
				throttled,
				emailThrottled,
			}
			if *runtimeCheckpointEnrollmentParents {
				parentSubmit := checkpointScenario{"enrollment.parent-submit", "POST", "/parent/enrollments/" + parentSubdomain + "/submit", 201, true,
					fmt.Sprintf(`{"phase_id":%d,"guardian_first_name":"Checkpoint","guardian_last_name":"Parent","guardian_email":"checkpoint-parent-{{attempt}}@example.test","consent_flags":{"agb":true,"data_processing":true,"email_contact":true,"photo":true},"children":[{"first_name":"Checkpoint","last_name":"ParentChild{{attempt}}","date_of_birth":"2018-04-15","target_grade_level":2}]}`, phase.ID)}
				scenarios = append(scenarios, parentSubmit)
				wrongScope := parentSubmit
				wrongScope.Name = "enrollment.parent-submit-wrong-scope"
				wrongScope.ExpectedStatus = http.StatusUnauthorized
				scenarios = append(scenarios, wrongScope)
			}
			for _, scenario := range scenarios {
				if scenario.Name == "enrollment.public-submit" {
					scenario.Name = "enrollment.public-submit-duplicate"
					scenario.ExpectedStatus = http.StatusConflict
					scenarios = append(scenarios, scenario)
					break
				}
			}
		}
	}
	counter := testpkg.CaptureQueries(t, api.db)
	counter.Stop()
	var postgresVersion string
	require.NoError(t, db.NewSelect().ColumnExpr("version()").Scan(context.Background(), &postgresVersion))
	var role string
	require.NoError(t, api.db.NewSelect().ColumnExpr("current_user").Scan(context.Background(), &role))
	require.Equal(t, "phoenix_auth", role)
	var databaseSettings []struct {
		Name    string `json:"name"`
		Setting string `json:"setting"`
		Unit    string `json:"unit"`
	}
	require.NoError(t, db.NewRaw("SELECT name, setting, COALESCE(unit, '') AS unit FROM pg_settings WHERE name IN ('server_version_num', 'max_connections', 'shared_buffers', 'work_mem', 'effective_cache_size', 'fsync', 'synchronous_commit', 'track_activities', 'track_counts', 'TimeZone') ORDER BY name").Scan(context.Background(), &databaseSettings))
	volumes := map[string]int64{}
	for _, table := range []string{"users.students", "users.guardian_profiles", "users.staff", "facilities.rooms", "education.groups", "activities.groups", "activities.categories", "enrollment.phases", "enrollment.care_offerings", "schedule.calendar_periods"} {
		var count int64
		require.NoError(t, db.NewRaw("SELECT count(*) FROM "+table+" WHERE tenant_id = ?", testpkg.Tenant(t)).Scan(context.Background(), &count))
		volumes[table] = count
	}
	environment := map[string]any{
		"started_at": time.Now().UTC().Format(time.RFC3339Nano), "gomaxprocs": runtime.GOMAXPROCS(0), "logical_cpus": runtime.NumCPU(),
		"pool_max_open_connections": api.db.Stats().MaxOpenConnections, "database_settings": databaseSettings, "fixture_rows": volumes,
		"http_transport": "in-process Runtime.Handler (no TCP/TLS)", "email_transport": "production mock mailer (unavailable)",
		"lock_poll_interval_ms": 2, "app_env": os.Getenv("APP_ENV"),
	}
	var runs [][]checkpointResult
	var workerRuns [][]testpkg.RuntimeCheckpointWorkerResult
	requestSequence := 0
	var preparedDuplicates []int
	runRequest := func(scenario checkpointScenario) *httptest.ResponseRecorder {
		sequence := 0
		if scenario.Name == "enrollment.public-submit-duplicate" {
			require.NotEmpty(t, preparedDuplicates)
			sequence, preparedDuplicates = preparedDuplicates[0], preparedDuplicates[1:]
		} else {
			requestSequence++
			sequence = requestSequence
		}
		requestToken := token
		if scenario.Name == "enrollment.parent-submit" {
			requestToken = parentToken
		}
		response := checkpointSequencedRequest(production.Handler(), scenario, requestToken, sequence)
		if scenario.ExpectedStatus == http.StatusTooManyRequests {
			require.Equal(t, "3600", response.Header().Get("Retry-After"))
		}
		return response
	}
	for range 3 {
		var results []checkpointResult
		for _, scenario := range scenarios {
			if scenario.Name == "enrollment.public-submit-duplicate" {
				// Seed one original per retry before warmup and all measurement
				// counters. Each client makes only two attempts, below both limits.
				for range 5 + 30 {
					requestSequence++
					original := checkpointSequencedRequest(production.Handler(), scenario, token, requestSequence)
					require.Equal(t, http.StatusCreated, original.Code, original.Body.String())
					preparedDuplicates = append(preparedDuplicates, requestSequence)
				}
			}
			for range 5 {
				response := runRequest(scenario)
				require.Equal(t, scenario.ExpectedStatus, response.Code, "warmup %s: %s", scenario.Name, response.Body.String())
			}
			result := checkpointResult{Scenario: scenario, MetricsBefore: checkpointMetrics(t)}
			deadlocks := func() int64 {
				var count int64
				require.NoError(t, db.NewRaw("SELECT deadlocks FROM pg_stat_database WHERE datname = current_database()").Scan(context.Background(), &count))
				return count
			}
			deadlocksBefore := deadlocks()
			stopSampling := testpkg.SampleCheckpointLocks(func(ctx context.Context) (int, error) {
				var waiting int
				err := db.NewRaw("SELECT count(*) FROM pg_stat_activity WHERE datname = current_database() AND usename = 'phoenix_auth' AND wait_event_type = 'Lock'").Scan(ctx, &waiting)
				return waiting, err
			})
			t.Cleanup(func() { _ = stopSampling() })
			for range 30 {
				counter.Reset()
				before := api.db.Stats()
				counter.Start()
				started := time.Now()
				response := runRequest(scenario)
				elapsed := time.Since(started)
				counter.Stop()
				after := api.db.Stats()
				result.Samples = append(result.Samples, checkpointSample{
					DurationMS: float64(elapsed) / float64(time.Millisecond), Queries: counter.Total(), Status: response.Code,
					PoolWaitCount: after.WaitCount - before.WaitCount,
					PoolWaitMS:    float64(after.WaitDuration-before.WaitDuration) / float64(time.Millisecond),
				})
				result.Samples[len(result.Samples)-1].RowsAffected, result.Samples[len(result.Samples)-1].StatementsWithRows = counter.Rows()
				writeRows := counter.WriteRows()
				result.Samples[len(result.Samples)-1].WriteRowsAffected = &writeRows
				if response.Code >= 400 {
					result.Samples[len(result.Samples)-1].ErrorBody = response.Body.String()
				}
				if response.Code != scenario.ExpectedStatus {
					result.UnexpectedStatuses++
					if result.UnexpectedStatuses == 1 {
						t.Logf("%s: expected %d, got %d: %s", scenario.Name, scenario.ExpectedStatus, response.Code, response.Body.String())
					}
				}
			}
			result.MetricsAfter = checkpointMetrics(t)
			result.LockSamples = stopSampling()
			require.Empty(t, result.LockSamples.Error)
			result.Deadlocks = deadlocks() - deadlocksBefore
			latencies := make([]float64, len(result.Samples))
			for i, sample := range result.Samples {
				latencies[i] = sample.DurationMS
			}
			sort.Float64s(latencies)
			result.P50MS = latencies[int(math.Ceil(float64(len(latencies))*0.5))-1]
			result.P95MS = latencies[int(math.Ceil(float64(len(latencies))*0.95))-1]
			results = append(results, result)
		}
		runs = append(runs, results)
		if *runtimeCheckpointEnrollment || *runtimeCheckpointEnrollmentWrites {
			continue
		}
		workers := testpkg.MeasureDeliveryCheckpoint(t, api.db, db, api.Services.EmailOutboxWorker, api.tenantRuntime, counter, func() string { return checkpointMetrics(t) })
		workers = append(workers, testpkg.MeasureTimetableCheckpoint(t, api.db, db, api.tenantRuntime, counter, templateID, func(ctx context.Context) (int, int, error) {
			result, materializeErr := api.Services.Materialization.MaterializeForTenant(ctx, "2026-09-07", "2026-09-07", "scheduler")
			if materializeErr != nil {
				return 0, 0, materializeErr
			}
			return result.InstancesCreated, result.CandidatesSkippedExisting, nil
		}, func() string { return checkpointMetrics(t) })...)
		workerRuns = append(workerRuns, workers)
	}
	var finalState map[string]int
	if *runtimeCheckpointEnrollmentWrites {
		var persistedName string
		require.NoError(t, db.NewRaw("SELECT name FROM enrollment.phases WHERE id = ? AND tenant_id = ?", phase.ID, testpkg.Tenant(t)).Scan(context.Background(), &persistedName))
		require.Equal(t, "Checkpoint Updated Phase", persistedName, "invalid updates must preserve the successful write")
		require.NoError(t, db.NewRaw("SELECT name FROM enrollment.form_schemas WHERE id = ? AND tenant_id = ?", checkpointSchemaID, testpkg.Tenant(t)).Scan(context.Background(), &persistedName))
		require.Equal(t, "Checkpoint Renamed Schema", persistedName, "conflicting renames must preserve the successful write")
		var versions, latestVersion int
		require.NoError(t, db.NewRaw("SELECT count(*), max(version) FROM enrollment.form_schemas WHERE name = ? AND tenant_id = ?", persistedName, testpkg.Tenant(t)).Scan(context.Background(), &versions, &latestVersion))
		require.Equal(t, 1+3*(5+30), versions, "each warmup and measured publication must append one version")
		require.Equal(t, versions, latestVersion, "publication must produce consecutive versions")
		var submittedRequests, submittedChildren int
		require.NoError(t, db.NewRaw("SELECT count(*) FROM enrollment.requests WHERE phase_id = ? AND tenant_id = ?", phase.ID, testpkg.Tenant(t)).Scan(context.Background(), &submittedRequests))
		require.NoError(t, db.NewRaw("SELECT count(*) FROM enrollment.request_children child JOIN enrollment.requests request ON request.id = child.request_id AND request.tenant_id = child.tenant_id WHERE request.phase_id = ? AND child.tenant_id = ?", phase.ID, testpkg.Tenant(t)).Scan(context.Background(), &submittedChildren))
		successfulPaths := 2
		if *runtimeCheckpointEnrollmentParents {
			successfulPaths++
		}
		require.Equal(t, successfulPaths*3*(5+30), submittedRequests, "only public and parent successes and duplicate originals may create requests")
		require.Equal(t, submittedRequests, submittedChildren)
		var parentRequests int
		require.NoError(t, db.NewRaw("SELECT count(*) FROM enrollment.requests WHERE phase_id = ? AND tenant_id = ? AND guardian_account_id = ?", phase.ID, testpkg.Tenant(t), parentAccountID).Scan(context.Background(), &parentRequests))
		expectedParentRequests := 0
		if *runtimeCheckpointEnrollmentParents {
			expectedParentRequests = 3 * (5 + 30)
		}
		require.Equal(t, expectedParentRequests, parentRequests, "parent intake must stamp the authenticated guardian account")
		var ipAttempts, emailAttempts int
		require.NoError(t, db.NewRaw("SELECT max(attempts) FROM enrollment.submission_rate_limits WHERE tenant_id = ? AND key_type = 'ip'", testpkg.Tenant(t)).Scan(context.Background(), &ipAttempts))
		require.NoError(t, db.NewRaw("SELECT attempts FROM enrollment.submission_rate_limits WHERE tenant_id = ? AND key_type = 'email' AND key_value = ?", testpkg.Tenant(t), "checkpoint-throttled@example.test").Scan(context.Background(), &emailAttempts))
		require.Equal(t, 10+3*(5+30), ipAttempts, "rejected IP attempts must remain committed")
		require.Equal(t, 5+3*(5+30), emailAttempts, "rejected email attempts must remain committed")
		finalState = map[string]int{"schema_versions": versions, "latest_schema_version": latestVersion,
			"requests": submittedRequests, "children": submittedChildren, "parent_requests": parentRequests,
			"throttled_ip_attempts": ipAttempts, "throttled_email_attempts": emailAttempts}
	}
	report := struct {
		WorkloadVersion    string                                    `json:"workload_version"`
		GoVersion          string                                    `json:"go_version"`
		Platform           string                                    `json:"platform"`
		PostgresVersion    string                                    `json:"postgres_version"`
		DatabaseRole       string                                    `json:"database_role"`
		Concurrency        int                                       `json:"concurrency"`
		WarmupPerScenario  int                                       `json:"warmup_per_scenario_per_run"`
		SamplesPerScenario int                                       `json:"samples_per_scenario_per_run"`
		Runs               [][]checkpointResult                      `json:"runs"`
		WorkerRuns         [][]testpkg.RuntimeCheckpointWorkerResult `json:"worker_runs"`
		Environment        map[string]any                            `json:"environment"`
		FinalState         map[string]int                            `json:"final_state,omitempty"`
	}{workloadVersion, runtime.Version(), runtime.GOOS + "/" + runtime.GOARCH, postgresVersion, role, 1, 5, 30, runs, workerRuns, environment, finalState}
	data, err := json.MarshalIndent(report, "", "  ")
	require.NoError(t, err)
	_, err = output.Write(append(data, '\n'))
	require.NoError(t, err)
	require.NoError(t, output.Close())
	for _, run := range runs {
		for _, result := range run {
			require.Zero(t, result.UnexpectedStatuses, result.Scenario.Name)
		}
	}
}

func checkpointRequest(handler http.Handler, scenario checkpointScenario, token string) *httptest.ResponseRecorder {
	return checkpointSequencedRequest(handler, scenario, token, 0)
}

func checkpointSequencedRequest(handler http.Handler, scenario checkpointScenario, token string, sequence int) *httptest.ResponseRecorder {
	uniqueClient := strings.Contains(scenario.Body, "{{attempt}}")
	scenario.Body = strings.ReplaceAll(scenario.Body, "{{attempt}}", fmt.Sprint(sequence))
	request := httptest.NewRequest(scenario.Method, "http://localhost"+scenario.Path, strings.NewReader(scenario.Body))
	if uniqueClient {
		request.RemoteAddr = fmt.Sprintf("[2001:db8::%x]:12345", sequence)
	}
	request.Header.Set("Content-Type", "application/json")
	if scenario.Authenticated {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func checkpointMetrics(t *testing.T) string {
	t.Helper()
	response := httptest.NewRecorder()
	metricsHandler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "http://localhost/internal/metrics", nil))
	require.Equal(t, http.StatusOK, response.Code)
	var lines []string
	for _, line := range strings.Split(response.Body.String(), "\n") {
		name, _, _ := strings.Cut(line, "{")
		name, _, _ = strings.Cut(name, " ")
		if strings.HasPrefix(name, "phoenix_") && strings.HasSuffix(name, "_total") &&
			!strings.HasPrefix(name, "phoenix_settings_") &&
			!strings.HasPrefix(name, "phoenix_backend_") &&
			!strings.HasPrefix(name, "phoenix_tenant_") {
			lines = append(lines, line)
		}
	}
	return strings.Join(lines, "\n") + "\n"
}
