package enrollmenthttp_test

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	enrollmentCapability "github.com/moto-nrw/project-phoenix/modules/enrollment"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// The opt-in "Neue Anmeldung" mail (#3780), end to end: real consent rows,
// real permissions and memberships, a real submission and the admin mails it
// enqueues.
func TestSubmit_AdminNotificationReachesSubscribedEnrollmentManagers(t *testing.T) {
	t.Parallel()

	env, cleanup := setupRequestTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)
	tenantID := testpkg.Tenant(t)

	subscribers, consent, err := testutil.NewEnrollmentAdminSubscribers(env.db)
	require.NoError(t, err)
	config := env.config
	config.AdminSubscribers = subscribers
	svc := newTestRequestService(config)

	const enrollmentMail = "enrollment_submitted"
	subscribe := func(t *testing.T, ctx context.Context, accountID int64, enabled bool) {
		t.Helper()
		require.NoError(t, consent.RecordConsent(ctx, accountID, enrollmentMail, enabled))
	}

	manager := testpkg.CreateTestAccount(t, env.db, "anmeldung-leitung")
	testpkg.GrantTestPermission(t, env.db, tenantID, manager.ID, "config:manage")
	subscribe(t, ctx, manager.ID, true)

	alsoListed := testpkg.CreateTestAccount(t, env.db, "anmeldung-liste")
	testpkg.GrantTestPermission(t, env.db, tenantID, alsoListed.ID, "config:manage")
	subscribe(t, ctx, alsoListed.ID, true)

	withoutRight := testpkg.CreateTestAccount(t, env.db, "anmeldung-ohne-recht")
	subscribe(t, ctx, withoutRight.ID, true)

	switchedOff := testpkg.CreateTestAccount(t, env.db, "anmeldung-aus")
	testpkg.GrantTestPermission(t, env.db, tenantID, switchedOff.ID, "config:manage")
	subscribe(t, ctx, switchedOff.ID, false)

	neverDecided := testpkg.CreateTestAccount(t, env.db, "anmeldung-offen")
	testpkg.GrantTestPermission(t, env.db, tenantID, neverDecided.ID, "config:manage")

	// A manager of another school who switched the mail on there.
	otherTenant, _ := testpkg.CreateTestTenant(t, env.db)
	foreign := testpkg.CreateTestAccount(t, env.db, "anmeldung-fremd")
	testpkg.EnsureAccountTenant(t, env.db, foreign.ID, otherTenant)
	testpkg.GrantTestPermission(t, env.db, otherTenant, foreign.ID, "config:manage")
	subscribe(t, testpkg.TenantContext(otherTenant), foreign.ID, true)

	env.settings.stringValues[configModel.KeyEnrollmentNotificationEmails] =
		"extern@example.test, " + strings.ToUpper(alsoListed.Email)

	_, err = svc.Submit(ctx, validSubmission(t, env.phaseID))
	require.NoError(t, err)

	assert.ElementsMatch(t,
		[]string{"extern@example.test", strings.ToUpper(alsoListed.Email), manager.Email},
		adminMailRecipients(env.outbox),
		"the address list stays; switched-on managers join it once; nobody without the right, the decision or the school")

	t.Run("a suppressed submission sends no opt-in mail either", func(t *testing.T) {
		before := len(adminMailRecipients(env.outbox))
		req := validSubmission(t, env.phaseID)
		req.GuardianEmail = "nachzuegler@example.com"
		req.SuppressSubmissionEmails = true

		_, err := svc.Submit(ctx, req)

		require.NoError(t, err)
		assert.Len(t, adminMailRecipients(env.outbox), before)
	})
}

func adminMailRecipients(outbox *recordingOutbox) []string {
	var recipients []string
	for _, entry := range outbox.ByKind("enrollment_admin_notification") {
		address, _ := entry.Payload[enrollmentCapability.EnrollmentPayloadRecipientEmail].(string)
		recipients = append(recipients, address)
	}
	return recipients
}
