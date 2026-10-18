package api

import (
	"log/slog"
	"testing"
	"time"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServiceGraphStudentConsentUsesAuditRoutedRepository(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	cfg := ServiceConfig{
		JWTSecret:        "test-secret-must-be-at-least-32-chars-long-for-real",
		JWTExpiry:        15 * time.Minute,
		JWTRefreshExpiry: 24 * time.Hour,
		FrontendURL:      "http://localhost:3000",
		PublicAPIURL:     "http://localhost:8080",
		ParentsURL:       "http://parents.localhost:3000",
		SchoolURL:        "http://schule.localhost:3000",
		TenantDomain:     "localhost",
		OperatorHostname: "operator.localhost:3000",
	}

	observedAppends := 0
	observe := testRootObservers()
	observe.AuditAppend = func(_ string, _ time.Duration, _ int, _ error) { observedAppends++ }
	_, _, _, _, _, _, services, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, err := composeTestRootWith(testpkg.NewDatabase(t, db), slog.Default(), cfg, observe)
	require.NoError(t, err)

	fixture := testpkg.CreateTestStudent(t, db, "Audit", "Routing", "1a")
	student, err := NewUnboundStudentRepository(testpkg.NewDatabase(t, db)).FindByID(testpkg.Ctx(t), fixture.ID)
	require.NoError(t, err)
	after := *student
	consentedAt := time.Date(2026, time.September, 2, 8, 30, 0, 0, time.UTC)
	after.PhotoConsentGivenAt = &consentedAt

	err = services.StudentConsents.RecordTransitions(
		testpkg.Ctx(t),
		student,
		&after,
		"tenant_portal",
		nil,
		consentedAt,
	)
	require.NoError(t, err)
	assert.Equal(t, 1, observedAppends)
}
