package api

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type payrollSeedServer struct {
	mu      sync.Mutex
	numbers map[int64]string
	auths   []string
	notes   []string
	status  string
}

func newPayrollSeedServer(t *testing.T, numbers map[int64]string, status string) (*payrollSeedServer, string) {
	t.Helper()
	server := &payrollSeedServer{numbers: numbers, status: status}
	srv := newSeedHTTPTestServer(func(w seedHTTPResponseWriter, r *seedHTTPRequest) {
		server.mu.Lock()
		defer server.mu.Unlock()
		server.auths = append(server.auths, r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/api/staff/":
			// Name order, not ID order: the step must not depend on it.
			_, _ = fmt.Fprint(w, `{"data":[{"id":"12"},{"id":"10"},{"id":"11"},{"id":"13"}]}`)
		case r.URL.Path == "/api/settings/payroll-status":
			_, _ = fmt.Fprint(w, server.status)
		case strings.HasSuffix(r.URL.Path, "/payroll-number"):
			var id int64
			_, _ = fmt.Sscanf(r.URL.Path, "/api/staff/%d/payroll-number", &id)
			if r.Method == "PUT" {
				var body struct {
					PersonnelNumber string `json:"personnel_number"`
					Note            string `json:"note"`
				}
				_ = json.NewDecoder(r.Body).Decode(&body)
				server.numbers[id] = body.PersonnelNumber
				server.notes = append(server.notes, body.Note)
			}
			number, ok := server.numbers[id]
			if !ok {
				_, _ = fmt.Fprintf(w, `{"data":{"staff_id":%d,"personnel_number":null}}`, id)
				return
			}
			_, _ = fmt.Fprintf(w, `{"data":{"staff_id":%d,"personnel_number":%q}}`, id, number)
		default:
			w.WriteHeader(404)
		}
	})
	t.Cleanup(srv.Close)
	return server, srv.URL
}

const completePayrollStatus = `{"data":{"lodas_header_complete":true,"configured_categories":7,"total_categories":7,"staff_total":4,"staff_without_personnel_number":0}}`

func TestSeedPayrollNumbersStep_NumbersEveryStaffMemberAsSchool(t *testing.T) {
	t.Parallel()

	// Staff 11 already holds the first demo number, as after a partial run.
	server, url := newPayrollSeedServer(t, map[int64]string{11: "70001"}, completePayrollStatus)
	rt := &Runtime{
		Client:       newTestClient(url, false),
		TenantAuth:   AuthRef{Kind: AuthBearer, Token: "tenant-token"},
		OperatorAuth: AuthRef{Kind: AuthBearer, Token: "operator-token"},
	}

	require.NoError(t, seedPayrollNumbersStep{}.Run(context.Background(), rt))

	assert.Equal(t, map[int64]string{10: "70002", 11: "70001", 12: "70003", 13: "70004"}, server.numbers)
	assert.Len(t, server.notes, 3)
	for _, auth := range server.auths {
		assert.Equal(t, "Bearer tenant-token", auth)
	}
}

func TestSeedPayrollNumbersStep_FailsOnIncompletePayrollStatus(t *testing.T) {
	t.Parallel()

	for name, status := range map[string]string{
		"wage types":    `{"data":{"lodas_header_complete":true,"configured_categories":6,"total_categories":7,"staff_total":4,"staff_without_personnel_number":0}}`,
		"DATEV client":  `{"data":{"lodas_header_complete":false,"configured_categories":7,"total_categories":7,"staff_total":4,"staff_without_personnel_number":0}}`,
		"staff numbers": `{"data":{"lodas_header_complete":true,"configured_categories":7,"total_categories":7,"staff_total":5,"staff_without_personnel_number":1}}`,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, url := newPayrollSeedServer(t, map[int64]string{}, status)
			rt := &Runtime{Client: newTestClient(url, false), TenantAuth: AuthRef{Kind: AuthBearer, Token: "tenant-token"}}
			assert.ErrorContains(t, seedPayrollNumbersStep{}.Run(context.Background(), rt), "payroll status")
		})
	}
}

func TestPayrollSettings_OnlyTheVollbetriebSchool(t *testing.T) {
	t.Parallel()

	full := fullOperationProfileDefinition().Settings
	payroll := demoPayrollSettings()
	require.Len(t, payroll, 12)
	for key, setting := range payroll {
		assert.Equal(t, setting, full[key], key)
		assert.Equal(t, SettingManagedByTenant, setting.ManagedBy, "%s is admin-only and written by the school", key)
	}
	for name, definition := range map[string]demoProfileDefinition{
		"manuell":    manualProfileDefinition(),
		"wochenplan": enrollmentWeeklyProfileDefinition(),
		"buchungen":  enrollmentBookingsProfileDefinition(),
	} {
		for key := range definition.Settings {
			assert.False(t, strings.HasPrefix(key, "payroll."), "%s sets %s", name, key)
		}
	}
}

func TestSeedPayrollNumbersStep_RunsAfterTheLastStaffAndBeforeTheProfileCheck(t *testing.T) {
	t.Parallel()

	for _, options := range []SeedOptions{{}, {DeferHistory: true}, {StandingDemo: true, DeferHistory: true}} {
		steps := fullDemoWorkflow(&Seeder{options: options}).Steps
		index := func(match func(Step) bool) int {
			for i, step := range steps {
				if match(step) {
					return i
				}
			}
			return -1
		}
		payroll := index(func(s Step) bool { _, ok := s.(seedPayrollNumbersStep); return ok })
		require.NotEqual(t, -1, payroll, "%+v", options)
		assert.Less(t, index(func(s Step) bool { _, ok := s.(seedExternalCaregiversStep); return ok }), payroll)
		assert.Less(t, index(func(s Step) bool { _, ok := s.(seedInactiveAccountStep); return ok }), payroll)
		assert.Less(t, index(func(s Step) bool { _, ok := s.(seedStaffOnboardingStep); return ok }), payroll)
		assert.Less(t, payroll, index(func(s Step) bool { _, ok := s.(verifyProfileStep); return ok }))
	}
}
