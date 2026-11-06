package api

import (
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	querybudgets "github.com/moto-nrw/project-phoenix/test/query-budgets"
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
	resource := newGuardiansResource(svc.PeopleDirectory, svc.NewGuardianDirectoryRuntime(testpkg.NewDatabase(t, db)), "", slog.Default())
	router := chi.NewRouter()
	router.Use(testpkg.TenantRuntimeMiddleware(t, db))
	router.Mount("/guardians", resource.Router())
	return guardianEmailRoute{db: db, router: router}
}

func (c guardianEmailRoute) do(t *testing.T, accountID int64, method, path string, perms ...string) *httptest.ResponseRecorder {
	t.Helper()
	req := testutil.NewAuthenticatedRequest(t, method, "/guardians"+path, nil, bearer(t, guardianStaffClaims(accountID, perms...)))
	return testutil.ExecuteRequest(c.router, req)
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
	blockPath := fmt.Sprintf("/%d/email-block", guardian.ID)
	testutil.AssertErrorResponse(t, c.do(t, account.ID, http.MethodGet, statusPath, permissions.StudentsRead), http.StatusForbidden)
	testutil.AssertErrorResponse(t, c.do(t, account.ID, http.MethodPost, blockPath, permissions.UsersRead), http.StatusForbidden)
	testutil.AssertErrorResponse(t, c.do(t, account.ID, http.MethodDelete, blockPath, permissions.UsersRead), http.StatusForbidden)

	assert.Equal(t, http.StatusOK, c.do(t, account.ID, http.MethodGet, statusPath, permissions.UsersRead).Code)
}

func TestGuardianEmailRoutesStayInsideTheSchool(t *testing.T) {
	t.Parallel()
	c := setupGuardianEmailRoute(t)
	_, account := testpkg.CreateTestStaffWithAccount(t, c.db, "Mail", "Schule")
	other := testpkg.NewTenantScope(t, c.db)
	otherStudent := testpkg.CreateTestStudentForTenant(t, c.db, other.TenantID, "Fremd", "Kind", "1a")
	var otherGuardian int64
	require.NoError(t, c.db.NewRaw(`INSERT INTO users.guardian_profiles (tenant_id, first_name, last_name, email)
		VALUES (?, 'Fremd', 'Eltern', 'fremd@example.com') RETURNING id`, other.TenantID).Scan(testpkg.Ctx(t), &otherGuardian))

	statusRR := c.do(t, account.ID, http.MethodGet, fmt.Sprintf("/students/%d/email-status", otherStudent.ID), permissions.UsersRead)
	assert.Equal(t, http.StatusNotFound, statusRR.Code, statusRR.Body.String())
	stopRR := c.do(t, account.ID, http.MethodPost, fmt.Sprintf("/%d/email-block", otherGuardian), permissions.UsersUpdate)
	assert.Equal(t, http.StatusNotFound, stopRR.Code, stopRR.Body.String())

	var marks int
	require.NoError(t, c.db.NewRaw(`SELECT count(*) FROM platform.email_suppressions WHERE address = 'fremd@example.com'`).Scan(testpkg.Ctx(t), &marks))
	assert.Zero(t, marks, "another school's address stays untouched")
}

// The whole path a school walks: stop, read, invite refused, lift, invite again.
func TestGuardianEmailStopRefusesTheInvitationUntilLifted(t *testing.T) {
	t.Parallel()
	c := setupGuardianEmailRoute(t)
	_, account := testpkg.CreateTestStaffWithAccount(t, c.db, "Mail", "Stopp")
	guardian := testpkg.CreateTestGuardianProfile(t, c.db, "mail-stop")
	student := testpkg.CreateTestStudent(t, c.db, "Mail", "Stoppkind", "1a")
	testpkg.CreateTestStudentGuardianLink(t, c.db, student.ID, guardian.ID, "primary_guardian")
	statusPath := fmt.Sprintf("/students/%d/email-status", student.ID)
	blockPath := fmt.Sprintf("/%d/email-block", guardian.ID)
	invitePath := fmt.Sprintf("/%d/invite", guardian.ID)

	require.Equal(t, http.StatusOK, c.do(t, account.ID, http.MethodPost, blockPath, permissions.UsersUpdate).Code)
	line := emailStatusLine(t, c.do(t, account.ID, http.MethodGet, statusPath, permissions.UsersRead), guardian.ID)
	assert.Equal(t, "school_stop", line["address_mark"])
	assert.Equal(t, "address", line["send_block"])

	refused := c.do(t, account.ID, http.MethodPost, invitePath, permissions.UsersCreate)
	testutil.AssertErrorResponse(t, refused, http.StatusConflict)
	assert.Contains(t, refused.Body.String(), common.CodeIdentityGuardianEmailAddressBlocked)

	require.Equal(t, http.StatusOK, c.do(t, account.ID, http.MethodDelete, blockPath, permissions.UsersUpdate).Code)
	line = emailStatusLine(t, c.do(t, account.ID, http.MethodGet, statusPath, permissions.UsersRead), guardian.ID)
	assert.Nil(t, line["address_mark"])

	invited := c.do(t, account.ID, http.MethodPost, invitePath, permissions.UsersCreate)
	assert.Less(t, invited.Code, 300, invited.Body.String())
}

func TestGuardianEmailComplaintIsNotLiftedByTheSchool(t *testing.T) {
	t.Parallel()
	c := setupGuardianEmailRoute(t)
	_, account := testpkg.CreateTestStaffWithAccount(t, c.db, "Mail", "Beschwerde")
	guardian := testpkg.CreateTestGuardianProfile(t, c.db, "mail-complaint")
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
	db, svc := setupGuardianModule(t)
	counter := testpkg.CaptureQueries(t, db)
	resource := newGuardiansResource(svc.PeopleDirectory, svc.NewGuardianDirectoryRuntime(testpkg.NewDatabase(t, db)), "", slog.Default())
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
		rr := testutil.ExecuteRequest(router, req)
		require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
		return len(counter.Queries())
	}
	add("budget-1")
	add("budget-2")
	first := read()
	querybudgets.AssertQueryBudget(t, "api.guardians.email_status", counter.Queries())
	add("budget-3")
	add("budget-4")
	assert.Equal(t, first, read(), "a further guardian adds no statement")
}
