package api

import (
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
)

// The lifecycle and the guardian invitation seams refuse to compose without
// their dependencies (ADR 0059): a missing one is a construction error, not
// a failure or a silent fallback of the first request.
func TestLifecycleDependenciesRequireEveryDependency(t *testing.T) {
	t.Parallel()

	_, err := lifecycleDependencies(LifecycleRepositories{}, nil, CaregiverProfiles{}, unsentGuardianMail{}, nil, slog.Default())
	require.EqualError(t, err, "identity access composition: every lifecycle repository and owner command, the audit command and the caregiver profiles are required")
}

func TestGuardianInvitationDependenciesRequireEveryDependency(t *testing.T) {
	t.Parallel()

	_, _, err := guardianInvitationDependencies(nil, nil, nil, nil, "http://localhost:3000", time.Hour, slog.Default())
	require.EqualError(t, err, "identity access composition: the guardian invitation seams need the school directory, the e-mail outbox and the enrollment claim")

	_, _, err = guardianInvitationDependencies(nil, struct{ organizationtenancy.Query }{}, struct{ MailResultOutbox }{}, struct{ GuardianEnrollmentClaims }{}, "http://localhost:3000", time.Hour, nil)
	require.EqualError(t, err, "identity access composition: the guardian invitation seams need a logger")
}
