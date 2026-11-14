package api

import (
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/moto-ogs/project-phoenix/api/common"
	"github.com/moto-ogs/project-phoenix/api/testutil"
	"github.com/moto-ogs/project-phoenix/auth/authorize/permissions"
	deliveryContract "github.com/moto-ogs/project-phoenix/modules/delivery/contract"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	querybudgets "github.com/moto-ogs/project-phoenix/test/query-budgets"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

type guardianEmailRoute struct {
	db     *bun.DB
	router chi.Router
}

func setupGuardianEmailRoute(t *testing.T) guardianEmailRoute {
	t.Helper()
	db, svc := setupGuardianModule(t)
	resource := NewGuardiansResource(svc.PeopleDirectory, svc.NewGuardianDirectoryRuntime(testpkg.NewDatabase(t, db)), "", slog.Default())
	router := chi.NewRouter()
	router.Use(testpkg.TenantRuntimeMiddleware(t, db))
	router.Mount("/guardians", resource.Router())
	return guardianEmailRoute{db: db, router: router}
}

func (c guardianEmailRoute) do(t *testing.T, accountID int64, method, path string, perms ...string) *httptest.ResponseRecorder {
	t.Helper()
	req := testutil.NewAuthenticatedRequest(t, method, "/guardians"+path, nil, bearer(t, guardianStaffClaims(accountID, perms...)))
	return testutil.ExecuteRequest(testpkg.MountedQueryBudgetRouter("/api", c.router), req)
}

func (c guardianEmailRoute) mark(t *testing.T, address, reason string) {
	t.Helper()
	_, err := c.db.ExecContext(testpkg.Ctx(t), `INSERT INTO platform.email_suppressions (tenant_id, address, reason) VALUES (?, ?, ?)`, testpkg.Tenant(t), address, reason)
	require.NoError(t, err)
}

func (c guardianEmailRoute) invitationMail(t *testing.T, address, state string) {
	t.Helper()
	_, err := c.db.ExecContext(testpkg.Ctx(t), `INSERT INTO platform.email_outbox
		(tenant_id, kind, recipient, payload, status, next_retry_at, sent_at, delivery_state, delivery_state_at)
		VALUES (?, 'guardian_invitation', jsonb_build_object('address', ?::text), '{}', 'sent', NOW(), NOW(), ?, NOW())`,
		testpkg.Tenant(t), address, state)
	require.NoError(t, err)
}

func emailStatusLine(t *testing.T, rr *httptest.ResponseRecorder, guardianID int64) map[string]any {
	t.Helper()
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	for _, raw := range dataArray(t, rr.Body.Bytes()) {
		line := raw.(map[string]any)
		if int64(line["guardian_profile_id"].(float64)) == guardianID {
			return line
		}
	}
	require.Failf(t, "no line", "guardian %d is missing: %s", guardianID, rr.Body.String())
	return nil
}

func TestGuardianEmailStatusNeedsItsPermissions(t *testing.T) {
	t.Parallel()
	c := setupGuardianEmailRoute(t)
	_, account := testpkg.CreateTestStaffWithAccount(t, c.db, "Mail", "Rechte")
	guardian := testpkg.CreateTestGuardianProfile(t, c.db, "mail-rights")
	student := testpkg.CreateTestStudent(t, c.db, "Mail", "Kind", "1a")
	testpkg.CreateTestStudentGuardianLink(t, c.db, student.ID, guardian.ID, "primary_guardian")

	statusPath := fmt.Sprintf("/students/%d/email-status", student.ID)
	testutil.AssertErrorResponse(t, c.do(t, account.ID, http.MethodGet, statusPath, permissions.StudentsRead), http.StatusForbidden)
	for _, path := range []string{fmt.Sprintf("/%d/email-stop", guardian.ID), fmt.Sprintf("/%d/email-block", guardian.ID)} {
		testutil.AssertErrorResponse(t, c.do(t, account.ID, http.MethodDelete, path, permissions.UsersRead), http.StatusForbidden)
	}
	testutil.AssertErrorResponse(t, c.do(t, account.ID, http.MethodPost, fmt.Sprintf("/%d/email-stop", guardian.ID), permissions.UsersRead), http.StatusForbidden)

	assert.Equal(t, http.StatusOK, c.do(t, account.ID, http.MethodGet, statusPath, permissions.UsersRead).Code)
}

// Finding 1 (#2329): users:update alone is not enough. An account without a staff
// record, and staff on a contact without a child, are refused like on every other
// change to a guardian.
func TestGuardianEmailChangesNeedARightToModifyTheGuardian(t *testing.T) {
	t.Parallel()
	c := setupGuardianEmailRoute(t)
	guardian := testpkg.CreateTestGuardianProfile(t, c.db, "mail-modify")
	student := testpkg.CreateTestStudent(t, c.db, "Mail", "Modify", "1a")
	testpkg.CreateTestStudentGuardianLink(t, c.db, student.ID, guardian.ID, "primary_guardian")
	withoutChild := testpkg.CreateTestGuardianProfile(t, c.db, "mail-no-child")
	_, staff := testpkg.CreateTestStaffWithAccount(t, c.db, "Mail", "Personal")
	noStaff := testpkg.CreateTestAccount(t, c.db, "mail-no-staff")

	for _, request := range []struct{ method, path string }{
		{http.MethodPost, "/%d/email-stop"}, {http.MethodDelete, "/%d/email-stop"}, {http.MethodDelete, "/%d/email-block"},
	} {
		rr := c.do(t, noStaff.ID, request.method, fmt.Sprintf(request.path, guardian.ID), permissions.UsersUpdate)
		testutil.AssertErrorResponse(t, rr, http.StatusForbidden)
		rr = c.do(t, staff.ID, request.method, fmt.Sprintf(request.path, withoutChild.ID), permissions.UsersUpdate)
		testutil.AssertErrorResponse(t, rr, http.StatusForbidden)
	}
	var stopped int
	require.NoError(t, c.db.NewRaw(`SELECT count(*) FROM users.guardian_profiles WHERE id IN (?, ?) AND emails_stopped_at IS NOT NULL`, guardian.ID, withoutChild.ID).Scan(testpkg.Ctx(t), &stopped))
	assert.Zero(t, stopped)
}

func TestGuardianEmailRoutesStayInsideTheSchool(t *testing.T) {
	t.Parallel()
	c := setupGuardianEmailRoute(t)
	_, account := testpkg.CreateTestStaffWithAccount(t, c.db, "Mail", "Schule")
	other := testpkg.NewTenantScope(t, c.db)
	otherStudent := testpkg.CreateTestStudentForTenant(t, c.db, other.TenantID, "Fremd", "Kind", "1a")
	otherGuardian := testpkg.CreateTestGuardianProfileForTenant(t, c.db, other.TenantID, "Fremd", "Eltern", "fremd-mail")

	statusRR := c.do(t, account.ID, http.MethodGet, fmt.Sprintf("/students/%d/email-status", otherStudent.ID), permissions.UsersRead)
	assert.Equal(t, http.StatusNotFound, statusRR.Code, statusRR.Body.String())
	for _, request := range []struct{ method, path string }{
		{http.MethodPost, "/%d/email-stop"}, {http.MethodDelete, "/%d/email-block"},
	} {
		rr := c.do(t, account.ID, request.method, fmt.Sprintf(request.path, otherGuardian.ID), permissions.UsersUpdate)
		assert.Contains(t, []int{http.StatusForbidden, http.StatusNotFound}, rr.Code, rr.Body.String())
	}
	var stopped int
	require.NoError(t, c.db.NewRaw(`SELECT count(*) FROM users.guardian_profiles WHERE id = ? AND emails_stopped_at IS NOT NULL`, otherGuardian.ID).Scan(testpkg.Ctx(t), &stopped))
	assert.Zero(t, stopped, "another school's guardian stays untouched")
}

// The whole path a school walks: stop, read, invite refused, new address, still
// stopped, take the stop back, invite. Both changes leave an audit row.
func TestGuardianEmailStopBelongsToThePerson(t *testing.T) {
	t.Parallel()
	c := setupGuardianEmailRoute(t)
	_, account := testpkg.CreateTestStaffWithAccount(t, c.db, "Mail", "Stopp")
	guardian := testpkg.CreateTestGuardianProfile(t, c.db, "mail-stop")
	student := testpkg.CreateTestStudent(t, c.db, "Mail", "Stoppkind", "1a")
	testpkg.CreateTestStudentGuardianLink(t, c.db, student.ID, guardian.ID, "primary_guardian")
	statusPath := fmt.Sprintf("/students/%d/email-status", student.ID)
	stopPath := fmt.Sprintf("/%d/email-stop", guardian.ID)
	invitePath := fmt.Sprintf("/%d/invite", guardian.ID)

	require.Equal(t, http.StatusOK, c.do(t, account.ID, http.MethodPost, stopPath, permissions.UsersUpdate).Code)
	line := emailStatusLine(t, c.do(t, account.ID, http.MethodGet, statusPath, permissions.UsersRead), guardian.ID)
	assert.Equal(t, true, line["emails_stopped"])
	assert.Equal(t, "stopped", line["send_block"])
	assert.Nil(t, line["address_mark"], "the stop is not on the address list")

	refused := c.do(t, account.ID, http.MethodPost, invitePath, permissions.UsersCreate)
	testutil.AssertErrorResponse(t, refused, http.StatusConflict)
	assert.Contains(t, refused.Body.String(), common.CodeIdentityGuardianEmailAddressBlocked)

	// A new address, the way the school or the parent changes it, ends nothing.
	_, err := c.db.ExecContext(testpkg.Ctx(t), `UPDATE users.guardian_profiles SET email = 'neu-' || email WHERE id = ?`, guardian.ID)
	require.NoError(t, err)
	line = emailStatusLine(t, c.do(t, account.ID, http.MethodGet, statusPath, permissions.UsersRead), guardian.ID)
	assert.Equal(t, true, line["emails_stopped"])
	refused = c.do(t, account.ID, http.MethodPost, invitePath, permissions.UsersCreate)
	testutil.AssertErrorResponse(t, refused, http.StatusConflict)

	require.Equal(t, http.StatusOK, c.do(t, account.ID, http.MethodDelete, stopPath, permissions.UsersUpdate).Code)
	line = emailStatusLine(t, c.do(t, account.ID, http.MethodGet, statusPath, permissions.UsersRead), guardian.ID)
	assert.Nil(t, line["emails_stopped"])
	invited := c.do(t, account.ID, http.MethodPost, invitePath, permissions.UsersCreate)
	assert.Less(t, invited.Code, 300, invited.Body.String())

	var audited int
	require.NoError(t, c.db.NewRaw(`SELECT count(*) FROM audit.data_access_log
		WHERE resource_type = 'guardian_email_stop' AND actor_account_id = ? AND metadata->>'guardian_profile_id' = ?`,
		account.ID, fmt.Sprint(guardian.ID)).Scan(testpkg.Ctx(t), &audited))
	assert.Equal(t, 2, audited)
}

// Saving a guardian's profile, as staff and the parent portal do, keeps the stop.
func TestSavingAGuardianKeepsTheStop(t *testing.T) {
	t.Parallel()
	c := setupGuardianEmailRoute(t)
	_, account := testpkg.CreateTestStaffWithAccount(t, c.db, "Mail", "Speichern")
	guardian := testpkg.CreateTestGuardianProfile(t, c.db, "mail-save")
	student := testpkg.CreateTestStudent(t, c.db, "Mail", "Speicherkind", "1a")
	testpkg.CreateTestStudentGuardianLink(t, c.db, student.ID, guardian.ID, "primary_guardian")
	require.Equal(t, http.StatusOK, c.do(t, account.ID, http.MethodPost, fmt.Sprintf("/%d/email-stop", guardian.ID), permissions.UsersUpdate).Code)

	req := testutil.NewAuthenticatedRequest(t, http.MethodPut, fmt.Sprintf("/guardians/%d", guardian.ID),
		map[string]any{"first_name": "Neu", "email": "neu-" + *guardian.Email}, bearer(t, guardianStaffClaims(account.ID, permissions.UsersUpdate)))
	rr := testutil.ExecuteRequest(testpkg.MountedQueryBudgetRouter("/api", c.router), req)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())

	var stopped bool
	require.NoError(t, c.db.NewRaw(`SELECT emails_stopped_at IS NOT NULL FROM users.guardian_profiles WHERE id = ?`, guardian.ID).Scan(testpkg.Ctx(t), &stopped))
	assert.True(t, stopped)
}

// Lifting only clears what the school caused, and the audit row names no address.
func TestGuardianEmailLiftIsAuditedWithoutTheAddress(t *testing.T) {
	t.Parallel()
	c := setupGuardianEmailRoute(t)
	_, account := testpkg.CreateTestStaffWithAccount(t, c.db, "Mail", "Sperre")
	guardian := testpkg.CreateTestGuardianProfile(t, c.db, "mail-lift")
	student := testpkg.CreateTestStudent(t, c.db, "Mail", "Sperrkind", "1a")
	testpkg.CreateTestStudentGuardianLink(t, c.db, student.ID, guardian.ID, "primary_guardian")
	c.mark(t, *guardian.Email, "failed")

	require.Equal(t, http.StatusOK, c.do(t, account.ID, http.MethodDelete, fmt.Sprintf("/%d/email-block", guardian.ID), permissions.UsersUpdate).Code)
	var metadata string
	require.NoError(t, c.db.NewRaw(`SELECT metadata::text FROM audit.data_access_log
		WHERE resource_type = 'guardian_email_lift' AND actor_account_id = ?`, account.ID).Scan(testpkg.Ctx(t), &metadata))
	assert.Contains(t, metadata, fmt.Sprint(guardian.ID))
	assert.NotContains(t, metadata, *guardian.Email)

	// Nothing on the list: the address is free as far as moto knows.
	require.Equal(t, http.StatusOK, c.do(t, account.ID, http.MethodDelete, fmt.Sprintf("/%d/email-block", guardian.ID), permissions.UsersUpdate).Code)
}

func TestGuardianEmailComplaintIsNotLiftedByTheSchool(t *testing.T) {
	t.Parallel()
	c := setupGuardianEmailRoute(t)
	_, account := testpkg.CreateTestStaffWithAccount(t, c.db, "Mail", "Beschwerde")
	guardian := testpkg.CreateTestGuardianProfile(t, c.db, "mail-complaint")
	student := testpkg.CreateTestStudent(t, c.db, "Mail", "Beschwerdekind", "1a")
	testpkg.CreateTestStudentGuardianLink(t, c.db, student.ID, guardian.ID, "primary_guardian")
	c.mark(t, *guardian.Email, "complained")

	rr := c.do(t, account.ID, http.MethodDelete, fmt.Sprintf("/%d/email-block", guardian.ID), permissions.UsersUpdate)
	testutil.AssertErrorResponse(t, rr, http.StatusConflict)
	assert.Contains(t, rr.Body.String(), common.CodeStudentsGuardianEmailBlockNeedsReview)
}

// A delayed mail is still on its way, so a second invitation is refused and the
// status says why. Never does the status claim more than the events say.
func TestGuardianEmailDelayedBlocksTheNextInvitation(t *testing.T) {
	t.Parallel()
	c := setupGuardianEmailRoute(t)
	_, account := testpkg.CreateTestStaffWithAccount(t, c.db, "Mail", "Warten")
	guardian := testpkg.CreateTestGuardianProfile(t, c.db, "mail-delayed")
	student := testpkg.CreateTestStudent(t, c.db, "Mail", "Wartekind", "1a")
	testpkg.CreateTestStudentGuardianLink(t, c.db, student.ID, guardian.ID, "primary_guardian")
	c.invitationMail(t, *guardian.Email, "delayed")

	line := emailStatusLine(t, c.do(t, account.ID, http.MethodGet, fmt.Sprintf("/students/%d/email-status", student.ID), permissions.UsersRead), guardian.ID)
	assert.Equal(t, "delayed", line["mail_outcome"])
	assert.Equal(t, "delayed", line["send_block"])

	refused := c.do(t, account.ID, http.MethodPost, fmt.Sprintf("/%d/invite", guardian.ID), permissions.UsersCreate)
	testutil.AssertErrorResponse(t, refused, http.StatusConflict)
	assert.Contains(t, refused.Body.String(), common.CodeIdentityInvitationMailDelayed)
}

func TestGuardianEmailStatusQueryBudget(t *testing.T) {
	t.Parallel()
	db, svc := setupGuardianModule(t)
	counter := testpkg.CaptureQueriesForContext(t, db)
	resource := NewGuardiansResource(svc.PeopleDirectory, svc.NewGuardianDirectoryRuntime(testpkg.NewDatabase(t, db)), "", slog.Default())
	router := chi.NewRouter()
	router.Use(testpkg.TenantRuntimeMiddleware(t, db))
	router.Mount("/guardians", resource.Router())
	c := guardianEmailRoute{db: db, router: router}
	_, account := testpkg.CreateTestStaffWithAccount(t, db, "Mail", "Budget")
	student := testpkg.CreateTestStudent(t, db, "Mail", "Budgetkind", "1a")
	add := func(seed string) {
		guardian := testpkg.CreateTestGuardianProfile(t, db, seed)
		testpkg.CreateTestStudentGuardianLink(t, db, student.ID, guardian.ID, "custom")
		c.invitationMail(t, *guardian.Email, "delivered")
	}
	read := func() int {
		counter.Reset()
		req := testutil.NewAuthenticatedRequest(t, http.MethodGet, fmt.Sprintf("/guardians/students/%d/email-status", student.ID), nil, bearer(t, guardianStaffClaims(account.ID, permissions.UsersRead)))
		req = req.WithContext(counter.Context(req.Context()))
		rr := testutil.ExecuteRequest(testpkg.MountedQueryBudgetRouter("/api", router), req)
		require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
		return len(counter.Queries())
	}
	add("budget-1")
	add("budget-2")
	first := read()
	querybudgets.AssertQueryBudget(t, "api.guardians.email_status", counter.Measurement(counter.Queries()))
	add("budget-3")
	add("budget-4")
	assert.Equal(t, first, read(), "a further guardian adds no statement")
}

// The API tells the toast that no mail reaches the person (#5934). This runs the
// whole route: a result that loses the value on its way still answers 200.
func TestInvitingAnExistingAccountSaysThatNoMailGoesOut(t *testing.T) {
	t.Parallel()
	c := setupGuardianEmailRoute(t)
	_, staff := testpkg.CreateTestStaffWithAccount(t, c.db, "Mail", "Konto")
	guardian := testpkg.CreateTestGuardianProfile(t, c.db, "mail-held-account")
	student := testpkg.CreateTestStudent(t, c.db, "Mail", "Kontokind", "1a")
	testpkg.CreateTestStudentGuardianLink(t, c.db, student.ID, guardian.ID, "primary_guardian")
	parent := testpkg.CreateTestAccount(t, c.db, "mail-held-login")
	_, err := c.db.ExecContext(testpkg.Ctx(t), `UPDATE users.guardian_profiles SET account_id = ?, has_account = TRUE, email = ? WHERE id = ?`, parent.ID, parent.Email, guardian.ID)
	require.NoError(t, err)
	c.mark(t, strings.ToLower(parent.Email), "failed")

	req := testutil.NewAuthenticatedRequest(t, http.MethodPost, fmt.Sprintf("/guardians/students/%d/invite", student.ID),
		map[string]any{"email": parent.Email}, bearer(t, guardianStaffClaims(staff.ID, permissions.UsersCreate)))
	rec := testutil.ExecuteRequest(testpkg.MountedQueryBudgetRouter("/api", c.router), req)

	require.Less(t, rec.Code, 300, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"mail_held":true`)
}

// Another school's block reaches this school as its own mail held back. As long as
// the provider holds it, no new invitation is offered, and nothing names the cause.
// Once it is gone, the old mail decides nothing.
func TestGuardianStatusFollowsTheCurrentProviderBlock(t *testing.T) {
	t.Parallel()
	c := setupGuardianEmailRoute(t)
	_, account := testpkg.CreateTestStaffWithAccount(t, c.db, "Mail", "Anbieter")
	guardian := testpkg.CreateTestGuardianProfile(t, c.db, "mail-provider")
	student := testpkg.CreateTestStudent(t, c.db, "Mail", "Anbieterkind", "1a")
	testpkg.CreateTestStudentGuardianLink(t, c.db, student.ID, guardian.ID, "primary_guardian")
	address := strings.ToLower(*guardian.Email)
	c.invitationMail(t, address, "suppressed")
	statusPath := fmt.Sprintf("/students/%d/email-status", student.ID)
	read := func() map[string]any {
		return emailStatusLine(t, c.do(t, account.ID, http.MethodGet, statusPath, permissions.UsersRead), guardian.ID)
	}

	testProviderBlocks.Store(address, deliveryContract.EmailProviderBlockActive)
	line := read()
	assert.Equal(t, "provider_blocked", line["mail_outcome"])
	assert.Equal(t, "provider", line["send_block"])
	assert.Equal(t, "active", line["provider_block"])
	assert.Nil(t, line["address_mark"], "it stays off the school's list")

	testProviderBlocks.Store(address, deliveryContract.EmailProviderBlockUnreadable)
	line = read()
	assert.Equal(t, "provider_unknown", line["send_block"], "an unknown state offers nothing")

	testProviderBlocks.Store(address, deliveryContract.EmailProviderBlockNone)
	line = read()
	assert.Equal(t, "provider_blocked", line["mail_outcome"], "the old mail keeps its result")
	assert.Nil(t, line["send_block"], "the block is gone: another try goes")
}

// The provider is only asked about an address the school's own mail ran into, so
// no school can probe another's addresses.
func TestGuardianStatusAsksTheProviderOnlyAfterItsOwnBlockedMail(t *testing.T) {
	t.Parallel()
	c := setupGuardianEmailRoute(t)
	_, account := testpkg.CreateTestStaffWithAccount(t, c.db, "Mail", "Frage")
	guardian := testpkg.CreateTestGuardianProfile(t, c.db, "mail-not-asked")
	student := testpkg.CreateTestStudent(t, c.db, "Mail", "Fragekind", "1a")
	testpkg.CreateTestStudentGuardianLink(t, c.db, student.ID, guardian.ID, "primary_guardian")
	address := strings.ToLower(*guardian.Email)
	c.invitationMail(t, address, "delivered")

	line := emailStatusLine(t, c.do(t, account.ID, http.MethodGet, fmt.Sprintf("/students/%d/email-status", student.ID), permissions.UsersRead), guardian.ID)

	assert.Equal(t, "delivered", line["mail_outcome"])
	_, asked := testProviderAsked.Load(address)
	assert.False(t, asked)
}

// A stop on a shared address must not read as if no mail went there any more.
func TestGuardianStatusSaysThatASharedAddressStillGetsMail(t *testing.T) {
	t.Parallel()
	c := setupGuardianEmailRoute(t)
	_, account := testpkg.CreateTestStaffWithAccount(t, c.db, "Mail", "Geteilt")
	stopped := testpkg.CreateTestGuardianProfile(t, c.db, "mail-shared-stopped")
	free := testpkg.CreateTestGuardianProfile(t, c.db, "mail-shared-free")
	student := testpkg.CreateTestStudent(t, c.db, "Mail", "Geteiltkind", "1a")
	testpkg.CreateTestStudentGuardianLink(t, c.db, student.ID, stopped.ID, "primary_guardian")
	login := testpkg.CreateTestAccount(t, c.db, "mail-shared-login")
	_, err := c.db.ExecContext(testpkg.Ctx(t), `UPDATE users.guardian_profiles SET account_id = ?, has_account = TRUE WHERE id = ?`, login.ID, free.ID)
	require.NoError(t, err)
	_, err = c.db.ExecContext(testpkg.Ctx(t), `UPDATE users.guardian_profiles SET email = ? WHERE id = ?`, login.Email, stopped.ID)
	require.NoError(t, err)
	statusPath := fmt.Sprintf("/students/%d/email-status", student.ID)
	require.Equal(t, http.StatusOK, c.do(t, account.ID, http.MethodPost, fmt.Sprintf("/%d/email-stop", stopped.ID), permissions.UsersUpdate).Code)

	line := emailStatusLine(t, c.do(t, account.ID, http.MethodGet, statusPath, permissions.UsersRead), stopped.ID)

	assert.Equal(t, true, line["emails_stopped"])
	assert.Equal(t, true, line["stop_shared"])
}
